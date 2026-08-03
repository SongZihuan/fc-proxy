# fc-proxy

一个单文件配置驱动的本地 HTTP 反向代理，用于在浏览器中直接访问需要固定 Token 鉴权的私人 HTTP 服务——典型场景是阿里云函数计算（FC）的 HTTP 触发器：

- **请求侧**：按规则注入 `Authorization: Bearer <token>` 等鉴权头，本地浏览器免登录访问 FC。
- **响应侧**：按规则改写 `Content-Disposition` 等响应头，让浏览器直接预览文件而非触发下载。

仅依赖标准库 + `gopkg.in/yaml.v3`，编译产物是单个二进制。

## 快速开始

```bash
# 构建
go build -o fc-proxy ./cmd/fc-proxy

# 准备配置（复制模板后填入真实域名和 token）
cp configs/config.example.yaml proxy.yaml

# 启动
./fc-proxy -config proxy.yaml
```

然后浏览器访问 `http://127.0.0.1:8080/...`，请求会以 HTTPS 转发到配置的目标服务器。

## 配置说明

```yaml
listen: "127.0.0.1:8080"       # 本地 HTTP 监听地址（必填）

target:
  host: "xxx.cn-shenzhen.fcapp.run"  # 目标域名或 IP（必填）
  scheme: https                # https（默认）| http（仅本地调试用）
  host_header: ""              # 请求头 Host，默认 = target.host
  sni: ""                      # TLS SNI，默认 = target.host（scheme=http 时忽略）

# 请求规则：按 URL path 顺序匹配，只命中第一条；无 path_regex 的是兜底规则
request_rules:
  - path_regex: "^/api"        # 可选；省略则匹配一切路径
    set_headers:               # 覆盖/新增请求头
      Authorization: "Bearer your-fixed-token"
    delete_headers: []         # 删除请求头

# 响应规则：同样按 path 匹配第一条
response_rules:
  - set_headers:
      Content-Disposition: "inline"
```

规则语义：

- 匹配对象是 URL path（不含 query），按配置顺序匹配，**只有第一条命中的规则生效**。
- `path_regex` 为空/省略 = 兜底规则，匹配一切路径；非法正则在启动时即报错。
- 每条规则先执行 `delete_headers` 再执行 `set_headers`，同名头以 set 为准。
- 修改配置后重启生效（暂不支持热加载）。

完整示例见 [configs/config.example.yaml](configs/config.example.yaml)。

## 安全提示

- 含真实 token 的配置文件不要提交：`.gitignore` 已忽略 `/proxy.yaml` 和 `/configs/*.yaml`（示例模板除外）。
- 上游连接强制 TLS 1.2+ 并校验证书；`scheme: http` 仅用于本地调试。
- 注入的鉴权头对监听地址上的任何调用者都生效，`listen` 请绑定 `127.0.0.1` 等可信地址。

## 开发

```bash
go test ./...    # 全部测试
go vet ./...     # 静态检查
```

项目结构：入口在 `cmd/fc-proxy`，内部包为 `internal/config`（配置加载）、`internal/rules`（规则引擎）、`internal/proxy`（反向代理）。更多约定见 [AGENTS.md](AGENTS.md)。

## 许可证

[MIT License](LICENSE) © 2026 ZIHUAN SONG
