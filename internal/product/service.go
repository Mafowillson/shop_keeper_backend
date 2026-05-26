package product

import (
	"context"
	"errors"
	"strings"
	"time"

	"shop_keeper_backend/internal/shop"

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

	if err := service.validateShopOwner(ctx, input.ShopID, ownerID); err != nil {
		return Product{}, err
	}

	if strings.TrimSpace(input.Name) == "" {
		return Product{}, errors.New("product name is required")
	}

	now := time.Now().UTC()
	product := Product{
		ID:                uuid.NewString(),
		ShopID:            input.ShopID,
		Name:              strings.TrimSpace(input.Name),
		Category:          strings.TrimSpace(input.Category),
		RetailPrice:       input.RetailPrice,
		CartonPrice:       input.CartonPrice,
		CartonQty:         input.CartonQty,
		StockQty:          input.StockQty,
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

func (service *Service) List(ctx context.Context, shopID, category, search string) ([]Product, error) {
	return service.repo.List(ctx, shopID, category, search)
}

func (service *Service) Update(ctx context.Context, id string, input UpdateProductInput, ownerID string) (Product, error) {
	if strings.TrimSpace(id) == "" {
		return Product{}, errors.New("product id is required")
	}

	if strings.TrimSpace(ownerID) == "" {
		return Product{}, errors.New("owner id is required")
	}

	if _, err := service.assertProductOwner(ctx, id, ownerID); err != nil {
		return Product{}, err
	}

	update := bson.M{"updated_at": time.Now().UTC()}

	if input.Name != nil {
		update["name"] = strings.TrimSpace(*input.Name)
	}
	if input.Category != nil {
		update["category"] = strings.TrimSpace(*input.Category)
	}
	if input.RetailPrice != nil {
		update["retail_price"] = *input.RetailPrice
	}
	if input.CartonPrice != nil {
		update["carton_price"] = *input.CartonPrice
	}
	if input.CartonQty != nil {
		update["carton_qty"] = *input.CartonQty
	}
	if input.StockQty != nil {
		update["stock_qty"] = *input.StockQty
	}
	if input.LowStockThreshold != nil {
		update["low_stock_threshold"] = *input.LowStockThreshold
	}
	if input.IsActive != nil {
		update["is_active"] = *input.IsActive
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
			RetailPrice:       item.RetailPrice,
			CartonPrice:       item.CartonPrice,
			CartonQty:         item.CartonQty,
			StockQty:          item.StockQty,
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
