package anomaly

import "time"

const (
	TriggerLargeSale   = "large_sale"
	TriggerOffHours    = "off_hours"
	TriggerRapidCredit = "rapid_credit"

	AlertStatusOpen         = "open"
	AlertStatusAcknowledged = "acknowledged"
)

// FraudAlert is the document stored in the "fraud_alerts" collection.
// One document is created per rule per sale — no deduplication across sales.
type FraudAlert struct {
	ID          string     `bson:"_id" json:"id"`
	ShopID      string     `bson:"shop_id" json:"shop_id"`
	SaleID      string     `bson:"sale_id" json:"sale_id"`
	TriggerType string     `bson:"trigger_type" json:"trigger_type"`
	Details     string     `bson:"details" json:"details"`
	Status      string     `bson:"status" json:"status"`
	CreatedAt   time.Time  `bson:"created_at" json:"created_at"`
	AcknowledgedAt *time.Time `bson:"acknowledged_at,omitempty" json:"acknowledged_at,omitempty"`
}
