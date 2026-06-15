package riskscoring

import (
	"context"
	"sort"
	"time"

	"shop_keeper_backend/internal/customer"
)

// Service computes the AI-2 rule-based credit risk score for a customer.
// It satisfies customer.RiskScorer.
type Service struct {
	customerRepo *customer.Repo
}

func NewService(repo *customer.Repo) *Service {
	return &Service{customerRepo: repo}
}

// Calculate scores a customer and persists the result.
// Minimum 3 debt events required; fewer yields "insufficient_history".
func (s *Service) Calculate(ctx context.Context, customerID string) error {
	records, err := s.customerRepo.GetDebtHistoryByCustomer(ctx, customerID)
	if err != nil {
		return err
	}

	now := time.Now().UTC()

	if len(records) < 3 {
		return s.customerRepo.UpdateRiskScore(ctx, customerID, 0, customer.RiskLevelInsufficientHistory, now)
	}

	c, err := s.customerRepo.GetCustomerByID(ctx, customerID)
	if err != nil {
		return err
	}

	// Sort ascending so time-based algorithms see records in chronological order.
	asc := make([]customer.DebtRecord, len(records))
	copy(asc, records)
	sort.Slice(asc, func(i, j int) bool {
		return asc[i].RecordedAt.Before(asc[j].RecordedAt)
	})

	since30 := now.AddDate(0, 0, -30)
	since90 := now.AddDate(0, 0, -90)

	f1 := scoreRepaymentTime(asc)
	f2 := scoreDebtGrowth(asc, since30, c.TotalDebt)
	f3 := scorePaymentFrequency(asc, since90)
	f4 := scoreDebtToCredit(asc, c.TotalDebt)

	composite := f1 + f2 + f3 + f4

	var level string
	switch {
	case composite <= 2:
		level = customer.RiskLevelLow
	case composite <= 5:
		level = customer.RiskLevelMedium
	default:
		level = customer.RiskLevelHigh
	}

	return s.customerRepo.UpdateRiskScore(ctx, customerID, composite, level, now)
}

// scoreRepaymentTime averages the gap (in days) between each payment event and
// the most recent credit event that preceded it.
// Score 2: avg < 7 days | Score 1: 7–30 days | Score 0: > 30 days or no payments.
func scoreRepaymentTime(asc []customer.DebtRecord) int {
	var gaps []float64
	for i, rec := range asc {
		if rec.Type != "payment" {
			continue
		}
		for j := i - 1; j >= 0; j-- {
			if asc[j].Type == "credit" {
				gap := rec.RecordedAt.Sub(asc[j].RecordedAt).Hours() / 24
				gaps = append(gaps, gap)
				break
			}
		}
	}
	if len(gaps) == 0 {
		return 0
	}
	var total float64
	for _, g := range gaps {
		total += g
	}
	avg := total / float64(len(gaps))
	switch {
	case avg < 7:
		return 2
	case avg <= 30:
		return 1
	default:
		return 0
	}
}

// scoreDebtGrowth computes the net debt change over the last 30 days as a
// fraction of the current outstanding balance.
// Score 2: shrinking/flat | Score 1: < 20% growth | Score 0: ≥ 20% growth.
func scoreDebtGrowth(asc []customer.DebtRecord, since30 time.Time, currentDebt float64) int {
	if currentDebt <= 0 {
		return 2
	}
	var netChange float64
	for _, rec := range asc {
		if rec.RecordedAt.Before(since30) {
			continue
		}
		if rec.Type == "credit" {
			netChange += rec.Amount
		} else {
			netChange -= rec.Amount
		}
	}
	rate := netChange / currentDebt
	switch {
	case rate <= 0:
		return 2
	case rate <= 0.20:
		return 1
	default:
		return 0
	}
}

// scorePaymentFrequency computes the ratio of payment events to credit events
// over the last 90 days.
// Score 2: ≥ 0.75 | Score 1: 0.40–0.74 | Score 0: < 0.40 or no credits.
func scorePaymentFrequency(asc []customer.DebtRecord, since90 time.Time) int {
	var credits, payments int
	for _, rec := range asc {
		if rec.RecordedAt.Before(since90) {
			continue
		}
		switch rec.Type {
		case "credit":
			credits++
		case "payment":
			payments++
		}
	}
	if credits == 0 {
		return 2
	}
	ratio := float64(payments) / float64(credits)
	switch {
	case ratio >= 0.75:
		return 2
	case ratio >= 0.40:
		return 1
	default:
		return 0
	}
}

// scoreDebtToCredit computes the ratio of current outstanding debt to the total
// credit ever extended to this customer.
// Score 2: ≤ 0.25 | Score 1: 0.26–0.60 | Score 0: > 0.60.
func scoreDebtToCredit(asc []customer.DebtRecord, currentDebt float64) int {
	var totalCredit float64
	for _, rec := range asc {
		if rec.Type == "credit" {
			totalCredit += rec.Amount
		}
	}
	if totalCredit == 0 {
		return 2
	}
	ratio := currentDebt / totalCredit
	switch {
	case ratio <= 0.25:
		return 2
	case ratio <= 0.60:
		return 1
	default:
		return 0
	}
}
