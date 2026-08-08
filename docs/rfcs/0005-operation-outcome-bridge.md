# RFC-0005：状态机阻断与复合写的 operation outcome 接入

- 状态：实施中；doc 三条复合写已进入 dual validation，个人订阅仍待语义审阅
- 日期：2026-08-09
- 适用范围：个人订阅尝试状态机、`doc` 复合写 shortcut
- 依赖：RFC-0001（统一返回渐进 rollout）、RFC-0003（稳定错误 subtype）和
  [分页框架契约与能力](../pagination-framework-contract.md)

## 1. 问题与边界

错误 registry 的第五批已把多数动态 `reason` 降为稳定 subtype 或诊断字段。源码扫描仍有
两个动态构造**位置**，但它们不能按“换成一个常量”处理：

| 路径 | 现在的表达 | 为什么不能直接替换 |
|---|---|---|
| `event.consume.personal.subscribe` | `personalSubscriptionFailureClass.reason` 与 persisted `AttemptState` 拼入 `WithReason`；同一阻断可按历史错误成为 auth、validation 或 api | `retryability`、`retry_after`、原始服务端码和当前阻断状态都是不同事实；把它们压成一个 subtype 会错误改变类别或诱导重试 |
| `doc create` / `doc checkpoint-update` / `doc history-revert` | 已完成步骤被塞进普通 API error 的 `details.status=partial_success` | “创建已成功、后续写失败”不是普通 failure；Agent 需要知道哪些步骤已生效、哪些失败、哪些还未知，避免重复创建或盲目回滚 |

该 RFC 的目标不是让 CoreCmd 自动验证业务终态。目标是建立一个窄的桥接方式：业务命令识别
到阻断、已应用、失败或未知时，框架能在**该命令已经进入统一输出**后如实表达；legacy 命令
保持既有字节和退出码，dual validation 只构建/校验 shadow result。

## 2. 三层事实模型

```text
request outcome      请求是否被 CLI/上游接受
operation outcome    哪些逻辑步骤已成功、失败或未知
verification outcome 写后读回是否确认预期终态
```

`failure` 只表示请求路径未给出完整成功。`partial_failure` 适用于至少一个可标识的步骤已
成功，另一个步骤失败或未知；退出码为 7。读回失败本身不自动等于“写入失败”，它必须被表示
为 failed/unknown verification step。框架只验证形状和不变量，业务命令仍负责提供真实步骤。

## 3. 个人订阅状态机

### 3.1 当前事实

持久化状态仅有 `in_flight`、`cooldown` 与 `terminal_hold`。它们表示**本次 CLI 被本地
状态机阻断，尚未再次发出订阅请求**；不是远端订阅的真实终态。当前错误同时携带：

- 状态机事实：state、next allowed time、retry after；
- 上一次失败留下的 server error code / trace；
- 先前失败的 retryability 和 auth/validation/api 分类。

其中 `personalSubscriptionFailureClass.reason` 有多种有限来源，虽然当前扫描只看到一次
`WithReason(reason)` 调用，不能把“2 个动态调用位置”误读成“2 个动态值”。

创建请求会把由 identity + event/rule/filter 导出的 `idempotencyKey` 放入请求 `ext`，并用
同一 key 构造本地 attempt fingerprint；这是避免本地重复尝试的必要条件。但仓库没有受控
服务端证据证明该 endpoint 对该 key 做到了去重或“响应丢失后的安全重放”，所以它**不是**
对 Agent 输出 `retryable:true` 的充分条件。

### 3.2 迁移方案

第一阶段只迁移**已被本地状态机阻断**的路径，不改变订阅请求失败分类：

1. 按最终 Category 选择三个稳定 subtype：
   `personal_subscription_auth_blocked`、
   `personal_subscription_validation_blocked`、
   `personal_subscription_attempt_blocked`（api）。
2. 将 `attempt_state`、`next_allowed_at`、原始 server code、trace 保留在 diagnostic
   fields；state 不再成为 subtype。
3. 仅在当前 invocation 尚未发送订阅请求的证明成立时，允许
   `execution_started:false`。这并不声称**上一次**请求没有写入。
4. `retryable:true` 只能表示“本地等待到期后可以再次尝试”；若 server 失败的幂等性和
   已执行状态不能证明，则省略 retryable，即使上一次错误文本看起来像超时。当前只证明
   key 已发送，尚未证明服务端去重，因此不得以此例外。

第二阶段再审阅 `personalSubscriptionFailureClass` 的每个来源：需要同时证明订阅请求的
idempotency key、`execution_started` 和服务端恢复语义；不能把 timeout/network/5xx 的
任意历史值直接升级为“安全可重试”。

### 3.3 验收

- 三个 persisted state 均只落入稳定阻断 subtype，并保留 `attempt_state`；
- auth/validation/api 类别和既有退出码不漂移；
- 当前阻断调用不发送第二次订阅请求；
- 未经幂等性证明的历史 timeout/network/5xx 不输出 `retryable:true`；
- Agent 扫描展示有限映射与每个状态的命令级测试，不保存运行时 JSON fixture。

## 4. 文档复合写

### 4.1 步骤到三通道的映射

`docPartialWriteError` 的现有 `steps` 是业务事实，应在其旁边建立 typed
`output.PartialData`，但不可由框架猜测：

| 现有步骤状态 | unified channel | 要求 |
|---|---|---|
| `success` | `succeeded[]` | 条目带稳定 `id:"step:<name>"`、必要输入和已应用结果 |
| `failed` | `failed[]` | 条目带同一稳定 ID 与 typed error；subtype 是审定的 doc 复合写条件 |
| `not_started` / 无法验证 | `unknown[]` | 条目带稳定 ID 与具体未知原因，不能伪装失败或成功 |

典型映射：

- JSONML create 后 `write_jsonml` 失败：`create_document` 成功、`write_jsonml` 失败；
- checkpoint update 后读回失败：checkpoint/update 成功、verify 失败；
- history revert 后读回失败：revert 成功、verify 失败；
- create 响应缺 node ID：create 成功、write_jsonml 未开始（unknown），不能断言新文档不存在。

为保持 legacy error `reason` wire，第一阶段 registry 使用与现有五个 reason 相同的稳定
值：`doc_create_missing_node_id`、`doc_create_initial_content_failed`、
`doc_checkpoint_update_failed`、`doc_checkpoint_verification_failed`、
`doc_history_revert_verification_failed`。它们都是 api 类别、不可自动重试；补偿计划和
已完成步骤仍保留在 details。

### 4.2 Runner bridge

`Shortcut.Execute` 的返回类型是 `error`，而 active unified command 需要向
`ResultStore` 交付 `CommandResult`。因此新增的桥接必须同时满足：

```text
legacy_only:
  原始 apperrors.Error → 原始 renderer / 原始 rc

dual_validate:
  原始 apperrors.Error → 原始 renderer / 原始 rc
  同一次业务执行构建 PartialData 并 ValidateResult；不写第二份结果

unified_active:
  同一次业务执行构建 PartialData → StoreResult(Partial(...))
  → 单一 machine result，rc=7；不再额外打印 legacy error
```

桥接不能重新调用 MCP，不能把 `partial_failure` 包装为 `ok:true`，也不能在已经写入结果后再
尝试第二个 error envelope。`doc` 每个 terminal leaf 独立 rollout：先是三个复合写命令
的 dual validation，确认 legacy golden 不变后才逐条 active。

### 4.3 验收

1. legacy/dual 的 stdout 与 rc 均逐字保持；同一 fixture 只触发一次 MCP 写调用；
2. active 的 create/checkpoint/revert partial 都是 `outcome:partial_failure`、rc=7，且
   `succeeded/failed/unknown/total` 对账；
3. create 成功后 content 写失败不会允许 Agent 按失败重试 create；补偿对象仍可见；
4. verification failure 不扩大为“写入未发生”或“验证通过”；
5. 全失败走普通 typed `failure`，不用空 `partial_failure`。

## 5. 迁移顺序与非目标

```text
P0  个人状态机的 Category/幂等性源码审阅；登记 doc 五个既有 reason descriptor（后者已完成）
P1  在 doc create / checkpoint-update / history-revert 建 shadow PartialData（已完成）；dual 下保留 legacy error 与 rc，并以单元测试锁定三通道与单次结果出口
P2  按命令转 active；受控真实账号复验已应用/补偿事实
P3  审阅个人订阅请求失败分类与 idempotency，再迁其余 failure family
```

本 RFC 不补服务端事务、不会自动执行补偿或回滚，也不把本地 attempt store 当作远端订阅
的权威状态。真实 API error 是否已写入、搜索索引是否完整，仍需对应产品服务端或写后验证
能力解决。
