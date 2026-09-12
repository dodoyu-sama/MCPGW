package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Transport TransportConfig `yaml:"transport"`
	Audit     AuditConfig     `yaml:"audit"`
	Masking   MaskingConfig   `yaml:"masking"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Admin     AdminConfig     `yaml:"admin"`
}

type ServerConfig struct {
	ClientID string   `yaml:"client_id"`
	Upstream []string `yaml:"upstream"` // stdio upstream command + args
}

// TransportConfig selects how MCPGW talks to the MCP client and to the upstream
// MCP server. "stdio" spawns/uses a subprocess; "sse" uses HTTP Server-Sent
// Events (the MCP remote transport).
type TransportConfig struct {
	Client      string `yaml:"client"`       // stdio | sse  (how clients connect to MCPGW)
	Listen      string `yaml:"listen"`       // address for the SSE server, e.g. ":8081"
	Upstream    string `yaml:"upstream"`     // stdio | sse  (how MCPGW connects to the real server)
	UpstreamURL string `yaml:"upstream_url"` // the upstream /sse endpoint, when upstream = sse
}

type AuditConfig struct {
	Enabled bool   `yaml:"enabled"`
	Driver  string `yaml:"driver"` // sqlite
	DSN     string `yaml:"dsn"`    // ./mcpgw.db
}

type MaskingConfig struct {
	Enabled bool       `yaml:"enabled"`
	Rules   []MaskRule `yaml:"rules"`
}

type MaskRule struct {
	Name     string   `yaml:"name"`
	Patterns []string `yaml:"patterns"` // regex
	Fields   []string `yaml:"fields"`   // field-name match
	MaskChar string   `yaml:"mask_char"`
}

type RateLimitConfig struct {
	Enabled    bool    `yaml:"enabled"`
	QPS        float64 `yaml:"qps"`
	DailyQuota int     `yaml:"daily_quota"`
}

type AdminConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
	Token   string `yaml:"token"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	applyDefaults(&c)
	return &c, nil
}

func Default() *Config {
	c := Config{}
	applyDefaults(&c)
	return &c
}

func applyDefaults(c *Config) {
	if c.Audit.Driver == "" {
		c.Audit.Driver = "sqlite"
	}
	if c.Audit.DSN == "" {
		c.Audit.DSN = "./mcpgw.db"
	}
	if c.Server.ClientID == "" {
		if v := os.Getenv("MCPGW_CLIENT_ID"); v != "" {
			c.Server.ClientID = v
		} else {
			c.Server.ClientID = "default"
		}
	}
	if c.Transport.Client == "" {
		c.Transport.Client = "stdio"
	}
	if c.Transport.Listen == "" {
		c.Transport.Listen = ":8081"
	}
	if c.Transport.Upstream == "" {
		c.Transport.Upstream = "stdio"
	}
	if c.Admin.Port == 0 {
		c.Admin.Port = 8080
	}
	if c.RateLimit.QPS == 0 {
		c.RateLimit.QPS = 10
	}
}
