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

	"shop_keeper_backend/internal/customer"
	"shop_keeper_backend/internal/product"
	"shop_keeper_backend/internal/sale"
	"shop_keeper_backend/internal/user"
)

const (
	// maxLLMHistory is the number of historical messages passed to the LLM per request.
	// Corresponds to the 4 most recent exchange pairs (spec AI-6).
	maxLLMHistory     = 8
	maxDisplayHistory = 100

	groqAPIURL = "https://api.groq.com/openai/v1/chat/completions"
	groqModel  = "llama-3.1-8b-instant"
)

// chatIntent represents the classified intent of a user message.
type chatIntent int

const (
	intentSales chatIntent = iota
	intentStock
	intentCustomerDebt
	intentProduct
	intentGeneral
)

// intentTable maps each intent to its keyword triggers (English + French).
var intentTable = []struct {
	intent   chatIntent
	keywords []string
}{
	{intentSales, []string{
		"sale", "sales", "revenue", "income", "earning", "transaction", "sold", "selling",
		"profit", "turnover", "receipt",
		"vente", "ventes", "revenu", "revenus", "chiffre", "vendu", "bénéfice", "recette",
	}},
	{intentStock, []string{
		"stock", "inventory", "restock", "quantity", "units", "out of", "low stock", "stockout",
		"rupture", "inventaire", "quantité", "réapprovisionner", "commander", "manque", "épuisé",
	}},
	{intentCustomerDebt, []string{
		"customer", "client", "clients", "debt", "credit", "owe", "payment", "balance",
		"outstanding", "borrow",
		"dette", "dettes", "crédit", "doit", "paiement", "solde", "emprunt", "débiteur",
	}},
	{intentProduct, []string{
		"product", "products", "item", "items", "price", "prices", "category", "catalogue",
		"catalog",
		"produit", "produits", "article", "articles", "prix", "catégorie",
	}},
}

// shopWords matches general shop-related topics that don't map to a specific intent.
// Used to distinguish "shop question with vague phrasing" from fully off-topic messages.
var shopWords = []string{
	"shop", "store", "business", "boutique", "magasin", "commerce",
	"staff", "employee", "personnel", "worker",
	"money", "argent", "fcfa", "franc",
	"open", "close", "hours", "heure", "horaire", "schedule",
	"loss", "weekly", "daily", "monthly", "rapport", "report", "summary", "résumé",
	"today", "yesterday", "week", "month", "year", "journalier", "hebdomadaire",
	"performance", "analyse", "analysis",
}

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
	userRepo     *user.Repo
	productRepo  *product.Repo
	saleRepo     *sale.Repo
	customerRepo *customer.Repo
	groqKey      string
	httpClient   *http.Client
}

func NewService(
	repo *Repo,
	userRepo *user.Repo,
	productRepo *product.Repo,
	saleRepo *sale.Repo,
	customerRepo *customer.Repo,
	groqKey string,
) *Service {
	return &Service{
		repo:         repo,
		userRepo:     userRepo,
		productRepo:  productRepo,
		saleRepo:     saleRepo,
		customerRepo: customerRepo,
		groqKey:      groqKey,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

// Send classifies the message, builds a tailored context, calls Groq, and persists both turns.
func (s *Service) Send(ctx context.Context, ownerID, message string) (Message, error) {
	u, err := s.userRepo.FindByID(ctx, ownerID)
	if err != nil {
		return Message{}, fmt.Errorf("chat: find owner: %w", err)
	}

	intent := classifyIntent(message)

	// Off-topic guardrail: general intent + no shop-related words → canned response, skip LLM.
	if intent == intentGeneral && !isShopRelated(message) {
		if _, err := s.repo.Save(ctx, ownerID, "user", message); err != nil {
			return Message{}, err
		}
		reply := cannedOffTopicResponse(message)
		saved, err := s.repo.Save(ctx, ownerID, "model", reply)
		if err != nil {
			return Message{}, err
		}
		return saved, nil
	}

	// Fetch the last 4 exchange pairs before this message (history is fetched before saving
	// the current message so it doesn't appear twice in the Groq request).
	history, err := s.repo.History(ctx, ownerID, maxLLMHistory)
	if err != nil {
		return Message{}, err
	}

	if _, err := s.repo.Save(ctx, ownerID, "user", message); err != nil {
		return Message{}, err
	}

	systemPrompt := s.buildContextForIntent(ctx, u.ShopID, intent)

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
	return s.repo.History(ctx, ownerID, maxDisplayHistory)
}

func (s *Service) Clear(ctx context.Context, ownerID string) error {
	return s.repo.Clear(ctx, ownerID)
}

// classifyIntent scores the message against each intent's keyword list and returns
// the highest-scoring category, or intentGeneral when no keywords match.
func classifyIntent(msg string) chatIntent {
	lower := strings.ToLower(msg)
	scores := make(map[chatIntent]int, 4)
	for _, group := range intentTable {
		for _, kw := range group.keywords {
			if strings.Contains(lower, kw) {
				scores[group.intent]++
			}
		}
	}
	best := intentGeneral
	bestScore := 0
	for intent, score := range scores {
		if score > bestScore {
			bestScore = score
			best = intent
		}
	}
	return best
}

// isShopRelated returns true when the message contains at least one shop-related keyword,
// used to distinguish a vague shop question from a fully off-topic message.
func isShopRelated(msg string) bool {
	lower := strings.ToLower(msg)
	for _, group := range intentTable {
		for _, kw := range group.keywords {
			if strings.Contains(lower, kw) {
				return true
			}
		}
	}
	for _, w := range shopWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

// detectFrench returns true if the message appears to be in French.
func detectFrench(msg string) bool {
	markers := []string{
		"bonjour", "comment", "quel", "quelle", " les ", " des ", " est ", " que ",
		" pas ", " sur ", " dans ", " je ", " tu ", " il ", " vous ", "qu'", "n'", "c'est",
	}
	lower := strings.ToLower(msg)
	for _, m := range markers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

// cannedOffTopicResponse returns a fixed reply in the appropriate language.
func cannedOffTopicResponse(msg string) string {
	if detectFrench(msg) {
		return "Je suis ShopKeeper AI, spécialisé dans la gestion de votre boutique. Posez-moi des questions sur vos ventes, vos stocks, vos produits ou vos clients."
	}
	return "I am ShopKeeper AI, specialised in managing your shop. Please ask me about your sales, stock, products, or customers."
}

// buildContextForIntent assembles a system prompt containing only the data collections
// relevant to the classified intent, minimising the data transmitted to the LLM.
func (s *Service) buildContextForIntent(ctx context.Context, shopID string, i chatIntent) string {
	var sb strings.Builder
	sb.WriteString("You are ShopKeeper AI, a business assistant for a retail shop in Cameroon. Currency is FCFA.\n")
	sb.WriteString("Answer only questions about this shop's business using the data provided below.\n")
	sb.WriteString("Never generate or invent numbers not present in this context.\n\n")

	if shopID != "" {
		switch i {
		case intentSales:
			s.appendSalesContext(ctx, shopID, &sb)
		case intentStock:
			s.appendStockContext(ctx, shopID, &sb)
		case intentCustomerDebt:
			s.appendCustomerContext(ctx, shopID, &sb)
		case intentProduct:
			s.appendProductContext(ctx, shopID, &sb)
		default:
			s.appendBasicContext(ctx, shopID, &sb)
		}
	}

	sb.WriteString("\nBe concise and practical. Cite actual numbers. Respond in the same language the owner uses (English or French).\n")
	return sb.String()
}

func (s *Service) appendSalesContext(ctx context.Context, shopID string, sb *strings.Builder) {
	now := time.Now().UTC()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	if rev, count, err := s.saleRepo.TodayStatsByShop(ctx, shopID, todayStart); err == nil {
		fmt.Fprintf(sb, "Today's sales: FCFA %.0f across %d transactions.\n", rev, count)
	}

	// Week starts on the most recent Monday.
	daysSinceMonday := (int(now.Weekday()) + 6) % 7
	weekStart := todayStart.AddDate(0, 0, -daysSinceMonday)
	if rev, count, err := s.saleRepo.PeriodStatsByShop(ctx, shopID, weekStart, now); err == nil {
		fmt.Fprintf(sb, "This week's sales: FCFA %.0f across %d transactions.\n", rev, count)
	}

	since30 := now.AddDate(0, 0, -30)
	if stats, err := s.saleRepo.TopProductsByRevenue(ctx, shopID, since30, 3); err == nil && len(stats) > 0 {
		ids := make([]string, len(stats))
		for i, st := range stats {
			ids[i] = st.ProductID
		}
		nameMap := map[string]string{}
		if products, err := s.productRepo.FindByIDs(ctx, ids); err == nil {
			for _, p := range products {
				nameMap[p.ID] = p.Name
			}
		}
		sb.WriteString("Top 3 products by revenue (last 30 days):\n")
		for _, st := range stats {
			name := nameMap[st.ProductID]
			if name == "" {
				name = st.ProductID
			}
			fmt.Fprintf(sb, "  - %s: FCFA %.0f, %d base units sold.\n", name, st.Revenue, st.UnitsSold)
		}
	}
}

func (s *Service) appendStockContext(ctx context.Context, shopID string, sb *strings.Builder) {
	products, total, err := s.productRepo.List(ctx, shopID, "", "", 1, 50)
	if err != nil {
		return
	}
	fmt.Fprintf(sb, "Total active products: %d.\n", total)

	outOfStock := 0
	var lowStock []product.Product
	for _, p := range products {
		if p.StockQty == 0 {
			outOfStock++
		} else if p.StockQty <= p.LowStockThreshold {
			lowStock = append(lowStock, p)
		}
	}

	if outOfStock > 0 {
		fmt.Fprintf(sb, "Products with zero stock: %d.\n", outOfStock)
	}
	if len(lowStock) > 0 {
		fmt.Fprintf(sb, "Low-stock products (%d):\n", len(lowStock))
		for _, p := range lowStock {
			fmt.Fprintf(sb, "  - %s: %d %s remaining (threshold: %d).\n",
				p.Name, p.StockQty, p.BaseUnit, p.LowStockThreshold)
		}
	}
	if outOfStock == 0 && len(lowStock) == 0 {
		sb.WriteString("All products are adequately stocked.\n")
	}
}

func (s *Service) appendCustomerContext(ctx context.Context, shopID string, sb *strings.Builder) {
	if totalDebt, err := s.customerRepo.TotalDebtByShop(ctx, shopID); err == nil {
		fmt.Fprintf(sb, "Total outstanding customer debt: FCFA %.0f.\n", totalDebt)
	}
	if count, err := s.customerRepo.CountWithDebt(ctx, shopID); err == nil {
		fmt.Fprintf(sb, "Customers with active debt: %d.\n", count)
	}
	if topDebtors, err := s.customerRepo.TopDebtorsByShop(ctx, shopID, 5); err == nil && len(topDebtors) > 0 {
		sb.WriteString("Top 5 debtors:\n")
		for _, c := range topDebtors {
			fmt.Fprintf(sb, "  - %s: FCFA %.0f owed.\n", c.Name, c.TotalDebt)
		}
	}
}

func (s *Service) appendProductContext(ctx context.Context, shopID string, sb *strings.Builder) {
	products, total, err := s.productRepo.List(ctx, shopID, "", "", 1, 20)
	if err != nil {
		return
	}
	fmt.Fprintf(sb, "Product catalogue (%d total, showing first %d):\n", total, len(products))
	for _, p := range products {
		basePrice := 0.0
		for _, u := range p.Units {
			if u.QuantityInBase == 1 {
				basePrice = u.Price
				break
			}
		}
		fmt.Fprintf(sb, "  - %s [%s]: FCFA %.0f/%s, stock=%d.\n",
			p.Name, p.Category, basePrice, p.BaseUnit, p.StockQty)
	}
}

func (s *Service) appendBasicContext(ctx context.Context, shopID string, sb *strings.Builder) {
	now := time.Now().UTC()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if rev, count, err := s.saleRepo.TodayStatsByShop(ctx, shopID, todayStart); err == nil {
		fmt.Fprintf(sb, "Today's sales: FCFA %.0f across %d transactions.\n", rev, count)
	}
	if lowStock, err := s.productRepo.ListLowStock(ctx, shopID, 10); err == nil && len(lowStock) > 0 {
		fmt.Fprintf(sb, "%d products are low on stock.\n", len(lowStock))
	}
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
		Temperature: 0.3,
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
