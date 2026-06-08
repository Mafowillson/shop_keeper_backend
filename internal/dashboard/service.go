package dashboard

import (
	"context"
	"fmt"
	"strings"
	"time"

	"shop_keeper_backend/internal/customer"
	"shop_keeper_backend/internal/i18n"
	"shop_keeper_backend/internal/product"
	"shop_keeper_backend/internal/sale"
	"shop_keeper_backend/internal/shop"
	"shop_keeper_backend/internal/staff"
	"shop_keeper_backend/internal/user"
)

type Service struct {
	userRepo     *user.Repo
	shopRepo     *shop.Repo
	saleRepo     *sale.Repo
	productRepo  *product.Repo
	customerRepo *customer.Repo
	staffRepo    *staff.Repo
}

func NewService(
	userRepo *user.Repo,
	shopRepo *shop.Repo,
	saleRepo *sale.Repo,
	productRepo *product.Repo,
	customerRepo *customer.Repo,
	staffRepo *staff.Repo,
) *Service {
	return &Service{
		userRepo:     userRepo,
		shopRepo:     shopRepo,
		saleRepo:     saleRepo,
		productRepo:  productRepo,
		customerRepo: customerRepo,
		staffRepo:    staffRepo,
	}
}

// GetOwnerStats builds the owner dashboard response for the given ownerID.
func (s *Service) GetOwnerStats(ctx context.Context, ownerID string, locale string, shopID string) (OwnerDashboardResponse, error) {
	shopID = strings.TrimSpace(shopID)
	if shopID == "" {
		return OwnerDashboardResponse{}, fmt.Errorf("dashboard: missing shop id")
	}

	u, err := s.userRepo.FindByID(ctx, ownerID)
	if err != nil {
		return OwnerDashboardResponse{}, fmt.Errorf("dashboard: find owner: %w", err)
	}

	if shopID != "" {
		shop, err := s.shopRepo.FindByIDAndOwner(ctx, shopID, ownerID)
		if err != nil {
			return OwnerDashboardResponse{}, fmt.Errorf("dashboard: invalid shop: %w", err)
		}
		shopID = shop.ID
	} else {
		if u.ShopID == "" {
			return OwnerDashboardResponse{}, fmt.Errorf("dashboard: owner has no shop")
		}
		shopID = u.ShopID
	}

	now := time.Now().UTC()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	todaySales, txCount, err := s.saleRepo.TodayStatsByShop(ctx, shopID, todayStart)
	if err != nil {
		return OwnerDashboardResponse{}, err
	}

	lowStock, err := s.productRepo.CountLowStock(ctx, shopID)
	if err != nil {
		return OwnerDashboardResponse{}, err
	}

	totalDebts, err := s.customerRepo.TotalDebtByShop(ctx, shopID)
	if err != nil {
		return OwnerDashboardResponse{}, err
	}

	// Weekly revenue: last 7 days ending today (index 0 = 6 days ago, index 6 = today).
	weekStart := todayStart.AddDate(0, 0, -6)
	weekly, err := s.saleRepo.WeeklyRevenueByShop(ctx, shopID, weekStart)
	if err != nil {
		return OwnerDashboardResponse{}, err
	}

	// Activity feed from the 10 most recent sales.
	recentSales, err := s.saleRepo.ListRecentByShop(ctx, shopID, 10)
	if err != nil {
		return OwnerDashboardResponse{}, err
	}
	msgs := i18n.Get(locale)
	feed := make([]ActivityItem, 0, len(recentSales))
	for _, sale := range recentSales {
		subtitle := fmt.Sprintf(msgs.ActivitySaleSubtitleFmt, sale.TotalAmount, len(sale.Items))
		if sale.IsCredit {
			subtitle += msgs.ActivityCreditSuffix
		}
		feed = append(feed, ActivityItem{
			ID:        sale.ID,
			Title:     msgs.ActivitySaleTitle,
			Subtitle:  subtitle,
			Timestamp: sale.CreatedAt,
			Type:      "sale",
		})
	}

	return OwnerDashboardResponse{
		TodaySales:       todaySales,
		TransactionCount: txCount,
		LowStockCount:    lowStock,
		TotalDebts:       totalDebts,
		WeeklyRevenue:    weekly,
		ActivityFeed:     feed,
	}, nil
}

// GetStaffStats builds the staff dashboard response for the given staffID.
func (s *Service) GetStaffStats(ctx context.Context, staffID string) (StaffDashboardResponse, error) {
	now := time.Now().UTC()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	revenue, count, err := s.saleRepo.TodayStatsByUser(ctx, staffID, todayStart)
	if err != nil {
		return StaffDashboardResponse{}, err
	}

	recentSales, err := s.saleRepo.ListRecentByUser(ctx, staffID, 5)
	if err != nil {
		return StaffDashboardResponse{}, err
	}

	recent := make([]RecentSale, 0, len(recentSales))
	for _, sale := range recentSales {
		recent = append(recent, RecentSale{
			ID:          sale.ID,
			TotalAmount: sale.TotalAmount,
			ItemCount:   len(sale.Items),
			CustomerID:  sale.CustomerID,
			CreatedAt:   sale.CreatedAt,
		})
	}

	return StaffDashboardResponse{
		MySalesToday:   count,
		MyRevenueToday: revenue,
		RecentSales:    recent,
	}, nil
}
