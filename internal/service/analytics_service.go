package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/realsend/be-realsend/internal/repository"
)

// AnalyticsService provides email analytics and statistics.
type AnalyticsService interface {
	GetOverview(ctx context.Context, userID uuid.UUID, period string) (map[string]interface{}, error)
	GetDailyBreakdown(ctx context.Context, userID uuid.UUID, startDate, endDate string) ([]repository.DailyStat, error)
	GetDomainBreakdown(ctx context.Context, userID uuid.UUID) ([]repository.DomainStat, error)
	// Admin
	GetGlobalOverview(ctx context.Context, period string) (map[string]interface{}, error)
}

type analyticsService struct {
	emailRepo repository.EmailRepository
}

// NewAnalyticsService creates a new AnalyticsService.
func NewAnalyticsService(emailRepo repository.EmailRepository) AnalyticsService {
	return &analyticsService{emailRepo: emailRepo}
}

func (s *analyticsService) GetOverview(ctx context.Context, userID uuid.UUID, period string) (map[string]interface{}, error) {
	startDate, endDate := periodToDateRange(period)

	counts, err := s.emailRepo.CountByUserAndStatus(ctx, userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("get overview: %w", err)
	}

	total := 0
	for _, v := range counts {
		total += v
	}

	return map[string]interface{}{
		"period":    period,
		"start":     startDate,
		"end":       endDate,
		"total":     total,
		"sent":      counts["sent"] + counts["delivered"] + counts["opened"] + counts["clicked"],
		"delivered": counts["delivered"],
		"bounced":   counts["bounced"],
		"failed":    counts["failed"],
		"opened":    counts["opened"],
		"clicked":   counts["clicked"],
		"queued":    counts["queued"],
		"rejected":  counts["rejected"],
	}, nil
}

func (s *analyticsService) GetDailyBreakdown(ctx context.Context, userID uuid.UUID, startDate, endDate string) ([]repository.DailyStat, error) {
	if startDate == "" || endDate == "" {
		startDate, endDate = periodToDateRange("30d")
	}
	return s.emailRepo.DailyBreakdown(ctx, userID, startDate, endDate)
}

func (s *analyticsService) GetDomainBreakdown(ctx context.Context, userID uuid.UUID) ([]repository.DomainStat, error) {
	return s.emailRepo.DomainBreakdown(ctx, userID)
}

func (s *analyticsService) GetGlobalOverview(ctx context.Context, period string) (map[string]interface{}, error) {
	startDate, endDate := periodToDateRange(period)

	counts, err := s.emailRepo.CountAllByStatus(ctx, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("global overview: %w", err)
	}

	total := 0
	for _, v := range counts {
		total += v
	}

	return map[string]interface{}{
		"period":    period,
		"start":     startDate,
		"end":       endDate,
		"total":     total,
		"sent":      counts["sent"] + counts["delivered"] + counts["opened"] + counts["clicked"],
		"delivered": counts["delivered"],
		"bounced":   counts["bounced"],
		"failed":    counts["failed"],
		"opened":    counts["opened"],
		"clicked":   counts["clicked"],
	}, nil
}

// periodToDateRange converts a period string (7d, 30d, 90d) to start/end date strings.
func periodToDateRange(period string) (string, string) {
	now := time.Now()
	endDate := now.Format("2006-01-02T15:04:05-07:00")

	var startDate string
	switch period {
	case "7d":
		startDate = now.AddDate(0, 0, -7).Format("2006-01-02T15:04:05-07:00")
	case "90d":
		startDate = now.AddDate(0, 0, -90).Format("2006-01-02T15:04:05-07:00")
	case "1y":
		startDate = now.AddDate(-1, 0, 0).Format("2006-01-02T15:04:05-07:00")
	default: // 30d
		startDate = now.AddDate(0, 0, -30).Format("2006-01-02T15:04:05-07:00")
	}

	return startDate, endDate
}
