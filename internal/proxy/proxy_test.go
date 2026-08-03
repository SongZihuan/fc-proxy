package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fc-proxy/internal/config"
	"fc-proxy/internal/rules"
)

// mustCompile 预编译规则正则，测试中手动构造 []rules.Rule 时必须调用。
func mustCompile(t *testing.T, rs []rules.Rule) []rules.Rule {
	t.Helper()
	for i := range rs {
		if err := rs[i].Compile(); err != nil {
			t.Fatalf("compile rule %d: %v", i, err)
		}
	}
	return rs
}

// TestProxyEndToEnd 起一个 TLS 假上游（模拟 FC），验证：
// 请求侧注入/删除头部与 Host 头生效；响应侧 Content-Disposition 被改写。
func TestProxyEndToEnd(t *testing.T) {
	var gotAuth, gotHost, gotSecret, gotPath string
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotHost = r.Host
		gotSecret = r.Header.Get("X-Secret")
		gotPath = r.URL.Path
		w.Header().Set("Content-Disposition", `attachment; filename="a.bin"`)
		w.Header().Set("X-Internal", "drop-me")
		io.WriteString(w, "hello")
	}))
	defer upstream.Close()

	upstreamHost := strings.TrimPrefix(upstream.URL, "https://")
	cfg := &config.Config{
		Listen: "127.0.0.1:0",
		Target: config.Target{
			Host:       upstreamHost,
			Scheme:     "https",
			HostHeader: "fc.example.com",
			// httptest 的测试证书覆盖了 example.com，用它作为 SNI 以通过主机名校验
			SNI: "example.com",
		},
		RequestRules: mustCompile(t, []rules.Rule{
			{
				PathRegex:     "^/private",
				SetHeaders:    map[string]string{"Authorization": "Bearer fixed-token"},
				DeleteHeaders: []string{"X-Secret"},
			},
			{SetHeaders: map[string]string{"Authorization": "Bearer fallback"}},
		}),
		ResponseRules: mustCompile(t, []rules.Rule{
			{
				SetHeaders:    map[string]string{"Content-Disposition": "inline"},
				DeleteHeaders: []string{"X-Internal"},
			},
		}),
	}

	// 代理的 Transport 信任假上游的证书（生产代码走系统根证书正常校验）
	pool := x509.NewCertPool()
	pool.AddCert(upstream.Certificate())
	proxy := httptest.NewServer(NewHandlerWithTransport(cfg, &http.Transport{
		TLSClientConfig: &tls.Config{ServerName: cfg.Target.SNI, RootCAs: pool},
	}))
	defer proxy.Close()

	req, _ := http.NewRequest("GET", proxy.URL+"/private/data?x=1", nil)
	req.Header.Set("X-Secret", "should-be-removed")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求代理失败: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if string(body) != "hello" {
		t.Errorf("响应体应为 hello，得到 %q", body)
	}
	if gotAuth != "Bearer fixed-token" {
		t.Errorf("上游应收到注入的鉴权头，得到 %q", gotAuth)
	}
	if gotSecret != "" {
		t.Errorf("X-Secret 应被删除，上游收到 %q", gotSecret)
	}
	if gotHost != "fc.example.com" {
		t.Errorf("上游 Host 头应为 fc.example.com，得到 %q", gotHost)
	}
	if gotPath != "/private/data" {
		t.Errorf("上游 path 应为 /private/data，得到 %q", gotPath)
	}
	if cd := resp.Header.Get("Content-Disposition"); cd != "inline" {
		t.Errorf("Content-Disposition 应被改写为 inline，得到 %q", cd)
	}
	if xi := resp.Header.Get("X-Internal"); xi != "" {
		t.Errorf("X-Internal 响应头应被删除，得到 %q", xi)
	}
}

// TestProxyFallbackRule 验证未命中带正则的规则时走兜底规则。
func TestProxyFallbackRule(t *testing.T) {
	var gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		io.WriteString(w, "ok")
	}))
	defer upstream.Close()

	cfg := &config.Config{
		Listen: "127.0.0.1:0",
		Target: config.Target{
			Host:       strings.TrimPrefix(upstream.URL, "http://"),
			Scheme:     "http",
			HostHeader: "example.com",
			SNI:        "example.com",
		},
		RequestRules: mustCompile(t, []rules.Rule{
			{PathRegex: "^/private", SetHeaders: map[string]string{"Authorization": "Bearer fixed-token"}},
			{SetHeaders: map[string]string{"Authorization": "Bearer fallback"}},
		}),
	}

	proxy := httptest.NewServer(NewHandler(cfg))
	defer proxy.Close()

	resp, err := http.Get(proxy.URL + "/public/page")
	if err != nil {
		t.Fatalf("请求代理失败: %v", err)
	}
	resp.Body.Close()

	if gotAuth != "Bearer fallback" {
		t.Errorf("/public 应走兜底规则，上游收到 %q", gotAuth)
	}
}
