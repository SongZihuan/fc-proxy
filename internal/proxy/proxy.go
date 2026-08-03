// Package proxy 基于 httputil.ReverseProxy 实现到上游的反向代理，
// 转发时改写 scheme/host/Host 头并应用请求规则，响应返回前应用响应规则。
package proxy

import (
	"crypto/tls"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"fc-proxy/internal/config"
	"fc-proxy/internal/rules"
)

// NewHandler 按配置构建反向代理。
func NewHandler(cfg *config.Config) http.Handler {
	return NewHandlerWithTransport(cfg, &http.Transport{
		TLSClientConfig: &tls.Config{
			ServerName: cfg.Target.SNI,
			MinVersion: tls.VersionTLS12,
		},
		ForceAttemptHTTP2: true,
	})
}

// NewHandlerWithTransport 与 NewHandler 相同，但允许注入 Transport（用于测试）。
func NewHandlerWithTransport(cfg *config.Config, transport http.RoundTripper) http.Handler {
	targetURL := &url.URL{Scheme: cfg.Target.Scheme, Host: cfg.Target.Host}

	rp := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(targetURL)
			r.Out.Host = cfg.Target.HostHeader
			rules.Apply(rules.MatchFirst(cfg.RequestRules, r.In.URL.Path), r.Out.Header)
			log.Printf("%s %s", r.In.Method, r.In.URL.RequestURI())
		},
		Transport: transport,
		ModifyResponse: func(resp *http.Response) error {
			rules.Apply(rules.MatchFirst(cfg.ResponseRules, resp.Request.URL.Path), resp.Header)
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("上游请求失败 %s %s: %v", r.Method, r.URL.RequestURI(), err)
			http.Error(w, "bad gateway: "+err.Error(), http.StatusBadGateway)
		},
	}
	return rp
}
