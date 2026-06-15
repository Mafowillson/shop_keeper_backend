package forecasting

import (
	"context"
	"log"
	"math"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	notification "shop_keeper_backend/internal/notifications"
	"shop_keeper_backend/internal/product"
	"shop_keeper_backend/internal/sale"
	"shop_keeper_backend/internal/shop"
)

const (
	velocityWindowDays = 7
	alertThresholdDays = 7
	reorderSupplyDays  = 14
)

type Service struct {
	productRepo *product.Repo
	saleRepo    *sale.Repo
	shopRepo    *shop.Repo
	notifSvc    *notification.Service
}

func NewService(
	productRepo *product.Repo,
	saleRepo *sale.Repo,
	shopRepo *shop.Repo,
	notifSvc *notification.Service,
) *Service {
	return &Service{
		productRepo: productRepo,
		saleRepo:    saleRepo,
		shopRepo:    shopRepo,
		notifSvc:    notifSvc,
	}
}

// StartNightlyJob launches a goroutine that calls Run once at midnight UTC each day.
// The goroutine exits when ctx is cancelled.
func (s *Service) StartNightlyJob(ctx context.Context) {
	go func() {
		for {
			next := nextMidnightUTC()
			select {
			case <-time.After(time.Until(next)):
				s.Run(context.Background())
			case <-ctx.Done():
				return
			}
		}
	}()
}

func nextMidnightUTC() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
}

// Run executes one full forecasting cycle across all active products in every shop.
func (s *Service) Run(ctx context.Context) {
	log.Println("forecasting: starting nightly run")

	allProducts, err := s.productRepo.ListAllActive(ctx)
	if err != nil {
		log.Printf("forecasting: list products: %v", err)
		return
	}
	if len(allProducts) == 0 {
		return
	}

	// Group by shop so we make one sales aggregation call per shop, not per product.
	byShop := make(map[string][]product.Product)
	for _, p := range allProducts {
		byShop[p.ShopID] = append(byShop[p.ShopID], p)
	}

	since := time.Now().UTC().Add(-velocityWindowDays * 24 * time.Hour)

	for shopID, shopProducts := range byShop {
		unitsSold, err := s.saleRepo.UnitsSoldByProductSince(ctx, shopID, since)
		if err != nil {
			log.Printf("forecasting: units sold for shop %s: %v", shopID, err)
			continue
		}

		sh, err := s.shopRepo.FindByID(ctx, shopID)
		if err != nil {
			log.Printf("forecasting: find shop %s: %v", shopID, err)
			continue
		}
		ownerOID, err := bson.ObjectIDFromHex(sh.OwnerID)
		if err != nil {
			log.Printf("forecasting: invalid owner ID for shop %s: %v", shopID, err)
			continue
		}

		for _, p := range shopProducts {
			s.processProduct(ctx, p, unitsSold[p.ID], shopID, ownerOID)
		}
	}

	log.Println("forecasting: nightly run complete")
}

func (s *Service) processProduct(
	ctx context.Context,
	p product.Product,
	unitsSoldInWindow int,
	shopID string,
	ownerOID bson.ObjectID,
) {
	// Velocity = total base units sold in the 7-day window / 7.
	velocity := float64(unitsSoldInWindow) / float64(velocityWindowDays)

	// No evidence of active demand — skip. The spec requires v > 0 to flag a product.
	if velocity == 0 {
		return
	}

	daysUntilStockout := float64(p.StockQty) / velocity
	reorderQty := int(math.Ceil(velocity * reorderSupplyDays))

	var alertSentAt *time.Time
	clearAlert := false
	sendAlert := false

	if daysUntilStockout <= float64(alertThresholdDays) {
		if p.StockoutAlertSentAt == nil {
			// First time crossing the threshold this episode — send the alert.
			now := time.Now().UTC()
			alertSentAt = &now
			sendAlert = true
		}
		// Already alerted for this episode: no action on the flag.
	} else {
		// Product is outside the alert zone.
		if p.StockoutAlertSentAt != nil {
			// Stock has recovered past the threshold — reset so a future crossing
			// triggers a fresh alert.
			clearAlert = true
		}
	}

	if err := s.productRepo.UpdateForecastFields(ctx, p.ID, daysUntilStockout, reorderQty, alertSentAt, clearAlert); err != nil {
		log.Printf("forecasting: update product %s: %v", p.ID, err)
		return
	}

	if sendAlert {
		s.notifSvc.NotifyStockoutForecast(ctx, ownerOID, shopID, p.ID, p.Name, daysUntilStockout, reorderQty)
	}
}
