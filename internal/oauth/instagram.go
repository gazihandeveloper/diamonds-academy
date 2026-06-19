package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/facebook"
)

// InstagramProvider wraps the Facebook OAuth2 config since Instagram login
// uses the Facebook Login API under the Meta platform.
// Note: Facebook OAuth returns Instagram-linked accounts when the user has
// connected their Instagram account in Facebook's settings.
type InstagramProvider struct {
	Config *oauth2.Config
}

// InstagramUserInfo represents the minimal user data from Facebook/Instagram.
type InstagramUserInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// NewInstagramProvider creates a Facebook OAuth config for Instagram login.
func NewInstagramProvider(clientID, clientSecret, redirectURL string) *InstagramProvider {
	return &InstagramProvider{
		Config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"email", "public_profile"},
			Endpoint:     facebook.Endpoint,
		},
	}
}

// ExchangeInstagramCode exchanges the code for a token and fetches the user profile.
func (p *InstagramProvider) ExchangeInstagramCode(ctx context.Context, code string) (*InstagramUserInfo, error) {
	tok, err := p.Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("facebook exchange: %w", err)
	}
	client := p.Config.Client(ctx, tok)
	resp, err := client.Get("https://graph.facebook.com/me?fields=id,name,email")
	if err != nil {
		return nil, fmt.Errorf("graph api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("graph api returned status %d", resp.StatusCode)
	}

	var u InstagramUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("graph decode: %w", err)
	}
	if u.ID == "" {
		return nil, fmt.Errorf("facebook did not return a user id")
	}
	return &u, nil
}
