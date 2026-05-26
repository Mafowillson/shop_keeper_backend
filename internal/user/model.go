package user

import (
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type User struct {
	ID bson.ObjectID `bson:"_id,omitempty" json:"id"`

	ShopID bson.ObjectID `bson:"shop_id"         json:"shop_id"`

	Name string `bson:"name"            json:"name"`

	Email string `bson:"email" json:"email"`

	PasswordHash string `bson:"PasswordHash" json:"-"`

	Role string `bson:"role" json:"role"`

	RefreshTokenHash string `bson:"refresh_token_hash,omitempty" json:"-"`

	IsActive bool `bson:"is_active"       json:"is_active"`

	CreatedAt time.Time `bson:"created_at" json:"created_at"`

	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

type PublicUser struct {
	ID        string    `json:"id"`
	ShopID    string    `json:"shop_id,omitempty"`
	Name      string    `json:"name,omitempty"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func ToPublic(u User) PublicUser {
	public := PublicUser{
		ID:        u.ID.Hex(),
		Name:      strings.TrimSpace(u.Name),
		Email:     u.Email,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}

	if !u.ShopID.IsZero() {
		public.ShopID = u.ShopID.Hex()
	}

	return public
}
