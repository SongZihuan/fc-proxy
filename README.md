# fc-proxy

一个单文件配置驱动的本地 HTTP 反向代理，用于在浏览器中直接访问需要固定 Token 鉴权的私人 HTTP 服务——典型场景是阿里云函数计算（FC）的 HTTP 触发器：

- **请求侧**：按规则注入 `Authorization: Bearer <token>` 等鉴权头，浏览器无需手动携带固定 Token。
- **响应侧**：按规则将 `Content-Disposition` 改为 `inline`，允许浏览器内联显示支持的内容。

仅依赖标准库 + `gopkg.in/yaml.v3`，编译产物是单个二进制。

## 快速开始

需要 Go 1.26.4 或更高版本。获取项目源码后，在项目根目录执行以下步骤。

1. 构建程序并复制配置模板：

   ```bash
   go build -o fc-proxy ./cmd/fc-proxy
   cp configs/config.example.yaml proxy.yaml
   ```

2. 编辑 `proxy.yaml`：将 `target.host` 替换为上游域名，将 `Bearer your-fixed-token` 中的占位值替换为实际 Token。`target.host` 不包含 `https://` 或路径。

3. 启动代理：

   ```bash
   ./fc-proxy -config proxy.yaml
   ```

4. 在浏览器中访问 `http://127.0.0.1:8080/`，或在地址后加上需要访问的路径。代理默认通过 HTTPS 转发到上游，并保留请求路径和查询参数。

不指定 `-config` 时默认读取当前目录的 `proxy.yaml`。按 `Ctrl+C` 停止代理。

## 配置说明

下面是与模板行为一致的最小配置：为所有路径注入 Token，并将响应的 `Content-Disposition` 设置为 `inline`。

```yaml
listen: "127.0.0.1:8080"

target:
  host: "your-service.example.com"

request_rules:
  - set_headers:
      Authorization: "Bearer your-fixed-token"

response_rules:
  - set_headers:
      Content-Disposition: "inline"
```

规则语义：

- 匹配对象是 URL path（不含 query），按配置顺序匹配，**只有第一条命中的规则生效**。
- `path_regex` 为空/省略 = 兜底规则，匹配一切路径，通常放在最后；非法正则在启动时即报错。
- 每条规则先执行 `delete_headers` 再执行 `set_headers`，同名头以 set 为准。
- 修改配置后重启生效（暂不支持热加载）。

Host、TLS SNI 和删除头部等选项见带注释的 [配置模板](configs/config.example.yaml)。

## 使用注意事项

- 每份配置只转发到一个上游，规则用于改写头部，不用于选择不同上游。
- 含真实 token 的配置文件不要提交：`.gitignore` 已忽略 `/proxy.yaml` 和 `/configs/*.yaml`（示例模板除外）。
- 上游连接强制 TLS 1.2+ 并校验证书；`scheme: http` 仅用于本地调试。
- 注入的鉴权头对监听地址上的任何调用者都生效，`listen` 请绑定 `127.0.0.1` 等可信地址。

## 许可证

[MIT License](LICENSE) © 2026 ZIHUAN SONG
