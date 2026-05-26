package product

import "time"

type Product struct {
	ID                string    `bson:"_id,omitempty" json:"id"`
	ShopID            string    `bson:"shop_id" json:"shop_id"`
	Name              string    `bson:"name" json:"name"`
	Category          string    `bson:"category" json:"category"`
	RetailPrice       float64   `bson:"retail_price" json:"retail_price"`
	CartonPrice       float64   `bson:"carton_price" json:"carton_price"`
	CartonQty         int       `bson:"carton_qty" json:"carton_qty"`
	StockQty          int       `bson:"stock_qty" json:"stock_qty"`
	LowStockThreshold int       `bson:"low_stock_threshold" json:"low_stock_threshold"`
	IsActive          bool      `bson:"is_active" json:"is_active"`
	CreatedAt         time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt         time.Time `bson:"updated_at" json:"updated_at"`
}

type CreateProductInput struct {
	ShopID            string  `json:"shop_id"`
	Name              string  `json:"name"`
	Category          string  `json:"category"`
	RetailPrice       float64 `json:"retail_price"`
	CartonPrice       float64 `json:"carton_price"`
	CartonQty         int     `json:"carton_qty"`
	StockQty          int     `json:"stock_qty"`
	LowStockThreshold int     `json:"low_stock_threshold"`
}

type UpdateProductInput struct {
	Name              *string  `json:"name,omitempty"`
	Category          *string  `json:"category,omitempty"`
	RetailPrice       *float64 `json:"retail_price,omitempty"`
	CartonPrice       *float64 `json:"carton_price,omitempty"`
	CartonQty         *int     `json:"carton_qty,omitempty"`
	StockQty          *int     `json:"stock_qty,omitempty"`
	LowStockThreshold *int     `json:"low_stock_threshold,omitempty"`
	IsActive          *bool    `json:"is_active,omitempty"`
}

type SyncProductItem struct {
	ID                string    `json:"id"`
	ShopID            string    `json:"shop_id"`
	Name              string    `json:"name"`
	Category          string    `json:"category"`
	RetailPrice       float64   `json:"retail_price"`
	CartonPrice       float64   `json:"carton_price"`
	CartonQty         int       `json:"carton_qty"`
	StockQty          int       `json:"stock_qty"`
	LowStockThreshold int       `json:"low_stock_threshold"`
	IsActive          bool      `json:"is_active"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type SyncProductsInput struct {
	Products []SyncProductItem `json:"products"`
}
