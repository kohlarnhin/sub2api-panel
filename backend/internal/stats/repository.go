package stats

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Repository 封装对 sub2api 数据库的只读查询。
type Repository struct {
	db       *sql.DB
	location *time.Location
	topN     int
}

// NewRepository 构造 Repository。location 用于把"今日"、"近7天"按指定时区分桶。
func NewRepository(db *sql.DB, loc *time.Location, topN int) *Repository {
	return &Repository{db: db, location: loc, topN: topN}
}

// dayRange 返回 [startOfDay, startOfDay+24h)，按 r.location 计算。
func (r *Repository) dayRange(t time.Time) (time.Time, time.Time) {
	local := t.In(r.location)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, r.location)
	return start, start.Add(24 * time.Hour)
}

// TodaySummary 查询今日整体汇总。
func (r *Repository) TodaySummary(ctx context.Context) (TodaySummary, error) {
	start, end := r.dayRange(time.Now())
	const q = `
		SELECT
			COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0) AS total_tokens,
			COALESCE(SUM(actual_cost), 0)::float8 AS total_cost,
			COUNT(*) AS total_requests,
			COUNT(DISTINCT user_id) AS active_users
		FROM usage_logs
		WHERE created_at >= $1 AND created_at < $2
	`
	var s TodaySummary
	if err := r.db.QueryRowContext(ctx, q, start, end).Scan(
		&s.TotalTokens, &s.TotalCost, &s.TotalRequests, &s.ActiveUsers,
	); err != nil {
		return TodaySummary{}, fmt.Errorf("today_summary: %w", err)
	}
	return s, nil
}

// TodayTokenRanking 查询今日按 token 排序的用户排行榜（金额作为一列同时返回）。
func (r *Repository) TodayTokenRanking(ctx context.Context) ([]UserTokenRank, error) {
	start, end := r.dayRange(time.Now())
	const q = `
		SELECT
			u.id,
			COALESCE(u.email, '') AS email,
			COALESCE(u.username, '') AS username,
			COALESCE(SUM(ul.input_tokens), 0) AS input_tokens,
			COALESCE(SUM(ul.output_tokens), 0) AS output_tokens,
			COALESCE(SUM(ul.cache_creation_tokens), 0) AS cache_creation_tokens,
			COALESCE(SUM(ul.cache_read_tokens), 0) AS cache_read_tokens,
			COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0) AS total_tokens,
			COALESCE(SUM(ul.actual_cost), 0)::float8 AS total_cost,
			COUNT(*) AS request_count
		FROM usage_logs ul
		JOIN users u ON u.id = ul.user_id
		WHERE ul.created_at >= $1 AND ul.created_at < $2
		  AND u.deleted_at IS NULL
		GROUP BY u.id, u.email, u.username
		ORDER BY total_tokens DESC, request_count DESC
		LIMIT $3
	`
	rows, err := r.db.QueryContext(ctx, q, start, end, r.topN)
	if err != nil {
		return nil, fmt.Errorf("today_token_ranking: %w", err)
	}
	defer rows.Close()

	out := make([]UserTokenRank, 0, r.topN)
	for rows.Next() {
		var u UserTokenRank
		if err := rows.Scan(
			&u.UserID, &u.Email, &u.Username,
			&u.InputTokens, &u.OutputTokens, &u.CacheCreationTokens, &u.CacheReadTokens,
			&u.TotalTokens, &u.TotalCost, &u.RequestCount,
		); err != nil {
			return nil, fmt.Errorf("scan token_ranking: %w", err)
		}
		u.Email = maskEmail(u.Email)
		out = append(out, u)
	}
	return out, rows.Err()
}

// TodayCostRanking 查询今日按金额排序的用户排行榜。
func (r *Repository) TodayCostRanking(ctx context.Context) ([]UserCostRank, error) {
	start, end := r.dayRange(time.Now())
	const q = `
		SELECT
			u.id,
			COALESCE(u.email, '') AS email,
			COALESCE(u.username, '') AS username,
			COALESCE(SUM(ul.actual_cost), 0)::float8 AS total_cost,
			COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0) AS total_tokens,
			COUNT(*) AS request_count
		FROM usage_logs ul
		JOIN users u ON u.id = ul.user_id
		WHERE ul.created_at >= $1 AND ul.created_at < $2
		  AND u.deleted_at IS NULL
		GROUP BY u.id, u.email, u.username
		ORDER BY total_cost DESC, request_count DESC
		LIMIT $3
	`
	rows, err := r.db.QueryContext(ctx, q, start, end, r.topN)
	if err != nil {
		return nil, fmt.Errorf("today_cost_ranking: %w", err)
	}
	defer rows.Close()

	out := make([]UserCostRank, 0, r.topN)
	for rows.Next() {
		var u UserCostRank
		if err := rows.Scan(
			&u.UserID, &u.Email, &u.Username,
			&u.TotalCost, &u.TotalTokens, &u.RequestCount,
		); err != nil {
			return nil, fmt.Errorf("scan cost_ranking: %w", err)
		}
		u.Email = maskEmail(u.Email)
		out = append(out, u)
	}
	return out, rows.Err()
}

// SevenDayTrend 返回最近 7 天（含今天）按天聚合的趋势，缺失的日期会被补 0。
func (r *Repository) SevenDayTrend(ctx context.Context) ([]DailyTrend, error) {
	todayStart, _ := r.dayRange(time.Now())
	start := todayStart.AddDate(0, 0, -6)
	end := todayStart.Add(24 * time.Hour)

	// 在 SQL 中按时区把 created_at 截断到日。lib/pq 接收 time.Time 会以 UTC 发送，PG 端做 AT TIME ZONE 即可。
	q := fmt.Sprintf(`
		SELECT
			to_char(date_trunc('day', created_at AT TIME ZONE %s), 'YYYY-MM-DD') AS day,
			COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0) AS total_tokens,
			COALESCE(SUM(actual_cost), 0)::float8 AS total_cost,
			COUNT(*) AS request_count
		FROM usage_logs
		WHERE created_at >= $1 AND created_at < $2
		GROUP BY day
		ORDER BY day ASC
	`, quoteLiteral(r.location.String()))

	rows, err := r.db.QueryContext(ctx, q, start, end)
	if err != nil {
		return nil, fmt.Errorf("seven_day_trend: %w", err)
	}
	defer rows.Close()

	byDay := make(map[string]DailyTrend, 7)
	for rows.Next() {
		var d DailyTrend
		if err := rows.Scan(&d.Date, &d.TotalTokens, &d.TotalCost, &d.RequestCount); err != nil {
			return nil, fmt.Errorf("scan seven_day_trend: %w", err)
		}
		byDay[d.Date] = d
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]DailyTrend, 0, 7)
	for i := 0; i < 7; i++ {
		day := start.AddDate(0, 0, i).Format("2006-01-02")
		if d, ok := byDay[day]; ok {
			out = append(out, d)
		} else {
			out = append(out, DailyTrend{Date: day})
		}
	}
	return out, nil
}

// TodayModelBreakdown 返回今日按模型聚合的统计，按金额降序。
func (r *Repository) TodayModelBreakdown(ctx context.Context) ([]ModelStat, error) {
	start, end := r.dayRange(time.Now())
	const q = `
		SELECT
			COALESCE(NULLIF(requested_model, ''), model) AS m,
			COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0) AS total_tokens,
			COALESCE(SUM(actual_cost), 0)::float8 AS total_cost,
			COUNT(*) AS request_count
		FROM usage_logs
		WHERE created_at >= $1 AND created_at < $2
		GROUP BY m
		ORDER BY total_cost DESC, total_tokens DESC
		LIMIT $3
	`
	rows, err := r.db.QueryContext(ctx, q, start, end, r.topN)
	if err != nil {
		return nil, fmt.Errorf("today_model_breakdown: %w", err)
	}
	defer rows.Close()

	out := make([]ModelStat, 0, r.topN)
	for rows.Next() {
		var m ModelStat
		if err := rows.Scan(&m.Model, &m.TotalTokens, &m.TotalCost, &m.RequestCount); err != nil {
			return nil, fmt.Errorf("scan model_breakdown: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// quoteLiteral 把字符串包装成 PostgreSQL 字符串字面量（仅用于已校验的时区名）。
func quoteLiteral(s string) string {
	// 这里的输入仅来自 time.LoadLocation 校验过的 location 名，不会出现引号；
	// 用最简单的方式生成字面量。
	escaped := ""
	for _, r := range s {
		if r == '\'' {
			escaped += "''"
			continue
		}
		escaped += string(r)
	}
	return "'" + escaped + "'"
}
