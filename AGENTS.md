# AGENTS.md

本文件面向 AI 编码代理，介绍本项目的结构、约定与常用命令。

## 项目概述

**fc-proxy** 是一个用 Go 编写的单文件配置驱动的本地 HTTP 反向代理，主要用途是把本地端口转发到阿里云函数计算（FC）的 HTTP 触发器，并在转发过程中按规则改写请求/响应头，典型场景：

- 请求侧：给所有请求注入固定的 `Authorization: Bearer <token>` 鉴权头（本地浏览器免登录访问 FC）。
- 响应侧：把 FC 默认的 `Content-Disposition` 改写为 `inline`，让浏览器直接预览文件而非下载。

## 技术栈与运行方式

- 语言：Go（`go.mod` 声明 `go 1.26.4`，模块名 `fc-proxy`）。
- 依赖：仅 `gopkg.in/yaml.v3` 一个第三方依赖，其余全部使用标准库（`net/http/httputil` 的 `ReverseProxy` 实现转发）。
- 运行时：单进程 HTTP 服务器，监听本地地址，转发到配置的上游（默认 HTTPS，强制 TLS 1.2+，SNI 可单独配置）。
- 部署：无 CI/CD、无容器化，直接 `go build` 出单个二进制 `fc-proxy` 运行；支持 SIGINT/SIGTERM 优雅退出（5 秒超时）。

## 代码结构

标准 Go 布局，入口与内部包分离：

- `cmd/fc-proxy/main.go` — 入口：解析 `-config` 参数（默认 `proxy.yaml`），加载配置，启动 HTTP 服务，处理信号优雅退出。
- `internal/config/config.go` — 配置模型（`Config`/`Target`）与 `Load`：读取并校验 YAML、填充默认值（scheme 默认 https、host_header/sni 默认等于 host）、预编译所有规则正则。
- `internal/rules/rules.go` — 规则引擎：`Rule` 类型（含 yaml 标签与 `Compile`/`Match` 方法）、`MatchFirst` 按顺序匹配 URL path（第一条命中即生效，无 `path_regex` 的是兜底规则）、`Apply` 先删 `delete_headers` 再覆盖 `set_headers`。
- `internal/proxy/proxy.go` — `NewHandler` 基于 `httputil.ReverseProxy` 构建代理：改写 scheme/host/Host 头、应用请求规则（`Rewrite`）与响应规则（`ModifyResponse`）；`NewHandlerWithTransport` 供测试注入 Transport。
- 各包内的 `*_test.go` — 与源文件对应的测试（见下文）。
- `configs/config.example.yaml` — 配置模板，注释即文档，修改配置字段时需同步更新。
- `configs/personal-website.yaml` — 作者自用的真实配置（含真实 token），已被 `.gitignore` 排除，不要当作测试 fixture 修改或提交变更。
- `.gitignore` — 参考 GitHub 官方 Go 模板：忽略编译产物、测试/覆盖率输出、IDE 与 macOS 杂物；本地配置按模式忽略（`/proxy.yaml`、`/configs/*.yaml`），仅保留 `configs/config.example.yaml` 示例模板。

## 构建与测试命令

```bash
go build -o fc-proxy ./cmd/fc-proxy   # 构建二进制
go test ./...                         # 运行全部测试
go vet ./...                          # 静态检查
./fc-proxy -config configs/config.example.yaml   # 指定配置文件运行（默认 proxy.yaml）
```

## 编码约定

- 注释、日志、错误信息和测试断言消息均使用**中文**；标识符使用英文。
- 面向错误的处理风格：配置加载时做全部校验和正则预编译，运行期不再处理编译错误。
- 规则语义：`path_regex` 为空字符串表示兜底规则（内部表示为 `regex == nil`）；只有第一条命中的规则生效；`set_headers` 与 `delete_headers` 同名时以 set 为准。
- 修改配置结构时，同步更新 `configs/config.example.yaml` 的注释和本文件。

## 测试策略

使用标准库 `testing` + `net/http/httptest`，无外部测试框架：

- `internal/config/config_test.go` — 通过临时 YAML 文件测试默认值填充、必填项校验、非法 scheme/正则报错（辅助函数 `writeTempConfig`）。
- `internal/rules/rules_test.go` — 单元测试规则匹配顺序、兜底规则、先删后覆盖语义、nil 规则（辅助函数 `mustCompile`）。
- `internal/proxy/proxy_test.go` — 端到端测试：`httptest.NewTLSServer` 起假上游，`NewHandlerWithTransport` 注入信任测试证书的 Transport，验证请求头注入/删除、Host 头改写、响应头改写。
- 测试中手动构造 `[]rules.Rule` 时必须逐条调用 `Compile()` 预编译正则（各测试包内有 `mustCompile` 辅助函数）。

## 安全注意事项

- `configs/personal-website.yaml` 等本地配置包含真实 Bearer token，已被 `.gitignore` 排除；不要在代码、测试或提交中泄露这类凭据。
- 上游连接强制 TLS 1.2+ 并做证书校验（测试除外）；`scheme: http` 仅用于本地调试。
- 代理会把客户端请求头原样转发，规则里注入的鉴权头对本地监听地址上的任何调用者都生效——`listen` 应绑定到 `127.0.0.1` 等可信地址。
