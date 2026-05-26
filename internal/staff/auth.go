package staff

import (
	"context"
	"fmt"
	"shop_keeper_backend/internal/auth"
	"strings"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

type AuthService struct {
	repo             *Repo
	jwtSecret        string
	jwtRefreshSecret string
}

func NewAuthService(repo *Repo, jwtSecret string, jwtRefreshSecret string) *AuthService {
	return &AuthService{
		repo:             repo,
		jwtSecret:        jwtSecret,
		jwtRefreshSecret: jwtRefreshSecret,
	}
}

type StaffLoginInput struct {
	Email       string `json:"email"`
	PhoneNumber string `json:"phone_number"`
}

type StaffAuthResult struct {
	Token        string      `json:"token"`
	RefreshToken string      `json:"refresh_token"`
	Staff        PublicStaff `json:"staff"`
}

func (as *AuthService) Login(ctx context.Context, input StaffLoginInput) (StaffAuthResult, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	phoneNumber := strings.TrimSpace(input.PhoneNumber)

	if email == "" {
		return StaffAuthResult{}, fmt.Errorf("email is required")
	}

	if phoneNumber == "" {
		return StaffAuthResult{}, fmt.Errorf("phone number is required")
	}

	staff, err := as.repo.FindByEmail(ctx, email)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return StaffAuthResult{}, fmt.Errorf("invalid credentials")
		}
		return StaffAuthResult{}, err
	}

	if !staff.IsActive {
		return StaffAuthResult{}, fmt.Errorf("staff account is inactive")
	}

	service := NewService(as.repo)
	if err := service.authenticatePassword(phoneNumber, staff.PasswordHash); err != nil {
		return StaffAuthResult{}, fmt.Errorf("invalid credentials")
	}

	accessToken, err := auth.CreateAccessToken(as.jwtSecret, staff.ID, "staff")
	if err != nil {
		return StaffAuthResult{}, err
	}

	refreshToken, err := auth.CreateRefreshToken(as.jwtRefreshSecret, staff.ID, "staff")
	if err != nil {
		return StaffAuthResult{}, err
	}

	return StaffAuthResult{
		Token:        accessToken,
		RefreshToken: refreshToken,
		Staff:        ToPublicStaff(staff),
	}, nil
}
