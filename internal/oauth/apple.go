package oauth

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// AppleUserInfo represents the user data returned by Apple after token exchange.
// Apple returns the user info only on the FIRST authentication for a given app.
// On subsequent logins, only the `sub` (user ID) is stable — name comes from the
// form_post body `user` field.
type AppleUserInfo struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// ParseAppleUserName extracts the full name from Apple's `user` JSON object.
// Apple sends this in the form_post body ONLY on first authorization.
// Format: {"name":{"firstName":"Ender","lastName":"Altıntaş"},"email":"..."}
func ParseAppleUserName(userJSON string) string {
	var payload struct {
		Name struct {
			FirstName string `json:"firstName"`
			LastName  string `json:"lastName"`
		} `json:"name"`
	}
	if err := json.Unmarshal([]byte(userJSON), &payload); err != nil {
		return ""
	}
	fn := strings.TrimSpace(payload.Name.FirstName)
	ln := strings.TrimSpace(payload.Name.LastName)
	if fn == "" && ln == "" {
		return ""
	}
	if fn == "" {
		return ln
	}
	if ln == "" {
		return fn
	}
	return fn + " " + ln
}

// AppleProvider holds the configuration needed for Sign In with Apple.
// The PrivateKey is the raw content of the .p8 file downloaded from Apple Developer.
type AppleProvider struct {
	TeamID     string
	ServiceID  string
	KeyID      string
	PrivateKey string // PEM-encoded PKCS#8 private key content
	RedirectURI string
}

// NewAppleProvider creates a new Apple Sign In provider.
func NewAppleProvider(teamID, serviceID, keyID, privateKeyStr, redirectURI string) *AppleProvider {
	return &AppleProvider{
		TeamID:      teamID,
		ServiceID:   serviceID,
		KeyID:       keyID,
		PrivateKey:  privateKeyStr,
		RedirectURI: redirectURI,
	}
}

const appleAuthURL = "https://appleid.apple.com/auth/authorize"
const appleTokenURL = "https://appleid.apple.com/auth/token"

// AuthCodeURL builds the authorization URL for Apple Sign In.
func (p *AppleProvider) AuthCodeURL(state string) string {
	vals := url.Values{}
	vals.Set("client_id", p.ServiceID)
	vals.Set("redirect_uri", p.RedirectURI)
	vals.Set("response_type", "code id_token")
	vals.Set("scope", "name email")
	vals.Set("state", state)
	vals.Set("response_mode", "form_post")
	return appleAuthURL + "?" + vals.Encode()
}

// ExchangeAppleCode exchanges an authorization code for tokens and returns user info.
// On the FIRST sign-in, Apple also sends a `user` JSON object with name/email in the
// form_post response. On subsequent sign-ins, only the ID token is available.
func (p *AppleProvider) ExchangeAppleCode(code string) (*AppleUserInfo, error) {
	clientSecret, err := p.generateClientSecret()
	if err != nil {
		return nil, fmt.Errorf("apple client secret: %w", err)
	}

	form := url.Values{}
	form.Set("client_id", p.ServiceID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", p.RedirectURI)

	req, err := http.NewRequest(http.MethodPost, appleTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apple token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		IDToken      string `json:"id_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("token decode: %w", err)
	}

	if tokenResp.IDToken == "" {
		return nil, fmt.Errorf("apple did not return an id_token")
	}

	return p.parseIDToken(tokenResp.IDToken)
}

// parseIDToken decodes the ID token without verifying signature (the token comes directly
// from Apple's token endpoint over HTTPS, so it's trusted). Returns the user's sub and email.
func (p *AppleProvider) parseIDToken(idToken string) (*AppleUserInfo, error) {
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	token, _, err := parser.ParseUnverified(idToken, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("id_token parse: %w", err)
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid id_token claims")
	}

	sub, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)
	if sub == "" {
		return nil, fmt.Errorf("id_token missing sub")
	}
	return &AppleUserInfo{Sub: sub, Email: email}, nil
}

// generateClientSecret creates a JWT signed with the private key.
func (p *AppleProvider) generateClientSecret() (string, error) {
	block, _ := pem.Decode([]byte(p.PrivateKey))
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM private key")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}
	ecKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("private key is not ECDSA")
	}

	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    p.TeamID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
		Audience:  jwt.ClaimStrings{"https://appleid.apple.com"},
		Subject:   p.ServiceID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = p.KeyID

	return token.SignedString(ecKey)
}
