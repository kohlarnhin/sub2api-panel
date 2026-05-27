package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/zhujiangyong/sub2api-panel/backend/internal/stats"
)

// SSEHandler 通过 Server-Sent Events 定时推送最新的统计快照。
type SSEHandler struct {
	svc      *stats.Service
	interval time.Duration
	logger   *zap.Logger
}

func NewSSEHandler(svc *stats.Service, interval time.Duration, logger *zap.Logger) *SSEHandler {
	return &SSEHandler{svc: svc, interval: interval, logger: logger}
}

// Stream GET /api/stats/stream
//
// 推送两类事件：
//   - event: snapshot   每个 interval 推送一次最新聚合结果
//   - event: ping       心跳，避免代理超时断连
func (h *SSEHandler) Stream(c *gin.Context) {
	w := c.Writer
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // 兼容 nginx 关闭缓冲

	flusher, ok := w.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming unsupported"})
		return
	}

	ctx := c.Request.Context()

	// 连接建立后立即推送一次
	if err := h.pushSnapshot(ctx, w, flusher); err != nil {
		h.logger.Warn("sse initial push failed", zap.Error(err))
		return
	}

	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	// 心跳间隔：取 SSE 间隔与 15s 中的较小者
	heartbeat := h.interval
	if heartbeat > 15*time.Second {
		heartbeat = 15 * time.Second
	}
	heartbeatTicker := time.NewTicker(heartbeat)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := h.pushSnapshot(ctx, w, flusher); err != nil {
				h.logger.Warn("sse push failed", zap.Error(err))
				return
			}
		case <-heartbeatTicker.C:
			if _, err := fmt.Fprintf(w, "event: ping\ndata: %d\n\n", time.Now().Unix()); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (h *SSEHandler) pushSnapshot(ctx context.Context, w http.ResponseWriter, flusher http.Flusher) error {
	snap, err := h.svc.Snapshot(ctx)
	if err != nil {
		// 失败时也告诉客户端，不立刻断开
		errPayload, _ := json.Marshal(map[string]string{"error": err.Error()})
		_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", errPayload)
		flusher.Flush()
		return err
	}

	payload, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	if _, err := fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", payload); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
