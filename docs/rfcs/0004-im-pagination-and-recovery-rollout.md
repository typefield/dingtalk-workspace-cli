# RFC-0004：IM 分页与错误恢复接入统一返回

- 状态：Implementing（PageLedger 已落地，IM terminal command 尚未激活）
- 日期：2026-08-08
- 适用范围：`internal/shortcut/chat` 的可终结、只读分页命令
- 依赖：RFC-0001（统一返回）、RFC-0003（错误 subtype 治理）

框架通用契约已抽取至 [分页框架契约与能力](../pagination-framework-contract.md)；本 RFC
仅保留 IM 的问题、适配和渐进接入范围。

## 1. 摘要

IM 已经实现了游标去重、页数上限、满页无游标探测、跨页去重和失败记录。这些执行层
算法应保留，但其结果仍以每个 shortcut 自定义的
`complete/hasMore/nextCursor/failures/partial` payload 表达；大部分 IM shortcut 也尚未
声明统一返回 rollout。

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

但有三个结构性缺口：

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

迁移单位是**一个 terminal command**，不是整个 `chat` 域。首批仅选择只读、分页事实明确
且没有资源下载副作用的命令：

1. `chat +flag-list`；
2. `chat +chat-search`；
3. `chat +conversation-list` / `+chat-list`（以当前 canonical path 为准）。

消息列表、线程回复、资源下载保持 legacy/dual_validate，直到它们的时间游标、资源下载
和页面错误单位均有单独的 CommandResult 映射。写命令不因本 RFC 改变 retry 行为。

每个 `dual_validate` 命令必须：一次业务调用、legacy stdout 字节不变、shadow
`CommandResult` 可验证并记录到 Agent 审阅台账；不允许在 dual 阶段重新取数或让 Agent
选择协议。

## 6. 验收

1. `hasMore=true` 必须带可续 `next_token`；`endpoint_exhausted:true` 禁止带 token。
2. 分页证据未知时不输出 `endpoint_exhausted:true`，也不以 `complete:true` 作替代。
3. 游标循环、游标无效、外层/内层矛盾均是 typed `pagination_inconsistent`，不输出成功空集。
4. 后续页失败的 active 命令输出 `partial_failure`、非零 exit，保留成功页面与失败/未知页。
5. 429 可输出 `rate_limit` 与服务端等待提示；写调用的含糊失败不能输出 `retryable:true`。
6. 由 Agent 扫描与受控 loopback/live audit 复核 pagination、failure、retryability；扫描只
   保存 Markdown 证据，不保存运行时 JSON fixture，也不被 CI 取代。

## 7. 非目标

本 RFC 不保证 IM 搜索索引完整，不解决后端目录/索引假阴，不证明 API error 没有产生写入，
也不改变前台事件流或 IM 资源下载的独立完整性协议。
