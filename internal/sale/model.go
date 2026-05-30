package sale

import "time"

type SaleItem struct {
	ProductID  string  `bson:"product_id" json:"product_id"`
	// Unit is which unit the customer bought (e.g. "carton", "roll", "packet").
	Unit       string  `bson:"unit" json:"unit"`
	Quantity   int     `bson:"quantity" json:"quantity"`
	UnitPrice  float64 `bson:"unit_price" json:"unit_price"`
	TotalPrice float64 `bson:"total_price" json:"total_price"`
	// BaseQtyDeducted is Quantity × unit.QuantityInBase — the number of base units
	// removed from stock. Stored for audit and restock calculations.
	BaseQtyDeducted int `bson:"base_qty_deducted" json:"base_qty_deducted"`
}

type Sale struct {
	ID          string     `bson:"_id,omitempty" json:"id"`
	ShopID      string     `bson:"shop_id" json:"shop_id"`
	OwnerID     string     `bson:"owner_id" json:"owner_id"`
	CustomerID  string     `bson:"customer_id,omitempty" json:"customer_id,omitempty"`
	Items       []SaleItem `bson:"items" json:"items"`
	TotalAmount float64    `bson:"total_amount" json:"total_amount"`
	PaidAmount  float64    `bson:"paid_amount" json:"paid_amount"`
	DueAmount   float64    `bson:"due_amount" json:"due_amount"`
	IsCredit    bool       `bson:"is_credit" json:"is_credit"`
	IsPaid      bool       `bson:"is_paid" json:"is_paid"`
	CreatedAt   time.Time  `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time  `bson:"updated_at" json:"updated_at"`
}

type CreateSaleItemInput struct {
	ProductID string `json:"product_id"`
	// Unit must match one of the product's defined unit names (case-insensitive).
	Unit      string `json:"unit"`
	Quantity  int    `json:"quantity"`
}

type CreateSaleInput struct {
	ShopID     string                `json:"shop_id"`
	CustomerID string                `json:"customer_id,omitempty"`
	Items      []CreateSaleItemInput `json:"items"`
	PaidAmount float64               `json:"paid_amount,omitempty"`
	IsCredit   bool                  `json:"is_credit,omitempty"`
}
