package shop

import "time"

type Shop struct {
	ID          string    `bson:"_id,omitempty" json:"id"`
	OwnerID     string    `bson:"owner_id" json:"owner_id"`
	Name        string    `bson:"name" json:"name"`
	Description string    `bson:"description,omitempty" json:"description,omitempty"`
	IsActive    bool      `bson:"is_active" json:"is_active"`
	CreatedAt   time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at" json:"updated_at"`
}

type CreateShopInput struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type UpdateShopInput struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	IsActive    *bool   `json:"is_active,omitempty"`
}
