# RFC-0003：DWS 错误 subtype 与恢复语义渐进治理

- 状态：Proposed
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

2026-08-08 的 Agent 源码扫描记录在
[`docs/agent-scans/error-contract-inventory-20260808.md`](../agent-scans/error-contract-inventory-20260808.md)：

| 事实 | 数量 | 含义 |
|---|---:|---|
| `WithReason("…")` 字面 subtype | 79 | 生产源码中已出现的自由字符串，不等于已稳定协议 |
| 相应调用点 | 159 | 同一 subtype 可能有多条、且恢复信息不同的构造路径 |
| 直接设置 `ErrorInfo.Subtype` | 6 | 绕过 `WithReason` 的第二条入口 |
| 动态 `WithReason(variable)` | 16 | 上游码或拼接文本可能直接变成 Agent 分支键 |
| 缺邻近 `WithHint` 的 subtype | 30 | 不能默认 Agent 有可靠恢复路径 |

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
    Description   string
}
```

Descriptor 的字段是规范，不是自动编造恢复文案的模板：具体 `hint/actions` 仍由掌握
命令上下文的业务层填写。Registry 只规定哪些情况下必须存在、哪些重试声明不合法。

### 4.2 首批稳定 subtype

先审定跨产品、高频且已有明确语义的值：

| subtype | Category | 恢复语义 |
|---|---|---|
| `missing_required_flags` | validation | 展示 `available_flags` 或可执行的参数补齐提示；不可重试 |
| `unknown_flag` | validation | 显示可见 flag；不可重试 |
| `confirmation_required` | validation | 只在尚未发起写请求时出现；给出安全的确认动作；不可自动重试 |
| `rate_limit` | api | 仅按服务端指令/安全策略声明可重试，并透传等待时间 |
| `pagination_inconsistent` | api | 禁止把不完整/矛盾页结果伪装成完整；先检查上游分页证据 |
| `projection_unknown` | api | 禁止把未知响应投影为空集合；保留最小诊断上下文 |

名称、Category 或重试语义的变动按 breaking change 评审。新增 descriptor 必须说明其
稳定性、触发边界和 Agent 恢复行为。

### 4.3 未注册与动态上游原因

禁止把服务端任意 `reason`、HTTP 文本或拼接后的字符串直接作为公开 subtype。迁移后：

1. 已知上游状态码/错误码映射到注册 descriptor；
2. 无法安全映射时使用 `api/upstream_unclassified`；
3. 原始上游 reason、HTTP/RPC code、trace 等放入已有诊断字段，不能成为 Agent 的
   稳定分支键；
4. 对写请求且执行是否开始未知时，不得声明 `retryable:true`；应省略该字段，并保留
   `execution_started` 的已知状态或缺席。

`upstream_unclassified` 的文案只要求 Agent 收集 trace/请求上下文或提示用户，不得诱导
盲目重试。

## 5. 实现原则

1. 新增 `WithSubtype(Subtype)` 或等价的 descriptor 构造器；新代码不再直接使用任意
   `WithReason` 构造公开 subtype。
2. `WithReason(string)` 保留为 source compatibility 迁移入口；在未迁移命令中不得改变
   输出形状或退出码。
3. `internal/errors.Error` 到 `output.ErrorInfo` 的投影从同一 descriptor 读取 Category、
   subtype、退出码和恢复约束。legacy JSON 保持其现有字段形状，但使用同一分类事实。
4. 统一返回的 active command 最终要求 `error.type` 与 `error.subtype` 均存在；在全量
   分类完成前，遗漏 subtype 必须降级为受控 `internal/unclassified` 或
   `api/upstream_unclassified`，不能直接拒绝用户请求后又改写为不相关 internal error。
5. `exit_code` 是框架根据 Category/subtype 推导的结果；业务代码不得自报任意退出码。
6. `partial_failure` 仍必须由 typed `succeeded/failed/unknown` 数据表达；不能用普通
   error reason 代替逐项事实。

## 6. 渐进迁移

```text
P0  Agent 扫描盘点（已完成）
P1  建 registry + 首批六个 descriptor；新增构造/投影单元测试
P2  逐命令迁移统一返回 active command；动态上游 reason 走映射器
P3  为每个公开 subtype 补齐 hint/action/retry/execution 语义，更新相关 Skill 反模式
P4  Agent 复扫并审阅真实 error 路径；未审定值继续留兼容层或归 unclassified
```

每一阶段均按 terminal command rollout；禁止要求 Agent 增加参数选择协议。现有命令仍只
通过 `--format json` 请求机器输出。

## 7. 验收

实现每一个迁移批次时，必须同时满足：

1. Agent 扫描产出 Markdown 台账，记录注册、未注册、动态映射和缺恢复字段；不保存
   运行时 JSON fixture，也不将 Agent 审核替换为 CI。
2. 对首批六个 subtype，Category、进程退出码、legacy 投影和统一返回投影一致。
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
