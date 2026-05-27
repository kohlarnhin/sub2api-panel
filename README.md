# sub2api-panel

一个独立的、**只读**的消耗统计仪表盘，配套 [sub2api](./sub2api/) 使用。

直接连接 sub2api 的 PostgreSQL，只查询 `usage_logs` 和 `users` 等表，不做任何写入操作。

## 功能

- 今日整体汇总（总 Token / 总消费 / 总请求 / 活跃用户）
- 今日用户 **Token 排行榜**（按 input + output + cache 总和）
- 今日用户 **消费金额排行榜**（按 `actual_cost`）
- **近 7 天趋势图**（柱状 Token + 折线金额）
- 今日 **模型用量占比**（环形 + 明细表）
- 通过 **SSE** 定时推送（默认 5 秒），前端实时刷新

## 目录结构

```
sub2api-panel/
├── backend/         # Go + Gin + PostgreSQL（只读）
│   ├── cmd/server/main.go
│   ├── internal/{config,db,stats,handler,router}/
│   ├── config.yaml.example
│   └── go.mod
└── frontend/        # Vite + React + TypeScript + Tailwind + Recharts
    ├── src/{components,hooks,lib}/
    └── package.json
```

## 1. 后端

### 配置

复制 `backend/config.yaml.example` 为 `backend/config.yaml`，按你的环境修改：

```yaml
server:
  addr: ":8088"
  timezone: "Asia/Shanghai"     # 用于"今日"/"近7天"按本地时间分桶
  sse_interval_seconds: 5       # SSE 推送间隔
  cache_ttl_seconds: 5          # 内存缓存 TTL，避免多客户端打爆数据库
  top_n: 20                     # 排行榜返回前 N 条
  account_monitor_group_id: 0   # 账号监控展示哪个分组（填 plus 分组的 group_id，0 = 关闭）

database:
  host: "127.0.0.1"
  port: 5432
  user: "sub2api"
  password: "your_password"
  dbname: "sub2api"
  sslmode: "require"

log:
  level: "info"
```

### 运行

```bash
cd backend
go mod tidy
go run ./cmd/server -config config.yaml
```

监听在 `http://127.0.0.1:8088`。

### 接口

所有接口都是 GET、只读、不需要认证：

| 路径 | 说明 |
|------|------|
| `/api/health` | 健康检查 |
| `/api/stats/snapshot` | 一次性返回所有聚合数据（一次 HTTP） |
| `/api/stats/stream` | **SSE** 流，定时推送 snapshot |
| `/api/stats/today/summary` | 今日整体汇总 |
| `/api/stats/today/tokens` | 今日 Token 排行（含 actual_cost 字段） |
| `/api/stats/today/cost` | 今日金额排行（保留兼容，UI 已合并到 token 排行） |
| `/api/stats/today/models` | 今日按模型占比 |
| `/api/stats/trend/7days` | 近 7 天每日聚合（无数据日补 0） |
| `/api/stats/accounts/monitor` | 指定分组的账号健康度（总量 / 可用 / 限流 / 异常） |
| `/api/stats/historical` | 全平台历史累计（来自 sub2api 自带 `usage_dashboard_daily`，未启用时返回 enabled=false） |

SSE 事件类型：

- `event: snapshot` — JSON 完整快照
- `event: ping` — 心跳（数据为 unix 时间戳）
- `event: error` — 查询出错（数据为 `{error: string}`）

## 2. 前端

### 安装与运行

```bash
cd frontend
pnpm install   # 或 npm install
pnpm dev       # 启动开发服务器，监听 5180
```

打开 [http://127.0.0.1:5180](http://127.0.0.1:5180)。

Vite 已配置 `/api` 代理到后端 `127.0.0.1:8088`（含 SSE 支持），改后端端口时同步修改 `frontend/vite.config.ts`。

### 生产构建

```bash
pnpm build       # 输出到 frontend/dist/
pnpm preview     # 本地预览构建产物
```

## 数据来源说明

| 字段 | 来源 |
|------|------|
| Token 总量 | `usage_logs.input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens` |
| 金额 | `usage_logs.actual_cost`（这是用户实际扣费金额，已考虑各种倍率） |
| 模型 | `COALESCE(NULLIF(requested_model, ''), model)` —— 客户端请求的模型名优先于上游真实模型名 |
| 用户名 | `users.username`，回退到 `users.email` |
| 时间分桶 | 按配置中的 `server.timezone` 在 PG 端 `AT TIME ZONE` 切分 |

## 性能注意

- 当前所有查询都直接 `SUM/GROUP BY` 跑在 `usage_logs` 上。`usage_logs` 表已有 `created_at` 和 `(user_id, created_at)`、`(group_id, created_at)` 等索引（见 sub2api 的迁移 010 / 062），今日范围扫描成本可控。
- 多个 SSE 客户端共用 `cache_ttl_seconds` 内的一次查询结果，不会因为浏览器开多个 tab 把数据库打爆。
- 如果 `usage_logs` 体量极大，可以考虑接 sub2api 自带的 `usage_dashboard_aggregation_tables`（迁移 034）等汇总表，本项目目前未使用以保持依赖最小化。
