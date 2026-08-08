# 分页框架契约与能力

- 状态：Implementing（框架 `PageLedger` 已落地，产品命令渐进接入中）
- 日期：2026-08-08
- 来源：从 IM 分页算法抽取；具体迁移计划见 [RFC-0004](rfcs/0004-im-pagination-and-recovery-rollout.md)

框架实现：[`internal/output/pagination_ledger.go`](../internal/output/pagination_ledger.go)。

## 0. Lark CLI 主干对照

Lark CLI 当前主干已经把分页执行抽到 `shortcuts/common`：

- [`PageAllFlags`](https://github.com/larksuite/cli/blob/main/shortcuts/common/page_all_flags.go)
  统一声明 `--page-all`、`--page-limit` 和 `--page-delay`；
- [`PaginateInto[T]`](https://github.com/larksuite/cli/blob/main/shortcuts/common/paginate_into.go)
  使用同一条路径处理单页与自动翻页，循环上界写在 `for` 条件中，逐页 typed decode，拒绝空游标与重复游标，并让产品 `PageAccumulator[T]` 决定怎样合并业务数据；
- [`PaginationMeta`](https://github.com/larksuite/cli/blob/main/internal/output/envelope.go)
  位于信封 `meta.pagination`，记录 `complete/pages/items/next_token`。源码明确规定
  `complete` 只在观察到服务端耗尽时为 true，而不是业务数据完整。

DWS 对齐的是这套**语义和分层**，不是逐字复制字段：

| 能力 | Lark main | DWS 分页规范 |
|---|---|---|
| 标准分页参数 | `page-all/page-limit/page-delay` | 对齐；允许产品声明更小的硬上限 |
| 有限执行 | 循环结构强制最大页数 | 对齐 |
| 游标安全 | 空 token、重复 token fail closed | 对齐，并统一为 `pagination_inconsistent` |
| 页面解析 | 泛型 typed decode + 产品 accumulator | 对齐产品 adapter 边界 |
| 耗尽语义 | `complete` 仅表示 endpoint exhausted | 使用更明确的 `endpoint_exhausted`，不重新引入 `complete` |
| 证据未知 | 没有独立 unknown wire；缺失 bool 容易落入零值 | DWS 增加第三态：不输出 pagination meta |
| 后续页失败 | 返回 error；调用方通常不输出已经累积的页 | DWS 使用 `partial_failure` 保留成功页面和失败/未知页 |
| 去重 | 由各产品 accumulator 负责 | 同样留在产品层，框架不猜业务 ID |
| 自动重试 | 分页器不自动重放 | 对齐；错误只给恢复证据 |

因此，DWS 规范是 Lark 分页执行器的兼容性增强：正常完成和截断语义一致，同时补齐
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
