package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type wellbiRequest struct {
	Message string `json:"message"`
}

type wellbiResponse struct {
	Reply string `json:"reply"`
}

type deepseekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type deepseekRequest struct {
	Model       string            `json:"model"`
	Messages    []deepseekMessage `json:"messages"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Temperature float64           `json:"temperature,omitempty"`
}

type deepseekChoice struct {
	Message deepseekMessage `json:"message"`
}

type deepseekResponse struct {
	Choices []deepseekChoice `json:"choices"`
	Usage   deepseekUsage    `json:"usage"`
}

type deepseekUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (h *Handler) WellbiChat(w http.ResponseWriter, r *http.Request) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		writeJSON(w, http.StatusInternalServerError, wellbiResponse{Reply: "Yapay zeka servisi şu anda yapılandırılmamış. Lütfen yöneticinize danışın."})
		return
	}

	var req wellbiRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, wellbiResponse{Reply: "Geçersiz istek formatı."})
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		writeJSON(w, http.StatusBadRequest, wellbiResponse{Reply: "Lütfen bir mesaj girin."})
		return
	}

	agentsBytes, err := os.ReadFile("AGENTS.md")
	systemPrompt := "Sen Wellbi'sin. Aşağıdaki kurallara harfiyen uy:\n\n"
	if err == nil {
		systemPrompt += string(agentsBytes)
	} else {
		systemPrompt += "Kullanıcılara yardımcı olan dostane bir AI asistanısın."
	}

	payload := deepseekRequest{
		Model: "deepseek-chat",
		Messages: []deepseekMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: req.Message},
		},
		MaxTokens:   512,
		Temperature: 0.7,
	}

	body, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 30 * time.Second}
	dsReq, _ := http.NewRequest("POST", "https://api.deepseek.com/v1/chat/completions", bytes.NewReader(body))
	dsReq.Header.Set("Content-Type", "application/json")
	dsReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(dsReq)
	if err != nil {
		log.Printf("wellbi deepseek error: %v", err)
		writeJSON(w, http.StatusInternalServerError, wellbiResponse{Reply: "Üzgünüm, bir bağlantı hatası oluştu. Lütfen daha sonra tekrar deneyin."})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		log.Printf("wellbi deepseek status %d: %s", resp.StatusCode, string(respBody))
		writeJSON(w, http.StatusInternalServerError, wellbiResponse{Reply: "Üzgünüm, bir hata oluştu. Lütfen daha sonra tekrar deneyin."})
		return
	}

	var dsResp deepseekResponse
	if err := json.Unmarshal(respBody, &dsResp); err != nil {
		log.Printf("wellbi deepseek unmarshal error: %v", err)
		writeJSON(w, http.StatusInternalServerError, wellbiResponse{Reply: "Üzgünüm, bir hata oluştu."})
		return
	}

	if len(dsResp.Choices) == 0 {
		writeJSON(w, http.StatusInternalServerError, wellbiResponse{Reply: "Üzgünüm, bir yanıt oluşturulamadı."})
		return
	}

	reply := strings.TrimSpace(dsResp.Choices[0].Message.Content)

	// Kullanım kaydet
	if h.DB != nil {
		uid := int64(0)
		if h.SM != nil {
			uid = h.SM.GetInt64(r.Context(), "userID")
		}
		_, _ = h.DB.ExecContext(r.Context(),
			`INSERT INTO deepseek_usage (user_id, prompt_tokens, completion_tokens, total_tokens) VALUES (?, ?, ?, ?)`,
			uid, dsResp.Usage.PromptTokens, dsResp.Usage.CompletionTokens, dsResp.Usage.TotalTokens)
	}

	writeJSON(w, http.StatusOK, wellbiResponse{Reply: reply})
}
