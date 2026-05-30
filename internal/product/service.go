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

	s, err := service.shopRepo.FindByID(ctx, shopID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("shop not found")
		}
		return err
	}

	if !s.IsActive {
		return errors.New("shop is not active")
	}

	if strings.TrimSpace(ownerID) == "" {
		return errors.New("owner id is required")
	}

	if s.OwnerID != ownerID {
		return errors.New("shop does not belong to current user")
	}

	return nil
}

func (service *Service) assertProductOwner(ctx context.Context, id string, ownerID string) (Product, error) {
	p, err := service.repo.FindByID(ctx, id)
	if err != nil {
		return Product{}, err
	}

	if err := service.validateShopOwner(ctx, p.ShopID, ownerID); err != nil {
		return Product{}, err
	}

	return p, nil
}

func (service *Service) Create(ctx context.Context, input CreateProductInput, ownerID string) (ProductResponse, error) {
	if strings.TrimSpace(ownerID) == "" {
		return ProductResponse{}, errors.New("owner id is required")
	}

	if err := validation.ValidateUUID(input.ShopID, "shop_id"); err != nil {
		return ProductResponse{}, err
	}

	if err := validation.ValidateString(input.Name, "product name", 3, 100); err != nil {
		return ProductResponse{}, err
	}

	if err := validation.ValidateString(input.Category, "category", 2, 50); err != nil {
		return ProductResponse{}, err
	}

	if err := validateUnits(input.Units); err != nil {
		return ProductResponse{}, err
	}

	if input.LowStockThreshold < 0 {
		return ProductResponse{}, errors.New("low_stock_threshold cannot be negative")
	}

	if err := service.validateShopOwner(ctx, input.ShopID, ownerID); err != nil {
		return ProductResponse{}, err
	}

	units := normaliseUnits(input.Units)
	stockQty := computeInitialStock(units, input.InitialStock)

	now := time.Now().UTC()
	p := Product{
		ID:                uuid.NewString(),
		ShopID:            input.ShopID,
		Name:              strings.TrimSpace(input.Name),
		Category:          strings.TrimSpace(input.Category),
		BaseUnit:          deriveBaseUnit(units),
		Units:             units,
		StockQty:          stockQty,
		LowStockThreshold: input.LowStockThreshold,
		IsActive:          true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	created, err := service.repo.Create(ctx, p)
	if err != nil {
		return ProductResponse{}, err
	}

	return created.ToResponse(), nil
}

func (service *Service) GetByID(ctx context.Context, id string) (ProductResponse, error) {
	if strings.TrimSpace(id) == "" {
		return ProductResponse{}, errors.New("product id is required")
	}

	p, err := service.repo.FindByID(ctx, id)
	if err != nil {
		return ProductResponse{}, err
	}

	return p.ToResponse(), nil
}

func (service *Service) List(ctx context.Context, shopID, category, search string, page, pageSize int) ([]ProductResponse, int64, error) {
	products, total, err := service.repo.List(ctx, shopID, category, search, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]ProductResponse, len(products))
	for i, p := range products {
		responses[i] = p.ToResponse()
	}

	return responses, total, nil
}

func (service *Service) Update(ctx context.Context, id string, input UpdateProductInput, ownerID string) (ProductResponse, error) {
	if strings.TrimSpace(id) == "" {
		return ProductResponse{}, errors.New("product id is required")
	}

	if strings.TrimSpace(ownerID) == "" {
		return ProductResponse{}, errors.New("owner id is required")
	}

	if _, err := service.assertProductOwner(ctx, id, ownerID); err != nil {
		return ProductResponse{}, err
	}

	update := bson.M{"updated_at": time.Now().UTC()}

	if input.Name != nil {
		update["name"] = strings.TrimSpace(*input.Name)
	}
	if input.Category != nil {
		update["category"] = strings.TrimSpace(*input.Category)
	}
	if len(input.Units) > 0 {
		if err := validateUnits(input.Units); err != nil {
			return ProductResponse{}, err
		}
		units := normaliseUnits(input.Units)
		update["units"] = units
		update["base_unit"] = deriveBaseUnit(units)
	}
	if input.LowStockThreshold != nil {
		if *input.LowStockThreshold < 0 {
			return ProductResponse{}, errors.New("low_stock_threshold cannot be negative")
		}
		update["low_stock_threshold"] = *input.LowStockThreshold
	}
	if input.IsActive != nil {
		update["is_active"] = *input.IsActive
	}

	if len(update) == 1 {
		return ProductResponse{}, errors.New("no updates provided")
	}

	updated, err := service.repo.Update(ctx, id, update)
	if err != nil {
		return ProductResponse{}, err
	}

	return updated.ToResponse(), nil
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

func (service *Service) Sync(ctx context.Context, input SyncProductsInput, ownerID string) ([]ProductResponse, error) {
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

		if err := validateUnits(item.Units); err != nil {
			return nil, err
		}

		if err := service.validateShopOwner(ctx, item.ShopID, ownerID); err != nil {
			return nil, err
		}

		id := strings.TrimSpace(item.ID)
		if id == "" {
			id = uuid.NewString()
		}

		units := normaliseUnits(item.Units)
		products = append(products, Product{
			ID:                id,
			ShopID:            item.ShopID,
			Name:              item.Name,
			Category:          item.Category,
			BaseUnit:          deriveBaseUnit(units),
			Units:             units,
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

	responses := make([]ProductResponse, len(products))
	for i, p := range products {
		responses[i] = p.ToResponse()
	}

	return responses, nil
}
