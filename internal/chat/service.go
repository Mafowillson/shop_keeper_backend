package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"shop_keeper_backend/internal/product"
	"shop_keeper_backend/internal/sale"
	"shop_keeper_backend/internal/user"
)

const (
	maxHistory   = 40
	groqAPIURL   = "https://api.groq.com/openai/v1/chat/completions"
	groqModel    = "llama-3.1-8b-instant"
)

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqRequest struct {
	Model       string        `json:"model"`
	Messages    []groqMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type groqResponse struct {
	Choices []struct {
		Message groqMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type Service struct {
	repo        *Repo
	userRepo    *user.Repo
	productRepo *product.Repo
	saleRepo    *sale.Repo
	groqKey     string
	httpClient  *http.Client
}

func NewService(
	repo *Repo,
	userRepo *user.Repo,
	productRepo *product.Repo,
	saleRepo *sale.Repo,
	groqKey string,
) *Service {
	return &Service{
		repo:        repo,
		userRepo:    userRepo,
		productRepo: productRepo,
		saleRepo:    saleRepo,
		groqKey:     groqKey,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *Service) Send(ctx context.Context, ownerID, message string) (Message, error) {
	u, err := s.userRepo.FindByID(ctx, ownerID)
	if err != nil {
		return Message{}, fmt.Errorf("chat: find owner: %w", err)
	}

	systemPrompt := s.buildSystemPrompt(ctx, u.ShopID)

	history, err := s.repo.History(ctx, ownerID, maxHistory)
	if err != nil {
		return Message{}, err
	}

	if _, err := s.repo.Save(ctx, ownerID, "user", message); err != nil {
		return Message{}, err
	}

	reply, err := s.callGroq(ctx, systemPrompt, history, message)
	if err != nil {
		return Message{}, err
	}

	saved, err := s.repo.Save(ctx, ownerID, "model", reply)
	if err != nil {
		return Message{}, err
	}

	return saved, nil
}

func (s *Service) GetHistory(ctx context.Context, ownerID string) ([]Message, error) {
	return s.repo.History(ctx, ownerID, maxHistory)
}

func (s *Service) Clear(ctx context.Context, ownerID string) error {
	return s.repo.Clear(ctx, ownerID)
}

func (s *Service) buildSystemPrompt(ctx context.Context, shopID string) string {
	var sb strings.Builder
	sb.WriteString("You are ShopKeeper AI, a business assistant for a retail shop in Cameroon.\n")
	sb.WriteString("Help the owner with inventory analysis, sales insights, restocking advice, and business decisions.\n")
	sb.WriteString("Currency is FCFA (Central African CFA franc).\n\n")

	if shopID == "" {
		sb.WriteString("Be concise and practical. Respond in the same language the owner uses (English or French).\n")
		return sb.String()
	}

	now := time.Now().UTC()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	if revenue, txCount, err := s.saleRepo.TodayStatsByShop(ctx, shopID, todayStart); err == nil {
		sb.WriteString(fmt.Sprintf("Today's revenue: FCFA %.0f (%d transactions)\n", revenue, txCount))
	}

	if lowStock, err := s.productRepo.ListLowStock(ctx, shopID, 10); err == nil && len(lowStock) > 0 {
		sb.WriteString(fmt.Sprintf("\nLow-stock items (%d products need restocking):\n", len(lowStock)))
		for _, p := range lowStock {
			sb.WriteString(fmt.Sprintf("  - %s: %d units remaining (threshold: %d)\n", p.Name, p.StockQty, p.LowStockThreshold))
		}
	}

	sb.WriteString("\nBe concise and practical. Cite actual numbers when recommending restocking.\n")
	sb.WriteString("Respond in the same language the owner uses (English or French).\n")

	return sb.String()
}

func (s *Service) callGroq(ctx context.Context, systemPrompt string, history []Message, message string) (string, error) {
	messages := make([]groqMessage, 0, len(history)+2)

	messages = append(messages, groqMessage{Role: "system", Content: systemPrompt})

	for _, msg := range history {
		role := msg.Role
		if role == "model" {
			role = "assistant"
		}
		messages = append(messages, groqMessage{Role: role, Content: msg.Text})
	}

	messages = append(messages, groqMessage{Role: "user", Content: message})

	body, err := json.Marshal(groqRequest{
		Model:       groqModel,
		Messages:    messages,
		Temperature: 0.7,
		MaxTokens:   1024,
	})
	if err != nil {
		return "", fmt.Errorf("groq: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, groqAPIURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("groq: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.groqKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("groq: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("groq: read response: %w", err)
	}

	var groqResp groqResponse
	if err := json.Unmarshal(respBytes, &groqResp); err != nil {
		return "", fmt.Errorf("groq: decode response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("status %d", resp.StatusCode)
		if groqResp.Error != nil {
			msg = groqResp.Error.Message
		}
		return "", fmt.Errorf("groq: %s", msg)
	}

	if len(groqResp.Choices) == 0 {
		return "", fmt.Errorf("groq: empty response")
	}

	return groqResp.Choices[0].Message.Content, nil
}
