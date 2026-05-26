package sale

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Repo struct {
	col *mongo.Collection
}

func NewRepo(db *mongo.Database) *Repo {
	return &Repo{col: db.Collection("sales")}
}

func (repo *Repo) Create(ctx context.Context, sale Sale) (Sale, error) {
	if _, err := repo.col.InsertOne(ctx, sale); err != nil {
		return Sale{}, fmt.Errorf("create sale failed: %w", err)
	}

	return sale, nil
}

func (repo *Repo) FindByIDAndOwner(ctx context.Context, id string, ownerID string) (Sale, error) {
	filter := bson.M{"_id": id, "owner_id": ownerID}

	var sale Sale
	if err := repo.col.FindOne(ctx, filter).Decode(&sale); err != nil {
		if err == mongo.ErrNoDocuments {
			return Sale{}, mongo.ErrNoDocuments
		}
		return Sale{}, fmt.Errorf("find sale failed: %w", err)
	}

	return sale, nil
}

func (repo *Repo) ListByOwner(ctx context.Context, ownerID string, shopID string) ([]Sale, error) {
	filter := bson.M{"owner_id": ownerID}
	if shopID != "" {
		filter["shop_id"] = shopID
	}

	cursor, err := repo.col.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list sales failed: %w", err)
	}
	defer cursor.Close(ctx)

	var sales []Sale
	if err := cursor.All(ctx, &sales); err != nil {
		return nil, fmt.Errorf("decode sales failed: %w", err)
	}

	return sales, nil
}
