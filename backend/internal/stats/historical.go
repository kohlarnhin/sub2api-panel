package stats

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"
)

// pgUndefinedTable 是 PostgreSQL 在表/视图不存在时返回的 SQLSTATE。
const pgUndefinedTable = "42P01"

// HistoricalTotals 查询 sub2api 自带的每日聚合表 `usage_dashboard_daily`，
// 返回全平台历史累计。
//
// 这张表由 sub2api 自身的迁移 034 引入、由其后台聚合任务维护，本项目只读不写。
// 当目标 sub2api 部署未启用该表时，会返回 Enabled=false（前端展示空态）而不是失败，
// 这样仪表盘的其他卡片仍然可用。
func (r *Repository) HistoricalTotals(ctx context.Context) (HistoricalSummary, error) {
	const q = `
		SELECT
			COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0) AS total_tokens,
			COALESCE(SUM(actual_cost), 0)::float8                                                     AS total_cost,
			COALESCE(SUM(total_requests), 0)                                                          AS total_requests,
			to_char(MIN(bucket_date), 'YYYY-MM-DD')                                                   AS since,
			to_char(MAX(bucket_date), 'YYYY-MM-DD')                                                   AS until,
			COUNT(*)                                                                                  AS days_covered
		FROM usage_dashboard_daily
	`

	var (
		totalTokens   int64
		totalCost     float64
		totalRequests int64
		since         sql.NullString
		until         sql.NullString
		days          int64
	)
	err := r.db.QueryRowContext(ctx, q).Scan(
		&totalTokens, &totalCost, &totalRequests, &since, &until, &days,
	)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && string(pqErr.Code) == pgUndefinedTable {
			// sub2api 未启用每日聚合，优雅降级。
			return HistoricalSummary{Enabled: false}, nil
		}
		return HistoricalSummary{}, fmt.Errorf("historical_totals: %w", err)
	}

	return HistoricalSummary{
		Enabled:       days > 0,
		TotalTokens:   totalTokens,
		TotalCost:     totalCost,
		TotalRequests: totalRequests,
		Since:         since.String,
		Until:         until.String,
		DaysCovered:   days,
	}, nil
}
