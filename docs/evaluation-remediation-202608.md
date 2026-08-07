# 2026-08 CLI 评测问题整改台账

## 口径

本台账对应以下两份外部评测快照：

- `CLI评测-v3完整评测报告-20260728晚.md`
- `CLI评测-v3完整评测报告-20260806.md`

状态只按当前代码和可复现证据认定：

- **已关闭**：代码、CLI 行为和覆盖范围一致，且有针对性测试或本地安全实跑证据。
- **代码已修，待真实环境复验**：本地契约已收口，但服务端终态、真实账号权限或异步投递仍需真实环境证明。
- **未关闭**：仍能复现，或当前证据不足以证明报告中的完整问题已经消失。

单元测试通过不等同于服务端问题关闭；Schema 可发现也不等同于真实调用成功。

## 当前结论

| 报告问题 | 当前状态 | 当前证据 | 剩余动作 |
|---|---|---|---|
| attendance 影子 shortcut：15 条不可发现、9 条写命令无门禁 | **已关闭（当前工作树）** | attendance 包内 33/33 shortcut 均公开并带 Contract；加 2 条 smart shortcut 后 `shortcut list --service attendance` 为 35。9/9 写命令均为 `confirmation:user_required`；`+boss-check` 非交互缺 `--yes` 返回 rc=3；`--dry-run` 只返回 `executed:false` | 合入独立提交；发布后二进制复验 |
| 全域影子 shortcut 扫描 | **未关闭** | attendance 已完成，但真实测试跟进清单仍有 156 条；其中包含真实后端报错，也包含尚未完成稳定接口/安全审阅的命令，不能一键公开。公开状态与真实调用结果是两个维度，不能因完成 Contract 审阅而删除失败证据 | 按产品逐条 Agent 审阅；写命令先审 Safety/dry-run，再决定公开或删除 |
| mail Schema 仅 41/68，缺不可逆命令 | **代码已修** | 当前 `schema mail` 返回 73 个工具，包含 `mail.recall_sent_message` 与 `mail.trash_mailbox_thread`；全局 Schema/Help/运行时门禁同源检查通过 | 以当前版本重新做“可执行叶子 vs Schema”全量对拍，避免沿用旧 68 基线 |
| 12 条读命令 12 种 JSON 信封 | **框架已接入，渐进迁移中** | Framework 2.0 已提供统一 `ok/outcome/data/error/meta`、强类型布尔、单 writer、partial/pending；不公开协议选择参数，不输出版本标记 | legacy 命令按 terminal command 继续迁移；不得把迁移责任交给 Agent |
| mail `success` 字段 string/bool 分裂 | **已关闭（JSON 输出路径）** | Mail JSON 输出会递归把键名严格等于 `success` 的 `"true"`/`"false"` 归一为 Go bool；不转换其他键或普通文本。端到端 helper 输出测试覆盖顶层、嵌套和数组 | 发布后二进制抽取报告同组 14 个样本，确认后端新增响应形状仍经过该 JSON 出口 |
| mail `--size` 等隐藏别名在 Skill 中被当作公开契约 | **已关闭（当前工作树）** | Help/Schema/Skill 按命令统一：消息列表公开 `--folder-id`，文件夹列表公开 `--folder`，相关命令公开 `--limit`/`--content`/`--from`；`--size`/`--page-size`/`--body`/`--sender` 及反向 folder 别名仅作隐藏兼容。mono/multi 脚本对子进程传 canonical flag，且可解包 legacy/v2 结果容器 | 发布前用 Agent 语义扫描继续检查其他 Mail 兼容 flag，不将隐参数重新暴露给 Agent |
| `retryable` 与实际重试反向、超时盲重放 | **框架已修，待产品回归** | transport 对执行状态不明确的写调用不再自动重放；错误恢复元数据和 context deadline 已进入统一路径 | 对幂等读、限流、写后超时分别做真实链路复验 |
| `wiki +node-list` 挂起 | **代码已修，待真实环境复验** | 已发布 canonical `wiki.shortcut_node_list` 及 read/low/not_required/idempotent Safety；无网络测试证明只调用一次 `doc/list_nodes`。未知响应形状不再伪装为空列表，分页矛盾会 typed failure，无分页证据时 `paginationKnown:false` | 发布后二进制执行真实根目录、空目录、分页三组读取，确认服务端不再挂起且字段形状与投影一致 |
| `drive list --pattern` 失效 | **代码已修** | `drive list` 已公开 `--pattern`；测试覆盖 pattern 投影、JSON 可解析以及与 `--versions` 的冲突 | 发布后二进制用真实目录做 3 组正反例复验 |
| `drive download --format json` 惰性、stdout 日志污染 | **代码已修** | 下载成功与 dry-run JSON 路径均有测试；Framework writer 统一 stdout/stderr 与 format 分发 | 发布后二进制执行成功/失败两条 `jq` 管道复验 |
| `drive delete --dry-run` 与确认门禁耦合、EOF 取消仍 rc=0 | **已关闭（当前工作树）** | `drive delete` 声明 destructive/high/user_required；专项测试证明 `--dry-run` 产生请求预览且写调用为 0，关闭 stdin 且无 `--yes` 时返回 typed `confirmation_required`、写调用仍为 0 | 发布后二进制复验 dry-run/EOF/`--yes` 三条路径 |
| `doc version revert --dry-run` 对不存在版本也放行 | **代码已修** | 针对版本 `999` 的预校验回归测试已存在；dry-run 不触发写请求 | 真实文档分别验证存在/不存在版本号 |
| `todo +add-participant` 报错但已写入 | **代码已修，待真实环境复验** | 参加人写入增加幂等/结果核验路径及专项测试 | 真实任务执行“成功、服务端报错但已写、明确失败”三态复验 |
| `todo task remove-attachment` 取消仍 rc=0、遗漏附件 ID 校验 | **已关闭（当前工作树）** | 删除附件已改用统一 destructive/high/user_required 门禁；拒绝确认返回 validation rc=3，不再静默成功。`task-id`/`attachment-id` 均在任何远端调用前校验；dry-run 不发写请求。helpers 与 CoreCmd 的公共必填校验统一产出 typed validation rc=3 | 发布后二进制复验取消、缺参、dry-run、`--yes` 四条路径；真实删除仅使用隔离待办附件 |
| IM 零结果却扩大成 `complete:true` | **CLI 语义已修，索引健康未解决** | 输出改为 `endpointExhausted` 与 `indexCoverageKnown:false`，不再把分页耗尽解释为业务全量完整 | 服务端需要提供索引覆盖/健康证据，CLI 不能自动推断 |
| AITable/Base 目录死条目与假阴 | **CLI 已 fail-closed，服务端未关闭** | 列表/搜索结果显式标记发现来源、分页已知性和索引覆盖未知，不再承诺权威全量目录 | 后端治理死条目；真实账号复测精度和召回率 |
| event 停机契约 | **安全入口已修，真实停机待复验** | `event stop` 为 destructive/high/user_required；无 `--yes` 拦截，`--dry-run` 不停订阅 | 真实订阅执行 stop 后验证进程、订阅和本地状态三者终态一致 |
| contact 投影层 `8/5/1` 数据丢失 | **代码已修，待真实环境复验** | `+list-roles` 已拍平分组 `labels[]`；角色/部门成员识别 `labelUserList`/`deptUserList` 并解包 `userInfo`。回归测试锁定报告中的 8/5/1 下层计数；任一非空条目无法投影时整体 fail-closed，不再返回成功空/残缺列表 | 用原评测账号复跑三条命令并对拍 lower/upper 数量及稳定 ID |
| Skill/Help/Schema 指令偏移 | **本轮已关闭已知项，持续审阅** | Agent 对拍发现部门查询隐藏别名、AITable 写示例缺身份参数、考勤/听记/会议室脚本隐藏 flag、mono devdoc 类型漂移；已同步修正代码声明、Help、Schema examples、mono/multi Skill 和脚本 | 发布前继续按产品执行 Agent 语义扫描；CI 路径检查不能代替 flag/结果/安全审阅 |
| public leaf 依赖 Schema exclusion | **渐进收敛中** | `contact label list/get/list-members`、`todo task list-sub/remove-attachment`、8 条 Chat 纯读命令、DING/OA 的 4 条纯读命令、DING 的 3 条本人身份写命令、OA 加签/可退回节点查询/退回 3 条命令、Calendar ACL 授权/撤权与日历本更新 3 条命令、Chat 当前用户会话状态 8 条命令，以及 Chat 会话分组 5 条写命令已完成 Identity、接口/参数、Safety 和 Agent 选型审阅并移出 exclusion；运行时 Schema 工具数由 1000 增至 1039。当前仍有 35 条 CLI 生命周期控制、16 条未开放 Agoal、9 条待审兼容 helper | 只按 terminal command 在完成接口、参数和安全审阅后移除；CLI 控制面和明确未开放产品不为追求数字清零而伪装成 Agent 工具 |
| sheet 二次回滚 bricking | **未关闭** | 破坏性问题不能以普通单测或无账号 dry-run 证明消失 | 隔离表格、备份和服务端协同下做官方自证；失败时保留可恢复证据 |
| approval 真实提单 | **未关闭** | Schema 与三件套能力存在，但原报告缺真实创建成功证据 | 使用获授权测试审批模板完成一次真实创建并清理测试实例 |
| `dws api` 默认 MCP 登录态不可用 | **未关闭（需要后端能力）** | 默认登录保存的是只能由 MCP 网关解密/代理的 token，不是可直接发给 `api.dingtalk.com` 的 access token；CLI 当前正确 fail-closed 并返回 typed auth 错误 | 后端提供受限 raw-API proxy/capability 才能让默认身份使用；在此之前仍要求自有 AppKey/AppSecret，禁止 CLI 伪造或转发不适用的密文 token |

## attendance 本轮验收证据

```text
shortcut count:      35
hidden:               0
write shortcuts:      9
ungated writes:       0
boss-check no --yes:  rc=3 confirmation_required
boss-check dry-run:   rc=0, executed=false
Schema canonical:     attendance.shortcut_boss_check
```

新增的域级门禁会同时校验：

1. 33 条 attendance 包内 shortcut 无 `Hidden`。
2. 33 条全部存在于公开目录。
3. 每条都有完整 Agent Contract。
4. 读命令必须是 read/not_required。
5. 写命令必须要求 user_required。

## Wiki node-list 本轮验收证据

```text
Schema canonical:            wiki.shortcut_node_list
Safety:                      read/low/not_required/idempotent
backend route:               doc/list_nodes
backend calls per execution: 1
unknown projection:          typed projection_unknown
pagination contradiction:    typed pagination_inconsistent
pagination without evidence: paginationKnown=false
```

这次只关闭 CLI 路由、可发现性和输出真实性问题，不以单元测试替代真实
Wiki 服务端读取。真实账号复验仍保留在发布验收中。

## Contact 投影层本轮验收证据

```text
contact +list-roles:        lower=8, projected=8
contact +list-role-members: lower=5, projected=5
contact +list-dept-members: lower=1, projected=1
unknown non-empty row:      projection_unknown（拒绝成功空/部分列表）
```

这里的“投影层”是把 MCP/API 原始响应转换成 Agent 直接消费的精简字段。
本轮不仅补齐报告命中的真实容器和 `userInfo` 包装，还要求投影后的行数与
下层行数一致；容器名已知但字段形状未知同样不能冒充成功。

## Chat 纯读命令 exclusion 收敛证据

本轮移出旧 compatibility exclusion 的命令为：

```text
chat group list-all
chat group list-join-validations
chat group members list-by-ids
chat group notice get
chat group notice list
chat list-all-conversations
chat message list-emotion-replies
chat text translate
```

逐条审阅证明：

1. 全部为 `read/low/not_required/idempotent`，不发布 dry-run 或写门禁声明。
2. 每次执行只调用一个确定的 `im/*` 远端方法；由于这些方法不在 pinned MCP
   metadata 中，Schema 诚实发布 `composite` 和具体 unpinned 原因，不伪造
   `interface_ref`。
3. Identity、CLI path、参数映射及 Agent 的 use/avoid/example 均有命令级测试。
4. 群列表、入群验证、群公告和会话列表的 limit 越界会在远端调用前返回
   typed validation；空成员列表和非法翻译语言同样 fail-closed。
5. Runtime Schema 为 1013 条工具，双次装配 registry/source hash 一致；Agent
   examples、Help 参数、Safety 同源和全策略门禁均通过。

## DING / OA 纯读命令 exclusion 收敛证据

本轮继续移出 4 条旧 compatibility exclusion：

```text
ding message list
ding message receiver-status
oa approval search-forms
oa approval ding-info
```

审阅结果：

1. 四条命令均为 `read/low/not_required/idempotent`，不执行发送、撤回或催办。
2. DING 两条命令调用 `im/list_ding_messages` 和
   `im/list_ding_receiver_status`；对应方法未进入 pinned metadata，因此发布
   带具体原因的 `composite`，不伪造 MCP ref。
3. OA 两条命令与 pinned `oa/search_form`、`oa/oa_ding_user` 的参数映射一致，
   分别绑定 `query` 和 `taskId`。
4. DING 的负游标和未知类型在远端调用前返回 typed validation；四条命令的
   缺参经生产根出口统一为 validation/rc=3。
5. Runtime Schema 工具数为 1017，命令路由、参数、Agent 选型、Help 和完整
   pinned interface 映射测试均通过。

## DING 写命令 exclusion 与安全门禁收敛证据

本轮移出 3 条本人身份写命令的旧 compatibility exclusion：

```text
ding message send-personal
ding message send-by-message
ding message recall-personal
```

审阅同时修正了已经公开的机器人 `ding message send/recall`，避免同一产品
因入口不同而具有两套安全语义：

1. 本人/机器人发送均为 `write/medium/user_required/unknown`；本人/机器人
   撤回均为 `destructive/high/user_required/unknown`。
2. 五条命令都声明 request 级 dry-run。生产二进制实跑证明 dry-run 在鉴权、
   endpoint 解析和 ToolCaller 之前返回本地请求预览，真实调用次数为 0。
3. 无 `--yes` 且 stdin 关闭时返回 `confirmation_required`、rc=3，远端调用为
   0；只有明确确认后才派发一次请求。
4. `--type` 仅接受 `app/sms/call`，大小写会归一；过去机器人入口把任意未知
   类型静默降级为应用内 DING，现改为 typed `invalid_argument`。
5. `--users` 经 CSV/JSON-array 归一后必须至少包含一个非空接收人；`,` 等
   伪非空值在调用前失败。
6. 可选 `--uuid` 保留并原样透传，供 Agent 在显式重试时复用幂等键；未提供
   uuid 时仍诚实声明 idempotency unknown，不假定服务端已经幂等。
7. 三条本人身份 RPC 未进入 pinned metadata，Schema 发布带具体路由证据的
   `composite`，不伪造 interface ref。Runtime Schema 工具数增至 1020。

## OA 加签与退回链路 exclusion、安全和参数语义收敛证据

本轮移出 OA 审批链路的 3 条旧 compatibility exclusion：

```text
oa approval append-task
oa approval revert-activities
oa approval revert-task
```

这三条命令原本已经被 mono/multi Skill 正向教授，但不在运行时 Schema 中，
导致 Skill 路由与 Agent 发现层相互矛盾。本轮按完整操作链路审阅：

1. `revert-activities` 是只读、低风险、幂等查询；Agent 必须先从其结果取得
   `activityId/revertAction`。结果为空时不得猜测节点或继续退回。
2. `append-task` 是高风险写操作，`revert-task` 是高风险破坏性操作；二者均
   `user_required`、request dry-run，未确认在调用前以 rc=3 拦截。
3. 生产二进制 dry-run 实跑返回 `executed:false` 和完整规范化请求，远端请求
   数为 0；非法枚举、空加签人和非法布尔值返回 typed `invalid_argument`。
4. `taskId` 先校验为正整数，再以 `json.Number` 传递，避免旧 `float64` 路径
   对大于 JavaScript 安全整数范围的审批任务 ID 造成精度损失。
5. 加签类型和激活方式大小写归一，但只允许服务端定义的枚举；退回发起人时
   强制 `targetActivityId=sid-startevent`。
6. 修正 multi Skill 快速示例中把字符串 flag `--agree-all` 当作无值 bool flag
   的错误，明确写为 `--agree-all true|false`。
7. Runtime Schema 工具数增至 1023；完整 policy 的 Schema 确定性、确认门禁
   同源和 97 条 Agent dry-run 示例全部通过。

## Calendar ACL 与日历本更新 exclusion 收敛证据

本轮移出 Calendar 的 3 条旧 compatibility exclusion：

```text
calendar acl add
calendar acl delete
calendar book update
```

审阅不只是让命令出现在 Schema，还收紧了权限和无效请求边界：

1. ACL 授权是 `write/high/user_required/unknown`，ACL 撤权是
   `destructive/high/user_required/unknown`；日历本更新是
   `write/medium/user_required/unknown`。三条都只在明确确认后调用后端。
2. 三条命令都声明 request 级 dry-run；dry-run 返回规范化请求且远端
   调用数为 0。无 `--yes` 且 stdin 关闭时返回
   `confirmation_required`/rc=3，不会静默取消或继续执行。
3. ACL 权限只接受 `free_busy_reader/title_reader/reader/writer`，大小写会归一；
   空 userId、空 aclId 和未知权限在任何远端调用前 typed failure。
4. `calendar book update` 禁止更新 `primary`，并要求至少提供非空
   `--summary` 或 `--desc`，不再把只含 calendarId 的 no-op 请求当作写操作。
5. mono/multi Calendar Skill 同步说明 owner/非主日历限制、`writer` 权限范围、
   aclId 必须先从 list 结果核对，不鼓励 Agent 猜测标识符。
6. Runtime Schema 工具数增至 1026；完整 policy 的 Schema 双次装配、
   Help/Schema 参数同源、确认门禁同源和 100 条 Agent dry-run 示例全部通过。

## Chat 当前用户会话状态 exclusion 收敛证据

本轮将 8 条只修改当前用户视角会话状态的命令移出 exclusion：

```text
chat mark-unread
chat mark-read
chat clear-red-point
chat clear-all-red-point
chat hide
chat mute-at-all
chat mute-red-envelope
chat group update-alias
```

这一组与群管理、公告、置顶消息和清空聊天记录分开审阅，避免用低风险
个人状态变更为更高风险的群级或破坏性操作背书：

1. 8 条全部是显式设置或清除状态的幂等写操作；单会话操作为 low risk，
   `clear-all-red-point` 因影响所有会话保留 medium risk。它们不修改其他成员数据。
2. 每条都新增 request 级 dry-run；专项测试证明预览返回规范化请求且
   远端调用数为 0。
3. 所有 ID 和群备注会去除首尾空白；空白 `conversationId/messageId/groupId`
   或空白备注会在远端调用前返回 typed validation。
4. 这 8 个 RPC 尚未进入 pinned MCP metadata；Schema 诚实发布带 `im/<rpc>`
   路由证据的 `composite`，而不是伪造 `interface_ref`。
5. Agent 公开参数只保留 `--conversation-id`；`--id/--chat` 仍可作为隐藏兼容别名
   执行，但已从 Contract、Schema example 和 mono/multi Skill 教程移除。
6. Runtime Schema 工具数增至 1034；完整 policy 的 Schema 双次装配、
   Help/Schema/Skill 同源、runtime Safety 同源和 108 条 Agent dry-run 示例全部通过。

## Chat 会话分组 exclusion 收敛证据

本轮将 5 条当前用户会话分组写命令移出 exclusion：

```text
chat category create
chat category delete
chat category rename
chat category add-conv
chat category remove-conv
```

审阅和修复边界如下：

1. `delete` 保持 destructive/high/user_required；手写保护与框架门禁均允许
   dry-run 无副作用预览，真实执行仍须在用户明确确认后追加 `--yes`。
2. `create` 保持非幂等写；`rename/add-conv/remove-conv` 是当前用户分组配置的
   幂等写。五条全部发布 request 级 dry-run，预览远端调用数必须为 0。
3. `category-id` 和 `category-ids` 只接受正整数；空值、负数、零和非数字
   在远端调用前统一返回 typed validation，不再落入普通 internal error。
4. 会话 ID 和分组名统一 trim；空白值在远端前失败。Agent 参数只发布
   `--group`，隐藏 `--conversation-id/--id` 只保留旧调用兼容。
5. Runtime Schema 工具数增至 1039；对应参数、Safety、dry-run、确认门禁和
   业务请求 exactly-once 由专项测试与完整 policy 锁定。

## 下一批优先级

1. 对剩余 9 条 compatibility helper 按操作风险继续做 Agent 审阅；写命令先完成真实
   Safety、确认门禁、dry-run 和幂等性审阅，禁止因为追求 exclusion 清零而直接公开。
2. 真实环境复验 contact 投影 `8/5/1`、`wiki +node-list`、event stop、todo participant、approval create-instance。
3. 在隔离数据上完成 sheet 二次回滚官方自证。
