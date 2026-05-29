package staff

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Repo struct {
	col *mongo.Collection
}

func NewRepo(db *mongo.Database) *Repo {
	return &Repo{col: db.Collection("staff")}
}

func (repo *Repo) Create(ctx context.Context, staff Staff) (Staff, error) {
	if _, err := repo.col.InsertOne(ctx, staff); err != nil {
		return Staff{}, fmt.Errorf("create staff failed: %w", err)
	}

	return staff, nil
}

func (repo *Repo) FindByID(ctx context.Context, id string) (Staff, error) {
	filter := bson.M{"_id": id}

	var staff Staff
	if err := repo.col.FindOne(ctx, filter).Decode(&staff); err != nil {
		if err == mongo.ErrNoDocuments {
			return Staff{}, mongo.ErrNoDocuments
		}
		return Staff{}, fmt.Errorf("find staff failed: %w", err)
	}

	return staff, nil
}

func (repo *Repo) FindByEmail(ctx context.Context, email string) (Staff, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	filter := bson.M{"email": email}

	var staff Staff
	if err := repo.col.FindOne(ctx, filter).Decode(&staff); err != nil {
		if err == mongo.ErrNoDocuments {
			return Staff{}, mongo.ErrNoDocuments
		}
		return Staff{}, fmt.Errorf("find staff by email failed: %w", err)
	}

	return staff, nil
}

func (repo *Repo) FindByIDAndOwner(ctx context.Context, id string, ownerID string) (Staff, error) {
	filter := bson.M{"_id": id, "owner_id": ownerID}

	var staff Staff
	if err := repo.col.FindOne(ctx, filter).Decode(&staff); err != nil {
		if err == mongo.ErrNoDocuments {
			return Staff{}, mongo.ErrNoDocuments
		}
		return Staff{}, fmt.Errorf("find staff by owner failed: %w", err)
	}

	return staff, nil
}

func (repo *Repo) ListByOwner(ctx context.Context, ownerID string, page, pageSize int) ([]Staff, int64, error) {
	filter := bson.M{"owner_id": ownerID}

	total, err := repo.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count staff failed: %w", err)
	}

	opts := options.Find().SetSkip(int64((page - 1) * pageSize)).SetLimit(int64(pageSize)).SetSort(bson.M{"created_at": -1})
	cursor, err := repo.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list staff failed: %w", err)
	}
	defer cursor.Close(ctx)

	var staffList []Staff
	if err := cursor.All(ctx, &staffList); err != nil {
		return nil, 0, fmt.Errorf("decode staff failed: %w", err)
	}

	return staffList, total, nil
}

func (repo *Repo) ListByShop(ctx context.Context, shopID string) ([]Staff, error) {
	filter := bson.M{"shop_id": shopID, "is_active": true}

	cursor, err := repo.col.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list staff by shop failed: %w", err)
	}
	defer cursor.Close(ctx)

	var staffList []Staff
	if err := cursor.All(ctx, &staffList); err != nil {
		return nil, fmt.Errorf("decode staff failed: %w", err)
	}

	return staffList, nil
}

func (repo *Repo) Update(ctx context.Context, id string, update bson.M) (Staff, error) {
	filter := bson.M{"_id": id}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var staff Staff
	if err := repo.col.FindOneAndUpdate(ctx, filter, bson.M{"$set": update}, opts).Decode(&staff); err != nil {
		if err == mongo.ErrNoDocuments {
			return Staff{}, mongo.ErrNoDocuments
		}
		return Staff{}, fmt.Errorf("update staff failed: %w", err)
	}

	return staff, nil
}

func (repo *Repo) SoftDelete(ctx context.Context, id string) error {
	filter := bson.M{"_id": id}
	update := bson.M{"$set": bson.M{"is_active": false, "updated_at": time.Now().UTC()}}

	if _, err := repo.col.UpdateOne(ctx, filter, update); err != nil {
		return fmt.Errorf("soft delete staff failed: %w", err)
	}

	return nil
}
