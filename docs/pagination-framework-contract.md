# 分页框架契约与能力

- 状态：Implementing（框架 `PageLedger` 已落地，产品命令渐进接入中）
- 日期：2026-08-08
- 来源：从 IM 分页算法抽取；具体迁移计划见 [RFC-0004](rfcs/0004-im-pagination-and-recovery-rollout.md)

框架实现：[`internal/output/pagination_ledger.go`](../internal/output/pagination_ledger.go)。

## 0. Lark CLI 主干对照

以 2026-08-08 的 [`larksuite/cli` main](https://github.com/larksuite/cli/tree/main) 为准，Lark 已经把**传输循环**和**输出表达**分开，但还没有一个完整的跨产品分页状态机：

- [`internal/client/client.go`](https://github.com/larksuite/cli/blob/main/internal/client/client.go) 的
  `paginateLoop`、`PaginateAll` 与 `StreamPages` 负责逐页请求、`page_token` 注入、页间延迟、页数上限和 JSON 聚合/流式分发；
- [`shortcuts/common/pagination.go`](https://github.com/larksuite/cli/blob/main/shortcuts/common/pagination.go)
  统一兼容 `page_token` / `next_page_token`，并以 `has_more` 决定是否存在下一页；
- [`internal/output/envelope.go`](https://github.com/larksuite/cli/blob/main/internal/output/envelope.go)
  的 `meta.pagination` 输出 `complete/pages/items/next_token`。源码注释明确：`complete:true`
  **仅**表示观察到服务端耗尽，绝不扩大为搜索索引健康或业务数据全量；
- [`internal/output/emitter.go`](https://github.com/larksuite/cli/blob/main/internal/output/emitter.go)
  规定 JSON 信封携带分页元数据，table/pretty 以人读摘要表达，CSV/NDJSON 保持 stdout 只含记录并将诊断写入 stderr。

这就是应当借鉴的框架边界：**框架统一执行与表达，产品命令保留字段解析、业务投影、去重和数据可信度判断。**

但不能把 Lark 主干描述得比实际更强：当前 `paginateLoop` 不拒绝重复游标；`has_more` 缺失会按 Go 零值落为 `false`；第 2 页及以后请求失败会停止循环并返回先前聚合结果，而不是生成显式的逐页 `partial_failure`；合并结果还会丢弃可续跑 token。因此，以下 DWS 能力是有意加强，而非 Lark 已有能力：游标一致性 fail-closed、证据未知、可续翻 token、后续页失败保留成功页并非零退出。

| 能力 | Lark main 的真实行为 | DWS 分页规范 |
|---|---|---|
| 传输循环 | `paginateLoop` 统一请求、延迟和 `page_token` 注入 | 由产品调用；`PageLedger` 只记录并验证已观察到的事实 |
| JSON 输出位置 | `Envelope.Meta.Pagination` | `meta.pagination`，不污染业务 `data` |
| 耗尽语义 | `complete` 仅表示观察到 endpoint exhausted | 使用更窄、更直白的 `endpoint_exhausted`，不重新引入 `complete` |
| 页数上限 | `PaginationOptions.PageLimit` | 同样要求有界；产品可以声明更小的硬上限 |
| token 字段兼容 | 支持 `page_token` 和 `next_page_token` | 由产品 adapter 归一到不透明 `next_token` |
| 流格式纪律 | CSV/NDJSON stdout 仅记录；诊断在 stderr | 对齐 |
| 游标安全 | 当前不验证重复/停滞 token | 空 token、重复/循环 token、矛盾字段统一为 `pagination_inconsistent` |
| 证据未知 | 缺失 `has_more` 容易收敛为 false | 不输出 pagination meta，禁止假称耗尽 |
| 后续页失败 | 停止循环；没有统一 partial 结果 | `partial_failure` 保留成功页与失败/未知页，非零退出 |
| 自动重试 | 分页循环不自动重放 | 对齐；只给恢复证据，不擅自重试 |

因此，DWS 不是复制 Lark 的字段名，而是保留 Lark 的正确分层和窄耗尽语义，并补齐
“上游分页字段无法确认”与“第 N 页失败但前 N-1 页已经成功”的 Agent 可信表达。

## 1. 目标与边界

分页框架**不负责把所有 API 自动翻完**，也不判断搜索索引或业务目录是否完整。它负责：

1. 收集产品命令已经观察到的分页事实；
2. 验证这些事实没有自相矛盾；
3. 将“已耗尽、可继续、未知、中断”统一映射为结果、分页元数据、退出码和恢复信息；
4. 防止游标循环、无界翻页和“短页即完整”的误报。

产品适配层仍负责上游字段解析、下一页请求、稳定业务 ID 去重、时间范围/窗口策略，以及
业务数据是否可被视为可信完整。框架不能从空列表自动推导“数据不存在”。

## 2. 统一分页状态

框架只承认以下三种可对外声明的端点状态：

| 已观察到的证据 | `meta.pagination` | Agent 可得出的结论 |
|---|---|---|
| 服务端明确无后续页 | `endpoint_exhausted: true`，无 `next_token` | 当前 endpoint 已耗尽 |
| 服务端给出有效续页游标，或调用者明确只读取一页 | `endpoint_exhausted: false` + `next_token` | 可以按 token 继续 |
| 短页、旧协议、字段冲突或无法判定 | **不输出 `meta.pagination`**；业务 data 可标 `pagination_known:false` | 不能判断是否完整 |

`endpoint_exhausted:true` 和 `next_token` 互斥。新统一返回中禁止使用 `complete:true`；它常只表示
本地算法停止，不能代表服务端 endpoint 已耗尽。

## 3. 框架内部能力：PageLedger

框架提供一个不绑定任何 IM API 的分页事实账本。建议的内部模型：

```go
type PageLedger struct {
    Pages             int
    Items             int
    Continuation      string
    EndpointExhausted *bool // nil：证据不足，不序列化为 pagination meta
    StopReason        string
    Failures          []PageFailure
}
```

它必须提供以下能力：

- 记录每页成功、失败或未知终态，以及页号、游标和可恢复诊断；
- 将游标作为不透明值做空值、重复、循环和页数上限检查；
- 校验 `has_more`、游标、终态声明之间的一致性；
- 聚合页数与项目数，但不替产品猜测业务总数；
- 生成符合本契约的 `meta.pagination`，或在证据不足时明确不生成；
- 将分页中断映射到统一的 `success`、`partial_failure` 或 `failure`。

上游的 `hasMore`、`nextCursor`、`pageToken`、短页启发式等字段只能由产品适配器转换为
ledger 证据，不能直接泄漏为统一返回的控制语义。

## 4. 统一返回映射

| 页执行结果 | `outcome` | 数据与错误表达 | 进程退出 |
|---|---|---|---|
| 没有成功页，首/唯一页失败 | `failure` | typed error；不把空列表包装成成功 | 非零 |
| 成功读取且端点证据明确，或调用者明确只取一页 | `success` | 业务 data + authoritative pagination meta | 0 |
| 已读取至少一页，后续已知页失败 | `partial_failure` | 保留成功页面摘要；`failed:[{id:"page:<n>", error}]` | 非零 |
| 已读取至少一页，但后续终态未知 | `partial_failure` | 保留成功页面摘要；`unknown:[{id:"page:<n>", ...}]` | 非零 |
| 游标循环、`hasMore=true` 无游标、内外层字段冲突 | `failure` | `pagination_inconsistent` | 非零 |

页不是业务对象。因此 `page:<n>` 仅用于说明恢复边界；成功页摘要必须保留该页业务项与游标
证据。Agent 应按 `outcome` 和分页元数据判断是否可继续，不能根据 `data.items` 是否为空判断。

## 5. 失败恢复与重试

分页框架将页失败保留下来，但不自行重放请求。错误分类必须同时获得：

```text
effect: read | write | unknown
idempotency: idempotent | key_required | unknown
execution_started: true | false | unknown
```

只有幂等读取、服务端明确未开始执行，或明确的执行前限流（HTTP 429）可声明
`retryable:true`。超时、网络错误、408、5xx 或含糊业务失败不能因为上游给出
`server_retryable` 就被标记为可安全重试，尤其不能用于写操作。

框架首先使用已治理的 `pagination_inconsistent`、`projection_unknown` 与 `rate_limit`。
未进入错误 subtype registry 的旧 reason 只能作为兼容诊断，不能成为稳定的 Agent 分支键。

## 6. 产品接入要求

每个分页 terminal command 必须提供：

1. 当前页 items、下一页游标和“是否明确还有更多”的上游证据；
2. 稳定 ID 去重策略和最大页面/窗口预算；
3. 页失败时的可恢复诊断，而不是只返回泛化 API error；
4. 从 `PageLedger` 到统一 `CommandResult` 的显式 adapter；
5. 对短页、空页、游标复用、游标缺失、部分失败的测试。

框架不接管：API 字段兼容、搜索索引健康、业务“全量”承诺、下载完整性、写操作的终态
校验。这些仍是各产品域的职责。

## 7. 渐进接入与验收

迁移单位是一个 terminal command：

```text
legacy_only -> dual_validate -> unified_active -> unified_stable
```

`dual_validate` 仅构建和校验 ledger/result；业务调用一次，外部 legacy stdout 字节不变。
进入 `unified_active` 后，`--format json` 才输出统一返回。Agent 不选择协议。

每次接入由 Agent 逐条扫描 Help、参数、结果形状、游标续跑和安全语义；扫描报告保存 Markdown
证据，不保存运行时 JSON fixture，也不以 CI 取代 Agent 复核。

最低验收：

1. `endpoint_exhausted:true` 从不携带 token；`false` 必有 token。
2. 证据未知时不伪造 endpoint exhausted 或 complete。
3. 游标循环、缺游标和冲突页信息 fail closed。
4. 后续页失败保留已成功页，并以 `partial_failure` 和非零退出结束。
5. 读写安全事实进入错误分类；含糊写失败不声称可安全重试。
