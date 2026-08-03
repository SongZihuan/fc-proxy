// Package config 定义配置文件模型并负责加载、校验和默认值填充。
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"fc-proxy/internal/rules"
)

// Target 描述上游服务器。
type Target struct {
	Host       string `yaml:"host"`        // 目标域名或 IP（必填）
	Scheme     string `yaml:"scheme"`      // https（默认）| http
	HostHeader string `yaml:"host_header"` // 请求头 Host，默认 = Host
	SNI        string `yaml:"sni"`         // TLS SNI，默认 = Host（scheme=http 时忽略）
}

// Config 是整个 yaml 配置。
type Config struct {
	Listen        string       `yaml:"listen"` // 本地 HTTP 监听地址（必填）
	Target        Target       `yaml:"target"`
	RequestRules  []rules.Rule `yaml:"request_rules"`
	ResponseRules []rules.Rule `yaml:"response_rules"`
}

// Load 读取并校验配置文件，填充默认值，预编译所有正则。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	if cfg.Listen == "" {
		return nil, fmt.Errorf("listen 不能为空")
	}
	if cfg.Target.Host == "" {
		return nil, fmt.Errorf("target.host 不能为空")
	}

	if cfg.Target.Scheme == "" {
		cfg.Target.Scheme = "https"
	}
	if cfg.Target.Scheme != "https" && cfg.Target.Scheme != "http" {
		return nil, fmt.Errorf("target.scheme 只能是 http 或 https，得到 %q", cfg.Target.Scheme)
	}
	if cfg.Target.HostHeader == "" {
		cfg.Target.HostHeader = cfg.Target.Host
	}
	if cfg.Target.SNI == "" {
		cfg.Target.SNI = cfg.Target.Host
	}

	for i := range cfg.RequestRules {
		if err := cfg.RequestRules[i].Compile(); err != nil {
			return nil, fmt.Errorf("request_rules[%d]: %w", i, err)
		}
	}
	for i := range cfg.ResponseRules {
		if err := cfg.ResponseRules[i].Compile(); err != nil {
			return nil, fmt.Errorf("response_rules[%d]: %w", i, err)
		}
	}

	return &cfg, nil
}
