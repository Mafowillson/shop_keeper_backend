package sale

import "time"

type SaleItem struct {
	ProductID  string  `bson:"product_id" json:"product_id"`
	Quantity   int     `bson:"quantity" json:"quantity"`
	UnitPrice  float64 `bson:"unit_price" json:"unit_price"`
	TotalPrice float64 `bson:"total_price" json:"total_price"`
}

type Sale struct {
	ID          string     `bson:"_id,omitempty" json:"id"`
	ShopID      string     `bson:"shop_id" json:"shop_id"`
	OwnerID     string     `bson:"owner_id" json:"owner_id"`
	Items       []SaleItem `bson:"items" json:"items"`
	TotalAmount float64    `bson:"total_amount" json:"total_amount"`
	PaidAmount  float64    `bson:"paid_amount" json:"paid_amount"`
	DueAmount   float64    `bson:"due_amount" json:"due_amount"`
	IsPaid      bool       `bson:"is_paid" json:"is_paid"`
	CreatedAt   time.Time  `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time  `bson:"updated_at" json:"updated_at"`
}

type CreateSaleItemInput struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

type CreateSaleInput struct {
	ShopID     string                `json:"shop_id"`
	Items      []CreateSaleItemInput `json:"items"`
	PaidAmount float64               `json:"paid_amount,omitempty"`
}
