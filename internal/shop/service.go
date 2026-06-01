package shop

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"shop_keeper_backend/internal/validation"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// UserUpdater lets the shop service persist the shop_id back to the owner record.
type UserUpdater interface {
	UpdateShopID(ctx context.Context, ownerID, shopID string) error
}

type Service struct {
	repo        *Repo
	userUpdater UserUpdater
}

func NewService(repo *Repo, userUpdater UserUpdater) *Service {
	return &Service{repo: repo, userUpdater: userUpdater}
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

	created, err := service.repo.Create(ctx, shop)
	if err != nil {
		return Shop{}, err
	}

	// Persist the shop_id back onto the owner's user record so login responses
	// include it without a separate lookup.
	if updateErr := service.userUpdater.UpdateShopID(ctx, ownerID, created.ID); updateErr != nil {
		fmt.Printf("[shop] failed to update owner shop_id: %v\n", updateErr)
	}

	return created, nil
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
