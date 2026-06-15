package customer

import (
	"context"
	"time"
)

// Risk level constants used by AI-2 and returned in API responses.
const (
	RiskLevelLow               = "low"
	RiskLevelMedium            = "medium"
	RiskLevelHigh              = "high"
	RiskLevelInsufficientHistory = "insufficient_history"
)

// RiskScorer is implemented by the AI-2 riskscoring service.
// Defined here to avoid an import cycle between the customer and riskscoring packages.
type RiskScorer interface {
	Calculate(ctx context.Context, customerID string) error
}

// Customer represents a credit customer for a shop.
type Customer struct {
	ID        string    `bson:"_id,omitempty" json:"id"`
	ShopID    string    `bson:"shop_id" json:"shop_id"`
	Name      string    `bson:"name" json:"name"`
	Phone     string    `bson:"phone,omitempty" json:"phone,omitempty"`
	TotalDebt float64   `bson:"total_debt" json:"total_debt"`
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`

	// AI-2: risk scoring fields — updated after every debt or payment event.
	RiskScore        int        `bson:"risk_score" json:"risk_score"`
	RiskLevel        string     `bson:"risk_level" json:"risk_level"`
	RiskCalculatedAt *time.Time `bson:"risk_calculated_at,omitempty" json:"risk_calculated_at,omitempty"`
}

// DebtRecord represents a credit or payment event for a customer.
type DebtRecord struct {
	ID           string    `bson:"_id,omitempty" json:"id"`
	CustomerID   string    `bson:"customer_id" json:"customer_id"`
	ShopID       string    `bson:"shop_id" json:"shop_id"`
	SaleID       string    `bson:"sale_id,omitempty" json:"sale_id,omitempty"`
	Type         string    `bson:"type" json:"type"` // "credit" or "payment"
	Amount       float64   `bson:"amount" json:"amount"`
	BalanceAfter float64   `bson:"balance_after" json:"balance_after"`
	Note         string    `bson:"note,omitempty" json:"note,omitempty"`
	RecordedBy   string    `bson:"recorded_by" json:"recorded_by"`
	RecordedAt   time.Time `bson:"recorded_at" json:"recorded_at"`
	// AI-2: true when this credit was approved despite a High-risk score.
	HighRiskOverride bool `bson:"high_risk_override,omitempty" json:"high_risk_override,omitempty"`
}

// CreateCustomerInput is the request body for creating a customer.
type CreateCustomerInput struct {
	ShopID string `json:"shop_id"`
	Name   string `json:"name"`
	Phone  string `json:"phone,omitempty"`
}

// RecordPaymentInput is the request body for recording a debt payment.
type RecordPaymentInput struct {
	Amount string `json:"amount"`
	Note   string `json:"note,omitempty"`
}

// CustomerWithDebt includes customer info and current debt balance.
type CustomerWithDebt struct {
	ID        string  `json:"id"`
	ShopID    string  `json:"shop_id"`
	Name      string  `json:"name"`
	Phone     string  `json:"phone,omitempty"`
	TotalDebt float64 `json:"total_debt"`
}
