package stats

// UserTokenRank 表示用户 token 用量排行榜的一行。
type UserTokenRank struct {
	UserID              int64   `json:"user_id"`
	Email               string  `json:"email"`
	Username            string  `json:"username"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	TotalTokens         int64   `json:"total_tokens"`
	TotalCost           float64 `json:"total_cost"`
	RequestCount        int64   `json:"request_count"`
}

// UserCostRank 表示用户消费金额排行榜的一行。
type UserCostRank struct {
	UserID       int64   `json:"user_id"`
	Email        string  `json:"email"`
	Username     string  `json:"username"`
	TotalCost    float64 `json:"total_cost"`
	TotalTokens  int64   `json:"total_tokens"`
	RequestCount int64   `json:"request_count"`
}

// DailyTrend 表示按日聚合的趋势点。
type DailyTrend struct {
	Date         string  `json:"date"` // YYYY-MM-DD（按服务端时区）
	TotalTokens  int64   `json:"total_tokens"`
	TotalCost    float64 `json:"total_cost"`
	RequestCount int64   `json:"request_count"`
}

// ModelStat 表示按模型聚合的统计。
type ModelStat struct {
	Model        string  `json:"model"`
	TotalTokens  int64   `json:"total_tokens"`
	TotalCost    float64 `json:"total_cost"`
	RequestCount int64   `json:"request_count"`
}

// TodaySummary 是今日整体汇总。
type TodaySummary struct {
	TotalTokens   int64   `json:"total_tokens"`
	TotalCost     float64 `json:"total_cost"`
	TotalRequests int64   `json:"total_requests"`
	ActiveUsers   int64   `json:"active_users"`
}

// AccountMonitor 表示指定分组下的账号健康度统计。
//
// 字段定义：
//   - total        :  分组下未软删除的账号总数
//   - available    :  当前可被调度的账号数（status=active, schedulable, 无任何限流/过载/临时不可调度，未过期）
//   - rate_limited :  正处于限流时间窗内的账号数（rate_limit_reset_at > now()）
//   - abnormal     :  total - available - rate_limited（其他故障/禁用/过期/过载等）
//
// group_id == 0 时整张表是禁用状态，前端应展示"未配置"。
type AccountMonitor struct {
	Enabled     bool   `json:"enabled"`
	GroupID     int64  `json:"group_id"`
	GroupName   string `json:"group_name"`
	Total       int64  `json:"total"`
	Available   int64  `json:"available"`
	RateLimited int64  `json:"rate_limited"`
	Abnormal    int64  `json:"abnormal"`
}

// HistoricalSummary 表示来自 sub2api 内置 `usage_dashboard_daily` 聚合表的全平台历史累计。
//
//   - enabled         :  数据源是否可用（false = sub2api 未启用每日聚合 / 表不存在 / 表为空）
//   - total_tokens    :  累计 Token（input+output+cache_creation+cache_read）
//   - total_cost      :  累计实际消费金额（actual_cost）
//   - total_requests  :  累计请求数
//   - since           :  最早一天的日期 (YYYY-MM-DD)，前端展示"自 X 起"
//   - until           :  最新一天的日期 (YYYY-MM-DD)，便于判断聚合写入是否滞后
//   - days_covered    :  聚合表里实际存在的日数
type HistoricalSummary struct {
	Enabled       bool    `json:"enabled"`
	TotalTokens   int64   `json:"total_tokens"`
	TotalCost     float64 `json:"total_cost"`
	TotalRequests int64   `json:"total_requests"`
	Since         string  `json:"since"`
	Until         string  `json:"until"`
	DaysCovered   int64   `json:"days_covered"`
}

// Snapshot 是一次聚合查询的完整结果，用于 SSE 推送和 REST 兼容。
type Snapshot struct {
	GeneratedAt    string            `json:"generated_at"` // RFC3339
	Timezone       string            `json:"timezone"`
	TodaySummary   TodaySummary      `json:"today_summary"`
	TokenRanking   []UserTokenRank   `json:"token_ranking"`
	CostRanking    []UserCostRank    `json:"cost_ranking"`
	SevenDayTrend  []DailyTrend      `json:"seven_day_trend"`
	ModelBreakdown []ModelStat       `json:"model_breakdown"`
	AccountMonitor AccountMonitor    `json:"account_monitor"`
	Historical     HistoricalSummary `json:"historical"`
}
