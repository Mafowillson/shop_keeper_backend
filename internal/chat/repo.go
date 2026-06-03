package chat

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
	return &Repo{col: db.Collection("chat_messages")}
}

func (r *Repo) Save(ctx context.Context, ownerID, role, text string) (Message, error) {
	msg := Message{
		ID:        uuid.NewString(),
		OwnerID:   ownerID,
		Role:      role,
		Text:      text,
		CreatedAt: time.Now().UTC(),
	}
	if _, err := r.col.InsertOne(ctx, msg); err != nil {
		return Message{}, fmt.Errorf("save chat message: %w", err)
	}
	return msg, nil
}

func (r *Repo) History(ctx context.Context, ownerID string, limit int) ([]Message, error) {
	filter := bson.M{"owner_id": ownerID}
	opts := options.Find().SetSort(bson.M{"created_at": 1}).SetLimit(int64(limit))
	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("chat history: %w", err)
	}
	defer cursor.Close(ctx)
	var msgs []Message
	if err := cursor.All(ctx, &msgs); err != nil {
		return nil, fmt.Errorf("decode chat history: %w", err)
	}
	return msgs, nil
}

func (r *Repo) Clear(ctx context.Context, ownerID string) error {
	if _, err := r.col.DeleteMany(ctx, bson.M{"owner_id": ownerID}); err != nil {
		return fmt.Errorf("clear chat history: %w", err)
	}
	return nil
}
