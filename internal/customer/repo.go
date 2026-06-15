package customer

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
	customerCol *mongo.Collection
	debtCol     *mongo.Collection
}

func NewRepo(db *mongo.Database) *Repo {
	return &Repo{
		customerCol: db.Collection("customers"),
		debtCol:     db.Collection("debt_records"),
	}
}

// CreateCustomer inserts a new customer.
func (repo *Repo) CreateCustomer(ctx context.Context, customer Customer) (Customer, error) {
	if _, err := repo.customerCol.InsertOne(ctx, customer); err != nil {
		return Customer{}, fmt.Errorf("create customer failed: %w", err)
	}
	return customer, nil
}

// GetCustomerByID retrieves a customer by ID.
func (repo *Repo) GetCustomerByID(ctx context.Context, id string) (Customer, error) {
	filter := bson.M{"_id": id}
	var customer Customer
	if err := repo.customerCol.FindOne(ctx, filter).Decode(&customer); err != nil {
		if err == mongo.ErrNoDocuments {
			return Customer{}, mongo.ErrNoDocuments
		}
		return Customer{}, fmt.Errorf("get customer failed: %w", err)
	}
	return customer, nil
}

// ListCustomersByShop retrieves all customers for a shop, optionally filtered by has_debt.
func (repo *Repo) ListCustomersByShop(ctx context.Context, shopID string, hasDebt *bool, page, pageSize int) ([]Customer, int64, error) {
	filter := bson.M{"shop_id": shopID}
	if hasDebt != nil {
		if *hasDebt {
			filter["total_debt"] = bson.M{"$gt": 0}
		} else {
			filter["total_debt"] = bson.M{"$eq": 0}
		}
	}

	total, err := repo.customerCol.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count customers failed: %w", err)
	}

	opts := options.Find().SetSkip(int64((page - 1) * pageSize)).SetLimit(int64(pageSize)).SetSort(bson.M{"created_at": -1})
	cursor, err := repo.customerCol.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list customers failed: %w", err)
	}
	defer cursor.Close(ctx)

	var customers []Customer
	if err := cursor.All(ctx, &customers); err != nil {
		return nil, 0, fmt.Errorf("decode customers failed: %w", err)
	}

	return customers, total, nil
}

// UpdateCustomerDebt updates the total_debt field for a customer.
func (repo *Repo) UpdateCustomerDebt(ctx context.Context, customerID string, newDebt float64) error {
	filter := bson.M{"_id": customerID}
	update := bson.M{
		"$set": bson.M{
			"total_debt": newDebt,
			"updated_at": time.Now().UTC(),
		},
	}

	if _, err := repo.customerCol.UpdateOne(ctx, filter, update); err != nil {
		return fmt.Errorf("update customer debt failed: %w", err)
	}

	return nil
}

// CreateDebtRecord inserts a new debt event.
func (repo *Repo) CreateDebtRecord(ctx context.Context, record DebtRecord) (DebtRecord, error) {
	if record.ID == "" {
		record.ID = uuid.NewString()
	}
	if _, err := repo.debtCol.InsertOne(ctx, record); err != nil {
		return DebtRecord{}, fmt.Errorf("create debt record failed: %w", err)
	}
	return record, nil
}

// GetDebtHistoryByCustomer retrieves all debt events for a customer, sorted by recorded_at descending.
func (repo *Repo) GetDebtHistoryByCustomer(ctx context.Context, customerID string) ([]DebtRecord, error) {
	filter := bson.M{"customer_id": customerID}
	opts := options.Find().SetSort(bson.M{"recorded_at": -1})

	cursor, err := repo.debtCol.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("get debt history failed: %w", err)
	}
	defer cursor.Close(ctx)

	var records []DebtRecord
	if err := cursor.All(ctx, &records); err != nil {
		return nil, fmt.Errorf("decode debt records failed: %w", err)
	}

	return records, nil
}

// CountWithDebt returns how many customers in a shop have total_debt > 0.
func (repo *Repo) CountWithDebt(ctx context.Context, shopID string) (int64, error) {
	count, err := repo.customerCol.CountDocuments(ctx, bson.M{"shop_id": shopID, "total_debt": bson.M{"$gt": 0}})
	if err != nil {
		return 0, fmt.Errorf("count with debt: %w", err)
	}
	return count, nil
}

// TopDebtorsByShop returns the [limit] customers with the highest total_debt, descending.
func (repo *Repo) TopDebtorsByShop(ctx context.Context, shopID string, limit int) ([]Customer, error) {
	filter := bson.M{"shop_id": shopID, "total_debt": bson.M{"$gt": 0}}
	opts := options.Find().SetSort(bson.M{"total_debt": -1}).SetLimit(int64(limit))
	cursor, err := repo.customerCol.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("top debtors: %w", err)
	}
	defer cursor.Close(ctx)
	var customers []Customer
	if err := cursor.All(ctx, &customers); err != nil {
		return nil, fmt.Errorf("decode top debtors: %w", err)
	}
	return customers, nil
}

// UpdateRiskScore persists the AI-2 computed risk score and level for a customer.
func (repo *Repo) UpdateRiskScore(ctx context.Context, customerID string, score int, level string, calculatedAt time.Time) error {
	filter := bson.M{"_id": customerID}
	update := bson.M{"$set": bson.M{
		"risk_score":         score,
		"risk_level":         level,
		"risk_calculated_at": calculatedAt,
		"updated_at":         calculatedAt,
	}}
	if _, err := repo.customerCol.UpdateOne(ctx, filter, update); err != nil {
		return fmt.Errorf("update risk score: %w", err)
	}
	return nil
}

// CountNewByShopSince returns how many customers were created for a shop since [since].
func (repo *Repo) CountNewByShopSince(ctx context.Context, shopID string, since time.Time) (int64, error) {
	count, err := repo.customerCol.CountDocuments(ctx, bson.M{
		"shop_id":    shopID,
		"created_at": bson.M{"$gte": since},
	})
	if err != nil {
		return 0, fmt.Errorf("count new customers: %w", err)
	}
	return count, nil
}

// TotalDebtByShop sums the total_debt field across all customers in a shop.
func (repo *Repo) TotalDebtByShop(ctx context.Context, shopID string) (float64, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"shop_id": shopID, "total_debt": bson.M{"$gt": 0}}}},
		{{Key: "$group", Value: bson.M{"_id": nil, "total": bson.M{"$sum": "$total_debt"}}}},
	}
	cursor, err := repo.customerCol.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, fmt.Errorf("total debt by shop: %w", err)
	}
	defer cursor.Close(ctx)
	var result []struct {
		Total float64 `bson:"total"`
	}
	if err := cursor.All(ctx, &result); err != nil || len(result) == 0 {
		return 0, nil
	}
	return result[0].Total, nil
}
