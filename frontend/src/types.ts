export type UserTokenRank = {
  user_id: number
  email: string
  username: string
  input_tokens: number
  output_tokens: number
  cache_creation_tokens: number
  cache_read_tokens: number
  total_tokens: number
  total_cost: number
  request_count: number
}

export type UserCostRank = {
  user_id: number
  email: string
  username: string
  total_cost: number
  total_tokens: number
  request_count: number
}

export type DailyTrend = {
  date: string
  total_tokens: number
  total_cost: number
  request_count: number
}

export type ModelStat = {
  model: string
  total_tokens: number
  total_cost: number
  request_count: number
}

export type TodaySummary = {
  total_tokens: number
  total_cost: number
  total_requests: number
  active_users: number
}

export type AccountMonitor = {
  enabled: boolean
  group_id: number
  group_name: string
  total: number
  available: number
  rate_limited: number
  abnormal: number
}

export type HistoricalSummary = {
  enabled: boolean
  total_tokens: number
  total_cost: number
  total_requests: number
  since: string
  until: string
  days_covered: number
}

export type Snapshot = {
  generated_at: string
  timezone: string
  today_summary: TodaySummary
  token_ranking: UserTokenRank[]
  cost_ranking: UserCostRank[]
  seven_day_trend: DailyTrend[]
  model_breakdown: ModelStat[]
  account_monitor: AccountMonitor
  historical: HistoricalSummary
}
