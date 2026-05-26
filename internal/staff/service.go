package staff

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

func (service *Service) authenticatePassword(password string, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func (service *Service) Create(ctx context.Context, ownerID string, input CreateStaffInput) (Staff, error) {
	if strings.TrimSpace(ownerID) == "" {
		return Staff{}, errors.New("owner id is required")
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Staff{}, errors.New("staff name is required")
	}

	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" {
		return Staff{}, errors.New("staff email is required")
	}

	phoneNumber := strings.TrimSpace(input.PhoneNumber)
	if phoneNumber == "" {
		return Staff{}, errors.New("staff phone number is required")
	}

	// Check if email already exists
	_, err := service.repo.FindByEmail(ctx, email)
	if err == nil {
		return Staff{}, errors.New("email already in use")
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return Staff{}, err
	}

	// Hash phone number as password
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(phoneNumber), bcrypt.DefaultCost)
	if err != nil {
		return Staff{}, err
	}

	now := time.Now().UTC()
	staff := Staff{
		ID:           uuid.NewString(),
		OwnerID:      ownerID,
		ShopID:       strings.TrimSpace(input.ShopID),
		Name:         name,
		Email:        email,
		PhoneNumber:  phoneNumber,
		PasswordHash: string(hashBytes),
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	return service.repo.Create(ctx, staff)
}

func (service *Service) GetByIDAndOwner(ctx context.Context, id string, ownerID string) (Staff, error) {
	if strings.TrimSpace(id) == "" {
		return Staff{}, errors.New("staff id is required")
	}

	if strings.TrimSpace(ownerID) == "" {
		return Staff{}, errors.New("owner id is required")
	}

	return service.repo.FindByIDAndOwner(ctx, id, ownerID)
}

func (service *Service) ListByOwner(ctx context.Context, ownerID string) ([]Staff, error) {
	if strings.TrimSpace(ownerID) == "" {
		return nil, errors.New("owner id is required")
	}

	return service.repo.ListByOwner(ctx, ownerID)
}

func (service *Service) ListByShop(ctx context.Context, shopID string) ([]Staff, error) {
	if strings.TrimSpace(shopID) == "" {
		return nil, errors.New("shop id is required")
	}

	return service.repo.ListByShop(ctx, shopID)
}

func (service *Service) Update(ctx context.Context, id string, ownerID string, input UpdateStaffInput) (Staff, error) {
	staff, err := service.GetByIDAndOwner(ctx, id, ownerID)
	if err != nil {
		return Staff{}, err
	}

	update := bson.M{"updated_at": time.Now().UTC()}

	if input.Name != nil {
		update["name"] = strings.TrimSpace(*input.Name)
	}
	if input.Email != nil {
		newEmail := strings.ToLower(strings.TrimSpace(*input.Email))
		if newEmail != staff.Email {
			// Check if new email already exists
			_, err := service.repo.FindByEmail(ctx, newEmail)
			if err == nil {
				return Staff{}, errors.New("email already in use")
			}
			if !errors.Is(err, mongo.ErrNoDocuments) {
				return Staff{}, err
			}
		}
		update["email"] = newEmail
	}
	if input.PhoneNumber != nil {
		phoneNumber := strings.TrimSpace(*input.PhoneNumber)
		hashBytes, err := bcrypt.GenerateFromPassword([]byte(phoneNumber), bcrypt.DefaultCost)
		if err != nil {
			return Staff{}, err
		}
		update["phone_number"] = phoneNumber
		update["password_hash"] = string(hashBytes)
	}
	if input.ShopID != nil {
		update["shop_id"] = strings.TrimSpace(*input.ShopID)
	}
	if input.IsActive != nil {
		update["is_active"] = *input.IsActive
	}

	if len(update) == 1 {
		return Staff{}, errors.New("no updates provided")
	}

	return service.repo.Update(ctx, staff.ID, update)
}

func (service *Service) Delete(ctx context.Context, id string, ownerID string) error {
	if _, err := service.GetByIDAndOwner(ctx, id, ownerID); err != nil {
		return err
	}

	return service.repo.SoftDelete(ctx, id)
}

func (service *Service) GetCredentials(ctx context.Context, id string, ownerID string) (StaffCredentials, error) {
	staff, err := service.GetByIDAndOwner(ctx, id, ownerID)
	if err != nil {
		return StaffCredentials{}, err
	}

	return StaffCredentials{
		Email:       staff.Email,
		PhoneNumber: staff.PhoneNumber,
	}, nil
}

func (service *Service) AuthenticateStaff(ctx context.Context, email string, phoneNumber string) (Staff, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return Staff{}, errors.New("email is required")
	}
	if strings.TrimSpace(phoneNumber) == "" {
		return Staff{}, errors.New("phone number is required")
	}

	staff, err := service.repo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return Staff{}, errors.New("invalid credentials")
		}
		return Staff{}, err
	}

	if !staff.IsActive {
		return Staff{}, errors.New("staff account is inactive")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(staff.PasswordHash), []byte(phoneNumber)); err != nil {
		return Staff{}, errors.New("invalid credentials")
	}

	return staff, nil
}
