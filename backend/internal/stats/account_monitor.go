package stats

import (
	"context"
	"database/sql"
	"fmt"
)

// AccountMonitorFor 查询指定分组下账号的健康度统计。
// 字段口径见 types.go 上的注释。
// 若 groupID <= 0，直接返回 Enabled=false 的空结果，不打数据库。
func (r *Repository) AccountMonitorFor(ctx context.Context, groupID int64) (AccountMonitor, error) {
	if groupID <= 0 {
		return AccountMonitor{Enabled: false}, nil
	}

	const q = `
		WITH g AS (
			SELECT id, name FROM groups WHERE id = $1
		),
		acc AS (
			SELECT a.*
			FROM accounts a
			JOIN account_groups ag ON ag.account_id = a.id
			WHERE ag.group_id = $1
			  AND a.deleted_at IS NULL
		)
		SELECT
			(SELECT name FROM g) AS group_name,
			COUNT(*) AS total,
			COUNT(*) FILTER (
				WHERE acc.status = 'active'
				  AND acc.schedulable = true
				  AND (acc.rate_limit_reset_at IS NULL OR acc.rate_limit_reset_at <= now())
				  AND (acc.overload_until        IS NULL OR acc.overload_until        <= now())
				  AND (acc.temp_unschedulable_until IS NULL OR acc.temp_unschedulable_until <= now())
				  AND (acc.expires_at            IS NULL OR acc.expires_at            >  now())
			) AS available,
			COUNT(*) FILTER (
				WHERE acc.rate_limit_reset_at IS NOT NULL
				  AND acc.rate_limit_reset_at > now()
			) AS rate_limited
		FROM acc
	`

	var (
		groupName   sql.NullString
		total       int64
		available   int64
		rateLimited int64
	)
	if err := r.db.QueryRowContext(ctx, q, groupID).Scan(
		&groupName, &total, &available, &rateLimited,
	); err != nil {
		return AccountMonitor{}, fmt.Errorf("account_monitor: %w", err)
	}

	// 异常 = 总量 - 限流 - 可用（按用户口径）
	abnormal := total - rateLimited - available
	if abnormal < 0 {
		abnormal = 0
	}

	return AccountMonitor{
		Enabled:     true,
		GroupID:     groupID,
		GroupName:   groupName.String,
		Total:       total,
		Available:   available,
		RateLimited: rateLimited,
		Abnormal:    abnormal,
	}, nil
}
