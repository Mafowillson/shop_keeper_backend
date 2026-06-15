package insights

import "time"

// WeeklyInsight is the document stored in the "weekly_insights" collection.
// One document per shop per week; the job upserts by (shop_id, week_start).
type WeeklyInsight struct {
	ID          string    `bson:"_id" json:"id"`
	ShopID      string    `bson:"shop_id" json:"shop_id"`
	WeekStart   time.Time `bson:"week_start" json:"week_start"`
	Content     string    `bson:"content" json:"content"`
	GeneratedAt time.Time `bson:"generated_at" json:"generated_at"`
}
