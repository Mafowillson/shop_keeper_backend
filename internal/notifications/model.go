package notification

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// -----------------------------------------------------------------------
// Notification types — these match FR-25 exactly.
// Each constant is a string so it can be stored in MongoDB as-is and
// sent to Flutter as part of the FCM data payload.
// -----------------------------------------------------------------------

type NotificationType string

const (
	// TypeLowStock fires when a product's stock_qty drops below low_stock_threshold.
	TypeLowStock NotificationType = "low_stock"

	// TypeLargeSale fires when a single sale exceeds the owner's configured threshold.
	TypeLargeSale NotificationType = "large_sale"

	// TypeDebtPayment fires when a customer makes a payment against their debt.
	TypeDebtPayment NotificationType = "debt_payment"

	// TypeStaffLogin fires every time a staff member successfully logs in.
	TypeStaffLogin NotificationType = "staff_login"
)

// -----------------------------------------------------------------------
// Notification — the document stored in the "notifications" MongoDB collection.
// -----------------------------------------------------------------------

type Notification struct {
	// ID is the MongoDB ObjectID, used as the document primary key.
	ID bson.ObjectID `bson:"_id,omitempty" json:"id"`

	// ShopID scopes this notification to one shop.
	// All notifications belong to a shop so the owner's inbox only shows
	// their own shop's events.
	ShopID bson.ObjectID `bson:"shop_id" json:"shop_id"`

	// OwnerID is the user ID of the shop owner who should receive this.
	// We store it so we can quickly fetch "all notifications for owner X".
	OwnerID bson.ObjectID `bson:"owner_id" json:"owner_id"`

	// Type is one of the four constants above.
	Type NotificationType `bson:"type" json:"type"`

	// Title is the bold heading shown in the FCM push and in the inbox.
	Title string `bson:"title" json:"title"`

	// Body is the descriptive text shown below the title.
	Body string `bson:"body" json:"body"`

	// Data holds extra key-value pairs sent silently with the FCM push.
	// Flutter uses these to know which screen to open when the user taps
	// the notification. E.g. {"product_id": "abc123"} for a low_stock alert.
	Data map[string]string `bson:"data,omitempty" json:"data,omitempty"`

	// Read tracks whether the owner has seen this notification in the inbox.
	// Default false. Set to true by PATCH /notifications/:id/read.
	Read bool `bson:"read" json:"read"`

	// CreatedAt is set once when the notification is first created.
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
}

// -----------------------------------------------------------------------
// Preferences — stored per owner in the "notification_preferences" collection.
// FR-26: the owner can toggle which notification types are active.
// -----------------------------------------------------------------------

type Preferences struct {
	ID      bson.ObjectID `bson:"_id,omitempty" json:"id"`
	OwnerID bson.ObjectID `bson:"owner_id" json:"owner_id"`

	// Each field maps to one NotificationType. true = send it, false = suppress.
	LowStock    bool `bson:"low_stock"    json:"low_stock"`
	LargeSale   bool `bson:"large_sale"   json:"large_sale"`
	DebtPayment bool `bson:"debt_payment" json:"debt_payment"`
	StaffLogin  bool `bson:"staff_login"  json:"staff_login"`

	// LargeSaleThreshold is the FCFA amount above which a sale is "large".
	// The owner sets this in preferences. Default 50000 FCFA.
	LargeSaleThreshold float64 `bson:"large_sale_threshold" json:"large_sale_threshold"`
}

// DefaultPreferences returns a Preferences with everything enabled
// and a sensible large-sale threshold.
// Called when an owner registers and has no preferences doc yet.
func DefaultPreferences(ownerID bson.ObjectID) Preferences {
	return Preferences{
		ID:                 bson.NewObjectID(),
		OwnerID:            ownerID,
		LowStock:           true,
		LargeSale:          true,
		DebtPayment:        true,
		StaffLogin:         true,
		LargeSaleThreshold: 50000,
	}
}

// -----------------------------------------------------------------------
// Request / Response DTOs
// -----------------------------------------------------------------------

// UpdatePreferencesRequest is the JSON body for PUT /notifications/preferences.
type UpdatePreferencesRequest struct {
	LowStock           *bool    `json:"low_stock"`
	LargeSale          *bool    `json:"large_sale"`
	DebtPayment        *bool    `json:"debt_payment"`
	StaffLogin         *bool    `json:"staff_login"`
	LargeSaleThreshold *float64 `json:"large_sale_threshold"`
}

// CreateNotificationInput is used internally by the service — not exposed via HTTP.
// Other packages (sale, customer, staff) call notification.Service.Notify() with this.
type CreateNotificationInput struct {
	ShopID  bson.ObjectID
	OwnerID bson.ObjectID
	Type    NotificationType
	Title   string
	Body    string
	// Data is optional extra context forwarded to Flutter via FCM.
	Data map[string]string
}
