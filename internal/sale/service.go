package sale

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"shop_keeper_backend/internal/customer"
	"shop_keeper_backend/internal/product"
	"shop_keeper_backend/internal/shop"
	"shop_keeper_backend/internal/staff"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Service struct {
	repo        *Repo
	productRepo *product.Repo
	shopRepo    *shop.Repo
	staffRepo   *staff.Repo
	customerSvc *customer.Service
}

func NewService(repo *Repo, productRepo *product.Repo, shopRepo *shop.Repo, staffRepo *staff.Repo, customerSvc *customer.Service) *Service {
	return &Service{repo: repo, productRepo: productRepo, shopRepo: shopRepo, staffRepo: staffRepo, customerSvc: customerSvc}
}

func (service *Service) validateShopUser(ctx context.Context, shopID string, userID string) error {
	if strings.TrimSpace(shopID) == "" {
		return errors.New("shop id is required")
	}

	s, err := service.shopRepo.FindByID(ctx, shopID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("shop not found")
		}
		return err
	}

	if !s.IsActive {
		return errors.New("shop is not active")
	}

	if strings.TrimSpace(userID) == "" {
		return errors.New("user id is required")
	}

	if s.OwnerID == userID {
		return nil
	}

	st, err := service.staffRepo.FindByID(ctx, userID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("user is not authorized for this shop")
		}
		return err
	}

	if !st.IsActive {
		return errors.New("staff account is inactive")
	}

	if st.ShopID != shopID {
		return errors.New("staff does not belong to this shop")
	}

	return nil
}

func (service *Service) Create(ctx context.Context, userID string, input CreateSaleInput) (Sale, error) {
	if strings.TrimSpace(userID) == "" {
		return Sale{}, errors.New("user id is required")
	}

	if strings.TrimSpace(input.ShopID) == "" {
		return Sale{}, errors.New("shop id is required")
	}

	if err := service.validateShopUser(ctx, input.ShopID, userID); err != nil {
		return Sale{}, err
	}

	if len(input.Items) == 0 {
		return Sale{}, errors.New("sale items are required")
	}

	productSeen := map[string]struct{}{}
	items := make([]SaleItem, 0, len(input.Items))
	totalAmount := 0.0
	updatedProducts := make([]struct {
		id        string
		prevStock int
	}, 0, len(input.Items))

	for _, item := range input.Items {
		productID := strings.TrimSpace(item.ProductID)
		if productID == "" {
			return Sale{}, errors.New("product_id is required for sale item")
		}

		unitName := strings.TrimSpace(item.Unit)
		if unitName == "" {
			return Sale{}, errors.New("unit is required for sale item")
		}

		if item.Quantity <= 0 {
			return Sale{}, errors.New("quantity must be greater than zero")
		}

		if _, found := productSeen[productID]; found {
			return Sale{}, fmt.Errorf("duplicate product %s in sale items", productID)
		}
		productSeen[productID] = struct{}{}

		prod, err := service.productRepo.FindByID(ctx, productID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return Sale{}, fmt.Errorf("product not found: %s", productID)
			}
			return Sale{}, err
		}

		if !prod.IsActive {
			return Sale{}, fmt.Errorf("product %s is not active", productID)
		}

		if prod.ShopID != input.ShopID {
			return Sale{}, fmt.Errorf("product %s does not belong to shop %s", productID, input.ShopID)
		}

		// Resolve the unit the customer is buying.
		unit, found := prod.FindUnit(unitName)
		if !found {
			return Sale{}, fmt.Errorf("unit '%s' is not defined for product '%s'", unitName, prod.Name)
		}

		// Convert the requested quantity into base units to check and deduct stock.
		baseQtyNeeded := item.Quantity * unit.QuantityInBase
		if prod.StockQty < baseQtyNeeded {
			availableInUnit := prod.StockQty / unit.QuantityInBase
			return Sale{}, fmt.Errorf(
				"insufficient stock for '%s': requested %d %s but only %d %s available",
				prod.Name, item.Quantity, unit.Name, availableInUnit, unit.Name,
			)
		}

		newStock := prod.StockQty - baseQtyNeeded
		if _, err := service.productRepo.Update(ctx, productID, bson.M{
			"stock_qty":  newStock,
			"updated_at": time.Now().UTC(),
		}); err != nil {
			// Roll back any stock changes made so far in this sale.
			for _, updated := range updatedProducts {
				_, _ = service.productRepo.Update(ctx, updated.id, bson.M{
					"stock_qty":  updated.prevStock,
					"updated_at": time.Now().UTC(),
				})
			}
			return Sale{}, fmt.Errorf("update stock failed for %s: %w", productID, err)
		}

		updatedProducts = append(updatedProducts, struct {
			id        string
			prevStock int
		}{id: productID, prevStock: prod.StockQty})

		itemTotal := float64(item.Quantity) * unit.Price
		items = append(items, SaleItem{
			ProductID:       productID,
			Unit:            unit.Name,
			Quantity:        item.Quantity,
			UnitPrice:       unit.Price,
			TotalPrice:      itemTotal,
			BaseQtyDeducted: baseQtyNeeded,
		})
		totalAmount += itemTotal
	}

	if input.IsCredit && strings.TrimSpace(input.CustomerID) == "" {
		return Sale{}, errors.New("customer_id is required for credit sales")
	}

	paidAmount := input.PaidAmount
	if paidAmount < 0 {
		return Sale{}, errors.New("paid amount cannot be negative")
	}

	if paidAmount > totalAmount {
		return Sale{}, errors.New("paid amount cannot exceed total amount")
	}

	dueAmount := totalAmount - paidAmount
	if !input.IsCredit && dueAmount > 0 {
		return Sale{}, errors.New("non-credit sale must be paid in full")
	}

	sale := Sale{
		ID:          uuid.NewString(),
		ShopID:      input.ShopID,
		OwnerID:     userID,
		CustomerID:  strings.TrimSpace(input.CustomerID),
		Items:       items,
		TotalAmount: totalAmount,
		PaidAmount:  paidAmount,
		DueAmount:   dueAmount,
		IsCredit:    input.IsCredit,
		IsPaid:      dueAmount == 0,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	created, err := service.repo.Create(ctx, sale)
	if err != nil {
		for _, updated := range updatedProducts {
			_, _ = service.productRepo.Update(ctx, updated.id, bson.M{
				"stock_qty":  updated.prevStock,
				"updated_at": time.Now().UTC(),
			})
		}
		return Sale{}, err
	}

	if input.IsCredit {
		if _, err := service.customerSvc.AddCredit(ctx, sale.CustomerID, sale.ShopID, sale.ID, userID, sale.DueAmount); err != nil {
			_ = service.repo.Delete(ctx, sale.ID)
			for _, updated := range updatedProducts {
				_, _ = service.productRepo.Update(ctx, updated.id, bson.M{
					"stock_qty":  updated.prevStock,
					"updated_at": time.Now().UTC(),
				})
			}
			return Sale{}, err
		}
	}

	return created, nil
}

func (service *Service) GetByIDAndOwner(ctx context.Context, id string, ownerID string) (Sale, error) {
	if strings.TrimSpace(id) == "" {
		return Sale{}, errors.New("sale id is required")
	}

	if strings.TrimSpace(ownerID) == "" {
		return Sale{}, errors.New("owner id is required")
	}

	return service.repo.FindByIDAndOwner(ctx, id, ownerID)
}

func (service *Service) ListByOwner(ctx context.Context, ownerID string, shopID string, page, pageSize int) ([]Sale, int64, error) {
	if strings.TrimSpace(ownerID) == "" {
		return nil, 0, errors.New("owner id is required")
	}

	return service.repo.ListByOwner(ctx, ownerID, shopID, page, pageSize)
}
