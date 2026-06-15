package sale

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
	return &Repo{col: db.Collection("sales")}
}

func (repo *Repo) Create(ctx context.Context, sale Sale) (Sale, error) {
	if _, err := repo.col.InsertOne(ctx, sale); err != nil {
		return Sale{}, fmt.Errorf("create sale failed: %w", err)
	}

	return sale, nil
}

func (repo *Repo) FindByID(ctx context.Context, id string) (Sale, error) {
	var sale Sale
	if err := repo.col.FindOne(ctx, bson.M{"_id": id}).Decode(&sale); err != nil {
		if err == mongo.ErrNoDocuments {
			return Sale{}, mongo.ErrNoDocuments
		}
		return Sale{}, fmt.Errorf("find sale failed: %w", err)
	}
	return sale, nil
}

func (repo *Repo) ListByOwner(ctx context.Context, ownerID string, shopID string, page, pageSize int) ([]Sale, int64, error) {
	// Filter by shop when available — captures all sales in the shop regardless
	// of whether they were recorded by the owner or a staff member.
	// Fall back to owner_id only when no shop is specified.
	var filter bson.M
	if strings.TrimSpace(shopID) != "" {
		filter = bson.M{"shop_id": shopID}
	} else {
		filter = bson.M{"owner_id": ownerID}
	}

	total, err := repo.col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("count sales failed: %w", err)
	}

	opts := options.Find().SetSkip(int64((page - 1) * pageSize)).SetLimit(int64(pageSize)).SetSort(bson.M{"created_at": -1})
	cursor, err := repo.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list sales failed: %w", err)
	}
	defer cursor.Close(ctx)

	var sales []Sale
	if err := cursor.All(ctx, &sales); err != nil {
		return nil, 0, fmt.Errorf("decode sales failed: %w", err)
	}

	return sales, total, nil
}

func (repo *Repo) Delete(ctx context.Context, id string) error {
	if _, err := repo.col.DeleteOne(ctx, bson.M{"_id": id}); err != nil {
		return fmt.Errorf("delete sale failed: %w", err)
	}
	return nil
}

// TodayStatsByShop returns total amount and count of all sales in a shop since [from].
func (repo *Repo) TodayStatsByShop(ctx context.Context, shopID string, from time.Time) (total float64, count int64, err error) {
	filter := bson.M{"shop_id": shopID, "created_at": bson.M{"$gte": from}}
	count, err = repo.col.CountDocuments(ctx, filter)
	if err != nil {
		return 0, 0, fmt.Errorf("count today sales: %w", err)
	}
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$group", Value: bson.M{"_id": nil, "total": bson.M{"$sum": "$total_amount"}}}},
	}
	cursor, err := repo.col.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, count, fmt.Errorf("sum today sales: %w", err)
	}
	defer cursor.Close(ctx)
	var result []struct {
		Total float64 `bson:"total"`
	}
	if err := cursor.All(ctx, &result); err != nil || len(result) == 0 {
		return 0, count, nil
	}
	return result[0].Total, count, nil
}

// TodayStatsByUser returns total amount and count of sales made BY a specific user since [from].
func (repo *Repo) TodayStatsByUser(ctx context.Context, userID string, from time.Time) (total float64, count int64, err error) {
	filter := bson.M{"owner_id": userID, "created_at": bson.M{"$gte": from}}
	count, err = repo.col.CountDocuments(ctx, filter)
	if err != nil {
		return 0, 0, fmt.Errorf("count user today sales: %w", err)
	}
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$group", Value: bson.M{"_id": nil, "total": bson.M{"$sum": "$total_amount"}}}},
	}
	cursor, err := repo.col.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, count, fmt.Errorf("sum user today sales: %w", err)
	}
	defer cursor.Close(ctx)
	var result []struct {
		Total float64 `bson:"total"`
	}
	if err := cursor.All(ctx, &result); err != nil || len(result) == 0 {
		return 0, count, nil
	}
	return result[0].Total, count, nil
}

// WeeklyRevenueByShop returns 7 daily totals starting from [weekStart] (index 0 = weekStart day).
func (repo *Repo) WeeklyRevenueByShop(ctx context.Context, shopID string, weekStart time.Time) ([7]float64, error) {
	weekEnd := weekStart.AddDate(0, 0, 7)
	filter := bson.M{"shop_id": shopID, "created_at": bson.M{"$gte": weekStart, "$lt": weekEnd}}

	cursor, err := repo.col.Find(ctx, filter, options.Find().SetProjection(bson.M{"total_amount": 1, "created_at": 1}))
	if err != nil {
		return [7]float64{}, fmt.Errorf("weekly revenue query: %w", err)
	}
	defer cursor.Close(ctx)

	var rows []struct {
		TotalAmount float64   `bson:"total_amount"`
		CreatedAt   time.Time `bson:"created_at"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return [7]float64{}, fmt.Errorf("decode weekly revenue: %w", err)
	}

	var totals [7]float64
	for _, r := range rows {
		day := int(r.CreatedAt.UTC().Truncate(24 * time.Hour).Sub(weekStart.UTC().Truncate(24 * time.Hour)).Hours() / 24)
		if day >= 0 && day < 7 {
			totals[day] += r.TotalAmount
		}
	}
	return totals, nil
}

// ListRecentByShop returns the [limit] most recent sales for a shop.
func (repo *Repo) ListRecentByShop(ctx context.Context, shopID string, limit int) ([]Sale, error) {
	opts := options.Find().SetSort(bson.M{"created_at": -1}).SetLimit(int64(limit))
	cursor, err := repo.col.Find(ctx, bson.M{"shop_id": shopID}, opts)
	if err != nil {
		return nil, fmt.Errorf("list recent shop sales: %w", err)
	}
	defer cursor.Close(ctx)
	var sales []Sale
	if err := cursor.All(ctx, &sales); err != nil {
		return nil, fmt.Errorf("decode recent sales: %w", err)
	}
	return sales, nil
}

// PeriodStatsByShop returns total revenue and transaction count for a shop within [from, to).
func (repo *Repo) PeriodStatsByShop(ctx context.Context, shopID string, from, to time.Time) (float64, int64, error) {
	filter := bson.M{"shop_id": shopID, "created_at": bson.M{"$gte": from, "$lt": to}}
	count, err := repo.col.CountDocuments(ctx, filter)
	if err != nil {
		return 0, 0, fmt.Errorf("count period sales: %w", err)
	}
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$group", Value: bson.M{"_id": nil, "total": bson.M{"$sum": "$total_amount"}}}},
	}
	cursor, err := repo.col.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, count, fmt.Errorf("sum period sales: %w", err)
	}
	defer cursor.Close(ctx)
	var result []struct {
		Total float64 `bson:"total"`
	}
	if err := cursor.All(ctx, &result); err != nil || len(result) == 0 {
		return 0, count, nil
	}
	return result[0].Total, count, nil
}

// UnitsSoldByProductSince returns the total base units sold per product_id
// across all sales in [shopID] since [since]. Products with no sales are absent from the map.
func (repo *Repo) UnitsSoldByProductSince(ctx context.Context, shopID string, since time.Time) (map[string]int, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"shop_id": shopID, "created_at": bson.M{"$gte": since}}}},
		{{Key: "$unwind", Value: "$items"}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$items.product_id",
			"units": bson.M{"$sum": "$items.base_qty_deducted"},
		}}},
	}
	cursor, err := repo.col.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("units sold by product: %w", err)
	}
	defer cursor.Close(ctx)
	var rows []struct {
		ProductID string `bson:"_id"`
		Units     int    `bson:"units"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("decode units sold: %w", err)
	}
	m := make(map[string]int, len(rows))
	for _, r := range rows {
		m[r.ProductID] = r.Units
	}
	return m, nil
}

// TopProductsByRevenue returns the top [limit] products by total revenue since [since],
// unwinding sale items and grouping by product_id.
func (repo *Repo) TopProductsByRevenue(ctx context.Context, shopID string, since time.Time, limit int) ([]ProductRevenueStat, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"shop_id": shopID, "created_at": bson.M{"$gte": since}}}},
		{{Key: "$unwind", Value: "$items"}},
		{{Key: "$group", Value: bson.M{
			"_id":     "$items.product_id",
			"revenue": bson.M{"$sum": "$items.total_price"},
			"units":   bson.M{"$sum": "$items.base_qty_deducted"},
		}}},
		{{Key: "$sort", Value: bson.M{"revenue": -1}}},
		{{Key: "$limit", Value: limit}},
	}
	cursor, err := repo.col.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("top products by revenue: %w", err)
	}
	defer cursor.Close(ctx)
	var stats []ProductRevenueStat
	if err := cursor.All(ctx, &stats); err != nil {
		return nil, fmt.Errorf("decode top products: %w", err)
	}
	return stats, nil
}

// ListRecentByShopBefore returns the [limit] most recent sales in a shop created strictly
// before [before]. Used by AI-4 to build the statistical baseline for large-sale detection.
func (repo *Repo) ListRecentByShopBefore(ctx context.Context, shopID string, before time.Time, limit int) ([]Sale, error) {
	filter := bson.M{"shop_id": shopID, "created_at": bson.M{"$lt": before}}
	opts := options.Find().SetSort(bson.M{"created_at": -1}).SetLimit(int64(limit))
	cursor, err := repo.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("list recent shop sales before: %w", err)
	}
	defer cursor.Close(ctx)
	var sales []Sale
	if err := cursor.All(ctx, &sales); err != nil {
		return nil, fmt.Errorf("decode recent sales before: %w", err)
	}
	return sales, nil
}

// CountRecentCreditsByCustomer counts credit sales for a specific customer in a shop
// since [since]. Used by AI-4 to detect rapid successive credit granting.
func (repo *Repo) CountRecentCreditsByCustomer(ctx context.Context, shopID, customerID string, since time.Time) (int64, error) {
	filter := bson.M{
		"shop_id":     shopID,
		"customer_id": customerID,
		"is_credit":   true,
		"created_at":  bson.M{"$gte": since},
	}
	count, err := repo.col.CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("count recent credits by customer: %w", err)
	}
	return count, nil
}

// ListRecentByUser returns the [limit] most recent sales recorded by a specific user.
func (repo *Repo) ListRecentByUser(ctx context.Context, userID string, limit int) ([]Sale, error) {
	opts := options.Find().SetSort(bson.M{"created_at": -1}).SetLimit(int64(limit))
	cursor, err := repo.col.Find(ctx, bson.M{"owner_id": userID}, opts)
	if err != nil {
		return nil, fmt.Errorf("list recent user sales: %w", err)
	}
	defer cursor.Close(ctx)
	var sales []Sale
	if err := cursor.All(ctx, &sales); err != nil {
		return nil, fmt.Errorf("decode recent user sales: %w", err)
	}
	return sales, nil
}
