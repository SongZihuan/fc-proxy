package config

import (
	"os"
	"path/filepath"
	"testing"

	"fc-proxy/internal/rules"
)

// writeTempConfig 把配置内容写入临时文件并返回路径。
func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "proxy.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadDefaults(t *testing.T) {
	path := writeTempConfig(t, `
listen: "127.0.0.1:8080"
target:
  host: "example.com"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Target.Scheme != "https" {
		t.Errorf("scheme 默认应为 https，得到 %q", cfg.Target.Scheme)
	}
	if cfg.Target.HostHeader != "example.com" {
		t.Errorf("host_header 默认应等于 host，得到 %q", cfg.Target.HostHeader)
	}
	if cfg.Target.SNI != "example.com" {
		t.Errorf("sni 默认应等于 host，得到 %q", cfg.Target.SNI)
	}
}

func TestLoadMissingRequired(t *testing.T) {
	path := writeTempConfig(t, "target:\n  host: example.com\n")
	if _, err := Load(path); err == nil {
		t.Error("缺少 listen 应报错")
	}

	path = writeTempConfig(t, "listen: 127.0.0.1:8080\n")
	if _, err := Load(path); err == nil {
		t.Error("缺少 target.host 应报错")
	}
}

func TestLoadInvalidScheme(t *testing.T) {
	path := writeTempConfig(t, `
listen: "127.0.0.1:8080"
target:
  host: "example.com"
  scheme: ftp
`)
	if _, err := Load(path); err == nil {
		t.Error("非法 scheme 应报错")
	}
}

func TestLoadInvalidRegex(t *testing.T) {
	path := writeTempConfig(t, `
listen: "127.0.0.1:8080"
target:
  host: "example.com"
request_rules:
  - path_regex: "([bad"
`)
	if _, err := Load(path); err == nil {
		t.Error("非法正则应报错")
	}
}

func TestLoadRulesCompiled(t *testing.T) {
	path := writeTempConfig(t, `
listen: "127.0.0.1:8080"
target:
  host: "example.com"
request_rules:
  - path_regex: "^/api"
    set_headers:
      Authorization: "Bearer t"
  - {}
response_rules:
  - set_headers:
      Content-Disposition: "inline"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if r := rules.MatchFirst(cfg.RequestRules, "/api/x"); r == nil || r.SetHeaders["Authorization"] != "Bearer t" {
		t.Errorf("/api/x 应命中第一条请求规则，得到 %+v", r)
	}
	if r := rules.MatchFirst(cfg.RequestRules, "/other"); r == nil {
		t.Error("/other 应命中兜底请求规则")
	}
	if r := rules.MatchFirst(cfg.ResponseRules, "/anything"); r == nil {
		t.Error("无 path_regex 的响应规则应作为兜底规则命中")
	}
}
