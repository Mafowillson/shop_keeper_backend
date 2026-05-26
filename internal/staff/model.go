package staff

import "time"

type Staff struct {
	ID           string    `bson:"_id,omitempty" json:"id"`
	OwnerID      string    `bson:"owner_id" json:"owner_id"`
	ShopID       string    `bson:"shop_id,omitempty" json:"shop_id,omitempty"`
	Name         string    `bson:"name" json:"name"`
	Email        string    `bson:"email" json:"email"`
	PhoneNumber  string    `bson:"phone_number" json:"phone_number"`
	PasswordHash string    `bson:"password_hash" json:"-"`
	IsActive     bool      `bson:"is_active" json:"is_active"`
	CreatedAt    time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time `bson:"updated_at" json:"updated_at"`
}

type CreateStaffInput struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	PhoneNumber string `json:"phone_number"`
	ShopID      string `json:"shop_id,omitempty"`
}

type UpdateStaffInput struct {
	Name        *string `json:"name,omitempty"`
	Email       *string `json:"email,omitempty"`
	PhoneNumber *string `json:"phone_number,omitempty"`
	ShopID      *string `json:"shop_id,omitempty"`
	IsActive    *bool   `json:"is_active,omitempty"`
}

type StaffCredentials struct {
	Email       string `json:"email"`
	PhoneNumber string `json:"phone_number"`
}

type PublicStaff struct {
	ID          string    `json:"id"`
	OwnerID     string    `json:"owner_id"`
	ShopID      string    `json:"shop_id,omitempty"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	PhoneNumber string    `json:"phone_number"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func ToPublicStaff(s Staff) PublicStaff {
	return PublicStaff{
		ID:          s.ID,
		OwnerID:     s.OwnerID,
		ShopID:      s.ShopID,
		Name:        s.Name,
		Email:       s.Email,
		PhoneNumber: s.PhoneNumber,
		IsActive:    s.IsActive,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}
