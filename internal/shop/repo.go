package shop

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
	return &Repo{col: db.Collection("shops")}
}

func (repo *Repo) Create(ctx context.Context, shop Shop) (Shop, error) {
	if _, err := repo.col.InsertOne(ctx, shop); err != nil {
		return Shop{}, fmt.Errorf("create shop failed: %w", err)
	}

	return shop, nil
}

func (repo *Repo) FindByID(ctx context.Context, id string) (Shop, error) {
	filter := bson.M{"_id": id}

	var shop Shop
	if err := repo.col.FindOne(ctx, filter).Decode(&shop); err != nil {
		if err == mongo.ErrNoDocuments {
			return Shop{}, mongo.ErrNoDocuments
		}
		return Shop{}, fmt.Errorf("find shop failed: %w", err)
	}

	return shop, nil
}

func (repo *Repo) FindByIDAndOwner(ctx context.Context, id string, ownerID string) (Shop, error) {
	filter := bson.M{"_id": id, "owner_id": ownerID}

	var shop Shop
	if err := repo.col.FindOne(ctx, filter).Decode(&shop); err != nil {
		if err == mongo.ErrNoDocuments {
			return Shop{}, mongo.ErrNoDocuments
		}
		return Shop{}, fmt.Errorf("find shop by owner failed: %w", err)
	}

	return shop, nil
}

func (repo *Repo) ListByOwner(ctx context.Context, ownerID string) ([]Shop, error) {
	filter := bson.M{"owner_id": ownerID}

	cursor, err := repo.col.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list shops failed: %w", err)
	}
	defer cursor.Close(ctx)

	var shops []Shop
	if err := cursor.All(ctx, &shops); err != nil {
		return nil, fmt.Errorf("decode shops failed: %w", err)
	}

	return shops, nil
}

func (repo *Repo) Update(ctx context.Context, id string, update bson.M) (Shop, error) {
	filter := bson.M{"_id": id}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var shop Shop
	if err := repo.col.FindOneAndUpdate(ctx, filter, bson.M{"$set": update}, opts).Decode(&shop); err != nil {
		if err == mongo.ErrNoDocuments {
			return Shop{}, mongo.ErrNoDocuments
		}
		return Shop{}, fmt.Errorf("update shop failed: %w", err)
	}

	return shop, nil
}

func (repo *Repo) SoftDelete(ctx context.Context, id string) error {
	filter := bson.M{"_id": id}
	update := bson.M{"$set": bson.M{"is_active": false, "updated_at": time.Now().UTC()}}

	if _, err := repo.col.UpdateOne(ctx, filter, update); err != nil {
		return fmt.Errorf("soft delete shop failed: %w", err)
	}

	return nil
}
