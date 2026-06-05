package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	AccountMonitorPlusShare = "plus-share"
	AccountMonitorFreeShare = "free-share"
)

type ServerConfig struct {
	Addr               string `mapstructure:"addr"`
	Timezone           string `mapstructure:"timezone"`
	SSEIntervalSeconds int    `mapstructure:"sse_interval_seconds"`
	CacheTTLSeconds    int    `mapstructure:"cache_ttl_seconds"`
	TopN               int    `mapstructure:"top_n"`

	// AccountMonitorGroups 保存一起刷新的 share 分组对应的 group_id。
	AccountMonitorGroups map[string]int64 `mapstructure:"account_monitor_groups"`

	// AccountMonitorGroupID 兼容旧配置；未配置 account_monitor_groups 时会作为 plus-share。
	AccountMonitorGroupID int64 `mapstructure:"account_monitor_group_id"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`

	// SSLMode 取值（lib/pq）：disable / require / verify-ca / verify-full
	SSLMode string `mapstructure:"sslmode"`

	MaxOpenConns           int `mapstructure:"max_open_conns"`
	MaxIdleConns           int `mapstructure:"max_idle_conns"`
	ConnMaxLifetimeMinutes int `mapstructure:"conn_max_lifetime_minutes"`
}

type Sub2APIConfig struct {
	BaseURL string `mapstructure:"base_url"`
	APIKey  string `mapstructure:"api_key"`
}

// FreemailConfig 配置 freemail (Cloudflare Email Worker) 邮箱 OTP 自动获取。
// Codex 登录绑定邮箱时，OpenAI 会把验证码发到 otp_mailbox，本服务通过
// worker 的 /api/emails 接口轮询读取。三项均配置后才启用自动获取，否则回退手动输入。
type FreemailConfig struct {
	// WorkerDomain 为 freemail worker 域名，可带或不带 https:// 前缀。
	WorkerDomain string `mapstructure:"worker_domain"`
	// Token 为 worker 的 Bearer Token。
	Token string `mapstructure:"token"`
	// OTPMailbox 为接收 OpenAI 验证码的统一收件箱地址。
	OTPMailbox string `mapstructure:"otp_mailbox"`
	// PollAttempts / PollIntervalSeconds 控制轮询次数与间隔（默认 60 次 × 2 秒）。
	PollAttempts        int `mapstructure:"poll_attempts"`
	PollIntervalSeconds int `mapstructure:"poll_interval_seconds"`
}

// IsConfigured 仅当域名、token、OTP 邮箱都配置后返回 true。
func (f FreemailConfig) IsConfigured() bool {
	return strings.TrimSpace(f.WorkerDomain) != "" &&
		strings.TrimSpace(f.Token) != "" &&
		strings.TrimSpace(f.OTPMailbox) != ""
}

type LogConfig struct {
	Level string `mapstructure:"level"`
}

type Config struct {
	Server           ServerConfig   `mapstructure:"server"`
	Database         DatabaseConfig `mapstructure:"database"`
	RegisterDatabase DatabaseConfig `mapstructure:"register_database"`
	Sub2API          Sub2APIConfig  `mapstructure:"sub2api"`
	Freemail         FreemailConfig `mapstructure:"freemail"`
	Log              LogConfig      `mapstructure:"log"`

	Location *time.Location `mapstructure:"-"`
}

// Load 从 path 读取配置文件。path 可以是带后缀的完整文件名或不带后缀。
func Load(path string) (*Config, error) {
	v := viper.New()

	if path == "" {
		path = "config.yaml"
	}
	v.SetConfigFile(path)

	v.SetEnvPrefix("PANEL")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	loc, err := time.LoadLocation(cfg.Server.Timezone)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone %q: %w", cfg.Server.Timezone, err)
	}
	cfg.Location = loc

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.addr", ":8088")
	v.SetDefault("server.timezone", "Asia/Shanghai")
	v.SetDefault("server.sse_interval_seconds", 5)
	v.SetDefault("server.cache_ttl_seconds", 5)
	v.SetDefault("server.top_n", 20)
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.sslmode", "require")
	v.SetDefault("database.max_open_conns", 10)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime_minutes", 30)
	v.SetDefault("register_database.port", 5432)
	v.SetDefault("register_database.sslmode", "require")
	v.SetDefault("register_database.max_open_conns", 10)
	v.SetDefault("register_database.max_idle_conns", 5)
	v.SetDefault("register_database.conn_max_lifetime_minutes", 30)
	v.SetDefault("freemail.poll_attempts", 60)
	v.SetDefault("freemail.poll_interval_seconds", 2)
	v.SetDefault("log.level", "info")
}

func (c *Config) validate() error {
	if err := c.Database.validate("database"); err != nil {
		return err
	}
	if c.RegisterDatabase.IsConfigured() {
		if err := c.RegisterDatabase.validate("register_database"); err != nil {
			return err
		}
	}
	if c.Server.SSEIntervalSeconds <= 0 {
		return fmt.Errorf("server.sse_interval_seconds must be > 0")
	}
	if c.Server.TopN <= 0 || c.Server.TopN > 200 {
		return fmt.Errorf("server.top_n must be in (0, 200]")
	}
	for share, groupID := range c.Server.AccountMonitorGroups {
		if share != AccountMonitorPlusShare && share != AccountMonitorFreeShare {
			return fmt.Errorf("server.account_monitor_groups contains unsupported share %q", share)
		}
		if groupID < 0 {
			return fmt.Errorf("server.account_monitor_groups.%s must be >= 0", share)
		}
	}
	return nil
}

func (s ServerConfig) AccountMonitorGroupMap() map[string]int64 {
	groups := make(map[string]int64, len(s.AccountMonitorGroups))
	for share, groupID := range s.AccountMonitorGroups {
		groups[share] = groupID
	}

	if groups[AccountMonitorPlusShare] == 0 && s.AccountMonitorGroupID > 0 {
		groups[AccountMonitorPlusShare] = s.AccountMonitorGroupID
	}

	return groups
}

func (d DatabaseConfig) IsConfigured() bool {
	return d.Host != "" ||
		d.User != "" ||
		d.Password != "" ||
		d.DBName != ""
}

func (d DatabaseConfig) validate(section string) error {
	if d.Host == "" {
		return fmt.Errorf("%s.host is required", section)
	}
	if d.User == "" {
		return fmt.Errorf("%s.user is required", section)
	}
	if d.DBName == "" {
		return fmt.Errorf("%s.dbname is required", section)
	}
	return nil
}

func (c *Config) RegisterDatabaseConfig() DatabaseConfig {
	if !c.RegisterDatabase.IsConfigured() {
		return c.Database
	}
	return c.RegisterDatabase
}

// DSN 生成 PostgreSQL DSN。
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode,
	)
}
