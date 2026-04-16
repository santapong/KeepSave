package service

import (
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/santapong/KeepSave/backend/internal/models"
	"github.com/santapong/KeepSave/backend/internal/repository"
)

type UsageAnalyticsService struct {
	db      *sql.DB
	dialect repository.Dialect
}

func NewUsageAnalyticsService(db *sql.DB, dialect repository.Dialect) *UsageAnalyticsService {
	return &UsageAnalyticsService{db: db, dialect: dialect}
}

func (s *UsageAnalyticsService) RecordAccess(projectID uuid.UUID, apiKeyID *uuid.UUID) {
	now := time.Now().Truncate(time.Hour)
	id := uuid.New()
	keyStr := ""
	if apiKeyID != nil {
		keyStr = apiKeyID.String()
	}
	s.db.Exec(`INSERT INTO access_timeseries (id, project_id, api_key_id, bucket, access_count, unique_keys, unique_ips, avg_latency_ms) VALUES ($1, $2, $3, $4, 1, 0, 0, 0) ON CONFLICT (project_id, api_key_id, bucket) DO UPDATE SET access_count = access_timeseries.access_count + 1`,
		id, projectID, keyStr, now)
}

func (s *UsageAnalyticsService) GetTrends(projectID uuid.UUID, period string, days int) ([]models.UsageTrend, error) {
	if days <= 0 {
		days = 30
	}
	since := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	var dateFmt string
	switch period {
	case "weekly":
		dateFmt = "%Y-W%W"
	case "monthly":
		dateFmt = "%Y-%m"
	default:
		period = "daily"
		dateFmt = "%Y-%m-%d"
	}
	query := fmt.Sprintf(`SELECT strftime('%s', bucket) as period_date, SUM(access_count) as total FROM access_timeseries WHERE project_id = $1 AND bucket >= $2 GROUP BY period_date ORDER BY period_date`, dateFmt)
	rows, err := s.db.Query(query, projectID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var trends []models.UsageTrend
	var prevCount int
	for rows.Next() {
		var t models.UsageTrend
		if err := rows.Scan(&t.Date, &t.AccessCount); err != nil {
			continue
		}
		t.Period = period
		if prevCount > 0 {
			t.Growth = float64(t.AccessCount-prevCount) / float64(prevCount) * 100
		}
		prevCount = t.AccessCount
		trends = append(trends, t)
	}
	return trends, nil
}

func (s *UsageAnalyticsService) Forecast(projectID uuid.UUID, daysAhead int) ([]models.UsageForecast, error) {
	if daysAhead <= 0 {
		daysAhead = 14
	}
	since := time.Now().Add(-30 * 24 * time.Hour)
	rows, err := s.db.Query(`SELECT strftime('%Y-%m-%d', bucket) as d, SUM(access_count) FROM access_timeseries WHERE project_id = $1 AND bucket >= $2 GROUP BY d ORDER BY d`, projectID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var xs, ys []float64
	i := 0.0
	for rows.Next() {
		var d string
		var count int
		if err := rows.Scan(&d, &count); err != nil {
			continue
		}
		xs = append(xs, i)
		ys = append(ys, float64(count))
		i++
	}
	if len(xs) < 3 {
		var forecasts []models.UsageForecast
		for d := 1; d <= daysAhead; d++ {
			date := time.Now().Add(time.Duration(d) * 24 * time.Hour).Format("2006-01-02")
			forecasts = append(forecasts, models.UsageForecast{Date: date, PredictedCount: 0, Confidence: 0})
		}
		return forecasts, nil
	}
	slope, intercept := linearRegression(xs, ys)
	var sumResidual float64
	for j := range xs {
		p := slope*xs[j] + intercept
		sumResidual += (ys[j] - p) * (ys[j] - p)
	}
	stdErr := math.Sqrt(sumResidual / float64(len(xs)))
	var forecasts []models.UsageForecast
	n := float64(len(xs))
	for d := 1; d <= daysAhead; d++ {
		x := n + float64(d) - 1
		predicted := slope*x + intercept
		if predicted < 0 {
			predicted = 0
		}
		confidence := math.Max(0, 1.0-float64(d)*0.03)
		date := time.Now().Add(time.Duration(d) * 24 * time.Hour).Format("2006-01-02")
		forecasts = append(forecasts, models.UsageForecast{
			Date: date, PredictedCount: math.Round(predicted*100) / 100,
			LowerBound: math.Max(0, math.Round((predicted-1.96*stdErr)*100)/100),
			UpperBound: math.Round((predicted+1.96*stdErr)*100) / 100,
			Confidence: math.Round(confidence*100) / 100,
		})
	}
	return forecasts, nil
}

// ExportCSV returns usage trends as CSV content.
func (s *UsageAnalyticsService) ExportCSV(projectID uuid.UUID, period string, days int) (string, error) {
	trends, err := s.GetTrends(projectID, period, days)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("period,date,access_count,unique_users,unique_keys,growth_percent\n")
	for _, t := range trends {
		b.WriteString(fmt.Sprintf("%s,%s,%d,%d,%d,%.2f\n", t.Period, t.Date, t.AccessCount, t.UniqueUsers, t.UniqueKeys, t.Growth))
	}
	return b.String(), nil
}

// --- Quota Management ---

// GetQuota returns the usage quota for an organization.
func (s *UsageAnalyticsService) GetQuota(orgID uuid.UUID) (*models.UsageQuota, error) {
	var q models.UsageQuota
	err := s.db.QueryRow(`SELECT id, organization_id, max_secrets, max_projects, max_api_keys, max_requests_per_day, current_secrets, current_projects, current_api_keys, created_at, updated_at FROM usage_quotas WHERE organization_id = $1`, orgID).Scan(
		&q.ID, &q.OrganizationID, &q.MaxSecrets, &q.MaxProjects, &q.MaxAPIKeys, &q.MaxRequestsPerDay,
		&q.CurrentSecrets, &q.CurrentProjects, &q.CurrentAPIKeys, &q.CreatedAt, &q.UpdatedAt)
	if err == sql.ErrNoRows {
		return &models.UsageQuota{OrganizationID: orgID, MaxSecrets: 1000, MaxProjects: 50, MaxAPIKeys: 100, MaxRequestsPerDay: 100000}, nil
	}
	if err != nil {
		return nil, err
	}
	return &q, nil
}

// SetQuota creates or updates the usage quota for an organization.
func (s *UsageAnalyticsService) SetQuota(orgID uuid.UUID, maxSecrets, maxProjects, maxAPIKeys, maxReqPerDay int) (*models.UsageQuota, error) {
	now := time.Now()
	id := uuid.New()
	_, err := s.db.Exec(`INSERT INTO usage_quotas (id, organization_id, max_secrets, max_projects, max_api_keys, max_requests_per_day, current_secrets, current_projects, current_api_keys, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,0,0,0,$7,$8) ON CONFLICT (organization_id) DO UPDATE SET max_secrets=$3, max_projects=$4, max_api_keys=$5, max_requests_per_day=$6, updated_at=$8`,
		id, orgID, maxSecrets, maxProjects, maxAPIKeys, maxReqPerDay, now, now)
	if err != nil {
		return nil, err
	}
	return s.GetQuota(orgID)
}

func linearRegression(xs, ys []float64) (slope, intercept float64) {
	n := float64(len(xs))
	var sumX, sumY, sumXY, sumX2 float64
	for i := range xs {
		sumX += xs[i]
		sumY += ys[i]
		sumXY += xs[i] * ys[i]
		sumX2 += xs[i] * xs[i]
	}
	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0, sumY / n
	}
	slope = (n*sumXY - sumX*sumY) / denom
	intercept = (sumY - slope*sumX) / n
	return
}
