package shop

import (
	"context"
	"errors"
	"strings"
	"time"

	"shop_keeper_backend/internal/validation"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

func (service *Service) Create(ctx context.Context, ownerID string, input CreateShopInput) (Shop, error) {
	if strings.TrimSpace(ownerID) == "" {
		return Shop{}, errors.New("owner id is required")
	}

	if err := validation.ValidateString(input.Name, "shop name", 3, 100); err != nil {
		return Shop{}, err
	}

	if err := validation.ValidateOptionalString(input.Description, "description", 400); err != nil {
		return Shop{}, err
	}

	name := strings.TrimSpace(input.Name)
	description := strings.TrimSpace(input.Description)

	now := time.Now().UTC()
	shop := Shop{
		ID:          uuid.NewString(),
		OwnerID:     ownerID,
		Name:        name,
		Description: description,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	return service.repo.Create(ctx, shop)
}

func (service *Service) GetByIDAndOwner(ctx context.Context, id string, ownerID string) (Shop, error) {
	if strings.TrimSpace(id) == "" {
		return Shop{}, errors.New("shop id is required")
	}

	if strings.TrimSpace(ownerID) == "" {
		return Shop{}, errors.New("owner id is required")
	}

	return service.repo.FindByIDAndOwner(ctx, id, ownerID)
}

func (service *Service) ListByOwner(ctx context.Context, ownerID string, page, pageSize int) ([]Shop, int64, error) {
	if strings.TrimSpace(ownerID) == "" {
		return nil, 0, errors.New("owner id is required")
	}

	return service.repo.ListByOwner(ctx, ownerID, page, pageSize)
}

func (service *Service) Update(ctx context.Context, id string, ownerID string, input UpdateShopInput) (Shop, error) {
	shop, err := service.GetByIDAndOwner(ctx, id, ownerID)
	if err != nil {
		return Shop{}, err
	}

	update := bson.M{"updated_at": time.Now().UTC()}

	if input.Name != nil {
		update["name"] = strings.TrimSpace(*input.Name)
	}
	if input.Description != nil {
		update["description"] = strings.TrimSpace(*input.Description)
	}
	if input.IsActive != nil {
		update["is_active"] = *input.IsActive
	}

	if len(update) == 1 {
		return Shop{}, errors.New("no updates provided")
	}

	return service.repo.Update(ctx, shop.ID, update)
}

func (service *Service) Delete(ctx context.Context, id string, ownerID string) error {
	if _, err := service.GetByIDAndOwner(ctx, id, ownerID); err != nil {
		return err
	}

	return service.repo.SoftDelete(ctx, id)
}
