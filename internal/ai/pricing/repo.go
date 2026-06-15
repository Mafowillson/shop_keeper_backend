package pricing

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Repo struct {
	col *mongo.Collection
}

func NewRepo(db *mongo.Database) *Repo {
	return &Repo{col: db.Collection("price_recommendations")}
}

// Upsert inserts a new pending recommendation for (shopID, productID), or replaces
// an existing pending one with fresh data. Accepted/dismissed docs are never touched.
func (r *Repo) Upsert(ctx context.Context, rec PriceRecommendation) error {
	filter := bson.M{
		"shop_id":    rec.ShopID,
		"product_id": rec.ProductID,
		"status":     StatusPending,
	}
	update := bson.M{
		"$set": bson.M{
			"product_name":      rec.ProductName,
			"sell_through_rate": rec.SellThroughRate,
			"units_sold_30d":    rec.UnitsSold30d,
			"avg_stock":         rec.AvgStock,
			"action":            rec.Action,
			"change_percent":    rec.ChangePercent,
			"reason":            rec.Reason,
			"current_prices":    rec.CurrentPrices,
			"suggested_prices":  rec.SuggestedPrices,
			"updated_at":        rec.UpdatedAt,
		},
		"$setOnInsert": bson.M{
			"_id":        uuid.NewString(),
			"shop_id":    rec.ShopID,
			"product_id": rec.ProductID,
			"status":     StatusPending,
			"created_at": rec.CreatedAt,
		},
	}
	opts := options.UpdateOne().SetUpsert(true)
	_, err := r.col.UpdateOne(ctx, filter, update, opts)
	return err
}

// FindByID retrieves a single recommendation by its document ID.
func (r *Repo) FindByID(ctx context.Context, id string) (PriceRecommendation, error) {
	var rec PriceRecommendation
	if err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&rec); err != nil {
		return PriceRecommendation{}, fmt.Errorf("find price recommendation: %w", err)
	}
	return rec, nil
}

// ListByShop returns paginated recommendations for a shop, optionally filtered by status.
// Pass an empty status to return all statuses.
func (r *Repo) ListByShop(ctx context.Context, shopID, status string, page, pageSize int) ([]PriceRecommendation, int64, error) {
	filter := bson.M{"shop_id": shopID}
	if status != "" {
		filter["status"] = status
	}

	total, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count price recs: %w", err)
	}

	opts := options.Find().
		SetSort(bson.M{"updated_at": -1}).
		SetSkip(int64((page - 1) * pageSize)).
		SetLimit(int64(pageSize))

	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list price recs: %w", err)
	}
	defer cursor.Close(ctx)

	var recs []PriceRecommendation
	if err := cursor.All(ctx, &recs); err != nil {
		return nil, 0, fmt.Errorf("decode price recs: %w", err)
	}
	return recs, total, nil
}

// UpdateStatus transitions a recommendation to accepted or dismissed and records the action time.
func (r *Repo) UpdateStatus(ctx context.Context, id, status string, actedAt time.Time) error {
	update := bson.M{"$set": bson.M{
		"status":     status,
		"acted_at":   actedAt,
		"updated_at": actedAt,
	}}
	res, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return fmt.Errorf("update price rec status: %w", err)
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}
