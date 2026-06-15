package anomaly

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Repo struct {
	col *mongo.Collection
}

func NewRepo(db *mongo.Database) *Repo {
	return &Repo{col: db.Collection("fraud_alerts")}
}

func (r *Repo) Insert(ctx context.Context, alert FraudAlert) error {
	_, err := r.col.InsertOne(ctx, alert)
	return err
}

func (r *Repo) FindByID(ctx context.Context, id string) (FraudAlert, error) {
	var alert FraudAlert
	if err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&alert); err != nil {
		return FraudAlert{}, fmt.Errorf("find fraud alert: %w", err)
	}
	return alert, nil
}

// ListByShop returns paginated alerts for a shop, optionally filtered by status.
// Pass an empty status to return all statuses.
func (r *Repo) ListByShop(ctx context.Context, shopID, status string, page, pageSize int) ([]FraudAlert, int64, error) {
	filter := bson.M{"shop_id": shopID}
	if status != "" {
		filter["status"] = status
	}

	total, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count fraud alerts: %w", err)
	}

	opts := options.Find().
		SetSort(bson.M{"created_at": -1}).
		SetSkip(int64((page - 1) * pageSize)).
		SetLimit(int64(pageSize))

	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list fraud alerts: %w", err)
	}
	defer cursor.Close(ctx)

	var alerts []FraudAlert
	if err := cursor.All(ctx, &alerts); err != nil {
		return nil, 0, fmt.Errorf("decode fraud alerts: %w", err)
	}
	return alerts, total, nil
}

// CountOpenByShop returns the number of unacknowledged fraud alerts for a shop.
func (r *Repo) CountOpenByShop(ctx context.Context, shopID string) (int64, error) {
	count, err := r.col.CountDocuments(ctx, bson.M{"shop_id": shopID, "status": AlertStatusOpen})
	if err != nil {
		return 0, fmt.Errorf("count open alerts: %w", err)
	}
	return count, nil
}

func (r *Repo) Acknowledge(ctx context.Context, id string, at time.Time) error {
	update := bson.M{"$set": bson.M{
		"status":          AlertStatusAcknowledged,
		"acknowledged_at": at,
	}}
	res, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return fmt.Errorf("acknowledge alert: %w", err)
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}
