package dashboard

import "time"

type ActivityItem struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Subtitle  string    `json:"subtitle"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
}

type OwnerDashboardResponse struct {
	TodaySales       float64        `json:"today_sales"`
	TransactionCount int64          `json:"transaction_count"`
	LowStockCount    int64          `json:"low_stock_count"`
	TotalDebts       float64        `json:"total_debts"`
	WeeklyRevenue    [7]float64     `json:"weekly_revenue"`
	ActivityFeed     []ActivityItem `json:"activity_feed"`
}

type RecentSale struct {
	ID          string    `json:"id"`
	TotalAmount float64   `json:"total_amount"`
	ItemCount   int       `json:"item_count"`
	CustomerID  string    `json:"customer_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type StaffDashboardResponse struct {
	MySalesToday   int64        `json:"my_sales_today"`
	MyRevenueToday float64      `json:"my_revenue_today"`
	RecentSales    []RecentSale `json:"recent_sales"`
}
