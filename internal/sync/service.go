package sync

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"shop_keeper_backend/internal/customer"
	"shop_keeper_backend/internal/product"
	"shop_keeper_backend/internal/sale"
)

type Service struct {
	repo        *Repo
	saleSvc     *sale.Service
	customerSvc *customer.Service
	productSvc  *product.Service
}

func NewService(
	repo *Repo,
	saleSvc *sale.Service,
	customerSvc *customer.Service,
	productSvc *product.Service,
) *Service {
	return &Service{
		repo:        repo,
		saleSvc:     saleSvc,
		customerSvc: customerSvc,
		productSvc:  productSvc,
	}
}

// Push processes a batch of offline sync records.
//
// Contract:
//   - Returns a non-nil error only for infrastructure failures (MongoDB down,
//     network error inside a domain service that isn't a business rule).
//     On infra failure the caller should treat the whole batch as a retry, not
//     a permanent conflict.
//   - Business-rule failures (product not found, insufficient stock, customer
//     not found, validation errors) are recorded in result.Conflicts so the
//     user can review and correct them.
//
// Processing order:
//  1. Sort records oldest-first to preserve causal ordering.
//  2. Skip already-processed records (idempotency via synced_entries).
//  3. Resolve offline temp IDs in each payload (current batch + DB lookup).
//  4. Dispatch to the matching domain service.
//  5. Mark committed records so future retries are no-ops.
func (svc *Service) Push(ctx context.Context, userID string, input PushInput) (PushResult, error) {
	result := PushResult{
		Synced:    make([]string, 0, len(input.Records)),
		Conflicts: make([]string, 0),
		IDMap:     make(map[string]string),
	}

	// Sort by timestamp to honour causal ordering within the batch.
	records := make([]SyncRecord, len(input.Records))
	copy(records, input.Records)
	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp.Before(records[j].Timestamp)
	})

	for _, record := range records {
		// ── Idempotency check ─────────────────────────────────────────────────
		// A non-nil error here means MongoDB is unreachable — abort the whole
		// batch so the client retries later rather than silently mis-classifying
		// every entry as a conflict.
		processed, realID, err := svc.repo.GetProcessed(ctx, record.ID)
		if err != nil {
			return result, fmt.Errorf("idempotency check failed: %w", err)
		}
		if processed {
			result.Synced = append(result.Synced, record.ID)
			tempID := extractTempID(record.ID)
			if tempID != "" && realID != "" {
				result.IDMap[tempID] = realID
			}
			continue
		}

		// ── Resolve offline temp IDs in the payload ───────────────────────────
		payload := svc.resolvePayloadIDs(ctx, record.Payload, result.IDMap)

		// ── Dispatch to the matching domain service ───────────────────────────
		newRealID, err := svc.dispatch(ctx, userID, record.EntityType, record.OperationType, payload)
		if err != nil {
			// Dispatch errors are business-rule failures: mark as conflict and
			// continue processing the rest of the batch.
			result.Conflicts = append(result.Conflicts, record.ID)
			continue
		}

		// ── Mark processed ────────────────────────────────────────────────────
		tempID := extractTempID(record.ID)
		_ = svc.repo.MarkProcessed(ctx, record.ID, tempID, newRealID)
		if tempID != "" && newRealID != "" {
			result.IDMap[tempID] = newRealID
		}
		result.Synced = append(result.Synced, record.ID)
	}

	return result, nil
}

// dispatch routes a single record to the matching domain service.
func (svc *Service) dispatch(
	ctx context.Context,
	userID, entityType, opType string,
	payload map[string]interface{},
) (string, error) {
	switch entityType {
	case "sale":
		if opType == "create" {
			return svc.createSale(ctx, userID, payload)
		}
	case "customer":
		if opType == "create" {
			return svc.createCustomer(ctx, payload)
		}
	case "debtRecord":
		if opType == "payment" {
			return svc.recordPayment(ctx, userID, payload)
		}
	case "product":
		switch opType {
		case "create":
			return svc.createProduct(ctx, userID, payload)
		case "update":
			return svc.updateProduct(ctx, userID, payload)
		case "delete":
			return svc.deleteProduct(ctx, userID, payload)
		}
	}
	// Unknown entity/operation — skip silently.
	return "", nil
}

// ── Domain dispatchers ────────────────────────────────────────────────────────

func (svc *Service) createSale(ctx context.Context, userID string, payload map[string]interface{}) (string, error) {
	input := sale.CreateSaleInput{
		ShopID:     stringField(payload, "shop_id"),
		CustomerID: stringField(payload, "customer_id"),
		PaidAmount: floatField(payload, "paid_amount"),
		IsCredit:   boolField(payload, "is_credit"),
	}
	rawItems, _ := payload["items"].([]interface{})
	for _, ri := range rawItems {
		item, ok := ri.(map[string]interface{})
		if !ok {
			continue
		}
		input.Items = append(input.Items, sale.CreateSaleItemInput{
			ProductID: stringField(item, "product_id"),
			Unit:      stringField(item, "unit"),
			Quantity:  intField(item, "quantity"),
		})
	}
	created, err := svc.saleSvc.Create(ctx, userID, input)
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

func (svc *Service) createCustomer(ctx context.Context, payload map[string]interface{}) (string, error) {
	input := customer.CreateCustomerInput{
		ShopID: stringField(payload, "shop_id"),
		Name:   stringField(payload, "name"),
		Phone:  stringField(payload, "phone"),
	}
	created, err := svc.customerSvc.Create(ctx, input)
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

func (svc *Service) recordPayment(ctx context.Context, userID string, payload map[string]interface{}) (string, error) {
	customerID := stringField(payload, "customer_id")
	if strings.TrimSpace(customerID) == "" {
		return "", fmt.Errorf("customer_id is required for payment")
	}
	// RecordPaymentInput.Amount is a string field; format the float value to match.
	input := customer.RecordPaymentInput{
		Amount: fmt.Sprintf("%g", floatField(payload, "amount")),
		Note:   stringField(payload, "note"),
	}
	record, err := svc.customerSvc.RecordPayment(ctx, customerID, userID, input)
	if err != nil {
		return "", err
	}
	return record.ID, nil
}

func (svc *Service) createProduct(ctx context.Context, userID string, payload map[string]interface{}) (string, error) {
	units, err := parseUnits(payload)
	if err != nil {
		return "", err
	}
	rawStock, _ := payload["initial_stock"].(map[string]interface{})
	initialStock := make(map[string]int, len(rawStock))
	for k, v := range rawStock {
		initialStock[k] = intField(map[string]interface{}{k: v}, k)
	}
	input := product.CreateProductInput{
		ShopID:            stringField(payload, "shop_id"),
		Name:              stringField(payload, "name"),
		Category:          stringField(payload, "category"),
		Units:             units,
		InitialStock:      initialStock,
		LowStockThreshold: intField(payload, "low_stock_threshold"),
	}
	created, err := svc.productSvc.Create(ctx, input, userID)
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

func (svc *Service) updateProduct(ctx context.Context, userID string, payload map[string]interface{}) (string, error) {
	productID := stringField(payload, "product_id")
	if strings.TrimSpace(productID) == "" {
		return "", fmt.Errorf("product_id is required for product update")
	}
	// The client always sends all fields in an update payload, so we set every
	// pointer field unconditionally.
	name := stringField(payload, "name")
	category := stringField(payload, "category")
	threshold := intField(payload, "low_stock_threshold")
	input := product.UpdateProductInput{
		Name:              &name,
		Category:          &category,
		LowStockThreshold: &threshold,
	}
	units, err := parseUnits(payload)
	if err == nil && len(units) > 0 {
		input.Units = units
	}
	// Product update doesn't create a new entity — no id_map entry needed.
	_, err = svc.productSvc.Update(ctx, productID, input, userID)
	return "", err
}

func (svc *Service) deleteProduct(ctx context.Context, userID string, payload map[string]interface{}) (string, error) {
	productID := stringField(payload, "product_id")
	if strings.TrimSpace(productID) == "" {
		return "", fmt.Errorf("product_id is required for product delete")
	}
	return "", svc.productSvc.Delete(ctx, productID, userID)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// resolvePayloadIDs replaces offline_ temp IDs in a payload with real server
// IDs.  It first checks the current-batch idMap, then falls back to the
// synced_entries collection for cross-batch references.  If the DB lookup
// fails, the temp ID is left unresolved — the dispatch will then fail and the
// entry will be marked as a conflict (safer than aborting the whole batch).
func (svc *Service) resolvePayloadIDs(
	ctx context.Context,
	payload map[string]interface{},
	idMap map[string]string,
) map[string]interface{} {
	resolved := make(map[string]interface{}, len(payload))
	for k, v := range payload {
		if s, ok := v.(string); ok && strings.HasPrefix(s, "offline_") {
			if realID, found := idMap[s]; found {
				resolved[k] = realID
				continue
			}
			if realID, _ := svc.repo.GetProcessedByTempID(ctx, s); realID != "" {
				resolved[k] = realID
				continue
			}
		}
		resolved[k] = v
	}
	return resolved
}

// extractTempID derives the offline temp entity ID from a sync log entry ID.
// Entry IDs follow the pattern "sync_offline_<ts>_<rand>"; the temp entity ID
// is the part after the leading "sync_" prefix.
func extractTempID(entryID string) string {
	const prefix = "sync_"
	if !strings.HasPrefix(entryID, prefix) {
		return ""
	}
	candidate := strings.TrimPrefix(entryID, prefix)
	if strings.HasPrefix(candidate, "offline_") {
		return candidate
	}
	return ""
}

// parseUnits extracts the units array from a generic payload map.
func parseUnits(payload map[string]interface{}) ([]product.UnitDefinition, error) {
	rawUnits, ok := payload["units"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("units field is missing or invalid")
	}
	units := make([]product.UnitDefinition, 0, len(rawUnits))
	for _, ru := range rawUnits {
		u, ok := ru.(map[string]interface{})
		if !ok {
			continue
		}
		units = append(units, product.UnitDefinition{
			Name:           stringField(u, "name"),
			QuantityInBase: intField(u, "quantity_in_base"),
			Price:          floatField(u, "price"),
		})
	}
	return units, nil
}

// ── Type-safe payload field extractors ───────────────────────────────────────

func stringField(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

func floatField(m map[string]interface{}, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	}
	return 0
}

func intField(m map[string]interface{}, key string) int {
	switch v := m[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	}
	return 0
}

func boolField(m map[string]interface{}, key string) bool {
	v, _ := m[key].(bool)
	return v
}
