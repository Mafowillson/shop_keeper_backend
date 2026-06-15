package pricing

import "time"

const (
	ActionIncrease5  = "increase_5"
	ActionIncrease3  = "increase_3"
	ActionDecrease5  = "decrease_5"
	ActionDecrease10 = "decrease_10"

	StatusPending   = "pending"
	StatusAccepted  = "accepted"
	StatusDismissed = "dismissed"
)

// PriceRecommendation is the MongoDB document stored in "price_recommendations".
// One pending document per product at a time — the weekly job upserts by (shop_id, product_id, status=pending).
type PriceRecommendation struct {
	ID          string `bson:"_id" json:"id"`
	ShopID      string `bson:"shop_id" json:"shop_id"`
	ProductID   string `bson:"product_id" json:"product_id"`
	ProductName string `bson:"product_name" json:"product_name"`

	// Metrics that drove the recommendation.
	SellThroughRate float64 `bson:"sell_through_rate" json:"sell_through_rate"`
	UnitsSold30d    int     `bson:"units_sold_30d" json:"units_sold_30d"`
	AvgStock        float64 `bson:"avg_stock" json:"avg_stock"`

	// Suggested change and reason.
	Action        string  `bson:"action" json:"action"` // one of the Action* constants
	ChangePercent float64 `bson:"change_percent" json:"change_percent"`
	Reason        string  `bson:"reason" json:"reason"`

	// Current vs. proposed prices, keyed by unit name.
	CurrentPrices   map[string]float64 `bson:"current_prices" json:"current_prices"`
	SuggestedPrices map[string]float64 `bson:"suggested_prices" json:"suggested_prices"`

	// Lifecycle.
	Status    string     `bson:"status" json:"status"`
	CreatedAt time.Time  `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time  `bson:"updated_at" json:"updated_at"`
	ActedAt   *time.Time `bson:"acted_at,omitempty" json:"acted_at,omitempty"`
}
