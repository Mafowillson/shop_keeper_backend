package user

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"shop_keeper_backend/internal/auth"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo *Repo

	jwtSecret        string
	jwtRefreshSecret string
}

func NewService(repo *Repo, jwtSecret string, jwtRefreshSecret string) *Service {
	return &Service{repo: repo, jwtSecret: jwtSecret, jwtRefreshSecret: jwtRefreshSecret}
}

type RegisterInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name,omitempty"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	DeviceID string `json:"device_id,omitempty"`
}

type RefreshInput struct {
	RefreshToken string `json:"refresh_token"`
}

type LogoutInput struct {
	RefreshToken string `json:"refresh_token"`
}

type AuthResult struct {
	Token        string     `json:"token"`
	RefreshToken string     `json:"refresh_token"`
	User         PublicUser `json:"user"`
}

func (service *Service) Register(ctx context.Context, input RegisterInput) (AuthResult, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	pass := strings.TrimSpace(input.Password)

	if email == "" || pass == "" {
		return AuthResult{}, errors.New("email and password are required")
	}

	if len(pass) < 6 {
		return AuthResult{}, errors.New("Password must be atleast 6 characters long")
	}

	_, err := service.repo.FindByEmail(ctx, email)
	if err == nil {
		return AuthResult{}, errors.New("Email is already registered! try using another email")
	}

	if !errors.Is(err, mongo.ErrNoDocuments) {
		return AuthResult{}, err
	}

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return AuthResult{}, fmt.Errorf("Hashing password failed: %w", err)
	}

	now := time.Now().UTC()

	u := User{
		Email:        email,
		Name:         strings.TrimSpace(input.Name),
		PasswordHash: string(hashBytes),
		Role:         "owner",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	created, err := service.repo.Create(ctx, u)
	if err != nil {
		return AuthResult{}, err
	}

	return service.createSession(ctx, created)
}

func (service *Service) Login(ctx context.Context, input LoginInput) (AuthResult, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	pass := strings.TrimSpace(input.Password)

	if email == "" || pass == "" {
		return AuthResult{}, errors.New("email and password are required")
	}

	if len(pass) < 6 {
		return AuthResult{}, errors.New("Password must be atleast 6 characters long")
	}

	u, err := service.repo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return AuthResult{}, errors.New("Invalid Credentials!")
		}
		return AuthResult{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(pass)); err != nil {
		return AuthResult{}, errors.New("Invalid credentials or wrong password!")
	}

	return service.createSession(ctx, u)
}

func (service *Service) Refresh(ctx context.Context, input RefreshInput) (AuthResult, error) {
	refreshToken := strings.TrimSpace(input.RefreshToken)
	if refreshToken == "" {
		return AuthResult{}, errors.New("refresh token is required")
	}

	claims, err := auth.ParseToken(service.jwtRefreshSecret, refreshToken)
	if err != nil {
		return AuthResult{}, errors.New("invalid refresh token")
	}

	u, err := service.repo.FindByID(ctx, claims.Subject)
	if err != nil {
		return AuthResult{}, errors.New("invalid refresh token")
	}

	if u.RefreshTokenHash == "" {
		return AuthResult{}, errors.New("refresh token is invalid")
	}

	if err := service.compareRefreshTokenHash(u.RefreshTokenHash, refreshToken); err != nil {
		return AuthResult{}, errors.New("invalid refresh token")
	}

	return service.createSession(ctx, u)
}

func (service *Service) Logout(ctx context.Context, input LogoutInput) error {
	refreshToken := strings.TrimSpace(input.RefreshToken)
	if refreshToken == "" {
		return errors.New("refresh token is required")
	}

	claims, err := auth.ParseToken(service.jwtRefreshSecret, refreshToken)
	if err != nil {
		return errors.New("invalid refresh token")
	}

	u, err := service.repo.FindByID(ctx, claims.Subject)
	if err != nil {
		return errors.New("invalid refresh token")
	}

	if u.RefreshTokenHash == "" {
		return errors.New("refresh token is invalid")
	}

	if err := service.compareRefreshTokenHash(u.RefreshTokenHash, refreshToken); err != nil {
		return errors.New("invalid refresh token")
	}

	return service.repo.ClearRefreshToken(ctx, u.ID.Hex())
}

func hashRefreshTokenToken(refreshToken string) []byte {
	digest := sha256.Sum256([]byte(refreshToken))
	return digest[:]
}

func (service *Service) hashRefreshToken(refreshToken string) ([]byte, error) {
	hashBytes, err := bcrypt.GenerateFromPassword(hashRefreshTokenToken(refreshToken), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing refresh token failed: %w", err)
	}
	return hashBytes, nil
}

func (service *Service) compareRefreshTokenHash(storedHash, refreshToken string) error {
	return bcrypt.CompareHashAndPassword([]byte(storedHash), hashRefreshTokenToken(refreshToken))
}

func (service *Service) createSession(ctx context.Context, user User) (AuthResult, error) {
	token, err := auth.CreateAccessToken(service.jwtSecret, user.ID.Hex(), user.Role)
	if err != nil {
		return AuthResult{}, err
	}

	refreshToken, err := auth.CreateRefreshToken(service.jwtRefreshSecret, user.ID.Hex(), user.Role)
	if err != nil {
		return AuthResult{}, err
	}

	hashBytes, err := service.hashRefreshToken(refreshToken)
	if err != nil {
		return AuthResult{}, err
	}

	if err := service.repo.UpdateRefreshToken(ctx, user.ID.Hex(), string(hashBytes)); err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		Token:        token,
		RefreshToken: refreshToken,
		User:         ToPublic(user),
	}, nil
}
