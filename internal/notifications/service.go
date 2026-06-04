package notification

import (
	"context"
	"fmt"
	"log"
	"time"

	"shop_keeper_backend/internal/fcm"
	"shop_keeper_backend/internal/staff"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Service is the brain of the notification module.
// It sits between the HTTP handlers (and other services that trigger events)
// and the repo + FCM client.
//
// Other packages (sale, customer, staff) depend on this interface — not on the
// concrete struct — so they stay loosely coupled and easy to test.
type Service struct {
	repo      *Repo
	fcm       *fcm.Client
	staffRepo *staff.Repo
}

// NewService wires the repo, FCM client, and staff repo into the service.
func NewService(repo *Repo, fcmClient *fcm.Client, staffRepo *staff.Repo) *Service {
	return &Service{repo: repo, fcm: fcmClient, staffRepo: staffRepo}
}

// -----------------------------------------------------------------------
// Notify — the single entry point every other package calls
// -----------------------------------------------------------------------

// Notify is the main method used by sale/service.go, customer/service.go,
// and staff/auth.go to trigger a notification.
//
// It does three things in order:
//  1. Check the owner's preferences — if this notification type is disabled, stop.
//  2. Save the notification to MongoDB so it appears in the owner's inbox.
//  3. Send an FCM push to the owner's device.
//
// Non-fatal errors (FCM send failed, no token) are logged but NOT returned —
// we never want a failed push to roll back a sale or payment.
func (s *Service) Notify(ctx context.Context, input CreateNotificationInput) error {
	// ── Step 1: Check preferences ────────────────────────────────────────
	prefs, err := s.repo.GetPreferences(ctx, input.OwnerID)
	if err != nil {
		// Preferences missing is non-fatal: log and continue with defaults.
		log.Printf("notification: could not load preferences for owner %s: %v", input.OwnerID.Hex(), err)
		defaults := DefaultPreferences(input.OwnerID)
		prefs = &defaults
	}

	// If the owner turned this notification type off, stop here.
	if !s.isTypeEnabled(prefs, input.Type) {
		log.Printf("notification: type %s is disabled for owner %s — skipping", input.Type, input.OwnerID.Hex())
		return nil
	}

	// ── Step 2: Persist to MongoDB ───────────────────────────────────────
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
		// A failed DB save IS fatal — the notification would be lost entirely.
		return fmt.Errorf("notification service: save: %w", err)
	}

	// ── Step 3: Send FCM push ────────────────────────────────────────────
	// Get the owner's FCM device token.
	token, err := s.repo.GetOwnerFCMToken(ctx, input.OwnerID)
	if err != nil || token == "" {
		// No token means the owner hasn't opened the Flutter app yet, or the
		// token fetch failed. Log and return nil — the notification is already
		// saved in the inbox so nothing is lost.
		log.Printf("notification: no FCM token for owner %s — push skipped: %v", input.OwnerID.Hex(), err)
		return nil
	}

	// Build the FCM data payload. Always include the notification type so
	// Flutter knows which screen to navigate to, plus the notification ID
	// so Flutter can mark it read when opened.
	fcmData := map[string]string{
		"type":            string(input.Type),
		"notification_id": n.ID.Hex(),
	}
	// Merge any extra data the caller passed (e.g. product_id, sale_id).
	for k, v := range input.Data {
		fcmData[k] = v
	}

	if err := s.fcm.SendToToken(ctx, token, input.Title, input.Body, fcmData); err != nil {
		// FCM failure is non-fatal. The notification is in the inbox.
		// Log it so you can monitor FCM health without crashing requests.
		log.Printf("notification: FCM send failed for owner %s: %v", input.OwnerID.Hex(), err)
	}

	return nil
}

// isTypeEnabled maps a NotificationType to the correct preferences field.
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
		return true // unknown types default to enabled
	}
}

// -----------------------------------------------------------------------
// Convenience helpers — called by other services with typed params
// -----------------------------------------------------------------------

// NotifyLowStock builds and sends a low-stock notification.
// Called from sale/service.go after stock is decremented.
//
// Parameters:
//   - ownerID / shopID : for scoping and FCM routing
//   - productName      : shown in the notification body
//   - productID        : passed as FCM data so Flutter navigates to that product
//   - currentStock     : shown in the body so the owner knows how many remain
func (s *Service) NotifyLowStock(
	ctx context.Context,
	ownerID bson.ObjectID,
	shopID string,
	productName, productID string,
	currentStock int,
) {
	// We fire-and-forget on a goroutine so the sale response is not delayed
	// by the notification round-trip to Firebase.
	go func() {
		// Use a fresh background context — the request context may already
		// be cancelled by the time the goroutine runs.
		bgCtx := context.Background()
		_ = s.Notify(bgCtx, CreateNotificationInput{
			ShopID:  shopID,
			OwnerID: ownerID,
			Type:    TypeLowStock,
			Title:   "⚠️ Stock faible",
			Body:    fmt.Sprintf("%s n'a plus que %d unité(s) en stock.", productName, currentStock),
			Data: map[string]string{
				"product_id":    productID,
				"current_stock": fmt.Sprintf("%d", currentStock),
			},
		})
	}()
}

// NotifyLargeSale fires when a sale total exceeds the owner's threshold.
// Called from sale/service.go after the sale is saved.
//
// How the threshold check works:
//   - The sale service calls GetPreferences to read LargeSaleThreshold.
//   - If saleTotal >= threshold, it calls this method.
//   - We pass the threshold check responsibility to the sale service because
//     it already has the total amount. The notification service just fires.
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
		_ = s.Notify(bgCtx, CreateNotificationInput{
			ShopID:  shopID,
			OwnerID: ownerID,
			Type:    TypeLargeSale,
			Title:   "💰 Grande vente enregistrée",
			Body:    fmt.Sprintf("%s a enregistré une vente de %.0f FCFA.", staffName, totalAmount),
			Data: map[string]string{
				"sale_id":      saleID,
				"total_amount": fmt.Sprintf("%.0f", totalAmount),
			},
		})
	}()
}

// NotifyDebtPayment fires when a customer makes a payment.
// Called from customer/service.go after the payment is recorded.
func (s *Service) NotifyDebtPayment(
	ctx context.Context,
	ownerID bson.ObjectID,
	shopID string,
	customerName, customerID string,
	amountPaid float64,
) {
	go func() {
		bgCtx := context.Background()
		_ = s.Notify(bgCtx, CreateNotificationInput{
			ShopID:  shopID,
			OwnerID: ownerID,
			Type:    TypeDebtPayment,
			Title:   "💳 Paiement de dette reçu",
			Body:    fmt.Sprintf("%s a payé %.0f FCFA.", customerName, amountPaid),
			Data: map[string]string{
				"customer_id": customerID,
				"amount_paid": fmt.Sprintf("%.0f", amountPaid),
			},
		})
	}()
}

// NotifyStaffLogin fires every time a staff member successfully logs in.
// Called from staff/auth.go after JWT is issued.
func (s *Service) NotifyStaffLogin(
	ctx context.Context,
	ownerID bson.ObjectID,
	shopID string,
	staffName string,
) {
	go func() {
		bgCtx := context.Background()
		_ = s.Notify(bgCtx, CreateNotificationInput{
			ShopID:  shopID,
			OwnerID: ownerID,
			Type:    TypeStaffLogin,
			Title:   "👤 Connexion du personnel",
			Body:    fmt.Sprintf("%s vient de se connecter.", staffName),
			Data: map[string]string{
				"staff_name": staffName,
			},
		})
	}()
}

// -----------------------------------------------------------------------
// Inbox operations — called by HTTP handlers
// -----------------------------------------------------------------------

// GetInbox returns the owner's paginated notification list.
// FR-27: GET /notifications?unread=true
func (s *Service) GetInbox(
	ctx context.Context,
	ownerID bson.ObjectID,
	unreadOnly bool,
	limit, skip int64,
) ([]Notification, int64, error) {
	notifications, err := s.repo.ListByOwner(ctx, ownerID, unreadOnly, limit, skip)
	if err != nil {
		return nil, 0, err
	}

	unreadCount, err := s.repo.CountUnread(ctx, ownerID)
	if err != nil {
		return nil, 0, err
	}

	return notifications, unreadCount, nil
}

// MarkRead marks a single notification as read.
// FR-28: PATCH /notifications/:id/read
func (s *Service) MarkRead(ctx context.Context, id, ownerID bson.ObjectID) error {
	return s.repo.MarkRead(ctx, id, ownerID)
}

// MarkAllRead marks every notification as read for the owner.
func (s *Service) MarkAllRead(ctx context.Context, ownerID bson.ObjectID) error {
	return s.repo.MarkAllRead(ctx, ownerID)
}

// -----------------------------------------------------------------------
// Preferences operations
// -----------------------------------------------------------------------

// GetPreferences returns the owner's current preferences.
func (s *Service) GetPreferences(ctx context.Context, ownerID bson.ObjectID) (*Preferences, error) {
	return s.repo.GetPreferences(ctx, ownerID)
}

// UpdatePreferences applies the owner's requested changes.
// FR-26: PUT /notifications/preferences
//
// We use pointer fields in UpdatePreferencesRequest so that a field omitted
// from the JSON body is nil and we don't overwrite existing values with zero.
func (s *Service) UpdatePreferences(
	ctx context.Context,
	ownerID bson.ObjectID,
	req UpdatePreferencesRequest,
) (*Preferences, error) {
	// Load existing prefs (or defaults if none saved yet).
	prefs, err := s.repo.GetPreferences(ctx, ownerID)
	if err != nil {
		return nil, err
	}

	prefs.OwnerID = ownerID

	// Only update fields the caller explicitly set (non-nil pointer).
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

// SaveFCMToken stores the owner's device FCM token.
// Called by the handler for POST /api/v1/owner/fcm-token.
func (s *Service) SaveFCMToken(ctx context.Context, ownerID bson.ObjectID, token string) error {
	return s.repo.SaveFCMToken(ctx, ownerID, token)
}

// -----------------------------------------------------------------------
// Staff notifications — product catalogue changes
// -----------------------------------------------------------------------

// NotifyStaff sends an FCM push to every active staff member in a shop.
// Unlike owner notifications these are NOT saved to the MongoDB inbox —
// they are push-only so staff devices refresh their product list in real time.
//
// Called fire-and-forget from product/service.go after create/update/delete.
func (s *Service) NotifyStaff(
	ctx context.Context,
	shopID, title, body string,
	notifType NotificationType,
	data map[string]string,
) {
	go func() {
		bgCtx := context.Background()

		// Load all active staff so we can save inbox notifications for every
		// member, regardless of whether they have an FCM token registered.
		staffList, err := s.staffRepo.ListByShop(bgCtx, shopID)
		if err != nil {
			log.Printf("notification: list staff for shop %s: %v", shopID, err)
			return
		}
		if len(staffList) == 0 {
			return
		}

		// Save one inbox notification per staff member.
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

		// Collect FCM tokens and send push to those who have one.
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

// GetStaffInbox returns the staff member's notification inbox with unread count.
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

// MarkStaffNotifRead marks one staff notification as read.
func (s *Service) MarkStaffNotifRead(ctx context.Context, notifID bson.ObjectID, staffID string) error {
	return s.repo.MarkStaffRead(ctx, notifID, staffID)
}

// MarkAllStaffNotifsRead marks all unread notifications as read for a staff member.
func (s *Service) MarkAllStaffNotifsRead(ctx context.Context, staffID string) error {
	return s.repo.MarkAllStaffRead(ctx, staffID)
}

// GetLargeSaleThreshold is a helper for sale/service.go.
// It fetches just the threshold so the sale service can decide whether
// to trigger a large-sale notification without knowing about Preferences.
func (s *Service) GetLargeSaleThreshold(ctx context.Context, ownerID bson.ObjectID) (float64, error) {
	prefs, err := s.repo.GetPreferences(ctx, ownerID)
	if err != nil {
		return 50000, nil // safe default if prefs fetch fails
	}
	return prefs.LargeSaleThreshold, nil
}
