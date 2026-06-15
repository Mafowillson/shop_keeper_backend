package anomaly

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"shop_keeper_backend/internal/i18n"
	notification "shop_keeper_backend/internal/notifications"
	"shop_keeper_backend/internal/sale"
	"shop_keeper_backend/internal/shop"
	"shop_keeper_backend/internal/user"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	largeSaleMinSamples    = 5
	largeSaleHistoryLimit  = 30
	largeSaleSigmaFactor   = 2.5
	rapidCreditThreshold   = 3
	rapidCreditWindowHours = 24
	// cameroon is UTC+1 (WAT — no DST)
	cameroonOffsetSecs = 3600
	offHoursStart      = 22 // 22:00 local
	offHoursEnd        = 6  // 06:00 local
)

type Service struct {
	repo      *Repo
	saleRepo  *sale.Repo
	shopRepo  *shop.Repo
	userRepo  *user.Repo
	notifSvc  *notification.Service
}

func NewService(
	repo *Repo,
	saleRepo *sale.Repo,
	shopRepo *shop.Repo,
	userRepo *user.Repo,
	notifSvc *notification.Service,
) *Service {
	return &Service{
		repo:     repo,
		saleRepo: saleRepo,
		shopRepo: shopRepo,
		userRepo: userRepo,
		notifSvc: notifSvc,
	}
}

// CheckSale runs all three anomaly rules against the given sale.
// Intended to be called in a goroutine from sale.Service.Create.
// Implements sale.AnomalyDetector.
func (s *Service) CheckSale(ctx context.Context, sl sale.Sale) {
	sh, err := s.shopRepo.FindByID(ctx, sl.ShopID)
	if err != nil {
		log.Printf("anomaly: find shop %s: %v", sl.ShopID, err)
		return
	}
	ownerOID, err := bson.ObjectIDFromHex(sh.OwnerID)
	if err != nil {
		log.Printf("anomaly: invalid owner ID for shop %s: %v", sl.ShopID, err)
		return
	}

	locale := "fr"
	if u, err := s.userRepo.FindByID(ctx, sh.OwnerID); err == nil && u.PreferredLocale != "" {
		locale = u.PreferredLocale
	}
	msgs := i18n.Get(locale)

	s.checkLargeSale(ctx, sl, ownerOID, msgs)
	s.checkOffHours(ctx, sl, ownerOID, msgs)
	if sl.IsCredit && sl.CustomerID != "" {
		s.checkRapidCredit(ctx, sl, ownerOID, msgs)
	}
}

func (s *Service) checkLargeSale(ctx context.Context, sl sale.Sale, ownerOID bson.ObjectID, msgs i18n.Messages) {
	recent, err := s.saleRepo.ListRecentByShopBefore(ctx, sl.ShopID, sl.CreatedAt, largeSaleHistoryLimit)
	if err != nil {
		log.Printf("anomaly: large_sale history for shop %s: %v", sl.ShopID, err)
		return
	}
	if len(recent) < largeSaleMinSamples {
		return
	}

	var sum float64
	for _, r := range recent {
		sum += r.TotalAmount
	}
	mean := sum / float64(len(recent))

	var variance float64
	for _, r := range recent {
		diff := r.TotalAmount - mean
		variance += diff * diff
	}
	sigma := math.Sqrt(variance / float64(len(recent)))
	threshold := mean + largeSaleSigmaFactor*sigma

	if sl.TotalAmount <= threshold {
		return
	}

	excess := sl.TotalAmount - threshold
	details := fmt.Sprintf(msgs.NotifFraudLargeSaleBody, sl.TotalAmount, excess)
	s.fireAlert(ctx, sl, TriggerLargeSale, details, ownerOID)
}

func (s *Service) checkOffHours(ctx context.Context, sl sale.Sale, ownerOID bson.ObjectID, msgs i18n.Messages) {
	wat := time.FixedZone("WAT", cameroonOffsetSecs)
	local := sl.CreatedAt.In(wat)
	h := local.Hour()
	if h < offHoursEnd || h >= offHoursStart {
		timeStr := local.Format("15:04")
		details := fmt.Sprintf(msgs.NotifFraudOffHoursBody, sl.TotalAmount, timeStr)
		s.fireAlert(ctx, sl, TriggerOffHours, details, ownerOID)
	}
}

func (s *Service) checkRapidCredit(ctx context.Context, sl sale.Sale, ownerOID bson.ObjectID, msgs i18n.Messages) {
	since := sl.CreatedAt.Add(-rapidCreditWindowHours * time.Hour)
	count, err := s.saleRepo.CountRecentCreditsByCustomer(ctx, sl.ShopID, sl.CustomerID, since)
	if err != nil {
		log.Printf("anomaly: rapid_credit count for customer %s: %v", sl.CustomerID, err)
		return
	}
	if count < rapidCreditThreshold {
		return
	}
	details := fmt.Sprintf(msgs.NotifFraudRapidCreditBody, sl.CustomerID, count)
	s.fireAlert(ctx, sl, TriggerRapidCredit, details, ownerOID)
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

func (s *Service) fireAlert(ctx context.Context, sl sale.Sale, triggerType, details string, ownerOID bson.ObjectID) {
	alert := FraudAlert{
		ID:          uuid.NewString(),
		ShopID:      sl.ShopID,
		SaleID:      sl.ID,
		TriggerType: triggerType,
		Details:     details,
		Status:      AlertStatusOpen,
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.repo.Insert(ctx, alert); err != nil {
		log.Printf("anomaly: insert %s alert for sale %s: %v", triggerType, sl.ID, err)
		return
	}
	s.notifSvc.NotifyFraudAlert(ctx, ownerOID, sl.ShopID, triggerType, details, sl.ID)
}
