# DWS 沙箱免登 Sidecar 技术方案（修订版）

> 状态：MCP Phase 1 MVP 已实现，等待评审与真实账号联调；Schema risk 自动判定保留在 Phase 2
> 目标版本：协议 `v1` / MCP 主链路 MVP
> 适用范围：不可信沙箱与可信宿主位于同一物理机，沙箱必须使用钉钉用户身份，但用户 token 不得进入沙箱
> 参考实现：飞书开源 CLI 的 auth sidecar；本文结合 DWS 的多 profile、双认证头和 Runtime Schema 安全元数据进行了收敛，不照搬其 demo 设计

## 1. 结论

DWS 已有三种“免交互登录”基础能力：file-DEK、认证包导入导出、应用凭证。但前两种仍会把用户凭证材料放进沙箱，应用凭证又无法覆盖所有必须使用用户身份的命令。

对于“必须是用户身份，并且 token 不得进入沙箱”的场景，建议增加同机 `dws-auth-sidecar`：

- 沙箱版 `dws` 不读取 Keychain、DEK、access token 或 refresh token，只持有一把可撤销的 HMAC capability key；
- 可信侧 Sidecar 根据 `keyId` 将该 key 固定绑定到一个精确的 DWS profile（`corpId:userId`）；
- 沙箱请求经 HMAC 验签、防重放和工具权限检查后，由 Sidecar 获取/刷新真实 token，注入 DWS 所需的两个用户认证头，再转发到钉钉 MCP 服务；
- 默认只允许 Unix Domain Socket，同机容器无法共享 socket 时才显式启用 loopback/same-host HTTP；
- MVP 只覆盖 DWS MCP HTTP 主链路，不覆盖 raw API、插件自定义端点、stdio MCP、event、skill、PAT 授权和凭证管理命令。

这能解决“token 泄露”和“refresh token 多沙箱轮转冲突”，但不能阻止已经攻陷沙箱的进程使用其 key 调用被授权的工具。因此，key 的权限必须按 profile、endpoint、工具、有效期和速率收敛；后续再叠加 Schema risk 自动判定。它本质上是一张短期、可撤销的能力票据。

## 2. 方案选择边界

按以下顺序选择身份方案：

1. 命令支持应用身份：优先注入 `DWS_CLIENT_ID` / `DWS_CLIENT_SECRET`，不启用 Sidecar；
2. 只要求免扫码，不要求 token 隔离：使用已有凭证迁移、认证包或 `edition.TokenProvider`；
3. 必须使用用户身份，且 token 禁止进入沙箱：使用本文 Sidecar；
4. 沙箱与可信凭证服务跨物理机：不使用本文协议，单独设计带 mTLS、服务身份、集中审计的 Broker/STS。

Sidecar 不是通用出站代理，也不是跨机认证服务。

## 3. DWS 现状与飞书方案借鉴

### 3.1 DWS 可复用能力

| 能力 | 当前代码锚点 | 本方案用途 |
|---|---|---|
| 用户 token 获取、刷新和落盘 | `internal/auth/token.go` | 仅可信侧复用 |
| 多账号 profile | `internal/auth/profiles.go` | key 固定绑定 `corpId:userId` |
| 进程内 token 缓存 | `internal/app/access_token_resolve.go` | 改造成显式 profile 解析后供 Sidecar 使用 |
| MCP HTTP 客户端 | `internal/transport/client.go` | 沙箱侧拦截和改写请求 |
| endpoint 信任检查 | `internal/transport/client.go` | Sidecar 上游 host 白名单的基础 |
| Schema 安全元数据 | `internal/cli/schema_catalog/` | Phase 2 风险上限的服务端依据；MVP 尚未接入 |
| 敏感头日志脱敏 | `internal/logging/redact.go` | Sidecar 审计复用规则 |
| Edition token hook | `pkg/edition/edition.go` | 可返回哨兵 token，但不能单独实现 token 隔离 |

当前 MCP 请求会同时注入：

```http
Authorization: Bearer <user-access-token>
x-user-access-token: <user-access-token>
```

因此 Sidecar 不允许沙箱选择“把 token 注入哪个头”，而是在可信侧固定注入这两个头。这比直接复制飞书的 `authHeader` 可选协议更适合 DWS，也减少了把 token 改注入 Cookie 或其他可记录头的风险。

### 3.2 从飞书实现继承的设计

- 哨兵 credential provider：真实 token 不进入沙箱认证链；
- transport interceptor：只拦截带哨兵 token 的请求；
- HMAC-SHA256 覆盖目标、方法、路径、查询和 body 摘要；
- build tag 隔离 Sidecar 发行物；
- 只允许 loopback / same-host 通道；
- 上游 host 白名单、认证头剥离和审计日志脱敏；
- 未绑定身份时 fail closed，不回退到默认用户。

### 3.3 相比飞书 demo 的必要修订

| 飞书 demo 形态 | DWS 修订 |
|---|---|
| 服务端遍历所有 key 尝试验签 | 请求携带已签名的 `keyId`，服务端 O(1) 定位 key |
| 时间戳窗口内同一请求可重放 | 增加 128-bit nonce 和有界 replay cache |
| 客户端声明 user/bot identity | `keyId` 在服务端固定绑定精确 DWS profile，客户端不能选身份 |
| 客户端声明 token 注入头 | DWS 服务端固定注入 `Authorization` 和 `x-user-access-token` |
| 主要按 host/identity 控制 | MVP 增加 exact endpoint、MCP tool、TTL、速率和 body 上限；Phase 2 接入 Schema risk |
| HMAC key 可直接放环境变量 | 环境变量只放 key 文件路径；key 文件只读挂载并要求 `0600` |
| loopback HTTP 为默认 | DWS 默认 Unix socket，容器场景才启用 same-host HTTP |
| 多租户登录管理面使用共享 key | MVP 不向沙箱开放登录管理面；key/profile 由可信侧管理员预绑定 |

## 4. 信任模型与安全目标

### 4.1 信任边界

```text
┌──────────────── 不可信沙箱 ────────────────┐
│ sidecar 版 dws                              │
│  - 无 Keychain / DEK / access token         │
│  - 无 refresh token                         │
│  - 持有 keyId + HMAC capability key         │
│  - 生成 nonce、签名并改写 MCP 请求           │
└───────────────────┬─────────────────────────┘
                    │ Unix socket（默认）
                    │ 或显式 same-host HTTP
┌───────────────────▼─────────────────────────┐
│ 可信宿主：dws-auth-sidecar                   │
│  - keyId → profile / policy                 │
│  - 验签、防重放、解析 MCP method/tool        │
│  - 获取/刷新 profile token                  │
│  - 固定注入双认证头并转发                    │
│  - 记录脱敏审计日志                          │
└───────────────────┬─────────────────────────┘
                    │ HTTPS
                    ▼
              钉钉 MCP 服务
```

### 4.2 安全目标

- 沙箱内存、环境变量和文件系统中不出现真实 access token、refresh token、DEK 或认证包；
- 捕获请求后修改 key、目标、路径、query 或 body 会导致验签失败；
- 同一签名请求不能在时间窗口内成功执行两次；
- 一个 key 只能使用服务端绑定的唯一 profile，不能通过 `--profile` 或请求头切换用户；
- Sidecar 不能被用作任意 URL 的 SSRF 代理；
- Sidecar 只代答 policy 明确允许的 MCP 操作；
- key 可独立禁用、轮转和过期，不影响真实用户登录态。

### 4.3 明确不保证

- 攻陷沙箱的进程仍可在 key 有效期内调用 policy 允许的工具；
- Sidecar 不防御已经取得可信宿主 root/管理员权限的攻击者；
- same-host DNS 别名依赖容器运行时和宿主网络配置；高安全场景必须使用 Unix socket；
- 本方案不把本地命令的交互确认当作可信授权信号。恶意沙箱可以伪造命令参数，所以最终权限必须由 Sidecar policy 决定。

## 5. 运行与发行模型

### 5.1 两个二进制

| 产物 | 运行位置 | 说明 |
|---|---|---|
| `dws`（`-tags authsidecar`） | 沙箱 | 包含哨兵 provider 和 Sidecar RoundTripper |
| `dws-auth-sidecar` | 可信宿主 | 持有 DWS 凭证存储访问权并转发请求 |

标准 `dws` 不包含 Sidecar transport。若设置 `DWS_AUTH_MODE=sidecar`，标准构建必须返回 `sidecar_build_required`，不得回退本地 token。

Sidecar 构建也不是“设置失败就忽略”：配置缺项、地址非法、key 权限错误或连接失败都必须在发出上游请求前终止。

### 5.2 沙箱侧配置

```bash
export DWS_AUTH_MODE=sidecar
export DWS_AUTH_SIDECAR_ADDR=unix:///run/dws-sidecar/agent-42.sock
export DWS_AUTH_SIDECAR_KEY_ID=sbx_agent_42
export DWS_AUTH_SIDECAR_KEY_FILE=/run/secrets/dws-sidecar.key
```

配置必须全量出现并通过校验：

- key 为至少 32 个随机字节，建议用 64 位 hex 文件表示；
- key 文件必须为普通文件，Unix 下拒绝 group/other 可读写权限；
- 禁止把 key 内容直接放进命令行参数；
- `--token` 与 Sidecar 模式互斥；
- `--profile` 在 MVP 中拒绝使用，profile 只能由服务端 key binding 决定；
- Sidecar 服务端启动时若检测到 `DWS_AUTH_MODE=sidecar`，必须拒绝启动，防止递归代理。

### 5.3 通道约束

优先级如下：

1. `unix:///absolute/path.sock`：默认；socket 父目录 `0700`，socket 仅授权目标沙箱用户；
2. `http://127.0.0.1:<port>` / `http://[::1]:<port>`：本机进程；
3. `http://host.docker.internal:<port>` 等经过审核的 same-host 别名：仅容器无法共享 socket 时显式启用。

拒绝 HTTPS、userinfo、URL path、跨机 IP、通配地址和未经配置的 DNS 名。跨机安全不能靠“把 Sidecar 地址换成 HTTPS”解决。

## 6. Wire Protocol v1

### 6.1 请求头

| Header | 内容 |
|---|---|
| `X-DWS-Sidecar-Version` | 固定 `v1` |
| `X-DWS-Sidecar-Key-Id` | capability key 标识，不是用户身份 |
| `X-DWS-Sidecar-Target` | 原始 origin，例如 `https://mcp.dingtalk.com` |
| `X-DWS-Sidecar-Timestamp` | Unix 秒 |
| `X-DWS-Sidecar-Nonce` | 每次 HTTP attempt 新生成的 128-bit 随机 hex |
| `X-DWS-Sidecar-Body-SHA256` | 原始 body 的 SHA-256 hex |
| `X-DWS-Sidecar-Signature` | canonical request 的 HMAC-SHA256 hex |

不提供 `identity`、`profile`、`authHeader` 三个可变头。它们分别由 key binding 和 DWS 固定转发规则决定。

### 6.2 Canonical request

字段按下列顺序以 `\n` 拼接，所有字符串使用原始字节，不做二次 URL decode：

```text
DWS-AUTHSIDECAR/v1
keyId
timestamp
nonce
METHOD
targetOrigin
pathAndQuery
bodySHA256
```

约束：

- `METHOD` 转为大写；
- `targetOrigin` 只能是 `https://host[:port]`，禁止 path、query、fragment、userinfo；
- `pathAndQuery` 使用 Go `URL.RequestURI()` 的结果；
- timestamp 允许漂移 `±60s`；
- nonce 使用 CSPRNG 生成 16 字节并 hex 编码；
- 服务端在验签成功后，以 `(keyId, nonce)` 原子写入 replay cache；重复值返回 `409 replay_detected`；
- replay cache TTL 至少 120 秒并设置总容量上限，满载时 fail closed，不能通过淘汰新项绕过重放防护；
- key 查找必须通过 `keyId` O(1) 完成，不遍历 key 集合。

### 6.3 转发头规则

Sidecar 只转发明确白名单中的普通头，例如 `Content-Type`、`Accept`、`X-Cli-Source`、`X-Cli-Version` 和合法的 execution ID。它必须：

1. 删除沙箱请求中的 `Authorization`、`x-user-access-token`、Cookie、Proxy-Authorization 和全部 Sidecar 协议头；
2. 根据 key binding 获取真实用户 token；
3. 固定设置：

   ```http
   Authorization: Bearer <real-token>
   x-user-access-token: <real-token>
   ```

4. 使用独立的 `http.Transport` 直连上游，忽略 `DWS_AUTH_SIDECAR_ADDR`，并沿用 DWS 的 TLS 最低版本、超时、重定向敏感头剥离和目标域校验；
5. 回包时删除 Sidecar 内部响应头，不把上游 token 或认证诊断内容写入响应。

## 7. 身份绑定与权限模型

### 7.1 key binding

可信侧配置示例：

```json
{
  "version": 1,
  "bindings": [
    {
      "key_id": "sbx_agent_42",
      "key_file": "/etc/dws-sidecar/keys/sbx_agent_42.key",
      "profile": "dingcorp123:staff456",
      "expires_at": "2026-08-01T12:00:00+08:00",
      "enabled": true,
      "policy": "agent-read-write"
    }
  ]
}
```

硬性要求：

- `profile` 必须解析为精确的 `corpId:userId`，禁止只绑定 corpId、current profile、profile 别名或“第一个账号”；
- key 未找到、过期、禁用、profile 缺失或 token 刷新失败时 fail closed；
- 禁止在 profile 解析失败时回退 legacy token；
- key 文件、binding 配置和 token 存储均只在可信侧；
- MVP 在启动时完整校验配置；Phase 2 支持无需重启的安全 reload，并在校验新配置后原子替换内存快照。

### 7.2 policy

```json
{
  "name": "agent-read-write",
  "allowed_origins": ["https://mcp.dingtalk.com"],
  "allowed_paths": ["/server/<reviewed-server-id>"],
  "allowed_tools": [
    "doc.get_document",
    "doc.create_document",
    "drive.search_files",
    "calendar.list_events"
  ],
  "requests_per_minute": 60,
  "max_body_bytes": 1048576
}
```

服务端必须从已验签的 JSON-RPC body 解析操作，不能信任客户端额外声明的 tool/risk：

| JSON-RPC method | MVP 规则 |
|---|---|
| `initialize`、`notifications/initialized`、`tools/list` | origin 和 exact endpoint path 允许时放行 |
| `tools/call` | 解析真实 tool name，并满足 exact allowlist |
| 其他 method | 默认拒绝 |

MVP 以服务端 exact endpoint path + exact tool allowlist 为最终授权依据，未知 endpoint 和工具一律拒绝。Phase 2 再由 Sidecar 进程内嵌的已发布 Schema Catalog 解析风险等级和确认要求，并与 exact allowlist 同时生效；Catalog 缺失或风险元数据不一致时按最高风险处理。CLI 本地的 `--yes`、确认结果或客户端传入的 risk 字段均不能提升权限。

建议默认 policy 仅开放只读工具。写操作按 exact tool 显式加入；删除、授权、发消息、发邮件等高影响操作单独使用更短 TTL 的 key，MVP 可以直接不开放 destructive 风险等级。

## 8. DWS 客户端执行流

1. 启动阶段校验 Sidecar build 和四项配置；
2. sidecar credential provider 返回内部哨兵值 `sidecar-managed-user-token`，不访问 token marker、profile 文件或 Keychain；
3. `internal/transport.Client` 按现状构造 MCP 请求并写入两个哨兵认证头；
4. Sidecar RoundTripper 仅识别两个头都携带正确哨兵值的请求；不带哨兵的请求在 MVP 中不代理；
5. RoundTripper 缓冲并恢复 body，生成 timestamp/nonce/body hash，签名后剥离哨兵头；
6. 保存原始 target origin，将 URL 改写为 Sidecar socket/地址；
7. Sidecar 完成验签、重放检查、ACL、profile token 解析和上游转发；
8. 客户端收到上游响应。Sidecar 自身拒绝时，通过稳定错误码返回，不伪装成上游 PAT 或 OAuth 错误。

每个 HTTP retry 都必须生成新 nonce 和签名。不能复用第一次 attempt 的签名，否则正常重试会被 replay cache 拒绝。

## 9. 服务端 token 解析改造

当前 `TokenManager.Get` 的 cache key 包含 profile，但 profile 来源仍是全局 `auth.RuntimeProfile()`。Sidecar 会并发服务多个 key/profile，不能通过 `SetRuntimeProfile` 在请求间切换，否则会产生串号竞态。

需要增加显式 profile API，例如：

```go
ResolveAccessTokenForProfile(
    ctx context.Context,
    configDir string,
    selector string,
) (AccessTokenSnapshot, error)
```

要求：

- selector 在调用入口已经是精确 `corpId:userId`；
- token cache 继续按 `(configDir, exactProfile)` 隔离；
- refresh token 的读取、轮转、发布 marker 和 profile 更新保持现有原子语义；
- Sidecar 并发解析不同 profile 时不修改任何进程全局 profile；
- 该 API 只返回给可信侧调用代码，不进入沙箱构建的实际执行路径。

## 10. MVP 命令范围

| 命令类别 | MVP | 说明 |
|---|---|---|
| `dws mcp ...` / 映射到 MCP HTTP 的产品命令 | 支持 | 必须通过 endpoint + tool ACL |
| `help`、`version`、`schema` 等无认证本地命令 | 支持 | 不经过 Sidecar |
| `auth`、`profile` | 拒绝 | 沙箱不能管理可信侧凭证或选择身份 |
| `api` 原始 HTTP | 拒绝 | 任意 path 难以做稳定工具级授权，后续单独评审 |
| 其他本地 utility（如 `doctor`、`config`、`upgrade`） | 拒绝 | MVP 只保留明确列出的无认证本地命令 |
| plugin 自定义 endpoint | 拒绝 | 上游和认证语义不可控 |
| stdio MCP | 拒绝 | 不经过 HTTP Sidecar |
| event 长连接 | 拒绝 | 生命周期、重连和凭证注入路径不同 |
| skill 安装/执行专用上传链路 | 拒绝 | 存在独立 HTTP client，需要逐条建模 |
| PAT 授权、自动 chmod | 拒绝 | 权限提升不能由不可信沙箱发起 |

“拒绝”必须是明确的 `sidecar_command_unsupported` 或 `sidecar_policy_denied`，不能静默回落现有本地凭证链。

## 11. 代码结构

当前实现落点：

```text
internal/authsidecar/
  protocol.go                 # header、canonical request、签名、地址校验
  protocol_test.go
  config.go                   # 环境变量、key/binding/policy 和地址校验
  client_mode_*.go            # build-tag 哨兵凭证与 fail-closed
  client_transport_*.go       # build-tag Sidecar RoundTripper
  replay_cache.go             # 服务端 nonce cache
  handler.go                  # binding/ACL、验签、注入、转发、脱敏审计
  token_resolver.go           # 可信侧精确 profile token resolver

internal/app/
  access_token_resolve.go     # 增加显式 profile token 解析 seam
  root.go                     # 启动校验、命令范围和插件禁用

cmd/
  dws-auth-sidecar/main.go    # 可信侧 server 入口
```

协议、policy 和服务端 handler 应放在不依赖 Cobra 的内层包。客户端 build-tag 文件只负责注册 provider/transport；不得复制整套命令树。

## 12. 实施阶段

### Phase 0：协议与边界测试

- 固化 header、canonical string、地址校验和错误码；
- 完成跨语言 test vectors：空 body、query、IPv6、重复 header、大小写方法；
- 完成 nonce cache 并发/容量测试；
- 确认 MCP endpoint 到 server/tool 的可信解析方式。

### Phase 1：单 profile MCP MVP

- sidecar build-tag client、哨兵 provider、RoundTripper；
- 单个 keyId 固定绑定单个精确 profile；
- Unix socket server；
- 固定 host/tool allowlist；
- 双认证头注入、token 自动刷新、结构化审计；
- 标准构建和所有半配置场景 fail closed。

### Phase 2：策略治理与运维面

- 配置热加载、key 安全轮转和 binding 生命周期审计；
- Schema risk、细粒度配额和 hard deny；
- same-host 容器链路的部署模板与集成测试；
- 运维命令：创建/吊销 key、查看 binding、健康检查和审计查询。

### Phase 3：按需扩面

仅在逐类建立独立威胁模型和 ACL 后，评估 raw API、event 或其他 HTTP client。跨机 Broker 不进入本项目 Phase 3。

## 13. 测试与验收

### 13.1 安全测试

- [ ] 沙箱进程环境、打开文件和 heap dump 中不存在真实 token/DEK；
- [ ] 修改 target、path、query、body、keyId、timestamp 或 nonce 后验签失败；
- [ ] 相同 `(keyId, nonce)` 首次成功、再次请求稳定返回 `replay_detected`；
- [ ] 未知、过期、禁用 key 均拒绝；
- [ ] key A 无法选择或回退到 key B 的 profile；
- [ ] `--profile`、`--token`、半配置、标准构建启用 Sidecar 全部 fail closed；
- [ ] 非白名单 host、endpoint path、tool、JSON-RPC method 均拒绝；Phase 2 增加风险等级拒绝测试；
- [ ] 客户端自带认证头、Cookie 和 Sidecar 头不会被转发；
- [ ] 跨 host redirect 不携带真实认证头；
- [ ] replay cache 满载时不放行未记录 nonce；
- [ ] 日志、错误、trace 和 recovery snapshot 不含 key、token、完整用户 ID 或敏感路径参数。

### 13.2 功能测试

- [ ] 允许的 `initialize`、`tools/list`、`tools/call` 正常工作；
- [ ] access token 过期后只在可信侧刷新，沙箱无感；
- [ ] 两个 profile 并发高频调用不串号；
- [ ] HTTP retry 使用新 nonce，且上游幂等/重试语义与现状一致；
- [ ] Unix socket 和批准的 same-host 容器链路通过集成测试；
- [ ] Sidecar 停止、超时、policy deny 和 token refresh 失败均返回可区分错误码。

### 13.3 仓库验证

```bash
make build-auth-sidecar
make test-auth-sidecar
DWS_PACKAGE_VERSION=0.0.0-test go test ./...
```

若命令可见性或 Schema 元数据发生变化，还需按仓库规则重新生成并检查 Schema Catalog；协议与 Sidecar policy 不得手工改写生成产物。

## 14. 审计与运维

每次请求至少记录：

- 时间、request ID、keyId 的不可逆短 hash；
- profile 的不可逆短 hash；
- target server、JSON-RPC method、canonical tool ID；
- policy 名称、决策、拒绝原因；
- 上游状态码、耗时、重试次数；
- token refresh 是否发生，但绝不记录 token 内容。

key 生命周期操作必须可审计：创建、加载、轮转、禁用、过期和删除。紧急处置只需禁用 key binding，不删除用户 profile，也不破坏宿主机正常 `dws` 登录态。

## 15. 仍需评审的决策

1. Unix socket 是否能被目标沙箱编排器稳定挂载；若不能，哪些 same-host alias 可以进入正式白名单；
2. MCP endpoint/server/tool 的服务端可信映射，是复用现有 discovery 快照，还是维护更小的 Sidecar policy 映射；
3. Schema risk 是否足以作为统一风险上限，哪些工具必须额外 hard deny；
4. Sidecar server 是否作为 DWS 官方发行产物，还是先提供实验性 build tag；
5. key 管理命令是否进入 `dws sidecar admin ...`，以及管理员认证和文件权限模型。

以上问题不改变核心安全边界：真实 token 只在可信侧、key 固定绑定 profile、请求必须防重放、授权由 Sidecar 服务端决定、任何错误都不得回退本地登录态。
