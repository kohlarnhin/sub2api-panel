package router

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/zhujiangyong/sub2api-panel/backend/internal/handler"
	"github.com/zhujiangyong/sub2api-panel/backend/internal/stats"
)

// New 构造 gin Engine，注册 /api/stats/* 路由。
func New(svc *stats.Service, sseInterval time.Duration, logger *zap.Logger) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(ginLogger(logger))

	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "OPTIONS"},
		AllowHeaders:     []string{"*"},
		ExposeHeaders:    []string{"Content-Type"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	statsH := handler.NewStatsHandler(svc)
	sseH := handler.NewSSEHandler(svc, sseInterval, logger)

	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		s := api.Group("/stats")
		s.GET("/snapshot", statsH.Snapshot)
		s.GET("/stream", sseH.Stream)
		s.GET("/today/summary", statsH.TodaySummary)
		s.GET("/today/tokens", statsH.TokenRanking)
		s.GET("/today/cost", statsH.CostRanking)
		s.GET("/today/models", statsH.ModelBreakdown)
		s.GET("/trend/7days", statsH.SevenDayTrend)
		s.GET("/accounts/monitor", statsH.AccountMonitor)
		s.GET("/historical", statsH.Historical)
	}

	return r
}

func ginLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Info("http",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
		)
	}
}
