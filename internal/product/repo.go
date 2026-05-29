package product

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
	return &Repo{col: db.Collection("products")}
}

func (repo *Repo) FindByID(ctx context.Context, id string) (Product, error) {
	filter := bson.M{"_id": id}

	var product Product
	if err := repo.col.FindOne(ctx, filter).Decode(&product); err != nil {
		if err == mongo.ErrNoDocuments {
			return Product{}, mongo.ErrNoDocuments
		}
		return Product{}, fmt.Errorf("find product failed: %w", err)
	}

	return product, nil
}

func (repo *Repo) List(ctx context.Context, shopID, category, search string, page, pageSize int) ([]Product, int64, error) {
	filter := bson.M{"is_active": true}
	if shopID != "" {
		filter["shop_id"] = shopID
	}
	if category != "" {
		filter["category"] = category
	}
	if search != "" {
		filter["name"] = bson.M{"$regex": search, "$options": "i"}
	}

	total, err := repo.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count products failed: %w", err)
	}

	opts := options.Find().SetSkip(int64((page - 1) * pageSize)).SetLimit(int64(pageSize)).SetSort(bson.M{"created_at": -1})
	cursor, err := repo.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list products failed: %w", err)
	}
	defer cursor.Close(ctx)

	var products []Product
	if err := cursor.All(ctx, &products); err != nil {
		return nil, 0, fmt.Errorf("decode products failed: %w", err)
	}

	return products, total, nil
}

func (repo *Repo) Create(ctx context.Context, product Product) (Product, error) {
	if _, err := repo.col.InsertOne(ctx, product); err != nil {
		return Product{}, fmt.Errorf("create product failed: %w", err)
	}

	return product, nil
}

func (repo *Repo) Update(ctx context.Context, id string, update bson.M) (Product, error) {
	filter := bson.M{"_id": id}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var product Product
	if err := repo.col.FindOneAndUpdate(ctx, filter, bson.M{"$set": update}, opts).Decode(&product); err != nil {
		if err == mongo.ErrNoDocuments {
			return Product{}, mongo.ErrNoDocuments
		}
		return Product{}, fmt.Errorf("update product failed: %w", err)
	}

	return product, nil
}

func (repo *Repo) SoftDelete(ctx context.Context, id string) error {
	filter := bson.M{"_id": id}
	update := bson.M{"$set": bson.M{"is_active": false, "updated_at": time.Now().UTC()}}

	if _, err := repo.col.UpdateOne(ctx, filter, update); err != nil {
		return fmt.Errorf("soft delete failed: %w", err)
	}

	return nil
}

func (repo *Repo) BulkUpsert(ctx context.Context, products []Product) error {
	models := make([]mongo.WriteModel, 0, len(products))

	for _, product := range products {
		replace := mongo.NewReplaceOneModel().SetFilter(bson.M{"_id": product.ID}).SetReplacement(product).SetUpsert(true)
		models = append(models, replace)
	}

	if len(models) == 0 {
		return nil
	}

	if _, err := repo.col.BulkWrite(ctx, models); err != nil {
		return fmt.Errorf("bulk upsert failed: %w", err)
	}

	return nil
}
