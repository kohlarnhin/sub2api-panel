package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/zhujiangyong/sub2api-panel/backend/internal/stats"
)

// StatsHandler 暴露 REST 风格的统计接口。
type StatsHandler struct {
	svc *stats.Service
}

func NewStatsHandler(svc *stats.Service) *StatsHandler {
	return &StatsHandler{svc: svc}
}

// Snapshot GET /api/stats/snapshot 一次性返回完整聚合结果。
func (h *StatsHandler) Snapshot(c *gin.Context) {
	snap, err := h.svc.Snapshot(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, snap)
}

// TodaySummary GET /api/stats/today/summary
func (h *StatsHandler) TodaySummary(c *gin.Context) {
	snap, err := h.svc.Snapshot(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, snap.TodaySummary)
}

// TokenRanking GET /api/stats/today/tokens
func (h *StatsHandler) TokenRanking(c *gin.Context) {
	snap, err := h.svc.Snapshot(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, snap.TokenRanking)
}

// CostRanking GET /api/stats/today/cost
func (h *StatsHandler) CostRanking(c *gin.Context) {
	snap, err := h.svc.Snapshot(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, snap.CostRanking)
}

// SevenDayTrend GET /api/stats/trend/7days
func (h *StatsHandler) SevenDayTrend(c *gin.Context) {
	snap, err := h.svc.Snapshot(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, snap.SevenDayTrend)
}

// ModelBreakdown GET /api/stats/today/models
func (h *StatsHandler) ModelBreakdown(c *gin.Context) {
	snap, err := h.svc.Snapshot(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, snap.ModelBreakdown)
}

// AccountMonitor GET /api/stats/accounts/monitor
func (h *StatsHandler) AccountMonitor(c *gin.Context) {
	snap, err := h.svc.Snapshot(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, snap.AccountMonitor)
}

// Historical GET /api/stats/historical
func (h *StatsHandler) Historical(c *gin.Context) {
	snap, err := h.svc.Snapshot(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, snap.Historical)
}
