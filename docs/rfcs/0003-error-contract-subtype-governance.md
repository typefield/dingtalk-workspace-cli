# RFC-0003：DWS 错误 subtype 与恢复语义渐进治理

- 状态：已实施（四批 registry）；渐进迁移进行中
- 日期：2026-08-08
- 适用仓库：`dingtalk-workspace-cli`
- 依赖：RFC-0001 的统一返回 rollout；Agent 扫描台账

## 1. 摘要

DWS 已有 `Category`、退出码、`hint`、`actions`、`retryable`、
`retry_after_seconds` 与执行状态等错误基础设施，但 `WithReason(string)` 允许任意
字符串进入机器输出。这样 `type/subtype` 不能被安全地当作 Agent 分支条件，也无法保证
同一错误在 legacy 和统一返回中有一致的恢复语义。

本 RFC 建立**受治理的公开 subtype registry**，并以逐命令、逐错误路径的方式迁移。
它不重命名现有命令，不公开协议选择参数，不增加 `contract_version`，也不把服务端终态
验证伪装成框架能力。

目标是让已经审定的 `type + subtype` 成为稳定的 Agent 接口；未审定或动态的上游原因
必须诚实归类，而不是被误报为稳定 subtype。

## 2. 当前基线

2026-08-09 的 Agent 源码扫描记录在
[`docs/agent-scans/error-contract-inventory-20260809.md`](../agent-scans/error-contract-inventory-20260809.md)：

| 事实 | 数量 | 含义 |
|---|---:|---|
| 已注册 descriptor / 直接 `WithSubtype(Subtype...)` 调用 / 间接映射 | 37 / 105 / 6 | 首批八个、输入/公式/下载完整性第二批五个、目标解析/版本预检第三批十七个，以及 transport/服务端响应第四批七个稳定 subtype 已落地；间接映射函数由单元测试证明只返回有限注册值；迁移保持既有 `Reason` 字符串 wire，不引入版本标记 |
| `WithReason("…")` 自由字面调用 | 54 | 生产源码中仍存在的自由字符串，不等于已稳定协议 |
| 全部 subtype / 调用点 | 79 / 159 | 同一 subtype 可能有多条、且恢复信息不同的构造路径 |
| 直接设置 `ErrorInfo.Subtype` | 6 | 绕过 `WithReason` 的第二条入口 |
| 动态 `WithReason(variable)` | 7 | 仍需审阅；transport 的 HTTP/RPC 拼接路径已改为有限 subtype 映射，原始码保留在诊断字段 |
| 缺有效恢复提示的 subtype | 16 | 既没有命令级 hint、也没有 registry 默认 hint，不能默认 Agent 有可靠恢复路径 |

现有分类及退出码保持不变：`api=1`、`auth=2`、`validation=3`、PAT 专属
`permission=4`、`internal=5`、`discovery=6`、`partial_failure=7`。
本 RFC **不**立即照搬 Lark 的九个类别；类别重分配会影响既有退出码、Skill 和宿主
消费行为，应另行评估。

## 3. 对齐与取舍

Lark 的可借鉴点是：错误类别/subtype 是有文档、可评审、wire-stable 的闭集；错误输出
提供恢复提示和精确的重试信息。DWS 采用相同的治理方向，但保留以下差异：

| 主题 | 本 RFC 的决定 |
|---|---|
| 输出流 | 保持 DWS 当前统一返回命令的单 primary result 设计；不照搬 Lark 的 stderr failure 约定 |
| 类别 | 先保留 DWS 既有 Category/退出码；稳定性先落在 subtype registry |
| 自动重试 | CLI 不因 registry 自动重放请求；`retryable` 仅表示可安全重试 |
| 业务终态 | registry 只能表达已知失败/未知执行，不能证明“报错必未写入” |
| legacy 兼容 | legacy renderer 与统一返回共享同一分类投影，但按命令 rollout，不全局改 wire |

## 4. 目标模型

### 4.1 Descriptor

每个可公开给 Agent 分支的 subtype 必须有一个 descriptor。建议的 Go 形态：

```go
type Subtype string

type Descriptor struct {
    Subtype       Subtype
    Category      errors.Category
    RetryPolicy   RetryPolicy // never / idempotent_read_only / server_directive
    RequireHint   bool
    RequireAction bool
    DefaultHint   string // 无命令级 hint 时使用的安全恢复提示
    Description   string
}
```

Descriptor 的字段是规范，不是自动编造资源 ID、凭证或业务终态的模板：具体
`hint/actions` 仍由掌握命令上下文的业务层填写。`DefaultHint` 只覆盖通用、安全的
恢复动作（如“补齐参数后重试”）；命令级 `WithHint` 始终优先。Registry 只规定哪些
情况下必须存在、哪些重试声明不合法。

### 4.2 首批稳定 subtype

先审定跨产品、高频且已有明确语义的值：

| subtype | Category | 恢复语义 |
|---|---|---|
| `missing_required_flags` | validation | 展示 `available_flags` 或可执行的参数补齐提示；不可重试 |
| `invalid_flag_value` | validation | 只表示本地 flag 值、格式或互斥关系不合法；按 Help 修正后再执行，不可重试 |
| `invalid_argument` | validation | 只表示本地参数或参数组合不合法；按 Help 修正后再执行，不可重试 |
| `unknown_flag` | validation | 显示可见 flag；不可重试 |
| `confirmation_required` | validation | 只在尚未发起写请求时出现；给出安全的确认动作；不可自动重试 |
| `rate_limit` | api | 仅按服务端指令/安全策略声明可重试，并透传等待时间 |
| `pagination_inconsistent` | api | 禁止把不完整/矛盾页结果伪装成完整；先检查上游分页证据 |
| `projection_unknown` | api | 禁止把未知响应投影为空集合；保留最小诊断上下文 |
| `input_read_failed` | validation | 本地文件或 stdin 无法读取；检查路径、权限和输入来源后重试 |
| `invalid_json_input` | validation | 本地 JSON 语法或字段形状错误；修正输入后重试 |
| `formula_errors_found` | validation | 公式扫描已完成但发现公式错误；按 details 中的位置/类型修正，不重试原参数 |
| `download_output_unavailable` | internal | 下载完成后无法检查本地输出；先核对路径、磁盘和权限，不据此断言文件可用 |
| `download_size_mismatch` | api | 下载字节与服务端元数据不一致；保留现有文件供核查，只能作为幂等读取重新下载 |

### 4.2.1 目标解析与版本预检第三批

以下值都发生在写请求前的目标选择、只读预检或安全回读中，因此采用稳定 key，且不会把
上游任意文本暴露给 Agent：

| 类别 | subtype | Agent 恢复边界 |
|---|---|---|
| 版本预检 | `version_not_found` | 查询可用版本后选择存在的版本；不可重试原参数 |
| 自然目标输入 | `target_type_mismatch`、`target_arguments_conflict`、`missing_target` | 修正类型、互斥关系或补齐稳定 ID；不可重试 |
| 人/群目标解析 | `resolution_not_found`、`resolution_ambiguous`、`resolution_batch_failed` | 提供更精确名称或稳定 ID；ambiguous 禁止默认选第一个 |
| 人/群读取不完整 | `resolution_incomplete` | 仅可重试只读解析，或改用稳定 ID；不得继续写入 |
| AITable URL/对象预检 | `invalid_aitable_url`、`target_not_found`、`target_ambiguous`、`target_type_conflict`、`key_value_conflict`、`attachment_tokens_unavailable` | 属于本地/已读事实，修正输入或改用安全写模式后再执行 |
| AITable 上游投影/回读 | `target_incomplete`、`target_invalid_response`、`target_verification_failed` | 不把未知响应当作不存在或已验证；先只读核查，不盲目重放写入 |

名称、Category 或重试语义的变动按 breaking change 评审。新增 descriptor 必须说明其
稳定性、触发边界和 Agent 恢复行为。

### 4.2.2 Transport 与服务端响应第四批

这一批解决的是“上游数字/文本被拼成 subtype”的问题。HTTP 状态和 JSON-RPC code 保持在
`http_status`、`rpc_code`、trace、诊断 details 等事实字段中，但不是 Agent 分支键：

| 场景 | subtype | Category | Agent 恢复边界 |
|---|---|---|---|
| 未分类 `tools/call` HTTP/RPC/响应体失败 | `upstream_unclassified` | api | 写调用可能已经开始；先核查 execution/trace，不宣称安全重试 |
| 未分类发现路径 HTTP/RPC/响应体失败 | `discovery_upstream_unclassified` | discovery | 仅作为幂等只读发现重试；先核对服务版本和网络 |
| HTTP / RPC 认证或授权拒绝 | `upstream_authentication_required` / `upstream_authorization_denied` | auth | 检查登录、凭证、租户身份或授权范围；不可盲目重放 |
| JSON-RPC `-32602` | `invalid_argument` | validation | 修正 schema/参数后重新执行；请求未通过工具输入校验 |
| `tools/call` JSON-RPC 协议不兼容 | `tool_protocol_incompatible` | discovery | 核对工具名、服务版本或升级 CLI；不当作业务写失败 |
| MCP 后端依赖不可用 / 已知后端参数拒绝 | `backend_dependency_unavailable` / `upstream_request_rejected` | api | 保留 trace；前者等待依赖恢复，后者核对 Help、Schema 与稳定 ID |

映射函数只能返回上述有限 descriptor，并由单元测试覆盖任意 HTTP/RPC 数字不会进入
subtype。`tools/call` 的 408/5xx 与网络丢响应仍保留 `execution_state=unknown`，且不输出
`retryable:true`。

### 4.3 未注册与动态上游原因

禁止把服务端任意 `reason`、HTTP 文本或拼接后的字符串直接作为公开 subtype。迁移后：

1. 已知上游状态码/错误码映射到注册 descriptor；
2. 无法安全映射时，按 Category 使用 `upstream_unclassified`（api）或
   `discovery_upstream_unclassified`（discovery）；
3. 原始上游 reason、HTTP/RPC code、trace 等放入已有诊断字段，不能成为 Agent 的
   稳定分支键；
4. 对写请求且执行是否开始未知时，不得声明 `retryable:true`；应省略该字段，并保留
   `execution_started` 的已知状态或缺席。

两个 unclassified subtype 的文案只要求 Agent 收集 trace/请求上下文或提示用户，不得诱导
盲目重试。

## 5. 实现原则

1. 新增 `WithSubtype(Subtype)` 或等价的 descriptor 构造器；新代码不再直接使用任意
   `WithReason` 构造公开 subtype。
2. `WithReason(string)` 保留为 source compatibility 迁移入口；在未迁移命令中不得改变
   输出形状或退出码。
3. `internal/errors.Error` 到 `output.ErrorInfo` 的投影从同一 descriptor 读取 Category、
   subtype、退出码和恢复约束。legacy JSON 保持其现有字段形状，但使用同一分类事实。
4. 统一返回的 active command 最终要求 `error.type` 与 `error.subtype` 均存在；在全量
   分类完成前，遗漏 subtype 必须降级为受控 `internal/unclassified`、
   `upstream_unclassified` 或 `discovery_upstream_unclassified`，不能直接拒绝用户请求后又改写为不相关 internal error。
5. `exit_code` 是框架根据 Category/subtype 推导的结果；业务代码不得自报任意退出码。
6. `partial_failure` 仍必须由 typed `succeeded/failed/unknown` 数据表达；不能用普通
   error reason 代替逐项事实。

## 6. 渐进迁移

```text
P0  Agent 扫描盘点（已完成）
P1  建 registry + 首批八个 descriptor；新增构造/投影单元测试（已完成）
P2  逐命令迁移：首批八个、输入/公式/下载完整性五个、目标解析/版本预检十七个、transport/服务端响应七个已登记 subtype 的生产调用已迁入 `WithSubtype`；HTTP/RPC 动态上游 reason 已走有限映射器，剩余动态变量继续逐项审阅（进行中）
P3  为每个公开 subtype 补齐 hint/action/retry/execution 语义，更新相关 Skill 反模式
P4  Agent 复扫并审阅真实 error 路径；未审定值继续留兼容层或归 unclassified
```

每一阶段均按 terminal command rollout；禁止要求 Agent 增加参数选择协议。现有命令仍只
通过 `--format json` 请求机器输出。

## 7. 验收

实现每一个迁移批次时，必须同时满足：

1. Agent 扫描产出 Markdown 台账，记录注册、未注册、动态映射和缺恢复字段；不保存
   运行时 JSON fixture，也不将 Agent 审核替换为 CI。
2. 对首批八个 subtype，Category、进程退出码、legacy 投影和统一返回投影一致。
3. `confirmation_required` 在请求发起前退出；写请求的模糊网络/5xx 失败不宣称安全重试。
4. `pagination_inconsistent` 与 `projection_unknown` 都 fail closed，不能输出完整空列表。
5. 缺参、未知 flag、限流、写请求超时、部分成功各有至少一条命令级回归路径。
6. 每个恢复 action 都是可执行且与当前命令上下文相符的建议；不能用模板伪造凭证、
   资源 ID 或终态结论。

## 8. 非目标与风险

本 RFC 不负责修复服务端索引、目录死条目、事务性污染、写后回读、真实审批提单或下载
内容摘要能力。它只确保 CLI 对已知情况给出一致、可恢复、不过度承诺的表达。

最大风险是过早把现有 79 个字符串全部视为公共协议：会冻结临时实现细节，也会把上游
不稳定文本暴露给 Agent。因此 registry 以少量高价值 descriptor 开始；Agent 扫描是
发布前的语义复核证据，而不是自动批准器。
