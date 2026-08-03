// fc-proxy 是一个本地 HTTP 反向代理：按配置注入鉴权请求头、改写响应头，
// 用于在浏览器中直接访问阿里云 FC 等需要固定 Token 鉴权的私人 HTTP 服务。
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fc-proxy/internal/config"
	"fc-proxy/internal/proxy"
)

func main() {
	configPath := flag.String("config", "proxy.yaml", "规则配置文件路径")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           proxy.NewHandler(cfg),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		log.Println("正在退出...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	log.Printf("监听 %s -> %s://%s", cfg.Listen, cfg.Target.Scheme, cfg.Target.Host)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("服务启动失败: %v", err)
	}
}
