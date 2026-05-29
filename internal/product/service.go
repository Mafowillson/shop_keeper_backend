package product

import (
	"context"
	"errors"
	"strings"
	"time"

	"shop_keeper_backend/internal/shop"
	"shop_keeper_backend/internal/validation"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Service struct {
	repo     *Repo
	shopRepo *shop.Repo
}

func NewService(repo *Repo, shopRepo *shop.Repo) *Service {
	return &Service{repo: repo, shopRepo: shopRepo}
}

func (service *Service) validateShopOwner(ctx context.Context, shopID string, ownerID string) error {
	if strings.TrimSpace(shopID) == "" {
		return errors.New("shop_id is required")
	}

	shop, err := service.shopRepo.FindByID(ctx, shopID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("shop not found")
		}
		return err
	}

	if !shop.IsActive {
		return errors.New("shop is not active")
	}

	if strings.TrimSpace(ownerID) == "" {
		return errors.New("owner id is required")
	}

	if shop.OwnerID != ownerID {
		return errors.New("shop does not belong to current user")
	}

	return nil
}

func (service *Service) assertProductOwner(ctx context.Context, id string, ownerID string) (Product, error) {
	product, err := service.repo.FindByID(ctx, id)
	if err != nil {
		return Product{}, err
	}

	if err := service.validateShopOwner(ctx, product.ShopID, ownerID); err != nil {
		return Product{}, err
	}

	return product, nil
}

func (service *Service) Create(ctx context.Context, input CreateProductInput, ownerID string) (Product, error) {
	if strings.TrimSpace(ownerID) == "" {
		return Product{}, errors.New("owner id is required")
	}

	if err := validation.ValidateUUID(input.ShopID, "shop_id"); err != nil {
		return Product{}, err
	}

	if err := validation.ValidateString(input.Name, "product name", 3, 100); err != nil {
		return Product{}, err
	}

	if err := validation.ValidateString(input.Category, "category", 2, 50); err != nil {
		return Product{}, err
	}

	if input.RetailPrice <= 0 {
		return Product{}, errors.New("retail price must be greater than zero")
	}

	if input.CartonPrice < 0 {
		return Product{}, errors.New("carton price cannot be negative")
	}

	if !IsValidProductUnit(input.Unit) {
		return Product{}, errors.New("unit must be one of: carton, bags, packets, rolls, pieces, box, bundle")
	}

	if input.CartonQty <= 0 {
		return Product{}, errors.New("carton quantity must be greater than zero")
	}

	if input.QtyPerCarton <= 0 {
		return Product{}, errors.New("qty_per_carton must be greater than zero")
	}

	if input.LowStockThreshold < 0 {
		return Product{}, errors.New("low stock threshold cannot be negative")
	}

	if err := service.validateShopOwner(ctx, input.ShopID, ownerID); err != nil {
		return Product{}, err
	}

	now := time.Now().UTC()
	product := Product{
		ID:                uuid.NewString(),
		ShopID:            input.ShopID,
		Name:              strings.TrimSpace(input.Name),
		Category:          strings.TrimSpace(input.Category),
		Unit:              strings.ToLower(strings.TrimSpace(input.Unit)),
		RetailPrice:       input.RetailPrice,
		CartonPrice:       input.CartonPrice,
		CartonQty:         input.CartonQty,
		QtyPerCarton:      input.QtyPerCarton,
		StockQty:          input.CartonQty * input.QtyPerCarton,
		LowStockThreshold: input.LowStockThreshold,
		IsActive:          true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	return service.repo.Create(ctx, product)
}

func (service *Service) GetByID(ctx context.Context, id string) (Product, error) {
	if strings.TrimSpace(id) == "" {
		return Product{}, errors.New("product id is required")
	}
	return service.repo.FindByID(ctx, id)
}

func (service *Service) List(ctx context.Context, shopID, category, search string, page, pageSize int) ([]Product, int64, error) {
	return service.repo.List(ctx, shopID, category, search, page, pageSize)
}

func (service *Service) Update(ctx context.Context, id string, input UpdateProductInput, ownerID string) (Product, error) {
	if strings.TrimSpace(id) == "" {
		return Product{}, errors.New("product id is required")
	}

	if strings.TrimSpace(ownerID) == "" {
		return Product{}, errors.New("owner id is required")
	}

	existing, err := service.assertProductOwner(ctx, id, ownerID)
	if err != nil {
		return Product{}, err
	}

	update := bson.M{"updated_at": time.Now().UTC()}

	if input.Name != nil {
		update["name"] = strings.TrimSpace(*input.Name)
	}
	if input.Category != nil {
		update["category"] = strings.TrimSpace(*input.Category)
	}
	if input.Unit != nil {
		if !IsValidProductUnit(*input.Unit) {
			return Product{}, errors.New("unit must be one of: carton, bags, packets, rolls, pieces, box, bundle")
		}
		update["unit"] = strings.ToLower(strings.TrimSpace(*input.Unit))
	}
	if input.RetailPrice != nil {
		update["retail_price"] = *input.RetailPrice
	}
	if input.CartonPrice != nil {
		update["carton_price"] = *input.CartonPrice
	}
	if input.CartonQty != nil {
		if *input.CartonQty <= 0 {
			return Product{}, errors.New("carton quantity must be greater than zero")
		}
		update["carton_qty"] = *input.CartonQty
	}
	if input.QtyPerCarton != nil {
		if *input.QtyPerCarton <= 0 {
			return Product{}, errors.New("qty_per_carton must be greater than zero")
		}
		update["qty_per_carton"] = *input.QtyPerCarton
	}
	if input.LowStockThreshold != nil {
		if *input.LowStockThreshold < 0 {
			return Product{}, errors.New("low stock threshold cannot be negative")
		}
		update["low_stock_threshold"] = *input.LowStockThreshold
	}
	if input.IsActive != nil {
		update["is_active"] = *input.IsActive
	}

	cartonQty := existing.CartonQty
	if input.CartonQty != nil {
		cartonQty = *input.CartonQty
	}
	qtyPerCarton := existing.QtyPerCarton
	if input.QtyPerCarton != nil {
		qtyPerCarton = *input.QtyPerCarton
	}
	if input.CartonQty != nil || input.QtyPerCarton != nil {
		update["stock_qty"] = cartonQty * qtyPerCarton
	}

	if len(update) == 1 {
		return Product{}, errors.New("no updates provided")
	}

	return service.repo.Update(ctx, id, update)
}

func (service *Service) Delete(ctx context.Context, id string, ownerID string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("product id is required")
	}

	if strings.TrimSpace(ownerID) == "" {
		return errors.New("owner id is required")
	}

	if _, err := service.assertProductOwner(ctx, id, ownerID); err != nil {
		return err
	}

	return service.repo.SoftDelete(ctx, id)
}

func (service *Service) Sync(ctx context.Context, input SyncProductsInput, ownerID string) ([]Product, error) {
	if strings.TrimSpace(ownerID) == "" {
		return nil, errors.New("owner id is required")
	}

	if len(input.Products) == 0 {
		return nil, errors.New("no products provided for sync")
	}

	products := make([]Product, 0, len(input.Products))
	for _, item := range input.Products {
		if strings.TrimSpace(item.ShopID) == "" {
			return nil, errors.New("shop_id is required for sync")
		}

		if !IsValidProductUnit(item.Unit) {
			return nil, errors.New("unit must be one of: carton, bags, packets, rolls, pieces, box, bundle")
		}

		if item.CartonQty <= 0 {
			return nil, errors.New("carton quantity must be greater than zero")
		}

		if item.QtyPerCarton <= 0 {
			return nil, errors.New("qty_per_carton must be greater than zero")
		}

		if err := service.validateShopOwner(ctx, item.ShopID, ownerID); err != nil {
			return nil, err
		}

		if strings.TrimSpace(item.ID) == "" {
			item.ID = uuid.NewString()
		}

		products = append(products, Product{
			ID:                item.ID,
			ShopID:            item.ShopID,
			Name:              item.Name,
			Category:          item.Category,
			Unit:              strings.ToLower(strings.TrimSpace(item.Unit)),
			RetailPrice:       item.RetailPrice,
			CartonPrice:       item.CartonPrice,
			CartonQty:         item.CartonQty,
			QtyPerCarton:      item.QtyPerCarton,
			StockQty:          item.CartonQty * item.QtyPerCarton,
			LowStockThreshold: item.LowStockThreshold,
			IsActive:          item.IsActive,
			CreatedAt:         time.Now().UTC(),
			UpdatedAt:         item.UpdatedAt,
		})
	}

	if err := service.repo.BulkUpsert(ctx, products); err != nil {
		return nil, err
	}

	return products, nil
}
