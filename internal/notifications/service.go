package notification

import (
	"context"
	"fmt"
	"log"
	"time"

	"shop_keeper_backend/internal/fcm"
	"shop_keeper_backend/internal/i18n"
	"shop_keeper_backend/internal/staff"
	"shop_keeper_backend/internal/user"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Service struct {
	repo      *Repo
	fcm       *fcm.Client
	staffRepo *staff.Repo
	userRepo  *user.Repo
}

func NewService(repo *Repo, fcmClient *fcm.Client, staffRepo *staff.Repo, userRepo *user.Repo) *Service {
	return &Service{repo: repo, fcm: fcmClient, staffRepo: staffRepo, userRepo: userRepo}
}

// -----------------------------------------------------------------------
// Notify — the single entry point every other package calls
// -----------------------------------------------------------------------

func (s *Service) Notify(ctx context.Context, input CreateNotificationInput) error {
	prefs, err := s.repo.GetPreferences(ctx, input.OwnerID)
	if err != nil {
		log.Printf("notification: could not load preferences for owner %s: %v", input.OwnerID.Hex(), err)
		defaults := DefaultPreferences(input.OwnerID)
		prefs = &defaults
	}

	if !s.isTypeEnabled(prefs, input.Type) {
		log.Printf("notification: type %s is disabled for owner %s — skipping", input.Type, input.OwnerID.Hex())
		return nil
	}

	n := &Notification{
		ID:        bson.NewObjectID(),
		ShopID:    input.ShopID,
		OwnerID:   input.OwnerID,
		Type:      input.Type,
		Title:     input.Title,
		Body:      input.Body,
		Data:      input.Data,
		Read:      false,
		CreatedAt: time.Now(),
	}

	if err := s.repo.Save(ctx, n); err != nil {
		return fmt.Errorf("notification service: save: %w", err)
	}

	token, err := s.repo.GetOwnerFCMToken(ctx, input.OwnerID)
	if err != nil || token == "" {
		log.Printf("notification: no FCM token for owner %s — push skipped: %v", input.OwnerID.Hex(), err)
		return nil
	}

	fcmData := map[string]string{
		"type":            string(input.Type),
		"notification_id": n.ID.Hex(),
	}
	for k, v := range input.Data {
		fcmData[k] = v
	}

	if err := s.fcm.SendToToken(ctx, token, input.Title, input.Body, fcmData); err != nil {
		log.Printf("notification: FCM send failed for owner %s: %v", input.OwnerID.Hex(), err)
	}

	return nil
}

func (s *Service) isTypeEnabled(prefs *Preferences, t NotificationType) bool {
	switch t {
	case TypeLowStock:
		return prefs.LowStock
	case TypeLargeSale:
		return prefs.LargeSale
	case TypeDebtPayment:
		return prefs.DebtPayment
	case TypeStaffLogin:
		return prefs.StaffLogin
	default:
		return true
	}
}

// -----------------------------------------------------------------------
// getOwnerLocale fetches the owner's stored locale preference.
// Falls back to "fr" on any error.
// -----------------------------------------------------------------------

func (s *Service) getOwnerLocale(ctx context.Context, ownerID bson.ObjectID) string {
	if s.userRepo == nil {
		return "fr"
	}
	u, err := s.userRepo.FindByID(ctx, ownerID.Hex())
	if err != nil || u.PreferredLocale == "" {
		return "fr"
	}
	return u.PreferredLocale
}

// -----------------------------------------------------------------------
// Convenience helpers — called by other services with typed params
// -----------------------------------------------------------------------

func (s *Service) NotifyLowStock(
	ctx context.Context,
	ownerID bson.ObjectID,
	shopID string,
	productName, productID string,
	currentStock int,
) {
	go func() {
		bgCtx := context.Background()
		msgs := i18n.Get(s.getOwnerLocale(bgCtx, ownerID))
		_ = s.Notify(bgCtx, CreateNotificationInput{
			ShopID:  shopID,
			OwnerID: ownerID,
			Type:    TypeLowStock,
			Title:   msgs.NotifLowStockTitle,
			Body:    fmt.Sprintf(msgs.NotifLowStockBody, productName, currentStock),
			Data: map[string]string{
				"product_id":    productID,
				"current_stock": fmt.Sprintf("%d", currentStock),
			},
		})
	}()
}

func (s *Service) NotifyLargeSale(
	ctx context.Context,
	ownerID bson.ObjectID,
	shopID string,
	saleID string,
	totalAmount float64,
	staffName string,
) {
	go func() {
		bgCtx := context.Background()
		msgs := i18n.Get(s.getOwnerLocale(bgCtx, ownerID))
		_ = s.Notify(bgCtx, CreateNotificationInput{
			ShopID:  shopID,
			OwnerID: ownerID,
			Type:    TypeLargeSale,
			Title:   msgs.NotifLargeSaleTitle,
			Body:    fmt.Sprintf(msgs.NotifLargeSaleBody, staffName, totalAmount),
			Data: map[string]string{
				"sale_id":      saleID,
				"total_amount": fmt.Sprintf("%.0f", totalAmount),
			},
		})
	}()
}

func (s *Service) NotifyDebtPayment(
	ctx context.Context,
	ownerID bson.ObjectID,
	shopID string,
	customerName, customerID string,
	amountPaid float64,
) {
	go func() {
		bgCtx := context.Background()
		msgs := i18n.Get(s.getOwnerLocale(bgCtx, ownerID))
		_ = s.Notify(bgCtx, CreateNotificationInput{
			ShopID:  shopID,
			OwnerID: ownerID,
			Type:    TypeDebtPayment,
			Title:   msgs.NotifDebtPaymentTitle,
			Body:    fmt.Sprintf(msgs.NotifDebtPaymentBody, customerName, amountPaid),
			Data: map[string]string{
				"customer_id": customerID,
				"amount_paid": fmt.Sprintf("%.0f", amountPaid),
			},
		})
	}()
}

func (s *Service) NotifyStaffLogin(
	ctx context.Context,
	ownerID bson.ObjectID,
	shopID string,
	staffName string,
) {
	go func() {
		bgCtx := context.Background()
		msgs := i18n.Get(s.getOwnerLocale(bgCtx, ownerID))
		_ = s.Notify(bgCtx, CreateNotificationInput{
			ShopID:  shopID,
			OwnerID: ownerID,
			Type:    TypeStaffLogin,
			Title:   msgs.NotifStaffLoginTitle,
			Body:    fmt.Sprintf(msgs.NotifStaffLoginBody, staffName),
			Data: map[string]string{
				"staff_name": staffName,
			},
		})
	}()
}

// -----------------------------------------------------------------------
// Inbox operations
// -----------------------------------------------------------------------

func (s *Service) GetInbox(
	ctx context.Context,
	ownerID bson.ObjectID,
	shopID string,
	unreadOnly bool,
	limit, skip int64,
) ([]Notification, int64, error) {
	notifications, err := s.repo.ListByOwner(ctx, ownerID, shopID, unreadOnly, limit, skip)
	if err != nil {
		return nil, 0, err
	}
	unreadCount, err := s.repo.CountUnread(ctx, ownerID, shopID)
	if err != nil {
		return nil, 0, err
	}
	return notifications, unreadCount, nil
}

func (s *Service) MarkRead(ctx context.Context, id, ownerID bson.ObjectID) error {
	return s.repo.MarkRead(ctx, id, ownerID)
}

func (s *Service) MarkAllRead(ctx context.Context, ownerID bson.ObjectID, shopID string) error {
	return s.repo.MarkAllRead(ctx, ownerID, shopID)
}

// -----------------------------------------------------------------------
// Preferences operations
// -----------------------------------------------------------------------

func (s *Service) GetPreferences(ctx context.Context, ownerID bson.ObjectID) (*Preferences, error) {
	return s.repo.GetPreferences(ctx, ownerID)
}

func (s *Service) UpdatePreferences(
	ctx context.Context,
	ownerID bson.ObjectID,
	req UpdatePreferencesRequest,
) (*Preferences, error) {
	prefs, err := s.repo.GetPreferences(ctx, ownerID)
	if err != nil {
		return nil, err
	}

	prefs.OwnerID = ownerID

	if req.LowStock != nil {
		prefs.LowStock = *req.LowStock
	}
	if req.LargeSale != nil {
		prefs.LargeSale = *req.LargeSale
	}
	if req.DebtPayment != nil {
		prefs.DebtPayment = *req.DebtPayment
	}
	if req.StaffLogin != nil {
		prefs.StaffLogin = *req.StaffLogin
	}
	if req.LargeSaleThreshold != nil && *req.LargeSaleThreshold > 0 {
		prefs.LargeSaleThreshold = *req.LargeSaleThreshold
	}

	if err := s.repo.UpsertPreferences(ctx, prefs); err != nil {
		return nil, err
	}
	return prefs, nil
}

// -----------------------------------------------------------------------
// FCM token management
// -----------------------------------------------------------------------

func (s *Service) SaveFCMToken(ctx context.Context, ownerID bson.ObjectID, token string) error {
	return s.repo.SaveFCMToken(ctx, ownerID, token)
}

// -----------------------------------------------------------------------
// Staff notifications — product catalogue changes
// -----------------------------------------------------------------------

func (s *Service) NotifyStaff(
	ctx context.Context,
	shopID, title, body string,
	notifType NotificationType,
	data map[string]string,
) {
	go func() {
		bgCtx := context.Background()

		staffList, err := s.staffRepo.ListByShop(bgCtx, shopID)
		if err != nil {
			log.Printf("notification: list staff for shop %s: %v", shopID, err)
			return
		}
		if len(staffList) == 0 {
			return
		}

		now := time.Now()
		for _, st := range staffList {
			n := &Notification{
				ID:        bson.NewObjectID(),
				ShopID:    shopID,
				StaffID:   st.ID,
				Type:      notifType,
				Title:     title,
				Body:      body,
				Data:      data,
				Read:      false,
				CreatedAt: now,
			}
			if err := s.repo.Save(bgCtx, n); err != nil {
				log.Printf("notification: save staff notif for %s: %v", st.ID, err)
			}
		}

		tokens := make([]string, 0, len(staffList))
		for _, st := range staffList {
			if st.FCMToken != "" {
				tokens = append(tokens, st.FCMToken)
			}
		}
		if len(tokens) == 0 {
			return
		}

		payload := map[string]string{"type": string(notifType)}
		for k, v := range data {
			payload[k] = v
		}
		if err := s.fcm.SendToMultiple(bgCtx, tokens, title, body, payload); err != nil {
			log.Printf("notification: staff multicast for shop %s failed: %v", shopID, err)
		}
	}()
}

func (s *Service) GetStaffInbox(ctx context.Context, staffID string, limit, skip int64) ([]Notification, int64, error) {
	notifications, err := s.repo.ListByStaff(ctx, staffID, limit, skip)
	if err != nil {
		return nil, 0, err
	}
	unread, err := s.repo.CountUnreadByStaff(ctx, staffID)
	if err != nil {
		return nil, 0, err
	}
	return notifications, unread, nil
}

func (s *Service) MarkStaffNotifRead(ctx context.Context, notifID bson.ObjectID, staffID string) error {
	return s.repo.MarkStaffRead(ctx, notifID, staffID)
}

func (s *Service) MarkAllStaffNotifsRead(ctx context.Context, staffID string) error {
	return s.repo.MarkAllStaffRead(ctx, staffID)
}

func (s *Service) GetLargeSaleThreshold(ctx context.Context, ownerID bson.ObjectID) (float64, error) {
	prefs, err := s.repo.GetPreferences(ctx, ownerID)
	if err != nil {
		return 50000, nil
	}
	return prefs.LargeSaleThreshold, nil
}
