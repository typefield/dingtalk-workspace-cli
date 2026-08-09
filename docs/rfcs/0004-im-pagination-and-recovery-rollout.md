# RFC-0004：IM 分页与错误恢复接入统一返回

- 状态：已实施（PageLedger 已落地；首批五条 IM 分页命令已进入 `unified_active`，真实服务端返回仍待 Agent 取证）
- 日期：2026-08-08
- 适用范围：`internal/shortcut/chat` 与 `internal/shortcut/smart` 的可终结分页命令；
  主路径为只读，部分命令可选执行本地资源下载或导出
- 依赖：RFC-0001（统一返回）、RFC-0003（错误 subtype 治理）

框架通用契约已抽取至 [分页框架契约与能力](../pagination-framework-contract.md)；本 RFC
仅保留 IM 的问题、适配和渐进接入范围。

## 1. 摘要

IM 已经实现了游标去重、页数上限、满页无游标探测、跨页去重和失败记录。这些执行层
算法应保留。RFC 起草时，其结果仍以每个 shortcut 自定义的
`complete/hasMore/nextCursor/failures/partial` payload 表达，且大部分 IM shortcut 尚未
声明统一返回 rollout。首批五条已经完成 active 迁移；其他 IM 分页入口仍按命令逐条迁移。

本 RFC 将 IM 改造为两层：

1. **PageLedger**：执行层收集分页事实，统一表达“服务端确认耗尽 / 可继续 / 未知 /
   中断”。
2. **CommandResult adapter**：仅在命令能够诚实表达结果单位时，把 ledger 映射为
   `success`、`partial_failure` 或 `failure`，以及统一返回的 `meta.pagination`。

它不重写 IM 的上游 API 适配，不把未知响应变成空列表，也不为写命令自动重试。

## 2. 当前事实

现有分页算法覆盖群搜索、会话列表、收藏、消息、线程回复等，典型机制包括：

- `seenCursors` 拒绝游标停滞或循环；
- 固定页面/窗口上限，防止 Agent 无限扫描；
- 稳定 ID 去重；
- `hasMore=true` 但无有效游标时 fail closed；
- 后续页读取失败时保留已读取内容和 `failures`；
- 未知响应形状使用 `projection_unknown`，而不是空列表。

对尚未迁入统一返回的 IM 入口，仍有三个结构性缺口：

| 缺口 | 当前表现 | 风险 |
|---|---|---|
| 完成语义 | `paginationKnown:false` 与 `complete:true` 可同时出现 | Agent 易把“短页推断停止”当端点耗尽 |
| 结果形态 | 每个命令手写 payload；统一输出没有 `meta.pagination` | 相同分页事实出现多套字段/不同含义 |
| 部分结果 | 先输出 legacy payload，再返回普通 API error | 迁入统一输出时可能被通用 payload mapper 误判为 success |

`server_failure_classifier` 目前只明确分类后端依赖不可用和参数错误。`WithServerDiag`
还会把上游 `server_retryable` 直接投影为 DWS `retryable`，但分类器不知道调用是否是写入、
是否幂等，也不知道执行是否开始；这不能作为写操作安全重试的依据。

## 3. 术语与不变量

### 3.1 PageLedger

```go
type PageLedger struct {
    Pages              int
    Items              int
    Continuation       string // 可继续时唯一有效
    EndpointExhausted  *bool  // nil = 无法观察到可靠耗尽证据
    StopReason         string
    Failures           []PageFailure
}
```

`EndpointExhausted` 的三态规则：

| 上游证据 | 值 | 统一返回 |
|---|---|---|
| `hasMore=false` 且终态游标 | `true` | `meta.pagination.endpoint_exhausted:true`，不带 token |
| `hasMore=true` 且有效游标，或命令主动仅取一页 | `false` | `endpoint_exhausted:false` + `next_token` |
| 短页、旧响应或字段冲突，无法证明 | `nil` | `meta.pagination` 缺席；业务 data 仅标 `pagination_known:false` |

`complete` 不再出现在新统一返回路径。legacy 输出可保留该字段一个迁移周期，但其含义仅是
“该算法停止”，不是 endpoint exhausted。

### 3.2 分页 failure 的结果单位

一个失败页面不是一个可枚举的业务对象；不知道其中有多少消息/群/收藏。因此不能把
“页 2 失败”伪装为“某个对象失败”。

首批命令采用如下规则：

| 情况 | unified outcome | 表达 |
|---|---|---|
| 没有任何成功页 | `failure` | typed error；不附空列表作为成功 data |
| 已完整读取，或只明确请求一页 | `success` | 原业务 data + authoritative `meta.pagination` |
| 已有成功页，后续页失败且知道失败页范围 | `partial_failure` | succeeded 使用**页面摘要**（含该页业务项）；failed 使用 `id:"page:<n>"` 的 typed error |
| 已有成功页，但上游中断且不知道失败页终态 | `partial_failure` | succeeded 页面摘要；unknown 使用 `id:"page:<n>"` 与原因 |

业务列表不丢失：每个 succeeded 页面摘要的 `data` 保留该页列表和游标证据。最终聚合列表
也可作为附加业务字段，但 Agent 判断终态必须以 outcome 和 `meta.pagination` 为准。

## 4. 错误恢复

### 4.1 调用安全必须进入分类器

为 `executor.Invocation` 或等价的运行时上下文增加只读事实：

```text
effect: read | write | unknown
idempotency: idempotent | key_required | unknown
execution_started: true | false | unknown
```

`server_retryable` 只能作为建议输入。最终 `retryable:true` 的条件是：

1. 调用是幂等读取；或
2. 服务端明确 `execution_started:false`；或
3. 服务端拒绝在执行前的明确限流（HTTP 429）。

写调用遇到 `NETWORK_ERROR`、超时、408、5xx 或含糊 business error 时，必须保留 trace/
execution 诊断，但不得宣称可安全重试。已开始与未知状态分别使用已知布尔或字段缺席，
不能猜测为 false。

### 4.2 分类闭集的优先级

先让 IM 接入 RFC-0003 已审定的 `rate_limit`、`pagination_inconsistent` 与
`projection_unknown`。`backend_dependency_unavailable`、`invalid_request`、权限、资源
不存在等在未进入 registry 前仍为兼容 reason，不能被称为稳定 Agent 分支键。

## 5. 渐进迁移

```text
legacy_only
  -> dual_validate（构建 PageLedger/CommandResult，仅保留 legacy bytes）
  -> unified_active（--format json 输出统一返回）
  -> unified_stable
```

迁移单位是**一个 terminal command**，不是整个 `chat` 域。首批五条命令已经完成
`dual_validate → unified_active`：

1. `chat +flag-list`；
2. `chat +chat-search`；
3. `chat +conversation-list`（`+chat-list` 为兼容别名）。
4. `chat +thread-replies`。
5. `chat +chat-messages`。

`+chat-messages` 与 `+thread-replies` 的 `--download-resources`、以及
`+chat-messages --output` 也使用该命令唯一的统一结果契约；Agent 仍只传
`--format json`，不能按 flag 选择旧/新协议。读取成功而资源下载或本地导出失败时，
适配器保留已读取页面并返回 `partial_failure`，不能把本地副作用失败包装为读取成功。
这不等于资源字节完整性或服务端搜索覆盖已获验证，它们仍是产品层证据。

每个 `dual_validate` 命令在晋级前必须：一次业务调用、legacy stdout 字节不变、shadow
`CommandResult` 可验证并记录到 Agent 审阅台账；不允许在 dual 阶段重新取数或让 Agent
选择协议。这个历史阶段已经完成，当前五条命令都由其真实 `OutputRollout` 声明为
`unified_active`，不是靠测试临时覆盖声明来观察新信封。

当前进度：`chat +flag-list`、`chat +chat-search`、`chat +conversation-list`、
`chat +thread-replies` 与 `chat +chat-messages` 都在单次业务执行中构建
PageLedger，并将成功、首屏失败、后续页失败/未知和分页边界矛盾投影为
`CommandResult`。历史 dual 阶段的 legacy JSON 逐字节 golden 仍保留，用于防止未来
回退阶段意外修改旧输出。当前 active 回归直接验证 continuation 的
`endpoint_exhausted:false + next_token`、未知时分页 meta 缺席，以及后续页失败的
`partial_failure + exit 7`。`chat-search` 的最大窗口二次探测被建模为同一 cursor 的
验证步骤：探测成功只记一个逻辑页，探测失败保留首批数据并进入 `partial_failure`，
不会把探测次数伪装成 endpoint 页数。

`conversation-list` 额外使用严格 shadow 投影区分“已识别的空数组”和“未知容器/非法
条目/缺稳定 ID”：legacy 投影与字节保持不变，统一侧将未知结构归为
`projection_unknown`。达到本地 `--page-limit` 且仍有合法 cursor 时，统一侧返回可续的
success，而不是把安全预算耗尽伪装成远端失败；后续页读取失败仍保留成功页并返回
`partial_failure`。

`thread-replies` 将其毫秒 `nextCursor` 归一为不透明 `next_token`，但只用该游标作为
可续跑证据，不再把“能转成时间边界”扩大为业务全量证明。缺失 `hasMore` 但无可用游标时，
统一结果明确为 pagination evidence unknown；后续读取失败、重复游标、`hasMore=false`
却携带可用游标、或未知/非法回复容器时，统一侧分别输出 `partial_failure` 或 typed
`pagination_inconsistent` / `projection_unknown`。dual 阶段所有 legacy bytes 保持不变。

`chat-messages` 保留既有的毫秒时间边界、跨页稳定 ID 去重、时间范围和最大结果预算；
统一侧只旁路观察同一次下层响应。已知继续页输出 `endpoint_exhausted:false + next_token`，
未知分页证据不伪造 meta，后续读取失败保留成功页面并输出 `partial_failure`；
`hasMore=false` 却携带游标以及无法投影的消息容器则为 typed
`pagination_inconsistent` / `projection_unknown`。时间范围后处理失败也会中断统一结果，
不会把已读页伪装成完整终态。

## 6. 验收

1. `hasMore=true` 必须带可续 `next_token`；`endpoint_exhausted:true` 禁止带 token。
2. 分页证据未知时不输出 `endpoint_exhausted:true`，也不以 `complete:true` 作替代。
3. 游标循环、游标无效、外层/内层矛盾均是 typed `pagination_inconsistent`，不输出成功空集。
4. 后续页失败的 active 命令输出 `partial_failure`、非零 exit，保留成功页面与失败/未知页。
5. 429 可输出 `rate_limit` 与服务端等待提示；写调用的含糊失败不能输出 `retryable:true`。
6. 由 Agent 扫描与受控 loopback/live audit 复核 pagination、failure、retryability；扫描只
   保存 Markdown 证据，不保存运行时 JSON fixture，也不被 CI 取代。

### 6.1 2026-08-08 Agent 声明面扫描

Agent 以当前源码构建临时 CLI，并逐项读取五条命令的 leaf Help、`schema --all` 中的精确
tool 声明，以及 multi chat Skill 的根路由和精确 reference。结果如下：

- 五条命令均只公开全局 `--format`，没有输出协议选择参数；
- canonical path、`--page-all`、`--page-limit`、page size/token 约束与运行时一致；
- Schema 的 effect/risk/confirmation/idempotency 均为 `read/low/not_required/idempotent`；
- Skill 路由均指向当前 canonical path，没有要求 Agent 选择 rollout 状态；
- 扫描发现 Help/Schema 与 Skill 曾把 legacy 的 `complete/hasMore/nextCursor` 写死为长期
  契约。本批已改成“明确 endpoint 耗尽证据/续页 token/失败未知页”的语义规则；Skill
  仅在解释当前返回时按字段是否存在读取，不暴露内部 rollout。

本次扫描不调用真实 IM，不保存 JSON fixture；证据结论仅记录在本 RFC。

## 7. 非目标

本 RFC 不保证 IM 搜索索引完整，不解决后端目录/索引假阴，不证明 API error 没有产生写入，
也不改变前台事件流或 IM 资源下载的独立完整性协议。
