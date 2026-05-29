package customer

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"shop_keeper_backend/internal/validation"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// Create creates a new customer for a shop.
func (svc *Service) Create(ctx context.Context, input CreateCustomerInput) (Customer, error) {
	if err := validation.ValidateUUID(input.ShopID, "shop_id"); err != nil {
		return Customer{}, err
	}

	if err := validation.ValidateString(input.Name, "customer name", 3, 100); err != nil {
		return Customer{}, err
	}

	if strings.TrimSpace(input.Phone) != "" {
		if err := validation.ValidatePhone(input.Phone); err != nil {
			return Customer{}, err
		}
	}

	now := time.Now().UTC()
	customer := Customer{
		ID:        uuid.NewString(),
		ShopID:    input.ShopID,
		Name:      strings.TrimSpace(input.Name),
		Phone:     strings.TrimSpace(input.Phone),
		TotalDebt: 0,
		CreatedAt: now,
		UpdatedAt: now,
	}

	return svc.repo.CreateCustomer(ctx, customer)
}

// GetByID retrieves a customer by ID.
func (svc *Service) GetByID(ctx context.Context, id string) (Customer, error) {
	if strings.TrimSpace(id) == "" {
		return Customer{}, errors.New("customer id is required")
	}
	return svc.repo.GetCustomerByID(ctx, id)
}

// List retrieves all customers for a shop, optionally filtered by debt status.
func (svc *Service) List(ctx context.Context, shopID string, hasDebt *bool, page, pageSize int) ([]Customer, int64, error) {
	if err := validation.ValidateUUID(shopID, "shop_id"); err != nil {
		return nil, 0, err
	}
	return svc.repo.ListCustomersByShop(ctx, shopID, hasDebt, page, pageSize)
}

// RecordPayment records a debt payment for a customer.
func (svc *Service) RecordPayment(ctx context.Context, customerID, recordedBy string, input RecordPaymentInput) (DebtRecord, error) {
	if strings.TrimSpace(customerID) == "" {
		return DebtRecord{}, errors.New("customer_id is required")
	}
	if strings.TrimSpace(recordedBy) == "" {
		return DebtRecord{}, errors.New("recorded_by (user id) is required")
	}

	amount, err := strconv.ParseFloat(strings.TrimSpace(input.Amount), 64)
	if err != nil || amount <= 0 {
		return DebtRecord{}, errors.New("amount must be a positive number")
	}

	customer, err := svc.repo.GetCustomerByID(ctx, customerID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return DebtRecord{}, errors.New("customer not found")
		}
		return DebtRecord{}, err
	}

	// Calculate new debt balance (payment reduces debt)
	newDebt := customer.TotalDebt - amount
	if newDebt < 0 {
		newDebt = 0
	}

	// Create debt record
	record := DebtRecord{
		ID:           uuid.NewString(),
		CustomerID:   customerID,
		ShopID:       customer.ShopID,
		Type:         "payment",
		Amount:       amount,
		BalanceAfter: newDebt,
		Note:         strings.TrimSpace(input.Note),
		RecordedBy:   recordedBy,
		RecordedAt:   time.Now().UTC(),
	}

	if _, err := svc.repo.CreateDebtRecord(ctx, record); err != nil {
		return DebtRecord{}, err
	}

	// Update customer's total debt
	if err := svc.repo.UpdateCustomerDebt(ctx, customerID, newDebt); err != nil {
		return DebtRecord{}, err
	}

	return record, nil
}

// GetDebtHistory retrieves the audit trail of all debt events for a customer.
func (svc *Service) GetDebtHistory(ctx context.Context, customerID string) ([]DebtRecord, error) {
	if strings.TrimSpace(customerID) == "" {
		return nil, errors.New("customer_id is required")
	}
	return svc.repo.GetDebtHistoryByCustomer(ctx, customerID)
}

// AddCredit adds a credit debt record when a sale is marked as credit.
// This is typically called from the sales service.
func (svc *Service) AddCredit(ctx context.Context, customerID, shopID, saleID, recordedBy string, creditAmount float64) (DebtRecord, error) {
	if creditAmount <= 0 {
		return DebtRecord{}, errors.New("credit amount must be positive")
	}

	customer, err := svc.repo.GetCustomerByID(ctx, customerID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return DebtRecord{}, errors.New("customer not found")
		}
		return DebtRecord{}, err
	}

	newDebt := customer.TotalDebt + creditAmount

	record := DebtRecord{
		ID:           uuid.NewString(),
		CustomerID:   customerID,
		ShopID:       shopID,
		SaleID:       saleID,
		Type:         "credit",
		Amount:       creditAmount,
		BalanceAfter: newDebt,
		RecordedBy:   recordedBy,
		RecordedAt:   time.Now().UTC(),
	}

	if _, err := svc.repo.CreateDebtRecord(ctx, record); err != nil {
		return DebtRecord{}, err
	}

	if err := svc.repo.UpdateCustomerDebt(ctx, customerID, newDebt); err != nil {
		return DebtRecord{}, err
	}

	return record, nil
}
