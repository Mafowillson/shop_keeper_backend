package insights

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Repo struct {
	col *mongo.Collection
}

func NewRepo(db *mongo.Database) *Repo {
	return &Repo{col: db.Collection("weekly_insights")}
}

// Upsert inserts a new insight for (shopID, weekStart) or replaces an existing one.
func (r *Repo) Upsert(ctx context.Context, insight WeeklyInsight) error {
	filter := bson.M{"shop_id": insight.ShopID, "week_start": insight.WeekStart}
	update := bson.M{
		"$set": bson.M{
			"content":      insight.Content,
			"generated_at": insight.GeneratedAt,
		},
		"$setOnInsert": bson.M{
			"_id":        uuid.NewString(),
			"shop_id":    insight.ShopID,
			"week_start": insight.WeekStart,
		},
	}
	opts := options.UpdateOne().SetUpsert(true)
	_, err := r.col.UpdateOne(ctx, filter, update, opts)
	return err
}

// ListByShop returns the most recent [limit] insights for a shop, newest first.
func (r *Repo) ListByShop(ctx context.Context, shopID string, limit int) ([]WeeklyInsight, error) {
	if limit <= 0 {
		limit = 4
	}
	opts := options.Find().SetSort(bson.M{"week_start": -1}).SetLimit(int64(limit))
	cursor, err := r.col.Find(ctx, bson.M{"shop_id": shopID}, opts)
	if err != nil {
		return nil, fmt.Errorf("list insights: %w", err)
	}
	defer cursor.Close(ctx)
	var insights []WeeklyInsight
	if err := cursor.All(ctx, &insights); err != nil {
		return nil, fmt.Errorf("decode insights: %w", err)
	}
	return insights, nil
}

// LatestByShop returns the single most recent insight for a shop.
func (r *Repo) LatestByShop(ctx context.Context, shopID string) (WeeklyInsight, error) {
	opts := options.FindOne().SetSort(bson.M{"week_start": -1})
	var insight WeeklyInsight
	if err := r.col.FindOne(ctx, bson.M{"shop_id": shopID}, opts).Decode(&insight); err != nil {
		return WeeklyInsight{}, fmt.Errorf("latest insight: %w", err)
	}
	return insight, nil
}

