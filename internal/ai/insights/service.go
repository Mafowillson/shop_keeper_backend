package insights

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"shop_keeper_backend/internal/ai/anomaly"
	"shop_keeper_backend/internal/customer"
	notification "shop_keeper_backend/internal/notifications"
	"shop_keeper_backend/internal/product"
	"shop_keeper_backend/internal/sale"
	"shop_keeper_backend/internal/shop"
	"shop_keeper_backend/internal/user"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	groqAPIURL   = "https://api.groq.com/openai/v1/chat/completions"
	groqModel    = "llama-3.1-8b-instant"
	insightTokens = 512
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
	repo         *Repo
	shopRepo     *shop.Repo
	saleRepo     *sale.Repo
	productRepo  *product.Repo
	customerRepo *customer.Repo
	anomalyRepo  *anomaly.Repo
	userRepo     *user.Repo
	notifSvc     *notification.Service
	groqKey      string
	httpClient   *http.Client
}

func NewService(
	repo *Repo,
	shopRepo *shop.Repo,
	saleRepo *sale.Repo,
	productRepo *product.Repo,
	customerRepo *customer.Repo,
	anomalyRepo *anomaly.Repo,
	userRepo *user.Repo,
	notifSvc *notification.Service,
	groqKey string,
) *Service {
	return &Service{
		repo:         repo,
		shopRepo:     shopRepo,
		saleRepo:     saleRepo,
		productRepo:  productRepo,
		customerRepo: customerRepo,
		anomalyRepo:  anomalyRepo,
		userRepo:     userRepo,
		notifSvc:     notifSvc,
		groqKey:      groqKey,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

// StartWeeklyJob launches a goroutine that calls Run each Sunday at 23:00 UTC.
func (s *Service) StartWeeklyJob(ctx context.Context) {
	go func() {
		for {
			next := nextSundayAt23UTC()
			select {
			case <-time.After(time.Until(next)):
				s.Run(context.Background())
			case <-ctx.Done():
				return
			}
		}
	}()
}

func nextSundayAt23UTC() time.Time {
	now := time.Now().UTC()
	weekday := int(now.Weekday()) // Sunday=0, Monday=1, …, Saturday=6

	var daysUntil int
	if weekday == 0 {
		target := time.Date(now.Year(), now.Month(), now.Day(), 23, 0, 0, 0, time.UTC)
		if now.Before(target) {
			return target
		}
		daysUntil = 7
	} else {
		daysUntil = (7 - weekday) % 7
		if daysUntil == 0 {
			daysUntil = 7
		}
	}
	next := now.AddDate(0, 0, daysUntil)
	return time.Date(next.Year(), next.Month(), next.Day(), 23, 0, 0, 0, time.UTC)
}

// currentWeekStart returns 00:00 UTC of the most recent Monday.
func currentWeekStart() time.Time {
	now := time.Now().UTC()
	weekday := int(now.Weekday())
	daysFromMonday := (weekday + 6) % 7 // Monday=0 … Sunday=6
	day := now.AddDate(0, 0, -daysFromMonday)
	return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
}

// Run generates weekly insights for every shop.
func (s *Service) Run(ctx context.Context) {
	log.Println("insights: starting weekly run")

	shops, err := s.shopRepo.ListAll(ctx)
	if err != nil {
		log.Printf("insights: list shops: %v", err)
		return
	}

	weekStart := currentWeekStart()
	weekEnd := weekStart.AddDate(0, 0, 7)
	prevWeekStart := weekStart.AddDate(0, 0, -7)

	for _, sh := range shops {
		if err := s.processShop(ctx, sh, weekStart, weekEnd, prevWeekStart); err != nil {
			log.Printf("insights: shop %s: %v", sh.ID, err)
		}
	}

	log.Println("insights: weekly run complete")
}

func (s *Service) processShop(
	ctx context.Context,
	sh shop.Shop,
	weekStart, weekEnd, prevWeekStart time.Time,
) error {
	// Gather metrics.
	revenue, txCount, err := s.saleRepo.PeriodStatsByShop(ctx, sh.ID, weekStart, weekEnd)
	if err != nil {
		return fmt.Errorf("week revenue: %w", err)
	}

	prevRevenue, _, err := s.saleRepo.PeriodStatsByShop(ctx, sh.ID, prevWeekStart, weekStart)
	if err != nil {
		return fmt.Errorf("prev week revenue: %w", err)
	}

	topStats, err := s.saleRepo.TopProductsByRevenue(ctx, sh.ID, weekStart, 5)
	if err != nil {
		return fmt.Errorf("top products: %w", err)
	}

	newCustomers, err := s.customerRepo.CountNewByShopSince(ctx, sh.ID, weekStart)
	if err != nil {
		return fmt.Errorf("new customers: %w", err)
	}

	totalDebt, err := s.customerRepo.TotalDebtByShop(ctx, sh.ID)
	if err != nil {
		return fmt.Errorf("total debt: %w", err)
	}

	openAlerts, err := s.anomalyRepo.CountOpenByShop(ctx, sh.ID)
	if err != nil {
		return fmt.Errorf("open alerts: %w", err)
	}

	stockoutWarnings, err := s.productRepo.CountStockoutWarningsByShop(ctx, sh.ID)
	if err != nil {
		return fmt.Errorf("stockout warnings: %w", err)
	}

	// Resolve product names for top products.
	topLines := s.buildTopProductLines(ctx, topStats)

	// Determine owner locale.
	locale := "fr"
	if u, err := s.userRepo.FindByID(ctx, sh.OwnerID); err == nil && u.PreferredLocale != "" {
		locale = u.PreferredLocale
	}

	// Build LLM prompt.
	systemPrompt := s.buildSystemPrompt(locale, sh.Name, weekStart, weekEnd,
		revenue, prevRevenue, txCount, topLines, newCustomers, totalDebt, openAlerts, stockoutWarnings)

	content, err := s.callGroq(ctx, systemPrompt)
	if err != nil {
		return fmt.Errorf("groq: %w", err)
	}

	now := time.Now().UTC()
	if err := s.repo.Upsert(ctx, WeeklyInsight{
		ShopID:      sh.ID,
		WeekStart:   weekStart,
		Content:     content,
		GeneratedAt: now,
	}); err != nil {
		return fmt.Errorf("upsert insight: %w", err)
	}

	ownerOID, err := bson.ObjectIDFromHex(sh.OwnerID)
	if err == nil {
		s.notifSvc.NotifyWeeklyInsights(ctx, ownerOID, sh.ID, weekStart)
	}

	return nil
}

func (s *Service) buildTopProductLines(ctx context.Context, stats []sale.ProductRevenueStat) []string {
	if len(stats) == 0 {
		return nil
	}
	ids := make([]string, len(stats))
	for i, st := range stats {
		ids[i] = st.ProductID
	}
	products, err := s.productRepo.FindByIDs(ctx, ids)
	nameByID := make(map[string]string, len(products))
	if err == nil {
		for _, p := range products {
			nameByID[p.ID] = p.Name
		}
	}
	lines := make([]string, len(stats))
	for i, st := range stats {
		name := nameByID[st.ProductID]
		if name == "" {
			name = st.ProductID
		}
		lines[i] = fmt.Sprintf("%s (%.0f FCFA, %d units)", name, st.Revenue, st.UnitsSold)
	}
	return lines
}

func (s *Service) buildSystemPrompt(
	locale, shopName string,
	weekStart, weekEnd time.Time,
	revenue, prevRevenue float64,
	txCount int64,
	topProducts []string,
	newCustomers int64,
	totalDebt float64,
	openAlerts, stockoutWarnings int64,
) string {
	var revChange string
	if prevRevenue > 0 {
		pct := ((revenue - prevRevenue) / prevRevenue) * 100
		if pct >= 0 {
			revChange = fmt.Sprintf("+%.1f%%", pct)
		} else {
			revChange = fmt.Sprintf("%.1f%%", pct)
		}
	} else {
		revChange = "N/A (no prior week data)"
	}

	topList := "None"
	if len(topProducts) > 0 {
		topList = strings.Join(topProducts, "; ")
	}

	dateFormat := "2 Jan 2006"
	weekLabel := fmt.Sprintf("%s – %s", weekStart.Format(dateFormat), weekEnd.AddDate(0, 0, -1).Format(dateFormat))

	var sb strings.Builder

	if locale == "en" {
		fmt.Fprintf(&sb, "You are a business analytics assistant for a small retail shop in Cameroon.\n")
		fmt.Fprintf(&sb, "Write a concise weekly insight report in English. Be direct, practical, and actionable.\n")
		fmt.Fprintf(&sb, "Limit your response to 200 words. Do not use headers or bullet points — write in short paragraphs.\n\n")
		fmt.Fprintf(&sb, "SHOP: %s\n", shopName)
		fmt.Fprintf(&sb, "WEEK: %s\n\n", weekLabel)
		fmt.Fprintf(&sb, "PERFORMANCE DATA:\n")
		fmt.Fprintf(&sb, "- Revenue: %.0f FCFA (%s vs last week)\n", revenue, revChange)
		fmt.Fprintf(&sb, "- Transactions: %d\n", txCount)
		fmt.Fprintf(&sb, "- Top products: %s\n", topList)
		fmt.Fprintf(&sb, "- New customers: %d\n", newCustomers)
		fmt.Fprintf(&sb, "- Total outstanding debt: %.0f FCFA\n", totalDebt)
		fmt.Fprintf(&sb, "- Open fraud alerts: %d\n", openAlerts)
		fmt.Fprintf(&sb, "- Products at stockout risk (≤7 days): %d\n", stockoutWarnings)
	} else {
		fmt.Fprintf(&sb, "Tu es un assistant d'analyse commerciale pour une boutique de détail au Cameroun.\n")
		fmt.Fprintf(&sb, "Écris un rapport d'analyse hebdomadaire concis en français. Sois direct, pratique et orienté action.\n")
		fmt.Fprintf(&sb, "Limite ta réponse à 200 mots. N'utilise pas d'en-têtes ni de listes — écris en courts paragraphes.\n\n")
		fmt.Fprintf(&sb, "BOUTIQUE : %s\n", shopName)
		fmt.Fprintf(&sb, "SEMAINE : %s\n\n", weekLabel)
		fmt.Fprintf(&sb, "DONNÉES DE PERFORMANCE :\n")
		fmt.Fprintf(&sb, "- Chiffre d'affaires : %.0f FCFA (%s par rapport à la semaine précédente)\n", revenue, revChange)
		fmt.Fprintf(&sb, "- Transactions : %d\n", txCount)
		fmt.Fprintf(&sb, "- Meilleurs produits : %s\n", topList)
		fmt.Fprintf(&sb, "- Nouveaux clients : %d\n", newCustomers)
		fmt.Fprintf(&sb, "- Dette totale en cours : %.0f FCFA\n", totalDebt)
		fmt.Fprintf(&sb, "- Alertes de fraude ouvertes : %d\n", openAlerts)
		fmt.Fprintf(&sb, "- Produits à risque de rupture (≤7 jours) : %d\n", stockoutWarnings)
	}

	return sb.String()
}

func (s *Service) callGroq(ctx context.Context, systemPrompt string) (string, error) {
	messages := []groqMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: "Generate the weekly insight report now."},
	}

	body, err := json.Marshal(groqRequest{
		Model:       groqModel,
		Messages:    messages,
		Temperature: 0.3,
		MaxTokens:   insightTokens,
	})
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, groqAPIURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.groqKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var groqResp groqResponse
	if err := json.Unmarshal(respBytes, &groqResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("status %d", resp.StatusCode)
		if groqResp.Error != nil {
			msg = groqResp.Error.Message
		}
		return "", fmt.Errorf("%s", msg)
	}

	if len(groqResp.Choices) == 0 {
		return "", fmt.Errorf("empty response")
	}

	return groqResp.Choices[0].Message.Content, nil
}

// ListByShop returns the last [limit] weekly insights for a shop, for use by the handler.
func (s *Service) ListByShop(ctx context.Context, shopID, ownerID string, limit int) ([]WeeklyInsight, error) {
	if err := s.verifyOwner(ctx, shopID, ownerID); err != nil {
		return nil, err
	}
	return s.repo.ListByShop(ctx, shopID, limit)
}

func (s *Service) verifyOwner(ctx context.Context, shopID, ownerID string) error {
	sh, err := s.shopRepo.FindByID(ctx, shopID)
	if err != nil {
		return fmt.Errorf("shop not found")
	}
	if sh.OwnerID != ownerID {
		return fmt.Errorf("unauthorized")
	}
	return nil
}
