package stats

import (
	"context"
	"sync"
	"time"
)

// Service 在 Repository 之上加一层短 TTL 内存缓存，避免高并发或多个 SSE 客户端打爆数据库。
type Service struct {
	repo                  *Repository
	ttl                   time.Duration
	tz                    string
	accountMonitorGroupID int64

	mu        sync.Mutex
	snapshot  *Snapshot
	expiresAt time.Time
}

// NewService 构造 Service。ttl=0 表示禁用缓存。
// accountMonitorGroupID == 0 时关闭账号监控卡片。
func NewService(repo *Repository, ttl time.Duration, tz string, accountMonitorGroupID int64) *Service {
	return &Service{
		repo:                  repo,
		ttl:                   ttl,
		tz:                    tz,
		accountMonitorGroupID: accountMonitorGroupID,
	}
}

// Snapshot 返回最新一份完整聚合结果，命中缓存时直接返回。
func (s *Service) Snapshot(ctx context.Context) (*Snapshot, error) {
	s.mu.Lock()
	if s.snapshot != nil && time.Now().Before(s.expiresAt) {
		snap := s.snapshot
		s.mu.Unlock()
		return snap, nil
	}
	s.mu.Unlock()

	snap, err := s.build(ctx)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.snapshot = snap
	s.expiresAt = time.Now().Add(s.ttl)
	s.mu.Unlock()

	return snap, nil
}

// Invalidate 立即清除缓存，下次调用会重新查询。
func (s *Service) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot = nil
	s.expiresAt = time.Time{}
}

func (s *Service) build(ctx context.Context) (*Snapshot, error) {
	summary, err := s.repo.TodaySummary(ctx)
	if err != nil {
		return nil, err
	}
	tokenRank, err := s.repo.TodayTokenRanking(ctx)
	if err != nil {
		return nil, err
	}
	costRank, err := s.repo.TodayCostRanking(ctx)
	if err != nil {
		return nil, err
	}
	trend, err := s.repo.SevenDayTrend(ctx)
	if err != nil {
		return nil, err
	}
	models, err := s.repo.TodayModelBreakdown(ctx)
	if err != nil {
		return nil, err
	}
	monitor, err := s.repo.AccountMonitorFor(ctx, s.accountMonitorGroupID)
	if err != nil {
		return nil, err
	}
	historical, err := s.repo.HistoricalTotals(ctx)
	if err != nil {
		return nil, err
	}

	return &Snapshot{
		GeneratedAt:    time.Now().Format(time.RFC3339),
		Timezone:       s.tz,
		TodaySummary:   summary,
		TokenRanking:   tokenRank,
		CostRanking:    costRank,
		SevenDayTrend:  trend,
		ModelBreakdown: models,
		AccountMonitor: monitor,
		Historical:     historical,
	}, nil
}
