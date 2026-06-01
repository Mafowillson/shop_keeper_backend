package user

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
	"shop_keeper_backend/internal/auth"
	"shop_keeper_backend/internal/email"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo         *Repo
	emailSvc     *email.Service
	jwtSecret        string
	jwtRefreshSecret string
}

func NewService(repo *Repo, emailSvc *email.Service, jwtSecret string, jwtRefreshSecret string) *Service {
	return &Service{repo: repo, emailSvc: emailSvc, jwtSecret: jwtSecret, jwtRefreshSecret: jwtRefreshSecret}
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

	code, expiry := service.generateCode()

	now := time.Now().UTC()
	u := User{
		Email:                  email,
		Name:                   strings.TrimSpace(input.Name),
		PasswordHash:           string(hashBytes),
		Role:                   "owner",
		EmailVerified:          false,
		VerificationCode:       code,
		VerificationCodeExpiry: expiry,
		VerificationSentAt:     now,
		CreatedAt:              now,
		UpdatedAt:              now,
	}

	created, err := service.repo.Create(ctx, u)
	if err != nil {
		return AuthResult{}, err
	}

	// Send verification email. Non-fatal — account is created regardless.
	if sendErr := service.emailSvc.SendVerificationCode(created.Email, created.Name, code); sendErr != nil {
		fmt.Printf("[email] Failed to send verification code to %s: %v\n", created.Email, sendErr)
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

// VerifyEmail validates the OTP code and marks the user's email as verified.
func (service *Service) VerifyEmail(ctx context.Context, userID string, input VerifyEmailInput) (AuthResult, error) {
	code := strings.TrimSpace(input.Code)
	if len(code) != 6 {
		return AuthResult{}, errors.New("verification code must be 6 digits")
	}

	u, err := service.repo.FindByID(ctx, userID)
	if err != nil {
		return AuthResult{}, errors.New("user not found")
	}

	if u.EmailVerified {
		// Idempotent — already verified, return success.
		return service.createSession(ctx, u)
	}

	if u.VerificationCode == "" {
		return AuthResult{}, errors.New("no verification code found — request a new one")
	}

	if time.Now().UTC().After(u.VerificationCodeExpiry) {
		return AuthResult{}, errors.New("verification code has expired — request a new one")
	}

	if u.VerificationCode != code {
		return AuthResult{}, errors.New("incorrect verification code")
	}

	if err := service.repo.MarkEmailVerified(ctx, userID); err != nil {
		return AuthResult{}, err
	}

	u.EmailVerified = true
	return service.createSession(ctx, u)
}

// ResendVerificationCode generates a fresh code and re-sends the email.
// Rate-limited to one resend per 60 seconds.
func (service *Service) ResendVerificationCode(ctx context.Context, userID string) error {
	u, err := service.repo.FindByID(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}

	if u.EmailVerified {
		return errors.New("email is already verified")
	}

	if !u.VerificationSentAt.IsZero() && time.Since(u.VerificationSentAt) < 60*time.Second {
		remaining := 60 - int(time.Since(u.VerificationSentAt).Seconds())
		return fmt.Errorf("please wait %d seconds before requesting a new code", remaining)
	}

	code, expiry := service.generateCode()

	if err := service.repo.SaveVerificationCode(ctx, userID, code, expiry); err != nil {
		return err
	}

	if sendErr := service.emailSvc.SendVerificationCode(u.Email, u.Name, code); sendErr != nil {
		fmt.Printf("[email] Failed to resend verification code to %s: %v\n", u.Email, sendErr)
	}

	return nil
}

// ForgotPassword generates a reset code and emails it to the user.
// Always returns nil regardless of whether the email exists — this prevents
// email enumeration attacks (attacker can't tell if an account exists).
func (service *Service) ForgotPassword(ctx context.Context, input ForgotPasswordInput) error {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" {
		return errors.New("email is required")
	}

	u, err := service.repo.FindByEmail(ctx, email)
	if err != nil {
		// Silently succeed — do not reveal whether the email is registered.
		return nil
	}

	// Rate-limit: one code per 60 seconds.
	if !u.PasswordResetSentAt.IsZero() && time.Since(u.PasswordResetSentAt) < 60*time.Second {
		return nil // Silently ignore to avoid timing side-channel.
	}

	code, expiry := service.generateCode()

	if err := service.repo.SavePasswordResetCode(ctx, u.ID.Hex(), code, expiry); err != nil {
		return err
	}

	if sendErr := service.emailSvc.SendPasswordResetCode(u.Email, u.Name, code); sendErr != nil {
		fmt.Printf("[email] Failed to send password reset to %s: %v\n", u.Email, sendErr)
	}

	return nil
}

// ResetPassword validates the code and updates the user's password.
// On success all active sessions are invalidated so the old password stops working.
func (service *Service) ResetPassword(ctx context.Context, input ResetPasswordInput) error {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	code := strings.TrimSpace(input.Code)
	newPass := strings.TrimSpace(input.NewPassword)

	if email == "" {
		return errors.New("email is required")
	}
	if len(code) != 6 {
		return errors.New("reset code must be 6 digits")
	}
	if len(newPass) < 6 {
		return errors.New("password must be at least 6 characters")
	}

	u, err := service.repo.FindByEmail(ctx, email)
	if err != nil {
		return errors.New("invalid reset code")
	}

	if u.PasswordResetCode == "" {
		return errors.New("no password reset was requested for this account")
	}

	if time.Now().UTC().After(u.PasswordResetExpiry) {
		return errors.New("reset code has expired — request a new one")
	}

	if u.PasswordResetCode != code {
		return errors.New("invalid reset code")
	}

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(newPass), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	return service.repo.ResetPassword(ctx, u.ID.Hex(), string(hashBytes))
}

// ── Private helpers ───────────────────────────────────────────────────────────

func (service *Service) generateCode() (code string, expiry time.Time) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		// Fallback (should never happen on supported platforms).
		n = big.NewInt(123456)
	}
	code = fmt.Sprintf("%06d", n.Int64())
	expiry = time.Now().UTC().Add(15 * time.Minute)
	return
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
