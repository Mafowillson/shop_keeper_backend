package user

import (
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type User struct {
	ID bson.ObjectID `bson:"_id,omitempty" json:"id"`

	ShopID string `bson:"shop_id,omitempty" json:"shop_id,omitempty"`

	Name string `bson:"name" json:"name"`

	Email string `bson:"email" json:"email"`

	PasswordHash string `bson:"PasswordHash" json:"-"`

	Role string `bson:"role" json:"role"`

	RefreshTokenHash string `bson:"refresh_token_hash,omitempty" json:"-"`

	IsActive bool `bson:"is_active" json:"is_active"`

	// Email verification
	EmailVerified          bool      `bson:"email_verified"                    json:"-"`
	VerificationCode       string    `bson:"verification_code,omitempty"        json:"-"`
	VerificationCodeExpiry time.Time `bson:"verification_code_expiry,omitempty" json:"-"`
	VerificationSentAt     time.Time `bson:"verification_sent_at,omitempty"     json:"-"`

	// Password reset
	PasswordResetCode    string    `bson:"password_reset_code,omitempty"    json:"-"`
	PasswordResetExpiry  time.Time `bson:"password_reset_expiry,omitempty"  json:"-"`
	PasswordResetSentAt  time.Time `bson:"password_reset_sent_at,omitempty" json:"-"`

	// Locale preference — "fr" or "en". Used to localise push notifications
	// that are sent outside of a request context (background goroutines).
	PreferredLocale string `bson:"preferred_locale,omitempty" json:"preferred_locale,omitempty"`

	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

type PublicUser struct {
	ID            string    `json:"id"`
	ShopID        string    `json:"shop_id,omitempty"`
	Name          string    `json:"name,omitempty"`
	Email         string    `json:"email"`
	Role          string    `json:"role"`
	EmailVerified bool      `json:"email_verified"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func ToPublic(u User) PublicUser {
	public := PublicUser{
		ID:            u.ID.Hex(),
		Name:          strings.TrimSpace(u.Name),
		Email:         u.Email,
		Role:          u.Role,
		EmailVerified: u.EmailVerified,
		CreatedAt:     u.CreatedAt,
		UpdatedAt:     u.UpdatedAt,
	}
	// Zero ObjectID hex ("000000000000000000000000") means no shop was set —
	// treat it the same as empty so the mobile app redirects to shop creation.
	if u.ShopID != "" && u.ShopID != "000000000000000000000000" {
		public.ShopID = u.ShopID
	}
	return public
}

// ── DTOs ─────────────────────────────────────────────────────────────────────

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

type VerifyEmailInput struct {
	Code string `json:"code"`
}

type ForgotPasswordInput struct {
	Email string `json:"email"`
}

type ResetPasswordInput struct {
	Email       string `json:"email"`
	Code        string `json:"code"`
	NewPassword string `json:"new_password"`
}

type AuthResult struct {
	Token        string     `json:"token"`
	RefreshToken string     `json:"refresh_token"`
	User         PublicUser `json:"user"`
}
