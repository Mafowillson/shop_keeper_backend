package pricing

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"shop_keeper_backend/internal/i18n"
	"shop_keeper_backend/internal/product"
	"shop_keeper_backend/internal/sale"
	"shop_keeper_backend/internal/shop"
	"shop_keeper_backend/internal/user"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const sellThroughWindowDays = 30

type Service struct {
	repo        *Repo
	productRepo *product.Repo
	saleRepo    *sale.Repo
	shopRepo    *shop.Repo
	userRepo    *user.Repo
}

func NewService(
	repo *Repo,
	productRepo *product.Repo,
	saleRepo *sale.Repo,
	shopRepo *shop.Repo,
	userRepo *user.Repo,
) *Service {
	return &Service{
		repo:        repo,
		productRepo: productRepo,
		saleRepo:    saleRepo,
		shopRepo:    shopRepo,
		userRepo:    userRepo,
	}
}

// StartWeeklyJob launches a goroutine that calls Run each Monday at 01:00 UTC.
func (s *Service) StartWeeklyJob(ctx context.Context) {
	go func() {
		for {
			next := nextMondayAt01UTC()
			select {
			case <-time.After(time.Until(next)):
				s.Run(context.Background())
			case <-ctx.Done():
				return
			}
		}
	}()
}

func nextMondayAt01UTC() time.Time {
	now := time.Now().UTC()
	weekday := int(now.Weekday()) // Sunday=0, Monday=1, …, Saturday=6

	var daysUntil int
	if weekday == 1 {
		// Today is Monday — run at 01:00 if we haven't passed it yet, else next Monday.
		target := time.Date(now.Year(), now.Month(), now.Day(), 1, 0, 0, 0, time.UTC)
		if now.Before(target) {
			return target
		}
		daysUntil = 7
	} else {
		// (8 - weekday) % 7 gives days until next Monday for all other days.
		daysUntil = (8 - weekday) % 7
		if daysUntil == 0 {
			daysUntil = 7
		}
	}
	next := now.AddDate(0, 0, daysUntil)
	return time.Date(next.Year(), next.Month(), next.Day(), 1, 0, 0, 0, time.UTC)
}

// Run executes one full pricing cycle across all active products in every shop.
func (s *Service) Run(ctx context.Context) {
	log.Println("pricing: starting weekly run")

	allProducts, err := s.productRepo.ListAllActive(ctx)
	if err != nil {
		log.Printf("pricing: list products: %v", err)
		return
	}
	if len(allProducts) == 0 {
		return
	}

	byShop := make(map[string][]product.Product)
	for _, p := range allProducts {
		byShop[p.ShopID] = append(byShop[p.ShopID], p)
	}

	since := time.Now().UTC().AddDate(0, 0, -sellThroughWindowDays)

	for shopID, shopProducts := range byShop {
		unitsSold, err := s.saleRepo.UnitsSoldByProductSince(ctx, shopID, since)
		if err != nil {
			log.Printf("pricing: units sold for shop %s: %v", shopID, err)
			continue
		}

		sh, err := s.shopRepo.FindByID(ctx, shopID)
		if err != nil {
			log.Printf("pricing: find shop %s: %v", shopID, err)
			continue
		}

		// Determine owner's preferred locale for reason string generation.
		locale := "fr"
		u, err := s.userRepo.FindByID(ctx, sh.OwnerID)
		if err == nil {
			locale = u.PreferredLocale
		}
		msgs := i18n.Get(locale)

		for _, p := range shopProducts {
			s.processProduct(ctx, p, unitsSold[p.ID], shopID, msgs)
		}
	}

	log.Println("pricing: weekly run complete")
}

func (s *Service) processProduct(
	ctx context.Context,
	p product.Product,
	unitsSold30d int,
	shopID string,
	msgs i18n.Messages,
) {
	// avg_stock ≈ (opening + closing) / 2
	// opening ≈ current_stock + units_sold_30d (what we had before selling)
	avgStock := float64(p.StockQty) + float64(unitsSold30d)/2
	if avgStock <= 0 {
		return // no stock and no sales — nothing to recommend
	}

	rate := float64(unitsSold30d) / avgStock

	// Map rate → action. Products in the 0.3–1.0 band need no price change.
	var action string
	var changePct float64
	var reason string

	switch {
	case rate > 1.5:
		action = ActionIncrease5
		changePct = 5.0
		reason = fmt.Sprintf(msgs.PriceRecHighDemandReason, unitsSold30d, rate)
	case rate > 1.0:
		action = ActionIncrease3
		changePct = 3.0
		reason = fmt.Sprintf(msgs.PriceRecGoodDemandReason, unitsSold30d, rate)
	case rate >= 0.3:
		return // no recommendation needed
	case rate >= 0.1:
		action = ActionDecrease5
		changePct = -5.0
		reason = fmt.Sprintf(msgs.PriceRecSlowMovementReason, unitsSold30d, rate)
	default:
		action = ActionDecrease10
		changePct = -10.0
		reason = fmt.Sprintf(msgs.PriceRecVerySlowReason, unitsSold30d, rate)
	}

	multiplier := 1 + changePct/100
	currentPrices := make(map[string]float64, len(p.Units))
	suggestedPrices := make(map[string]float64, len(p.Units))
	for _, u := range p.Units {
		currentPrices[u.Name] = u.Price
		suggestedPrices[u.Name] = math.Round(u.Price * multiplier)
	}

	now := time.Now().UTC()
	rec := PriceRecommendation{
		ShopID:          shopID,
		ProductID:       p.ID,
		ProductName:     p.Name,
		SellThroughRate: rate,
		UnitsSold30d:    unitsSold30d,
		AvgStock:        avgStock,
		Action:          action,
		ChangePercent:   changePct,
		Reason:          reason,
		CurrentPrices:   currentPrices,
		SuggestedPrices: suggestedPrices,
		Status:          StatusPending,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.repo.Upsert(ctx, rec); err != nil {
		log.Printf("pricing: upsert rec for product %s: %v", p.ID, err)
	}
}

// Accept applies the suggested price change to the product and marks the recommendation accepted.
func (s *Service) Accept(ctx context.Context, recID, ownerID string) error {
	rec, err := s.repo.FindByID(ctx, recID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("recommendation not found")
		}
		return err
	}
	if err := s.verifyOwner(ctx, rec.ShopID, ownerID); err != nil {
		return err
	}
	if rec.Status != StatusPending {
		return errors.New("recommendation already acted upon")
	}

	p, err := s.productRepo.FindByID(ctx, rec.ProductID)
	if err != nil {
		return fmt.Errorf("fetch product: %w", err)
	}

	multiplier := 1 + rec.ChangePercent/100
	updatedUnits := make([]product.UnitDefinition, len(p.Units))
	copy(updatedUnits, p.Units)
	for i, u := range updatedUnits {
		updatedUnits[i].Price = math.Round(u.Price * multiplier)
	}

	if _, err := s.productRepo.Update(ctx, rec.ProductID, bson.M{
		"units":      updatedUnits,
		"updated_at": time.Now().UTC(),
	}); err != nil {
		return fmt.Errorf("apply price change: %w", err)
	}

	return s.repo.UpdateStatus(ctx, recID, StatusAccepted, time.Now().UTC())
}

// Dismiss marks the recommendation dismissed without changing the product's prices.
func (s *Service) Dismiss(ctx context.Context, recID, ownerID string) error {
	rec, err := s.repo.FindByID(ctx, recID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("recommendation not found")
		}
		return err
	}
	if err := s.verifyOwner(ctx, rec.ShopID, ownerID); err != nil {
		return err
	}
	if rec.Status != StatusPending {
		return errors.New("recommendation already acted upon")
	}
	return s.repo.UpdateStatus(ctx, recID, StatusDismissed, time.Now().UTC())
}

// List returns paginated price recommendations for the given shop, optionally filtered by status.
func (s *Service) List(ctx context.Context, ownerID, shopID, status string, page, pageSize int) ([]PriceRecommendation, int64, error) {
	if err := s.verifyOwner(ctx, shopID, ownerID); err != nil {
		return nil, 0, err
	}
	return s.repo.ListByShop(ctx, shopID, status, page, pageSize)
}

func (s *Service) verifyOwner(ctx context.Context, shopID, ownerID string) error {
	sh, err := s.shopRepo.FindByID(ctx, shopID)
	if err != nil {
		return errors.New("shop not found")
	}
	if sh.OwnerID != ownerID {
		return errors.New("unauthorized")
	}
	return nil
}
