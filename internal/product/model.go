package product

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// UnitDefinition describes one way a product can be sold.
// A product can have 1–5 units ordered largest → smallest.
//
// Example for sugar:
//
//	{ name: "carton", quantity_in_base: 25, price: 10000 }
//	{ name: "roll",   quantity_in_base: 5,  price: 2200  }
//	{ name: "packet", quantity_in_base: 1,  price: 500   }  ← base unit
type UnitDefinition struct {
	Name           string  `bson:"name" json:"name"`
	QuantityInBase int     `bson:"quantity_in_base" json:"quantity_in_base"`
	Price          float64 `bson:"price" json:"price"`
}

// StockLevel is a computed field returned in API responses — never stored in MongoDB.
type StockLevel struct {
	Unit           string `json:"unit"`
	QuantityInBase int    `json:"quantity_in_base"`
	Available      int    `json:"available"` // floor(stock_qty / quantity_in_base)
}

// Product is the MongoDB document stored in the "products" collection.
// StockQty is always kept in base units (the unit whose QuantityInBase == 1).
type Product struct {
	ID                string           `bson:"_id,omitempty" json:"id"`
	ShopID            string           `bson:"shop_id" json:"shop_id"`
	Name              string           `bson:"name" json:"name"`
	Category          string           `bson:"category" json:"category"`
	BaseUnit          string           `bson:"base_unit" json:"base_unit"`
	Units             []UnitDefinition `bson:"units" json:"units"`
	StockQty          int              `bson:"stock_qty" json:"stock_qty"`
	LowStockThreshold int              `bson:"low_stock_threshold" json:"low_stock_threshold"`
	IsActive          bool             `bson:"is_active" json:"is_active"`
	CreatedAt         time.Time        `bson:"created_at" json:"created_at"`
	UpdatedAt         time.Time        `bson:"updated_at" json:"updated_at"`
}

// ProductResponse wraps Product and appends a computed stock breakdown.
// The breakdown shows how much stock is available in every unit the product supports.
type ProductResponse struct {
	Product
	StockBreakdown []StockLevel `json:"stock_breakdown"`
}

// ToResponse attaches a computed stock breakdown to the product.
// Units in the breakdown appear in the same order as Product.Units (largest first).
func (p Product) ToResponse() ProductResponse {
	levels := make([]StockLevel, len(p.Units))
	for i, u := range p.Units {
		levels[i] = StockLevel{
			Unit:           u.Name,
			QuantityInBase: u.QuantityInBase,
			Available:      p.StockQty / u.QuantityInBase,
		}
	}
	return ProductResponse{Product: p, StockBreakdown: levels}
}

// FindUnit returns the unit definition matching the given name (case-insensitive).
// Used by the sale service to resolve which unit a customer is buying.
func (p Product) FindUnit(name string) (UnitDefinition, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, u := range p.Units {
		if strings.ToLower(u.Name) == name {
			return u, true
		}
	}
	return UnitDefinition{}, false
}

// validateUnits ensures the units slice is well-formed:
//   - 1 to 5 units
//   - all names non-empty and unique (case-insensitive)
//   - all QuantityInBase >= 1, all prices > 0
//   - exactly one unit with QuantityInBase == 1 (the base unit)
func validateUnits(units []UnitDefinition) error {
	if len(units) == 0 {
		return fmt.Errorf("at least one unit is required")
	}
	if len(units) > 5 {
		return fmt.Errorf("a product may have at most 5 units")
	}

	seen := map[string]struct{}{}
	baseCount := 0

	for i, u := range units {
		name := strings.TrimSpace(u.Name)
		if name == "" {
			return fmt.Errorf("unit[%d]: name cannot be empty", i)
		}
		key := strings.ToLower(name)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate unit name '%s'", name)
		}
		seen[key] = struct{}{}

		if u.QuantityInBase < 1 {
			return fmt.Errorf("unit '%s': quantity_in_base must be >= 1", name)
		}
		if u.Price <= 0 {
			return fmt.Errorf("unit '%s': price must be greater than zero", name)
		}
		if u.QuantityInBase == 1 {
			baseCount++
		}
	}

	if baseCount == 0 {
		return fmt.Errorf("exactly one unit must have quantity_in_base == 1 (the base unit)")
	}
	if baseCount > 1 {
		return fmt.Errorf("only one unit can have quantity_in_base == 1 (the base unit), found %d", baseCount)
	}

	return nil
}

// deriveBaseUnit returns the name of the unit with QuantityInBase == 1.
// Call validateUnits before this to guarantee exactly one base unit exists.
func deriveBaseUnit(units []UnitDefinition) string {
	for _, u := range units {
		if u.QuantityInBase == 1 {
			return strings.ToLower(strings.TrimSpace(u.Name))
		}
	}
	return ""
}

// normaliseUnits lowercases unit names, trims whitespace, and sorts
// the slice descending by QuantityInBase so the stored order is always
// largest unit first (e.g. carton, roll, packet).
func normaliseUnits(units []UnitDefinition) []UnitDefinition {
	out := make([]UnitDefinition, len(units))
	copy(out, units)
	for i := range out {
		out[i].Name = strings.ToLower(strings.TrimSpace(out[i].Name))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].QuantityInBase > out[j].QuantityInBase
	})
	return out
}

// computeInitialStock converts a mixed-unit count map into a base-unit total.
//
// Example: units = [carton×25, roll×5, packet×1], initialStock = {"carton":10, "roll":2, "packet":3}
// → (10×25) + (2×5) + (3×1) = 263 packets
//
// Keys that do not match a unit name are silently ignored.
func computeInitialStock(units []UnitDefinition, initialStock map[string]int) int {
	qibByName := make(map[string]int, len(units))
	for _, u := range units {
		qibByName[strings.ToLower(u.Name)] = u.QuantityInBase
	}
	total := 0
	for rawName, qty := range initialStock {
		if qib, ok := qibByName[strings.ToLower(rawName)]; ok && qty > 0 {
			total += qty * qib
		}
	}
	return total
}

// ---- DTOs ---------------------------------------------------------------

type CreateProductInput struct {
	ShopID            string           `json:"shop_id"`
	Name              string           `json:"name"`
	Category          string           `json:"category"`
	Units             []UnitDefinition `json:"units"`
	// InitialStock is a map of unit name → quantity for the opening stock count.
	// Example: {"carton": 10, "roll": 2, "packet": 3}
	// The system converts the mix into base units and stores the total in stock_qty.
	InitialStock      map[string]int   `json:"initial_stock"`
	LowStockThreshold int              `json:"low_stock_threshold"`
}

type UpdateProductInput struct {
	Name              *string          `json:"name,omitempty"`
	Category          *string          `json:"category,omitempty"`
	// Units replaces the entire units array when provided. stock_qty is not
	// recalculated — it remains in the existing base units.
	Units             []UnitDefinition `json:"units,omitempty"`
	LowStockThreshold *int             `json:"low_stock_threshold,omitempty"`
	IsActive          *bool            `json:"is_active,omitempty"`
}

type SyncProductItem struct {
	ID                string           `json:"id"`
	ShopID            string           `json:"shop_id"`
	Name              string           `json:"name"`
	Category          string           `json:"category"`
	Units             []UnitDefinition `json:"units"`
	BaseUnit          string           `json:"base_unit"`
	StockQty          int              `json:"stock_qty"`
	LowStockThreshold int              `json:"low_stock_threshold"`
	IsActive          bool             `json:"is_active"`
	UpdatedAt         time.Time        `json:"updated_at"`
}

type SyncProductsInput struct {
	Products []SyncProductItem `json:"products"`
}
