package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/zhujiangyong/sub2api-panel/backend/internal/config"
	"github.com/zhujiangyong/sub2api-panel/backend/internal/db"
	"github.com/zhujiangyong/sub2api-panel/backend/internal/router"
	"github.com/zhujiangyong/sub2api-panel/backend/internal/stats"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	staticPath := flag.String("static", "", "path to frontend dist directory")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	logger, err := buildLogger(cfg.Log.Level)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := db.Open(ctx, cfg.Database)
	if err != nil {
		logger.Fatal("open database", zap.Error(err))
	}
	defer conn.Close()
	logger.Info("database connected",
		zap.String("host", cfg.Database.Host),
		zap.String("dbname", cfg.Database.DBName),
	)

	repo := stats.NewRepository(conn, cfg.Location, cfg.Server.TopN)
	svc := stats.NewService(
		repo,
		time.Duration(cfg.Server.CacheTTLSeconds)*time.Second,
		cfg.Location.String(),
		cfg.Server.AccountMonitorGroupMap(),
	)

	r := router.New(svc, time.Duration(cfg.Server.SSEIntervalSeconds)*time.Second, logger, *staticPath)

	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		// 不设 WriteTimeout：SSE 长连接需要无超时写入
	}

	go func() {
		logger.Info("server starting", zap.String("addr", cfg.Server.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server crashed", zap.Error(err))
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	logger.Info("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}
}

func buildLogger(level string) (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	if err := cfg.Level.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("invalid log level %q: %w", level, err)
	}
	cfg.OutputPaths = []string{"stdout"}
	cfg.ErrorOutputPaths = []string{"stderr"}
	return cfg.Build()
}
