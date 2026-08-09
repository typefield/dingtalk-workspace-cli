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

## 2026-08-08 真实只读复验（脱敏）

在有效用户登录态下，以下命令均以 `--format json` 运行，退出码为 0，stdout
为单一 JSON 文档；未执行任何写入、删除或审批动作：

| 命令 | 结果 | 验证点 |
|---|---:|---|
| `calendar +book-list` | 2 条日历本 | `result` 数组投影到 `calendars[]`，稳定 ID、名称和权限字段可用 |
| `todo +get-my-tasks --size 1` | 1 条待办 | `result.todoCards` 投影到 `todos[]`，任务 ID 与主题字段可用 |
| `minutes +list-mine --limit 1` | 1 条听记 | `result.itemList` 投影到 `minutes[]`，`taskUuid` 和 URL 可用 |
| `oa +list-forms --limit 1` | 1 条审批表单 | 表单投影保留稳定 `processCode` 与名称，不被 fail-closed 规则误拒绝 |
| `chat +chat-list-all --limit 1` | 1 条群 | 统一结果为 `ok:true/outcome:success`，`meta.pagination` 正确表达 `endpoint_exhausted:false + next_token`，不扩大为完整群目录 |
| `report +inbox-list`（当天、`--size 1`） | 0 条日报 | 真实终态哨兵 `nextCursor=0` 归一后，统一结果为 `ok:true/outcome:success` 与 `endpoint_exhausted:true`，无续页令牌 |
| `report +outbox-list`（当天、`--size 1`） | 0 条日报 | 与收件箱一致验证真实终态零游标归一；统一结果不含续页令牌或版本字段 |

这证明正常非空服务端形状仍兼容本轮 fail-closed 投影改造；空结果、分页终态与
异常形状仍按各行的后续真实复验项继续取证。为保护隐私，本台账不保留实际标题、ID、
邮箱、日历名或听记链接。

## 2026-08-09 Minutes 最近听记目标选择复核

`+latest-minutes`、`+action-items` 和 `+transcript` 在从列表选择后续详情目标时，只接受
`taskUuid`、`taskUUID` 或 `uuid`。可能代表听记文档而非异步任务的通用 `id` 不再被当作
task UUID；只有该字段的条目返回不可重试的 `projection_unknown`，不会在错误资源上继续读取。
时间证据也已收紧：全部条目有合法时间才按时间取最大值，全部无时间才采用服务端顺序；部分有时间
或时间值非法时 fail-closed，不再猜测“最新”。三条入口继续保持隐藏，未扩大 Agent 公开命令面；
当前查询仍只覆盖首批 20 条且无 endpoint 覆盖证明。Agent 内存测试证据见
`agent-scans/minutes-latest-target-projection-20260809.md`；真实服务端形状、排序、分页和详情终态仍待隔离账号复验。

## 2026-08-09 Report latest 投影复核

`report +report-latest` 原先把未知列表容器与显式空列表都归成“暂无日志”，并在列表条目缺少
稳定 `reportId` 时直接输出原始行成功。当前投影已区分已知空、未知容器、非法条目和缺稳定 ID；
部分条目有时间、部分没有时也不再猜测“最新”。不可信形状统一返回不可重试的
`api/projection_unknown`，带恢复 hint，而不是空结果或成功详情。

该命令仍保留在 Agent exclusion：它与规范的 `report outbox list + detail` 路径重叠，并缺少真实账号的
稳定详情样本。本轮只关闭本地投影错误，不形成第二个 canonical。Agent Markdown 证据见
`agent-scans/report-latest-projection-20260809.md`。

## 2026-08-09 Sheet 工作表列表输出晋级复核

`sheet +list-sheets` 已从 `dual_validate` 迁入 `unified_active`：普通 `--format json` 直接输出统一结果，
其中 `data.count` 与 `meta.count` 对齐，且不带版本选择或版本标记。投影只接受 `sheetId` / `sheet_id`；
可能代表工作簿或展示记录的通用 `id` 不能再充当后续操作的工作表 ID。该命令没有可靠的上游分页事实，
因此不伪造 endpoint 完整性。Agent 内存审阅证据见
`agent-scans/sheet-list-projection-20260809.md`；真实工作表的权限、空表和服务端形状仍待隔离文档复验。

## 2026-08-09 DevApp 成员读取输出晋级复核

`devapp +member-list` 已按 terminal command 从 `dual_validate` 单步晋级
`unified_active`，没有批量切换相邻写命令。普通 `--format json` 直接返回
`ok/outcome/data/error/meta`，不含公开协议选择参数或版本标记；它与
`dev app member list` 共用同一 `ServiceResult -> CommandResult` 分类器。专项回归锁定
一次业务调用、稳定成功信封和两入口 outcome 对齐，rollout Agent ledger 确认
`dual_validate=19`、`unified_active=121`。

真实只读探针确认 DevApp 原始终页为 11 项、`hasMore=false`，同时携带非空位置 cursor；
用该 cursor 续读得到 0 项、同一 cursor、仍为终页，证明它不是 continuation。客户端仅在
DevApp 适配层将该形状归一为 `endpoint_exhausted:true` 且不输出 `next_token`。修复后二进制
的应用列表为统一 success，随后成员读取也为 success，返回 1 条带稳定成员 ID 的记录；
整个探针不保存或打印应用、人员、cursor 和原始 JSON。当前结果不证明企业应用权限覆盖、
空成员或权限受限场景。

同一账号继续完成权限 5 页/249 条、事件 3 页/255 条和版本 1 页/0 条的全量 endpoint
续翻，所有 token 均可推进并最终耗尽。四条列表的 active `data` 现只保留资源数组，
`meta.count/meta.pagination` 是唯一 count/分页事实；不再重复发布
`data.count/hasMore/nextCursor`。
脱敏证据见 [DevApp member-list Agent review](agent-scans/devapp-member-list-rollout-20260809.md)。

## 2026-08-09 Sheet Skill 路由复核

`sheet +list-sheets` 已晋级为统一结果入口后，Skill 曾仍把 Agent 引向 legacy `sheet list`，使同一
“先列出工作表、再使用 sheetId”工作流绕过新结果契约。本轮将 Mono/Multi Skill 的 **97** 处正向示例
迁至 `dws sheet +list-sheets --node <NODE_ID_OR_URL> --format json`；旧 `sheet list` 继续作为 CLI
兼容命令保留，但不再是 Agent 教学路径。六个受影响 Skill 的语义版本从 `1.0.0` 升至 `1.0.1`。

Agent 扫描在实际 Skill 语料与当前 Sheet 回归上确认：legacy 正向路由为 0，统一入口为 97 处、覆盖
42 个文件，焦点测试通过。证据见 `agent-scans/sheet-list-skill-routing-20260809.md` 和
`agent-scans/skill-cli-versions-20260809.md`；这只证明本地路由和投影契约，不证明真实文档权限或服务端响应形状。

## 2026-08-09 复合读取类型化错误保真复核

分页/聚合读取原先会给未分类的幂等读错误提供 `retryable:true`，但也因此把下游已经明确给出的
`auth`、`validation`、稳定 subtype、hint、actions 与 `retryable:false` 抹平成通用 API 可重试错误。
这会诱导 Agent 对重新登录、修正参数等确定性失败继续重放。当前共享投影器在保留失败页、artifact、
operation 与 stage 上下文的同时，以已分类下游错误为恢复事实来源；普通未分类读错误仍保留安全的读重试建议。
已登记 subtype 还会服从错误 registry 的 retry policy；`RetryNever` 不会继承外层幂等读取的 `retryable:true`。

该规则已覆盖 `minutes +detail`，Chat 的 `+chat-messages`、`+thread-replies`、`+at-me`、`+search-msg`、
`+chat-search`、`+conversation-list`、`+flag-list`，以及 `+my-groups`、`+chat-list-all` 和 Todo 聚合读取候选，
均已接入该规则。消息搜索的详情富化还会按失败批次生成独立 typed failed item，不再把多个原因压成一个字符串；合计覆盖
**12** 个错误投影点。
四条 IM 复合读取的资源下载也改为双通道：legacy `resourceDownloads.failures[].error` 保持字符串形状，
统一结果则为每个失败资源分别保留下游 `auth` / `validation` / `projection` / `network` 事实及资源 ID，
不再把所有失败资源压成一个通用 API 错误。
`+chat-messages` 的发送者解析与本地导出失败也改用同一投影器，补齐此前手写字段拷贝遗漏的
actions、retry policy、retry-after 与下游 details。
Agent 矩阵测试和源码关系证据见
`agent-scans/composite-typed-error-propagation-20260809.md`。这证明本地错误投影，不证明真实账号权限、
服务端可用性或页面终态。

## 当前结论

| 报告问题 | 当前状态 | 当前证据 | 剩余动作 |
|---|---|---|---|
| attendance 影子 shortcut：15 条不可发现、9 条写命令无门禁 | **已关闭（当前工作树）** | attendance 包内 33/33 shortcut 均公开并带 Contract；加 2 条 smart shortcut 后 `shortcut list --service attendance` 为 35。9/9 写命令均为 `confirmation:user_required`；`+boss-check` 非交互缺 `--yes` 返回 rc=3；`--dry-run` 只返回 `executed:false` | 合入独立提交；发布后二进制复验 |
| 全域影子 shortcut 扫描 | **exclusion 已逐条审阅，公开面仍未关闭** | 当前 Runtime Schema 实测 415 条 shortcut，其中 383 条 public、32 条 exclusion；32/32 exclusion 已由 Agent 写入明确保留理由，未审阅数为 0。新增 `scripts/agent/scan_shortcut_exclusions.py` 现在同时识别 Runtime semantic review 和显式 retained-hidden 决策；本轮将已有 Contract/Safety 的 `todo +related-tasks`、`todo +due-today` 移入 public，并同步 Runtime 目录与 Mono Skill。剩余 exclusion 有敏感 HR 字段、生命周期/权限写操作、重复旧入口和分页/结果契约不足，不能为了清零数字而公开。attendance 已完成，但真实测试跟进清单仍有 156 条 | 继续由 Agent 按真实产品证据决定是否开放；写命令先审 Safety/dry-run，公开前必须完成 Contract/结果投影/真实或受控验证；保留隐藏属于明确产品边界，不再记为无人审阅债务 |
| mail Schema 仅 41/68，缺不可逆命令 | **代码已修** | 当前 `schema mail` 返回 73 个工具，包含 `mail.recall_sent_message` 与 `mail.trash_mailbox_thread`；全局 Schema/Help/运行时门禁同源检查通过 | 以当前版本重新做“可执行叶子 vs Schema”全量对拍，避免沿用旧 68 基线 |
| 12 条读命令 12 种 JSON 信封 | **框架已接入，渐进迁移中** | Framework 2.0 已提供统一 `ok/outcome/data/error/meta`、强类型布尔、单 writer、partial/pending；不公开协议选择参数，不输出版本标记。`drive download` 与 `sheet export` 已作为文件传输首批迁入；高频只读 `aitable +base-list/+base-search`、`drive +recent`、`+search`、`+search-docs`、`doc +search/+list`、`todo +get-my-tasks/+list-sub/+list-attachment/+list-comment`、`oa +list-pending/+list-forms/+search-forms/+list-executed/+list-submitted/+list-cc`、`contact +list-followings/+search-user/+search-mobile/+list-sub-depts/+list-roles/+list-role-members/+list-dept-members`、`calendar +agenda/+attendee-list/+room-search/+room-groups/+book-list/+book-search/+today/+tomorrow/+week/+next-event/+free-slots/+conflicts/+find-room/+free/+my-free`、`minutes +list-mine/+list-shared/+list-all/+minutes-search/+detail`、窄搜索 `+find-file` 和 `sheet +list-sheets` 已迁入。后者经命令级 Agent 审阅已从 `dual_validate` 晋级为 `unified_active`，Skill 的正向发现路线也已同步迁至该入口；既有 `sheet list` 只保留 CLI 兼容。已迁命令的已知空列表、可续页和 endpoint 终态映射为 `meta.count/meta.pagination`；四条搜索和 Calendar 的会议室/日程/忙闲查询在服务端未给分页或覆盖事实时明确输出 `data.pagination_known:false` / `index_coverage_known:false` / `event_coverage_known:false` / `busy_coverage_known:false`，而不把当前搜索页或响应页扩大成业务完整。Doc、Todo、OA 审批发现命令在尚未安全投影续页事实时同样输出 `data.pagination_known:false`；`+find-file` 的未知容器、非对象行或无稳定 `dentryId` 条目，`+search-docs`、Doc 两条发现命令的无稳定 `nodeId` 条目，Todo 读命令的无稳定 `taskId`/`attachmentId`/`commentId` 条目，OA 表单/实例读的无稳定 `processCode`/`processInstanceId` 条目，Contact 读的无稳定用户标识、`deptId`、`labelId` 或 `userId` 条目，Sheet 读的无稳定 `sheetId` 条目，Calendar 的六条基础读缺 eventId/userId/roomId/groupId/calendarId，Minutes 四条列表/搜索读缺 taskUuid，`+find-room` 的无稳定 `roomId` 条目，以及 Calendar 派生读路径的未知日程/忙闲容器、非法行、无稳定 ID 或不可解析的时间区间现在 fail-closed；Minutes 聚合读取的部分 artifact 失败也不再内嵌成人类错误字符串，而以 `partial_failure`/rc=7 保留成功 artifact 与 typed failed 明细。缺列表容器、不可续游标、分页矛盾和无稳定 nodeId/dentryId/spaceId 条目都不再压成空列表或暴露旧 `nextCursor/hasMore` 作为统一结果事实。`scan_rollout_ledger.go` 在隔离配置中从真实 Cobra tree 生成 [Markdown inventory](agent-scans/rollout-ledger-20260809.md)：本轮以迁移前 ledger 为基线，确认四条 devapp 分页读取均为合法的 `dual_validate → unified_active` 单步迁移；跳级或未注明批准的回退才明确标为 `REVIEW`，不把同一提交的自对拍误写成历史迁移证据。Generic MCP passthrough dual 阶段现复用 legacy formatter；[Agent 兼容审阅](agent-scans/dual-validate-legacy-compat-20260809.md) 覆盖 `&<>` JSON、原文、空/无 text block 与单次调用 | legacy 命令按 terminal command 继续迁移；后续发布以该 Markdown 作为 `--baseline` 由 Agent 审阅跳级/回退，不得把迁移责任交给 Agent 或接入 CI |
| 统一输出审计被规划为 CI / policy gate | **已纠正为 Agent 审阅（当前工作树）** | `scripts/agent/scan_unified_result_surface.py` 在临时目录构建当前二进制，运行离线安全样本的 stdout、布尔和顶层键审计，只写 Markdown 证据。`unified-result-surface-20260809.md` 实测 3 个 self-test 与 3 个离线表面检查均通过；临时 `contract_version` 仅是“必须拒绝”的负向样本，未进入 CLI 输出或仓库结果。RFC 与工具说明均明确不接入 `make policy` / CI。 | 真实账号、写操作、分页、异步和服务端终态继续由 Agent 按命令取证；不得把离线样本通过扩大成全表面或业务终态已验证 |
| metadata-only 命令的本地参数错误被归类为 internal | **框架已修** | `LeafSpec.Validate` 是写入前的本地输入校验钩子；普通 error 现由框架统一转为 typed validation、rc=3，主动返回的 API/auth 等 typed error 保持原类别。`event stop` 缺 `subscribe_id/--all`、目标冲突、无 `--yes` 与有效 dry-run，以及 `event consume --as` 的无效身份均已以 CLI 实跑验证 | 继续将旧命令的 RunE 内部参数检查迁到声明层；不把远端响应或本地 I/O 故障误包为 validation |
| 错误 subtype / 恢复提示缺少闭集治理 | **十五批 registry 已关闭变量 reason；真实服务端终态仍待审阅** | `scripts/agent/scan_error_contract.py` 从非测试 Go 源码确认 **105** 个 descriptor、**183** 个直接 `WithSubtype(...)` 或兼容桥调用和 **12** 个经测试的有限间接映射；字面与变量 `WithReason` 已从 31 降为 **0**，9 个直接 `ErrorInfo.Subtype` 均已登记，所有 stable subtype 均有有效恢复提示。扫描表现在区分显式 `retryable:true`、`false`、未声明和未知表达式，避免把 `WithRetryable(false)` 误读为“可重试”；第十五批个人订阅、toolbar `SYSTEM_BUSY`、`request_build_failed` 与模糊 `tools/call` 失败的恢复边界维持原有保守语义。扫描只产出 Markdown 评测证据，不保存 JSON fixture，也不把 Agent 审阅替换成 CI。 | 用受控账号验证个人订阅服务端对 idempotency key 的去重、响应丢失和历史 cooldown 语义；文档/event 的 typed result bridge 按 [RFC-0005](rfcs/0005-operation-outcome-bridge.md) 完成 legacy golden 与真实账号验证后再转 active；不得把 registry 或本地单测误写成服务端终态已经验证 |
| IM 分页不完整错误仍是自由字符串 | **第六批 registry 与首批统一分页读路径已收口** | 群搜索/已加入群/收藏/会话/单聊消息/全量消息/我的群/话题回复的 8 条 `*_incomplete` reason 已改为稳定 subtype；保持既有 reason wire，不改变 legacy 消费方。每条声明为 API + `idempotent_read_only`，命令级 hint 继续要求保留已读结果与 `failures`，按 `nextCursor`/`nextPage` 续读，不能把当前结果当完整。`chat +chat-search`、`+flag-list`、`+conversation-list`、`+thread-replies`、`+chat-messages`、默认多维检索 `+search-msg` 与 @我读取 `+at-me` 已实际进入唯一的 `unified_active` 返回，`meta.pagination` 只在 endpoint 证据明确时出现；`+search-msg` 保留 `indexCoverageKnown:false`，`+at-me` 在单页分页证据未知时保留 `pagination_known:false`，二者都不会把空命中扩大成不存在。后续页与资源/导出失败保留成功页并输出 `partial_failure`；其中 `+chat-messages` 保留后续页下游 typed error 的 category/subtype/hint/actions/retry 安全性，不再抹平成 `api + retryable:true`；[Agent 审阅](agent-scans/im-pagination-rollout-20260809.md)、[复合错误保真审阅](agent-scans/composite-typed-error-propagation-20260809.md) 已覆盖本地投影 | 用真实 IM 账号复验正常多页、空结果、游标冲突、第二页失败及资源下载/导出失败；新增状态机必须按读写安全性、幂等性与终态证据映射稳定 subtype，不能把写请求的模糊失败套用此 read-only 策略 |
| 统一输出直接 subtype 未登记/缺恢复提示 | **第七批已关闭直接分支键缺口** | DevApp 分页 `pagination_invalid` / `pagination_incomplete` / `pagination_conflict`、shortcut 非布尔 `success`、以及 Skill 安装全失败/非法 partial 均改用登记 enum，并输出 hint；DevApp 分页错误还带 `operation=devapp.pagination_projection`，shortcut 类型漂移带 `operation=shortcut.response_projection`，Skill 安装全失败带 `operation=skill.setup`。现有结果类型、reason、退出码不变；Agent 扫描确认 9 个直接 `ErrorInfo.Subtype` 均已登记 | 字面与变量 `WithReason` 已为 **0**；不得把“已登记”误写成真实服务端终态已验证 |
| 复合写失败没有稳定恢复指令 | **第八批已收口三条高风险写路径** | `batch_write_failed`、`doc_grant_permission_partial_failure` 与 `doc_share_message_failed` 已登记为 API + `RetryNever`；保留原有 `reason`、退出码及逐项 details，并新增/强制 hint 与 actions：批量聊天仅重试未确认目标，文档授权先查当前权限/补偿，分享先查 recipients 台账。扫描从 46→43 个 free reason、16→13 个缺有效 hint family | 继续用隔离账号验证“上游报错但实际已写”的真实终态；不能把 registry 标注等同于服务端 exactly-once 证明 |
| 个人订阅 timeout/network/5xx 被标为安全可重试 | **代码已收口，服务端幂等性仍待取证** | 创建请求会带确定性 idempotency key，但无服务端去重证据。当前和历史 `cooldown` 路径对上游 `retryable=true`、408/425/429/5xx、timeout、网络中断一律保留本地退避、向 Agent 省略 `retryable`/`retry_after_seconds`/`next_retry_at`，并提示先核查订阅；明确 auth/validation/non-retryable 拒绝仍为 `retryable:false`。有限本地状态机已映射到 stable `personal_subscription_unverified` / `rejected` / `auth`；HTTP 响应带 `execution_started:true`，当前 blocked 调用带 `false`，网络/超时保持未知。针对分类与旧持久化记录的单元回归已覆盖 | 以受控账号证明服务端对 idempotency key 的去重与响应丢失恢复语义；之后才可为确实安全的 create 重放恢复 `retryable:true` |
| `doc style` 本地输入错误被误归类为 internal | **代码已修，CLI 实跑验证** | `mustFlagOrFallback` 在 helper 边界将“主 flag 与所有兼容别名均为空”统一转为 typed validation；`cover set` 的 image/file 互斥、缺来源、位置越界，以及 `background set` 的缺色/非法色也在发起 MCP 调用前返回 validation。实跑 `cover set --node … --dry-run --format json` 与缺 node 的 `background set` 均为 `type=validation`、rc=3 | 继续将旧 RunE 的本地参数检查收敛到 typed validation；文件读取、上传和远端错误维持原始错误分类 |
| `sheet export` / `drive upload` / `drive download` 缺参被归类为 internal | **代码已修，CLI 实跑验证** | `sheet export` 缺 `--node`、`drive upload` 缺 `--file`、`drive download` 缺 `--node` 或 `--output` 均在任何导出任务、文件读取、上传、下载或远端调用前返回 `validation`、`reason=missing_required_flags`、rc=3；本地路径不存在/目录也以 validation 拒绝，避免 Agent 将修正参数误判为 CLI 内部故障。`sheet export` 与 `drive download --format json` 的成功和 dry-run 现均经统一返回：stdout 仅含顶层 `ok/outcome/dry_run/data`，进度/人读信息不污染 JSON | 继续对旧 RunE 的本地路径、互斥参数和值域校验逐命令收口；不把 MCP/网络/写后不确定错误误归为 validation |
| devapp shortcut 列表投影/分页丢失 | **CLI 已收口；两入口四列表分页与成员正向已真实复验** | `dev app ...` 与 `devapp +...` 的四类列表现共用唯一 `ProjectDevAppListPage`；两入口只把明确空数组投影为成功空列表；缺容器、非法行或缺稳定 ID 均 fail-closed 为 `projection_unknown`。字段类型错误、`hasMore=true` 缺游标、跨层 `hasMore`/cursor 冲突仍为不可重试的 `pagination_invalid/incomplete/conflict`。真实 DevApp 终页会在 `hasMore=false` 时保留位置 cursor；脱敏续读证明该 cursor 返回 0 项且保持同值，不是 continuation，因此 DevApp 适配层输出 `endpoint_exhausted:true` 但不发布 `next_token`。真实分页对拍完成应用 1 页/11 条、权限 5 页/249 条、事件 3 页/255 条、版本 1 页/0 条，均无 token 缺失或循环；业务 `data` 只保留资源数组，count/分页只在 `meta`。两入口首屏脱敏对拍的业务键、count 与分页终态完全一致；成员读取另返回 1 条稳定记录，全部为统一结果、无版本标记。投影迁移后发现 native `dev app list` 的 ResultSpec 仍宣称旧 `items/hasMore/nextCursor`、其余三条和四个 shortcut 未声明 active 结果；现八条命令共用 `DevAppListResultSpec`，live Schema 只发布 `apps/permissions/events/versions`，required 与 NDJSON record path 同源，分页不再重复进业务 data。证据：`agent-scans/devapp-pagination-projection-20260809.md`、`agent-scans/devapp-member-list-rollout-20260809.md` | 继续验证权限受限、其他账号及响应异常；当前读取不证明企业应用目录权限覆盖或业务完整性 |
| devapp credentials shortcut 缺少敏感输出声明 | **代码已修，已纳入 Agent 公开目录** | `devapp +credentials-get` 已补齐 Identity、Safety、ResultSpec 和 `SensitivePaths`，与 `dev app credentials get` 的敏感字段声明对齐；新增 devapp 语义目录后，7 个统一结果试点（含 credentials）均可从 `shortcut list --service devapp` 公开发现，dual-validate 叶子仍按原有渐进状态保留；共享 mapper 现在保留 shortcut 调用参数，`+version-check-approval` 的 `precheckOnly=true` 不会误判为异步发布 | 发布后二进制确认 Schema 不把 secret 写入示例、日志或普通选型说明 |
| mail `success` 字段 string/bool 分裂 | **已关闭（JSON 输出路径）** | Mail JSON 输出会递归把键名严格等于 `success` 的 `"true"`/`"false"` 归一为 Go bool；不转换其他键或普通文本。端到端 helper 输出测试覆盖顶层、嵌套和数组；shortcut 统一结果 mapper 也拒绝非布尔 `success`，避免后续统一迁移重新引入类型漂移 | 发布后二进制抽取报告同组 14 个样本，确认后端新增响应形状仍经过该 JSON 出口 |
| mail `--size` 等隐藏别名在 Skill 中被当作公开契约 | **已关闭（当前工作树）** | Help/Schema/Skill 按命令统一：消息列表公开 `--folder-id`，文件夹列表公开 `--folder`，相关命令公开 `--limit`/`--content`/`--from`；`--size`/`--page-size`/`--body`/`--sender` 及反向 folder 别名仅作隐藏兼容。mono/multi 脚本对子进程传 canonical flag，且可解包历史或统一结果容器 | 发布前用 Agent 语义扫描继续检查其他 Mail 兼容 flag，不将隐参数重新暴露给 Agent |
| Mail 公开读取列表未知响应被伪装为成功空结果 | **CLI 已 fail-closed；6 条点名列表均已迁入统一返回** | `mail +thread-list`、`+folder-list`、`+tag-list`、`+user-search`、`+template-list`、`+contact-list` 现区分已知空数组与未知响应；未知容器、非法行或无可识别字段返回 typed `projection_unknown`，不再向 Agent 声称“没有会话/文件夹/标签/用户/模板/联系人”。`+thread-list`、`+user-search`、`+template-list`、`+contact-list` 还严格投影其公开的 `hasMore/nextCursor`：有续页必须带 token、终态不得带普通 token、外层/嵌套矛盾返回 `pagination_inconsistent`；统一返回保留 `meta.count` 与已知的 `meta.pagination`。`+folder-list`/`+tag-list` 没有公开的分页字段，只输出 `meta.count`，不伪造 endpoint 耗尽 | 用真实邮箱复验六条命令的正常空结果、续页（适用时）和异常形状；后端新增分页字段前不扩大 folder/tag 的完整性声明 |
| `retryable` 与实际重试反向、超时盲重放 | **框架已修，待产品回归** | transport 对 `tools/call` 的 HTTP/网络不确定失败不再自动重放，也不再伪造 `execution_started=true` 或安全 `retryable=true`；改为 `execution_state=unknown` 并保留 execution/trace 恢复线索，错误恢复元数据和 context deadline 已进入统一路径 | 对幂等读、限流、写后超时分别做真实链路复验 |
| macOS Keychain 启动探测无界阻塞 | **代码已修** | `security default-keychain` 探测改用 2 秒 `CommandContext` 超时；避免认证/插件发现阶段因系统安全代理无响应而拖死 CLI。Keychain 包测试与此前超时的 IM 参数等价回归通过 | 发布后二进制在无响应/锁定 Keychain 环境验证快速降级到可诊断错误；不影响正常 Keychain 读取 |
| `dev connect --daemon` 启动/重启过早报告 success | **代码已修，待真实环境复验** | 统一结果路径的 start/restart 现在经过 bounded readiness handshake：匹配 supervisor state、进程存活且 worker heartbeat 已连接才返回 success；supervisor 存活但 worker 未连接返回 `pending` + 可执行 status 命令；状态缺失/进程退出返回 typed internal failure。legacy 前台输出保持不变；`TestDaemonReadinessHandshake` 与 daemon 生命周期回归通过 | 用真实 detached 子进程验证启动失败、worker 立即退出、连接成功和 restart 恢复；确认真实环境下 heartbeat 产生时间不超过 5 秒 |
| `dev connect stop/restart` 可在无确认下发送本地终止信号 | **已关闭（当前工作树）** | 两条本地守护进程控制命令现声明 `destructive/high/user_required`；无 `--yes` 的非交互调用在进入 `daemonStop` 前返回 typed `confirmation_required`、rc=3。专项回归以可被计数的信号 seam 证明 stop/restart 均未发送 SIGTERM/SIGKILL；`--dry-run` 仍只返回 `dry_run:true` 的计划。CLI 黑盒复验确认 stdout 为单一 JSON failure，不含诊断污染 | 发布后二进制复验真实守护进程的确认、dry-run 与确认后 stop/restart 路径；不要把本地测试扩大为真实重启终态已验证 |
| `dev connect status/list` 公开专属 `--json` 与统一输出入口并存 | **已关闭（当前工作树）** | 两条 unified active 命令仅在 Help/Schema/Skill 公开 `--format json`；历史 `--json` 保留为 hidden argv 兼容别名，现有 launchd/cron 调用仍可运行。专项回归锁定 alias 隐藏，原有兼容调用继续覆盖 JSON 信封 | 一个发布周期后根据遥测/兼容策略决定是否删除 alias；在此之前不再新增或文档化局部输出选择参数 |
| `dev connect list` 远端应用名称补全失败被静默吞掉 | **已关闭（当前工作树）** | 本地连接器清单仍以 `success` 返回；远端 `list_dev_app` 失败、未知列表形状、缺少分页状态、缺/重复续页游标或超过安全页数时，统一信封以可选 `_notice.app_name_enrichment` 明确标记 `unavailable/remote_lookup_failed`，不把“名称未返回”伪装成“应用没有名称”。专项回归覆盖成功补全、离线补全和不完整分页 | 用真实 devapp 账号复验名称解析、分页和仅本地连接器场景；本地健康状态不依赖远端名称查询 |
| `wiki +node-list` 挂起 | **CLI 已修并迁入统一返回，待真实环境复验** | 已发布 canonical `wiki.shortcut_node_list` 及 read/low/not_required/idempotent Safety；无网络测试证明只调用一次 `doc/list_nodes`。未知响应形状不再伪装为空列表；分页事实现在由统一返回的 `meta.count/meta.pagination` 承载，业务 `data` 仍保留兼容字段。无分页证据时 `paginationKnown:false`；续页缺 cursor、终态携普通 cursor 或外层/嵌套字段互相矛盾均返回 typed `pagination_inconsistent`，不再猜测 endpoint 完整。当前身份用源码构建二进制的只读根目录实测已返回统一 success，46 条节点均有稳定 `nodeId`，`meta.count` 对齐，且 `hasMore:false` / `endpoint_exhausted:true` 自洽，见 `agent-scans/wiki-node-list-live-projection-20260809.md`。 | 继续用真实环境复验空目录、续页和嵌套分页三组读取；当前正常根目录实测不等同于这些边界已验证。 |
| Doc 搜索/节点列表未知响应伪装为空结果 | **CLI 已 fail-closed，统一输出已 active** | `doc +search` 与 `doc +list` 现在区分已知空列表和未知响应；未知容器、非法行、仅展示字段或无稳定 `nodeId` 返回 typed `projection_unknown`，不再把后端形状漂移扩大成“无匹配文档/空文件夹”。普通 `--format json` 直接输出统一结果；服务端续页事实尚未安全投影时只标 `data.pagination_known:false`，不伪造终页或 token | 发布后二进制以真实搜索无命中、空文件夹、正常续页和异常响应形状分别复验 |
| `drive list --pattern` 失效 | **代码已修，当前回归复核通过** | `drive list` 已公开 `--pattern`；当前回归覆盖当前页通配过滤、JSON 可解析、保留服务端 `nextToken`、未知响应 fail-closed，以及与 `--versions` 的冲突。客户端过滤只声明当前 endpoint 页，绝不把未读取的后续页伪装成目录已耗尽 | 发布后二进制用真实目录做 3 组正反例复验 |
| Drive 列表/搜索未知响应伪装为空结果 | **CLI 已 fail-closed；目录列表 unified active** | `drive +list`、`+search`、`+find-file`、`+search-docs` 区分已知空结果与无法投影的响应；未知容器、非法行或缺稳定 ID 返回 typed `projection_unknown`。`+list` 现以普通 `--format json` 输出统一 `ok/outcome/data/meta`，非空条目必须带 string `dentryId`，并固定声明 `inventory_scope=requested_location`。真实接口采用 token-only 分页：非空 token 只证明可续页，投影为 `endpoint_exhausted:false + next_token`；token 缺失且没有显式终态布尔时保持 `pagination_known:false`，不伪造 endpoint exhaustion。源码构建二进制已脱敏实测首屏、续页和宽页，count/meta 对齐且无协议版本或 legacy 分页键，见 `agent-scans/drive-list-live-projection-20260809.md`；rollout ledger 记录合法 `dual_validate → unified_active` 单步迁移 | 用真实空目录、指定子目录、权限受限和异常响应继续复验；当前根目录读取不证明租户 Drive 全量目录、召回率或权限覆盖 |
| Report 收/发件列表未知响应、分页状态被丢弃 | **CLI 已接入统一结果；收/发件箱终态已真实复验** | `report +inbox-list` 与 `+outbox-list` 已逐条启用统一结果；已知空列表仍成功，未知容器/非法条目返回 typed `projection_unknown`。上游 `hasMore/nextCursor` 同时投影到 `meta.pagination` 与数据字段；缺 cursor 的续页、末页携 cursor、嵌套分页矛盾均返回 `pagination_inconsistent`，不再伪装终态。真实收/发件箱均发现终态 `nextCursor=0` 哨兵，现归一为空续页令牌并输出 `endpoint_exhausted:true` | 用真实收/发件箱继续验证非空首页、续页与无分页信号；确认 JSON 仅有 `ok/outcome/data/meta`，不输出协议版本标记 |
| `drive download --format json` 惰性、stdout 日志污染 | **代码已修，已迁入统一返回** | 最新版与历史版下载均由 Framework writer 单次输出：`drive download`、`drive download-version` 及兼容写法 `drive download --version N` 的成功/preview 都是 stdout 统一返回，失败同样为可机读 failure；进度只写 stderr。当前 Agent 安全 dry-run 对拍两条历史版本路径均为 `ok:true/outcome:success/dry_run:true`，`data` 明确 `operation=download_drive_file_version` 与版本号，不含协议版本字段；缺 `--node`/`--output` 与非法版本号均走 typed validation。大文件下载已具备 Range 探测、并发分片、`Content-Range` 区间/总长校验、短读拒绝、checkpoint 续传、凭证刷新、最终文件长度校验和原子重命名；不是“没有分片下载”。若 MCP 给出 `fileSize`，CLI 还会将本地实际长度与之对拍：不一致返回可重试的 typed `download_size_mismatch`，成功才声明 `verification.state=size_verified`/`method=source_file_size`。当前 SHA-256 仅用于 checkpoint 指纹，服务端尚未提供可对拍的权威整文件摘要，因此不能声称端到端内容摘要已验证 | 发布后二进制执行真实下载成功与上游失败两条 `jq` 管道复验；若上游可提供 checksum/强 ETag，再增加整文件摘要对拍并显式输出验证状态 |
| `drive delete --dry-run` 与确认门禁耦合、EOF 取消仍 rc=0 | **已关闭（当前工作树）** | `drive delete` 声明 destructive/high/user_required；专项测试证明 `--dry-run` 产生请求预览且写调用为 0，关闭 stdin 且无 `--yes` 时返回 typed `confirmation_required`、写调用仍为 0 | 发布后二进制复验 dry-run/EOF/`--yes` 三条路径 |
| `doc version revert --dry-run` 对不存在版本也放行 | **代码已修** | 回滚现在先校验版本号为正整数，再通过版本列表预校验目标存在性；`999` 和非正版本号均在任何远端写入前返回 typed validation，dry-run 对有效目标只读不写；`TestDocVersionRevert*` 回归覆盖存在、缺失和非法版本 | 真实文档分别验证存在/不存在版本号 |
| `todo +add-participant` 报错但已写入 | **代码已修，待真实环境复验** | 参加人写入增加幂等/结果核验路径及专项测试；写请求报错后的“部分已落库”和“未知”路径现在保留结构化 `outcome`、`verification`、`execution_started`、`retryable`、已落库/未确认 ID；即使回读暂未发现任何目标，也保留 `execution_state=unknown`、原始错误链和恢复动作，避免 Agent 盲目重试或只能解析人类错误文本 | 真实任务执行“成功、服务端报错但已写、明确失败”三态复验 |
| Todo 读取未知响应伪装为空结果 | **CLI 已 fail-closed，四条读命令 unified active** | `todo +get-my-tasks` 现在仅在 `result.todoCards` 被明确识别为空数组时返回成功空列表；`+list-sub`、`+list-attachment`、`+list-comment` 也改为仅识别明确列表容器的已知空结果。四条命令对未知容器、非法行、仅展示字段或无稳定 `taskId`/`attachmentId`/`commentId` 返回 typed `projection_unknown`，不再把上游响应漂移说成“没有待办/子任务/附件/评论”，也不再悄悄丢掉不可继续操作的行。普通 `--format json` 直接输出统一结果；我的待办与评论页码输入不等于 endpoint 终态，续页事实未确认时显式标 `data.pagination_known:false` | 用真实组织验证四条入口的空列表、正常任务、分页和服务端异常形状；写入终态问题仍按 `+add-participant` 的真实环境验收执行 |
| Todo 聚合 shortcut 将未知响应伪装为空集合；后续页错误丢弃已读结果 | **两条 Agent 主入口已迁入 unified active；分页覆盖仍保守** | `+related-tasks`、`+due-today`、`+created-todos`、`+overdue` 与写前定位的 `+todo-done` 复用严格 `todoCards` 投影：仅明确数组可继续；缺容器、容器非数组、非法行或无稳定 `taskId` 均在本地 fail-closed 为不可重试的 `api/projection_unknown` / `response_projection`。`+related-tasks`、`+due-today` 已完成 legacy byte、统一成功、后续页 partial 和写前 fail-closed 对拍，普通 `--format json` 直接输出统一结果；第二页失败以 `partial_failure` / rc=7 保留成功页和 typed failed page。当前账号真实只读复验分别得到明确空结果与 62 条稳定 taskId，均为单一 success、count 对齐且 stderr 为空。Todo API 未提供权威 continuation，因此数据始终声明 `pagination_known:false`，不输出伪造的 endpoint exhaustion。`+todo-done` 继续通过 fail-closed facade 读取，任何不完整聚合都会在写请求前停止。Agent 证据：`docs/agent-scans/todo-aggregate-projection-20260809.md`、`docs/agent-scans/todo-aggregate-live-20260809.md` | 用隔离账号继续复验真实多页、第二页失败、权限受限和未知形状；`+todo-done` 验证不完整聚合时绝不发写请求。当前真实结果只证明响应可投影，不证明短页等于 Todo 服务端权威终页 |
| `todo task remove-attachment` 取消仍 rc=0、遗漏附件 ID 校验 | **已关闭（当前工作树）** | 删除附件已改用统一 destructive/high/user_required 门禁；拒绝确认返回 validation rc=3，不再静默成功。`task-id`/`attachment-id` 均在任何远端调用前校验；dry-run 不发写请求。helpers 与 CoreCmd 的公共必填校验统一产出 typed validation rc=3 | 发布后二进制复验取消、缺参、dry-run、`--yes` 四条路径；真实删除仅使用隔离待办附件 |
| IM 零结果却扩大成 `complete:true` | **CLI 语义已修，索引健康未解决** | 统一结果仅以 `meta.pagination.endpoint_exhausted` 表示端点分页耗尽，并保留业务 data 的 `indexCoverageKnown:false`；不再把分页耗尽解释为业务全量完整 | 服务端需要提供索引覆盖/健康证据，CLI 不能自动推断 |
| Chat 已加入群列表未知响应伪装为空结果 | **CLI 已 fail-closed；两个公开群目录入口均 unified active** | `chat +chat-list-all` 与 Agent 主入口 `chat +my-groups` 仅对明确识别的群数组返回已知空列表；未知容器、非法条目或无稳定群 ID 返回 typed `projection_unknown`。自动翻页的后续读取、投影或游标边界失败以 `partial_failure`/rc=7 保留已读页面，不把部分目录包装成完整成功。普通 `--format json` 直接输出单一 `ok/outcome/data/meta`，群句柄统一为 `openConversationId`，legacy `complete/hasMore/nextCursor/stopReason` 与协议版本字段不再进入 active data。当前源码构建二进制对两个入口的真实只读扫描均返回 1 条稳定群、`meta.count` 对齐和 `endpoint_exhausted:false + next_token`，见 `agent-scans/chat-my-groups-live-projection-20260809.md`；fixture 审阅分别为 7/7 PASS。rollout ledger 记录两条合法 `dual_validate → unified_active` 单步迁移。 | 用隔离账号继续验证空群、完整跨页、游标错误和网关响应形状漂移；`endpoint_exhausted` 只表示当前接口分页耗尽，不能扩大为租户群目录完整或索引健康 |
| IM 分页算法与统一返回尚未合流 | **框架基元已落地；九条主要只读入口已 active** | `internal/output.PageLedger` 已实现有限页预算、游标推进/循环校验、耗尽/续页/未知三态、分页 meta 生成，以及首屏失败→`failure`、后续页失败/未知→`partial_failure` 的统一结果映射。`+flag-list`、`+chat-search`、`+conversation-list`、`+thread-replies`、`+chat-messages`、`+search-msg`、`+at-me`、`+my-groups` 与 `+chat-list-all` 的普通 `--format json` 直接生成单一 `ok/outcome/data/error/meta`，不公开版本或协议选择参数；本地页上限保留 `endpoint_exhausted:false + next_token`，后续页或明确的分页边界失败保留成功页并返回 `partial_failure`/rc=7。会话列表、消息历史、话题回复、群目录、默认多维检索和 @我读取都只在服务端证据明确时声明 endpoint 耗尽；非零、非法或浮点 JSON `MaxInt64` 精度边界游标均 fail-closed。群目录统一发布稳定 `openConversationId`，真实只读复验证明两个入口的可续页形状一致，但不承诺租户目录完整。消息历史的后续读取、发送者筛选、资源下载与本地导出失败，搜索的未知容器/缺稳定消息 ID/富化失败，以及 @我资源下载失败，均保留已读页并输出 typed `partial_failure`；搜索继续保留 `indexCoverageKnown:false`。标准 `tools/call` runtime 的 MCP/business 错误统一进入 `server_failure_classifier`：只有 `queryToolMeta` 明确证明业务工具未执行时才保留 `retryable:true`；其余网络不确定失败保留诊断但不鼓励重放。Agent 证据见 `chat-message-cursor-boundaries-20260809.md`、`thread-replies-contract-20260809.md`、`chat-messages-contract-20260809.md`、`im-pagination-rollout-20260809.md`、`chat-my-groups-live-projection-20260809.md` 与 `server-failure-recovery-20260809.md`。 | 以真实账号继续复验九个 active 入口的空结果、完整跨页、可见范围、响应形状和索引健康；其余消息 shortcut 按 legacy→dual→active 逐条迁移。写调用先接入安全上下文，再治理 retryable |
| AITable/Base 目录死条目与假阴 | **CLI 投影与统一输出已收口，服务端精度/召回率未关闭** | `+base-list/+base-search` 已从 `dual_validate` 单步晋级 `unified_active`；Agent 使用 `--format json` 直接获得顶层 `ok/outcome/data/meta`，不再解析 historical 裸 payload。列表/搜索结果显式标记 `sourceKind`、`authoritativeInventory:false`、`inventoryCoverageKnown:false`，搜索额外标记 `indexCoverageKnown:false`；空命中不扩大成业务上不存在。顶层与 `result/data` 的分页证据会聚合校验：`hasMore=true` 无游标、`hasMore=false` 带游标、字段非布尔、多 continuation 不一致或外层/嵌套矛盾均返回不可重试的 `pagination_inconsistent`。续页真相现只出现在 `meta.pagination`，业务 `data` 仅保留 `paginationKnown`，不再重复输出 `hasMore/endpointExhausted/nextCursor/complete`。真实只读 Agent 对拍于内存中从最近访问结果取名称做搜索：两条命令均返回 1 条稳定 `baseId`、count/meta 对齐且 stderr 为空，未保存名称、ID、查询词或原始 JSON，见 `agent-scans/aitable-base-discovery-live-20260809.md`。Skill 正向路由已统一到 `+base-list/+base-search`；rollout ledger 记录两条合法单步迁移。 | 后端继续治理死条目与搜索索引；用真实账号复测已删除 Base、漏召回 Base、空目录、续页和权限受限。当前一组正常响应不证明服务端目录精度或索引覆盖 |
| event 停机契约 | **CLI 已迁入统一返回；真实停机待复验** | `event stop` 为 destructive/high/user_required；无 `--yes` 拦截，`--dry-run` 不停订阅；缺 `subscribe_id/--all` 或两者冲突均在本地返回 typed validation、rc=3，不再误报 internal、rc=5。dry-run、终态成功和失败统一经 `StoreResult → PostRun` 输出，普通 `--format json` 只有 `ok/outcome/data/error/meta`，不公开协议版本。本地消费者停止失败时保留 run state；远端订阅已部分取消但后续取消/本地清理失败时，确认取消进入 `succeeded[]`，无法确认的 subscription/清理阶段进入 `unknown[]` 并输出 typed `partial_failure`、rc=7，首项失败为普通 failure。`event-stop-contract-20260809.md` Agent 扫描为 8/8 PASS | 用隔离账号执行 stop 后验证远端订阅、本地消费者进程和 run-state 三者终态一致；本地 fixture 不替代真实控制面证据 |
| contact 投影层 `8/5/1` 数据丢失 | **代码已修，已完成一组真实只读投影复验** | `+list-roles` 已拍平分组 `labels[]`；角色/部门成员识别 `labelUserList`/`deptUserList` 并解包 `userInfo`。回归测试锁定报告中的 8/5/1 下层计数；任一非空条目无法投影时整体 fail-closed，不再返回成功空/残缺列表。真实组织响应另包含一条 `{labelId:null,name:null}` 的成对空占位行，旧 projector 将它误判为数据丢失并返回 `projection_unknown`；现仅滤除该严格占位形状，半空或未知行继续 fail-closed。源码构建二进制的 Agent 实测：下层 57 条标签行减 1 条占位后，统一输出 56 条角色，lower/upper 对齐；随后一个角色成员与根部门成员读取分别为 4/2 条，均与 `meta.count` 对齐并带稳定 `userId`。证据只保存结构与计数，见 `agent-scans/contact-roles-live-projection-20260809.md`、`agent-scans/contact-members-live-projection-20260809.md`。三条 8/5/1 读取已迁入统一结果；缺稳定 `labelId` 或 `userId` 的展示行会 fail-closed，仍保留数值型 label ID。另 `+list-followings`、姓名/手机号搜索、`+list-sub-depts` 也已进入统一结果：缺明确列表容器、非法行、缺稳定 `userId/openDingTalkId` 或 `deptId` 均返回 typed `projection_unknown`；特别关注保留服务端真实的 `openDingTalkId`，部门数值 ID 也保持原类型，不强制转换为展示字符串 | 继续用原评测账号复跑空/权限受限及其他组织层级，并复验四条发现命令；当前一组真实读取不等同于整个组织目录覆盖或所有异常形状已证明 |
| `contact +me` 原始资料信封与未知投影不可信 | **渐进迁移已完成，真实只读结构复验通过** | 该入口先以独立提交完成 `legacy_only -> dual_validate`：业务只调用一次，ResultSpec 影子校验通过且 legacy stdout exact-byte 不变；再以该阶段 ledger 为基线迁入 `unified_active`。普通 `--format json` 现在只返回 `ok/outcome/data`，不含协议选择 flag 或版本标记。成功结果必须有稳定 `data.userId`，只投影 `userId/name/mobile/email/org/dept`；空数组、多记录、display-only 或未知响应均 fail-closed 为 `api/projection_unknown`，不再回退输出原始服务端对象。`email/mobile` 已声明为 sensitive paths。真实只读 Agent 探针确认单一 success、稳定 userId、stderr 为空且 Schema 与数据字段同源；证据见 `agent-scans/contact-me-dual-validation-20260809.md`、`agent-scans/contact-me-unified-projection-20260809.md` | 用权限受限账号、字段缺失账号和后端异常形状继续复验；当前身份读取只证明本次 endpoint 可投影，不证明组织目录覆盖、资料长期完整或跨租户可见性 |
| Skill/Help/Schema 指令偏移 | **本轮已关闭已知项，持续审阅** | Agent 对拍发现部门查询隐藏别名、AITable 写示例缺身份参数、考勤/听记/会议室脚本隐藏 flag、mono devdoc 类型漂移、devapp 文档误把 devdoc 页码分页写成游标分页，以及运行时已公开但目录漏收 `devapp +credentials-get`；已同步修正代码声明、Help、Schema examples、mono/multi Skill、脚本和公开目录生成器。当前运行时公开集合、提交目录、Mono Skill 三者均为 383 条且集合完全相同；另外将已有 Contract/Safety 的 `todo +related-tasks`、`todo +due-today` 以及此前已审阅 shortcut 从 exclusion 移入 public，并同步 Skill；通用 Lite Recipe 的“所有命令必须 JSON”绝对表述已收窄为终结型命令默认 JSON、脚本按 Help、流式按 NDJSON。`scripts/agent/run_skill_contract_audit.py` 现同时执行路径/显式 flags 和隐藏兼容 flag 逐条 Agent 对拍，并对临时探针路径做脱敏，当前为 1141 条可执行命令且 PASS、隐藏 alias 正向引用为 0；`scripts/agent/scan_shortcut_surface_alignment.py` 的 Agent 扫描证据为 PASS，`gen_skill_shortcut_sections.py --check` 通过；mono/multi `cli_version` 已统一为 `>=1.0.15`，且 14/14 Skill 已补独立 `version: 1.0.0`，Agent 版本扫描同时验证两套版本字段；版本扫描报告已生成 | 发布前继续按产品执行 Agent 语义扫描；CLI 路径/flag 对拍不能代替隐藏兼容别名的 canonical 选择、结果投影和安全审阅 |
| D10 per-Skill 版本声明 | **已关闭（Agent 可验证）** | 14/14 个 mono/multi Skill 根文件均声明独立 SemVer `version: 1.0.0`，并保留独立的 `cli_version: ">=1.0.15"`；`scripts/agent/scan_skill_cli_versions.py` 同时校验字段存在性、SemVer 格式和 CLI 兼容版本覆盖，当前 PASS | 后续 Skill 内容/路由/安全契约发生不兼容变化时递增对应 Skill 版本；不得只改 CLI 版本或所有版本静默保持不变 |
| Mono 脚本 Help/Skill 统计与错误路径漂移 | **Help/机器错误边界与关键写编排已收口；只读聚合继续逐脚本治理** | 35 个 Python 文件按 AST 分为 32 个 Agent 入口、3 个内部模块；Agent 扫描实测 32/32 暴露 `--dry-run` 与 `--format`，Help 非零为 0。所有入口均经 `_runtime.run_main` 统一机器异常边界：未捕获异常、stdout 污染、非法 JSON、无 typed error、meta/dry_run 类型错误或结果/退出码矛盾，都会改写为单一 typed failure；业务 `SystemExit(0)` 也不能绕过该校验，仅显式 `-h/--help` 保留 argparse 人读输出。旧 `success:false`、非布尔/矛盾状态和子 pending 都不会冒充终态成功。Skill 已说明 `partial_failure`/rc=7、三通道和 meta。Mono/Multi Todo 的日期汇总/逾期检查不再用手写 subprocess 抹掉 child 结果：首屏 typed failure 原样分类，后续页失败保留已读页并返回 `partial_failure`/rc=7，达到 10 页硬上限时以 `unknown page:11` 拒绝伪称完整，page meta 透传到外层。 | `probe_mono_result_contract.py` 为 **26/26 PASS**，覆盖 32/32 入口异常边界、严格 child 结果解析与代表性写编排；`probe_mono_dry_run.py` 为 **10/10 PASS**；`probe_mono_todo_read_contract.py` 已扩到 Mono/Multi 四入口，为 **24/24 PASS**，以临时 child runner 对拍短页、后续页失败、首页失败、硬上限、text rc 与 dry-run。以上均不替代真实账号副作用、端点耗尽和服务端终态证据；其余仍手写 child subprocess 的只读脚本须按实际路由继续迁移，不能把共享 `meta` 参数存在扩大成全量已透传。 |
| Mono 邮件/日程/表格写入结果真实性 | **代码已收口，真实终态待验证** | 邮件旧 `success:false` 不再误报已发送；日程在事件创建后参会人/订房失败时保留 `succeeded[]/failed[]/unknown[]` 并返回 `partial_failure`。记录导入、字段创建、AITable 文件导入和附件上传均保留确认成功、明确失败与未知写入事实。异步导入在后续 `import_data` 不可信时保留 importId、阶段及 per-step meta；异步导出则将任务与本地文件分开：创建/轮询不可信不建议重试，任务 pending 有 next_command，本地下载以原子落盘和 SHA-256 表达，且无上游 checksum 时明确不是远端验证。 | `mono-result-contract-20260809.md` 为 **26/26 PASS**；`mono-aitable-export-contract-20260809.md` 为 **10/10 PASS**，仅以临时 child runner/HTTP server 对拍脚本编排、任务结果表达与本地原子写入。仍须在隔离账号验证邮件、日程、导入/导出、字段和附件的真实最终状态；受控 fixture 不等同于服务端 exactly-once 证据。 |
| Multi 脚本 Help/写入预览与结果漂移 | **受控结果边界已覆盖 AITable、考勤排班、Todo、OA、Mail、Doc 与 Calendar 的高风险入口；真实终态待验证** | 当前文件/入口统计以最新 Agent 全量审计报告为准。AITable import/field/record/attachment 与异步 export 五个写入或文件落盘入口支持脚本级 `--dry-run`；受控 fixture 不调用 dws/OSS。`aitable_export_via_task.py` 以任务创建、可恢复 pending、明确终态失败和本地下载分层：任务未完成时返回 `taskId/next_command`，不重复创建；下载使用同目录临时文件、`fsync`、原子替换并报告本地 SHA-256，但服务端未提供 checksum 时只标 `source_integrity=unverified_no_remote_checksum`，不伪称远端完整性已验证。专项 Agent 探针为 9/9 PASS。`attendance_schedule_import.py` 现以 `--dry-run --format json` 产出只读校验后的 `preview`，以 `--yes` 作为确认门禁；未确认不写入，写请求异常明确标 `execution_state=unknown`，请求接受仅标 `verification.state=not_verified`，避免将 API 返回扩大成逐条排班终态。排班写入的校验边界只信任考勤组关联班次：组详情缺少标签时，仅对已绑定 `classIds` 精确调用 `attendance class get`；组绑定缺失、详情失败或没有名称都在确认和写入前 fail-closed，绝不以企业全局 `class search` 填充预览，更不展示 `未知班次(ID:...)`。专项 Agent 探针为 8/8 PASS。Todo 和 OA 分别保留逐项结果与审批任务选择事实。AITable 维持可独立安装的局部 `_runtime.py`；Mail、Doc 与 Calendar 均在自身 Skill 中明确要求加载 `dingtalk-shared`，因此迁入其 shared runtime，由它统一 `--format text|json|ndjson`、`--dry-run`、异常兜底、child DWS 明确失败/终态未知分类和单一 JSON stdout。`mail_send_with_cc.py` 现在要求 `--yes`，dry-run 不读取邮箱或发送；旧业务布尔 `success:false` 返回 typed failure，发送后必须取得 `internetMessageId` 并以 `mail message verify` 的 `sendStatus=success` 才标为 `verification.state=verified`。投递中、无标识或查询失败都保留未知/非终态，不建议重发；服务端仅报告 `partial_success` 却不给逐收件人明细时，输出 `partial_unknown` failure 而不伪造 `succeeded[]/failed[]`。`doc_create_and_write.py` 同样要求 `--yes`，dry-run 不发任何 child 调用，拒绝非幂等自动重试，逐块保留 `succeeded[]/failed[]/unknown[]`；所有写调用成功后再 `doc read`，只有读回正文逐块对拍才标 `verification.state=verified`。`calendar_schedule_meeting.py` 在 `--yes` 前仅允许零调用预览；带 `--book-room` 时先做只读搜房，未获得可信 roomId 即失败且不创建日程，后续建日程、加参会人、订房分别进入三通道，最后仅以 `event get` 回读完整对拍标记 verified。Mail 8/8、Doc 6/6、Calendar 7/7 Agent 探针只保存 Markdown，不保存 JSON fixture。 | 按实际 Skill 路由逐个迁移其余高风险脚本；先做受控 child-runner 与真实终态验证，再扩展共享运行时，禁止把“有 dry-run”扩大成“写入终态已验证” |
| Multi Todo 批量创建错误路径与结果压缩 | **代码已收口；终态验证待真实复验** | `todo_batch_create.py` 不再对类型错误直接 `.strip()` 或向 JSON stdout 泄漏 traceback；新增局部运行时负责 `--format text|json|ndjson`、`--dry-run`、单一 writer、异常边界、旧 `success:false` 严格识别和 child meta 透传。每个待办都进入 `succeeded[]/failed[]/unknown[]`：创建成功后，脚本只会在可识别 `taskId` 且 `task get` 回读成功时放入 `succeeded[]` 并标注 `verification.state=verified`；创建已受理但缺 ID、回读失败或响应不可信时均保留在 `unknown[]`，不会把 request success 扩大为业务终态成功。首项已验证、后项不可信时返回 `partial_failure`/rc=7，不抹掉已确认项目，也不鼓励盲重试。`scripts/agent/probe_multi_todo_batch_contract.py` 只产生 Markdown Agent 语义证据，不保存 JSON fixture。 | 用隔离账号分别验证成功创建、上游报错但已创建、明确拒绝和逐项回读；不得以受控 child runner 或 API 成功替代真实待办终态 |
| Multi OA 批量审批任务选择与结果压缩 | **代码已收口；审批终态待真实复验** | `oa_batch_approve.py` 现要求用户确认后的 `--yes`；JSON 非确认路径返回 typed `confirmation_required`，不会读写审批。dry-run 只输出计划，不调用 dws。待审批列表与 tasks 返回形状不明时 fail-closed；每个实例必须恰有一个可执行 taskId，多个任务不会任选第一个。批量结果保留 `succeeded[]/failed[]/unknown[]`：任务查询或动作响应不可信时保留 unknown，首项动作受理、后项未知时返回 `partial_failure`/rc=7。动作后尝试读取 detail，但只标 `verification.state=not_verified` 或 `verification_failed`，不将详情可读或 API success 扩大为审批终态成功。`scripts/agent/probe_multi_oa_batch_contract.py` 只输出 Markdown Agent 语义证据，不保存 JSON fixture。 | 用隔离账号验证批准、拒绝、无 task、多 task、动作已受理但响应丢失、实例状态推进与终态；不以受控 child runner 代替真实审批效果 |
| Multi 局部结果边界之间的实现漂移 | **Agent 语义对拍已建立；仍为渐进迁移** | AITable（含异步 export）、Todo、Misc（含 attendance schedule import）继续保留可独立安装的局部 `_runtime.py`。Misc runtime 还修正了 JSON 默认脚本的 `argparse --help` 捕获：Help 以发现文本原样输出，不再空 stdout 成功。Mail、Doc 和 Calendar 均明确要求加载 `dingtalk-shared`，因此显式使用其中的 shared runtime，而不是隐藏地依赖另一个产品 Skill；四个运行时同样对严格布尔/非布尔旧结果、矛盾、未知或非字符串的 `ok/outcome` 负责；coherent `pending` 必须保留任务 meta 且转为 `unknown/operation_pending`，不扩张成终态成功。`failure` 缺 typed error、或 `meta/dry_run` 类型错误，也会被统一出口改写成带稳定 `machine_stdout_contract` 诊断的 failure。异常边界、三通道、stdout 与 0/1/7 退出码继续由共用运行时收口。`scripts/agent/probe_multi_runtime_boundaries.py` 横向验证本地与 shared 运行时，新增上述形状错误、矛盾、非字符串或非终态 pending 统一信封不能被判为成功的 **36/36** 对拍；Mail/Doc/Calendar/AITable-export/attendance-schedule 专项探针验证产品语义；它们仅输出 Markdown Agent 证据，不保存 JSON fixture。 | 随下一批迁移继续执行 Agent 对拍；共享运行时只允许被 Skill 明确要求加载的依赖使用，避免脆弱的跨产品相对导入 |
| Calendar 公开读取未知响应伪装为空结果 | **CLI 已 fail-closed** | `calendar +agenda`、`+attendee-list`、`+room-search`、`+room-groups`、`+book-list`、`+book-search`、`+today`、`+tomorrow`、`+week`、`+next-event`、`+free-slots`、`+conflicts`、`+find-room`、`+free` 与 `+my-free` 现在区分已知空数组和未知响应；未知容器、非法行或无稳定字段返回 typed `projection_unknown`。基础日程/参会人/会议室/分组/日历本投影现在也必须带可继续操作的非空 string ID（eventId/userId/roomId/groupId/calendarId），展示名或数值/空 ID 不再作为成功结果泄漏给 Agent。`+find-room` 只接受带非空 `roomId` 的会议室条目；日程展示路径拒绝无稳定 ID，派生空档/冲突路径拒绝无开始或结束时间，忙闲路径拒绝无 `scheduleItems` 或无可识别区间的条目，并将 Calendar endpoint/忙闲覆盖事实标为未知，不会把响应结构漂移表述为“没有日程/无冲突/全天有空/某人空闲/没有会议室/日历本” | 用真实账号分别验证空结果、嵌套容器、各入口的分页与服务端异常形状；endpoint 耗尽语义仍需以实际分页字段为准 |
| Mono Skill 深层 doc/Minutes 指令偏移 | **已关闭（文档规范面）** | doc 文件管理深层 reference 已改为迁移说明，不再正向教授 deprecated `doc upload/download/copy/move/rename/delete`；Minutes 文档按当前 Help 统一使用 `--limit/--cursor`，旧 `--max/--next-token` 只作为兼容事实 | 发布前继续由 Agent 扫描深层 reference，不把路径存在性检查当作 canonical flag 验证 |
| Minutes 听记列表未知响应伪装为空结果 | **CLI 已 fail-closed** | `minutes +list-mine`、`+list-shared`、`+list-all` 共享投影器以及 `+minutes-search` 现在区分已知空列表和未知响应；未知容器、非法条目或没有稳定 `taskUuid` 的条目返回 typed `projection_unknown`，不再把服务端形状漂移扩大成“没有听记”。三条列表已迁入统一结果并明确 `pagination_known:false`：上游未给出可安全投影的续页事实时，不会把当前页放大成 endpoint exhausted。搜索、`+latest-minutes`、`+action-items` 与 `+transcript` 现共用同一 `list_by_keyword_and_time_range` 容器识别，包含服务端常见 `result.itemList`；后面三条复合读只有明确空数组才会提示“暂无妙记”，未知容器、非法项或无稳定 taskUuid 均 fail-closed。搜索结果额外明确 `index_coverage_known:false` / `pagination_known:false`，空命中不扩张成索引健康或业务不存在。`+detail` 也已迁入统一结果：多 artifact 读取全成功才返回 success；部分失败保留成功 artifact 与原始 typed error（category/subtype/hint/actions/retry guidance）并以 `partial_failure`/rc=7 返回；全失败仍是普通 API failure，但 `error.details.failed_artifacts[]` 保留每个 artifact 的 typed error，不再压缩成名称列表。[Agent 审阅](agent-scans/minutes-detail-artifact-failures-20260809.md)、[复合错误保真审阅](agent-scans/composite-typed-error-propagation-20260809.md) 验证该投影。 | 用真实账号分别验证我创建、他人共享、关键词无命中、`result.itemList` 正常形状及分页 token；对 `+detail` 用真实任务验证基础信息/摘要/关键词/逐字稿/待办的单项失败与全部失败；当前仍不把无完整分页或索引证据的命令承诺为完整终态 |
| public leaf 依赖 Schema exclusion | **兼容 helper 已清零；产品边界 exclusion 已逐条审阅** | `contact label list/get/list-members`、`todo task list-sub/remove-attachment`、Chat/DING/OA/Calendar 的已审阅命令均已完成 Identity、接口/参数、Safety 和 Agent 选型审阅并移出 exclusion；当前 Runtime shortcut 为 415 条，其中 383 条进入 public，32 条保留隐藏，32/32 已有明确 Agent 决策理由。保留项集中在敏感 HR 数据、应用/订阅/权限生命周期写操作、重复旧入口以及分页/结果契约尚不足的读取入口；不因可执行就强行公开。上游 main 合并后当前运行时 Schema 工具数为 1119 | 继续按真实产品证据决定是否开放；保留隐藏是产品边界，不再作为无人审阅债务 |
| sheet 二次回滚 bricking | **CLI 侧已加防护，终态未关闭** | `sheet version revert` 现在先通过只读版本列表预校验；不存在版本不会发起回滚，dry-run 允许读版本但不写入；版本列表游标重复或超过安全页数时返回 `pagination_inconsistent`，不会把未证实目标当作不存在或继续写入 | 仍需隔离表格、备份和服务端协同做二次回滚官方自证；失败时保留可恢复证据 |
| `sheet +list-sheets` 未知响应可能伪装为空列表 | **CLI 已 fail-closed；统一输出已 active** | `get_all_sheets` 投影现在区分“已知空列表”和“未知响应”；未知容器、非法行或只有展示标题而无稳定 `sheetId` 时返回 typed `projection_unknown`，不会向 Agent 产出成功的空或不可继续操作的工作表列表。统一 `CommandResult` 带 `meta.count`；命令级 Agent 审阅和焦点回归后，普通 `--format json` 已直接输出统一结果。该入口没有可靠上游分页事实，明确不声明分页终态。Skill 的 97 处正向发现路径已同步切换到它；旧 `sheet list` 不再被教学。回归覆盖空列表、嵌套容器、非法响应、display-only 条目与通用 `id` 不能充当 `sheetId` | 发布后二进制用真实表格验证正常空表格与后端响应漂移两条路径 |
| sheet `formula-verify` 0/11 | **CLI 契约已修，服务端能力未关闭** | 命令已接入 `verify_formula`，支持整表/范围/targets、dry-run 与 `--exit-on-error`；发现公式错误时现在返回 typed validation `reason=formula_errors_found`，保留 `status/totalErrors/scannedCells` details，而不是被归类成内部错误；参数冲突、非正限制、targets 文件/stdin/JSON 错误也统一为 typed validation | 服务端上线并用评测表格复跑 11 个公式校验样本；确认错误位置、错误类型和数量投影完整 |
| approval 真实提单 | **未关闭** | Schema 与三件套能力存在，但原报告缺真实创建成功证据；简单模式 `--form-values` 现在按字段名稳定排序，避免 map 随机顺序造成请求不稳定。当前身份的只读 Agent 预检可以列出模板，但没有名称可识别的隔离测试候选（服务端 total 为未知 sentinel）；证据仅记录聚合数，不保存模板名、processCode 或响应 JSON，见 `agent-scans/approval-create-preflight-20260809.md` | 使用获授权测试审批模板完成一次真实创建并清理测试实例；嵌套表格仍用 `--request` 并做真实链路复验 |
| OA 审批表单/实例列表未知响应伪装为空结果 | **CLI 已 fail-closed，六条发现命令 unified active** | `oa +list-forms`、`+search-forms`、`+list-pending`、`+list-executed`、`+list-submitted`、`+list-cc` 现明确区分已知空列表和无法投影的响应；未知容器、非法行、仅展示字段或无稳定 `processCode`/`processInstanceId` 返回 typed `projection_unknown`，不再把上游形状漂移伪装成“没有表单/审批单”，也不把无法继续查询或处理的行交给 Agent。普通 `--format json` 直接输出统一结果；当前上游分页事实未安全投影，显式标 `data.pagination_known:false` 而不伪造审批目录终态。当前身份用源码构建二进制做的只读实测返回统一 `ok:true/outcome:success`，`meta.count` 与投影条数一致，且在服务端 `totalCount=-1` 时保持 `pagination_known:false`、不输出虚构 pagination，见 `agent-scans/oa-list-forms-live-projection-20260809.md` | 用真实审批账号分别复验空结果、含 `result.processCodeList`/`result.values` 的正常响应、翻页和服务端异常形状；真实提单仍需获授权测试模板 |
| `dws api` 默认 MCP 登录态不可用 | **未关闭（需要后端能力）** | 默认登录保存的是只能由 MCP 网关解密/代理的 token，不是可直接发给 `api.dingtalk.com` 的 access token；CLI 当前 fail-closed，并返回 `raw_api_credentials_required`、`mcp_default_token_usable:false`、可执行认证 actions，且不标记可重试 | 后端提供受限 raw-API proxy/capability 才能让默认身份使用；在此之前仍要求自有 AppKey/AppSecret，禁止 CLI 伪造或转发不适用的密文 token |

## Minutes 聚合脚本本轮验收证据

Mono 与 Multi 的 `minutes_recent_summary.py`、`minutes_extract_todos.py` 已从手写
`subprocess.run + None` 迁入共享 child-result 边界：

1. 列表只接受已知容器与稳定 `taskUuid/taskUUID/uuid`；通用展示 `id`、未知容器、
   非法行不再被压成空列表，而是 `api/projection_unknown`。
2. 单条摘要/待办失败不会被替换为“暂无摘要”或静默跳过；成功项进入
   `succeeded[]`，失败项保留 typed error，混合结果为 `partial_failure` / rc 7。
3. 子 `dws` 的 list/detail meta 按稳定步骤 ID 透传；text 与 JSON 使用同一 outcome。
4. `minutes_extract_todos.py --id ... --dry-run` 不再实际调用 `minutes get todos`。
5. Multi Skill 的正向脚本路线现显式使用 `--format json`；需要摘要文件时使用
   `--format text --output <path>`，不把机器模式与本地文件交付混在一起。

`docs/agent-scans/minutes-aggregate-contract-20260809.md` 使用临时 child runner 对拍
Mono/Multi 四入口的成功、逐项失败、投影漂移、display-only ID、text rc、meta 和指定
ID dry-run，为 **22/22 PASS**。该证据不证明听记索引覆盖、服务端分页耗尽或真实内容。

## Report 聚合脚本本轮验收证据

Mono 的 `report_inbox_today.py` 与兼容入口 `report_received_today.py` 已改为共用同一
实现；Multi Misc 的 `report_received_today.py` 同步使用相同边界，不再保留单页查询、
无机器格式和详情失败静默丢失的旧实现：

1. 首页失败返回 typed failure，不再伪装成“暂无日志”；后续页失败保留已取得日志并
   返回 `partial_failure` / rc 7。
2. 只有明确观察到 `hasMore=false` 才输出 `endpoint_exhausted:true`；`hasMore=true`
   却无 cursor、cursor 循环或超过 100 页均 fail-closed 为 `pagination_inconsistent`。
3. `--detail` 在 text/JSON 两种模式都实际逐条读取正文；单条详情失败保留成功详情和
   失败 reportId，不再只在 text 模式执行或静默跳过。
4. `result[]` 与 `_internalDetailCommands[]` 无法逐项对应、详情内容形状未知时返回
   `projection_unknown`，不把展示 ID 当稳定 reportId。
5. `--dry-run --detail` 只生成列表与详情计划，受控探针证明零 child 进程。

`docs/agent-scans/report-aggregate-contract-20260809.md` 对三个入口验证两页耗尽、首页/
后续页失败、分页矛盾、JSON detail partial 与 dry-run，为 **18/18 PASS**。该探针使用
临时假 `dws`，不保存 JSON fixture，也不证明真实账号的日志可见性或服务端终态。

## 未读邮件聚合脚本本轮验收证据

Mono/Multi 的 `mail_unread_summary.py` 已从手写 subprocess 迁入共享 child-result 边界：

1. 邮箱发现失败保留 child typed error，并在第一步终止；不会继续用猜测邮箱搜索。
2. 邮箱列表只接受已知 `emailAccounts[]`/邮箱对象，并优先选择 `type=ORG`；未知形状
   返回 `projection_unknown`，真实空账号返回 `mailbox_not_found`。
3. 消息搜索失败不再显示“收件箱清空”；只有已知 `items[]/messages[]` 为空才返回
   `success + count:0`。
4. 邮箱发现和消息搜索的 child meta 均按步骤 ID 透传；`--limit <= 0` 在任何 child
   调用前返回 validation；dry-run 为零 child 进程。
5. Multi Mail 的正向 Skill 路由显式追加 `--format json`。

`docs/agent-scans/mail-unread-contract-20260809.md` 对两个入口验证成功、已知空、邮箱/
搜索 typed failure、两类投影漂移、dry-run 和参数校验，为 **16/16 PASS**。临时探针不
证明真实邮箱权限、索引完整性或服务端终态。

## Calendar 只读脚本本轮验收证据

Mono/Multi 的 `calendar_today_agenda.py` 与 `calendar_free_slot_finder.py` 已从宽松
`None/[]` 投影迁入共享 child-result 边界：

1. 日程列表只有识别到已知 `events[]` 才能返回空/非空成功；child failure、未知容器、
   非对象事件和不可识别时间均返回 typed failure，不再显示“暂无日程”。
2. 忙闲查询必须证明所有显式参与人的覆盖；缺任一 `userId` 返回
   `coverage_unknown`，不会把缺失用户当全天空闲。
3. 每个非 FREE 时间段必须有可解析且 `end > start` 的时间；非法/未知形状返回
   `projection_unknown`，不会生成推荐 slots。
4. 参数校验（用户、日期、时长、工作时间）在 child 调用前完成；dry-run 为零 child
   进程；成功路径输出 `coverage.complete:true` 和 child meta。
5. Multi Calendar 的正向脚本路线统一显式使用 `--format json`，Skill 明确只有覆盖完整
   才能向用户推荐空闲时段。

`docs/agent-scans/calendar-read-contract-20260809.md` 对四入口验证已知空、typed failure、
投影漂移、参与人覆盖、忙时段、参数校验、meta 和 dry-run，为 **26/26 PASS**。临时探针
不证明真实日历权限、数据覆盖或服务端终态。

## Contact 部门成员聚合脚本本轮验收证据

Mono/Multi 的 `contact_dept_members.py` 已从“失败即 None、逐部门 continue”迁入共享
child-result 边界，并统一使用公开 canonical `--depts`：

1. 部门搜索 child failure 原样保留 typed error；未知容器、非法行、缺稳定 `deptId`
   返回 `projection_unknown`，不再静默跳过。
2. 每个匹配部门独立查询成员；单部门失败/投影漂移保留成功部门并返回
   `partial_failure` / rc 7，全失败才返回 failure。
3. 成员只接受已知 `deptUserList[]/userlist[]` 与稳定 `userId`；未知结构不伪装成
   “暂无成员”，已知空数组则保留对应部门并返回空成员成功。
4. 子调用 meta 按 search/department ID 透传；dry-run 为零 child 进程；空 query 在调用前
   validation。
5. 输出明确 `coverage.scope=server_search_response, complete=false`，避免把关键词搜索结果
   扩大成完整组织目录；Multi/Mono 正向脚本示例统一显式 `--format json`。

`docs/agent-scans/contact-dept-members-contract-20260809.md` 对两入口验证搜索、稳定 ID、
逐部门 partial、投影漂移、已知空、meta、dry-run 和参数校验，为 **22/22 PASS**。临时
探针不证明真实通讯录权限、索引覆盖或跨层级完整性。

## OA 待审聚合脚本本轮验收证据

Mono/Multi 的 `oa_pending_review.py` 已从“列表失败/详情失败都继续输出”迁入共享
child-result 边界：

1. 待审列表 child failure 原样返回 typed error；未知容器、非法行、缺稳定
   `processInstanceId` 返回 `projection_unknown`，不会伪装为暂无待审或跳项。
2. 每个实例独立读取详情；单详情失败/投影漂移保留成功实例并返回
   `partial_failure` / rc 7，全详情失败返回 failure。
3. JSON 与 text 使用相同业务执行，JSON 现在实际包含成功读取的
   `formComponentValues[]`；不再只在 stderr 打印详情后丢失。
4. list/detail child meta 按步骤 ID 透传；dry-run 为零 child 进程；非法 days 调用前
   validation；输出明确列表覆盖未验证为完整。
5. Mono/Multi 正向 OA 脚本示例统一显式 `--format json`。

`docs/agent-scans/oa-pending-contract-20260809.md` 对两入口验证列表、稳定实例 ID、逐详情
partial、投影漂移、已知空、meta、dry-run 与参数校验，为 **20/20 PASS**。临时探针不
证明真实审批权限、可见范围或服务端终态。

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
8. 实测 follow-up 发现 `send_ding_by_message` 的上游枚举要求为
   `APP/SMS/PHONE`，而 CLI 继续对用户接受 `app/sms/call`；已在 RPC 边界做
   `app→APP`、`sms→SMS`、`call→PHONE` 归一，并用路由契约测试锁定。需在真实
   账号上用合法消息/群 fixture 复跑，确认后端业务校验通过；本地修复不等同于
   真实环境闭环。

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

## Chat 共享内容写入 exclusion 收敛证据

本轮将 5 条会改变公共可见状态或向他人发送内容的命令移出 exclusion：

```text
chat message set-top-msg
chat message unset-top-msg
chat group notice create
chat group notice edit
chat group share-invite
```

审阅和修复边界如下：

1. 五条均保持 write/medium，并统一改为 `confirmation:user_required`；无明确确认
   不会发起远端调用，Agent examples 不携带 `--yes`。
2. 五条均发布 request 级 dry-run；预览无需确认且远端写调用数为 0。
3. 会话、消息、公告和接收方 ID 在远端前 trim 并拒绝空白；公告正文只用 trim
   判断空白但保留原始 Markdown 字节，不破坏代码块缩进与换行。
4. 定时公告 `--run-at` 只接受带时区 RFC3339 或按北京时区解释的不带时区
   ISO-8601 时间；分享有效期拒绝负数。
5. 分享邀请把 `--target/--receiver` 的 exactly-one 关系同时发布成
   `require_one_of + mutually_exclusive`，Help、Schema、Skill 和运行时一致。
6. 公告创建和未显式幂等键的邀请分享按 `non_idempotent` 发布；公告编辑因
   `--send-ding` 可能重复触达保守标为 `unknown`；消息置顶/取消置顶为幂等。
7. Runtime Schema 工具数增至 1044；剩余兼容 helper exclusion 降至 4 条。

## Chat 最后 4 条 compatibility helper exclusion 清零证据

本轮完成最后 4 条待审命令，`compatibility-helpers-pending-review` 分组已删除：

```text
chat chmod
chat data-auth cross-org
chat clear-messages
chat group audit-join-validation
```

审阅和修复结论：

1. `chat chmod` 与 `data-auth cross-org` 都会扩大授权范围，统一按
   high/user_required 发布；仅在本地参数全部合法后进入确认门，request dry-run
   不调用授权 RPC。`chat chmod` 只接受 `chat.*` scope，目标选择关系进入 Schema；
   跨组织授权的 `--target-org-id/--all` 以 exactly-one 契约发布。
2. `clear-messages` 保持 destructive/high，逻辑终态幂等；只公开 canonical
   `--conversation-id`，隐藏兼容别名不再进入 Agent 参数面。空白 ID 在确认前以
   typed validation 拒绝，dry-run 不调用清空 RPC。
3. `audit-join-validation` 按群成员访问变更提升为 high/user_required；CLI、Help、
   Schema 与 Skill 只发布真机可用的 `AuditApprove/AuditDelete`，其余服务端拒绝值
   在本地确认前 fail-fast。申请人与邀请人明确为记录返回的 `userId`。
4. 四条命令均由专项测试锁定 ContractFinal、参数投影、确认前零远端调用、
   dry-run 零远端调用以及真实执行 exactly-once。
5. Runtime Schema 工具数增至 1055；待审 compatibility helper exclusion 为 0。

## Skill setup 生命周期 exclusion 收敛证据

`skill setup` 原先因本地写入、互斥目录清理和交互确认语义未完成而处于
Schema exclusion。本轮将它作为一个高风险本地能力逐项收口，并在完整 policy
中确认运行时 Schema 工具数由 1054 增至 1055：

1. **确认门禁统一**：命令声明 `write/high/user_required/unknown`，确认由
   Framework 2.0 在执行前处理；关闭 stdin 且未提供 `--yes` 时返回
   `confirmation_required`/rc=3，嵌入 Skill 不会被物化，也不会修改目标目录。
2. **dry-run 无副作用**：默认嵌入源直接通过 `embed.FS` 检查，输出完整的
   `mode/source/targets/skills/removals/installs` 计划；不会创建临时目录、复制
   文件或删除互斥模式目录。显式 `--source` 只做本地只读检查。
3. **互斥清理失败即停止**：mono/multi 切换时，旧目录删除失败会进入失败或
   unknown 结果，不再“清理失败但继续安装”，避免两套 Skill 同时生效。
4. **部分结果诚实表达**：多目标安装保留 `succeeded[]`、`failed[]`、
   `unknown[]` 三组；已发生文件系统变更但结果不确定的路径进入 `unknown`，
   部分失败使用非零退出码，全部失败保留逐路径诊断信息。
5. **目标边界明确**：`skill setup` 只接受已注册 Agent 目标和 `all`，不再把
   当前目录 `.` 误宣传为 setup 能力；当前目录安装仍由单 Skill 的 `skill install`
   负责。
6. **Agent 使用约束同步**：Skill 文档要求先运行 `--dry-run --format json`
   查看删除/安装计划，再由用户明确确认后执行 `--yes`；不把临时源路径或内部
   rollout 信息暴露给 Agent。

这些证据关闭的是 CLI 本地生命周期和契约问题，不等同于已证明所有 Agent
环境都安装成功；发布后仍需在隔离目录复验 mono、多选 multi、失败恢复和升级覆盖。

## Agent 指令偏移专项扫描证据

本轮按 Agent 实际加载的 mono Skill 做了语义扫描，不只检查命令路径：

- **2026-08-09 复扫**：`mono-script-contract-20260809.md` 确认 35 个 Python 文件中 32 个为 Agent 入口，Help 为 `32/32` dry-run、`32/32` format、0 个非零；`mono-result-contract-20260809.md` 对共享 `run_main`、stdout 污染拒绝、结果/退出码一致性、异常/非零 `SystemExit`、业务 `SystemExit(0)` 不可绕过契约、显式 Help 保留、partial rc=7、meta 类型、严格 child 状态和关键写编排为 `26/26 PASS`。记录导入、字段创建、文件导入和附件上传都保留确认成功、明确失败或不确定状态，文件导入还保留两步 child meta。`mono-dry-run-probe-20260809.md` 以临时 HOME/工作区和 sentinel dws 对 10 个深层门控写入口验证单一 JSON stdout、无子进程调用、无额外本地文件；`mono-aitable-export-contract-20260809.md` 另以临时 child runner/HTTP server 对异步导出任务、pending、失败、本地原子下载与覆盖保护为 `10/10 PASS`。这些证据均不替代真实后端零写、账号权限或服务端终态验证。
- **Mono 复合写确认门禁（2026-08-09）**：此前 10 个复合远端写入口只依赖 Skill 的文字约束，脚本自身缺少统一 `--yes`，可被直接调用绕过；考勤排班还公开了另一套 `--confirm` 语义，并在多次远端只读校验后才停止。现已将确认门禁收进 `_runtime.py`，并接入考勤排班、文档创建写入、邮件发送、日程创建、批量待办、OA 批量审批、字段创建、记录导入、AITable 文件导入和附件上传。`attendance_schedule_import.py` 公开 `--yes`，历史 `--confirm` 仅作为隐藏兼容别名；所有缺确认调用都在 child CLI 前返回 `policy/confirmation_required`、`execution_state=not_executed`。该脚本确认后把脚本级 `--yes` 显式映射为 leaf 当前公开的 `--user-say-yes`，避免把隐藏兼容 `--yes` 当作内部契约；API 接受只报告 `execution_state=accepted` 与 `verification.state=not_verified`，写请求异常报告 `schedule_import_unconfirmed + execution_state=unknown` 并去除可自动重放提示。`mono-write-confirmation-20260809.md` 为 10/10，`mono-attendance-schedule-contract-20260809.md` 为 6/6，均以临时 HOME/工作区和 child runner 取证，未证明真实租户终态或 exactly-once。`--dry-run` 仍可免确认生成计划。
- **Mono AITable 异步导出（2026-08-09）**：`aitable_export_via_task.py` 已按“远端任务”和“本地文件”两层建模：创建或轮询不可信时不建议重复创建；未完成时输出可恢复的 `pending + taskId + next_command`，明确任务失败才输出 typed failure；下载先写同目录临时文件、`fsync` 后原子替换，输出本地 SHA-256，但没有服务端 checksum 时只声明 `source_integrity=unverified_no_remote_checksum`。`probe_mono_aitable_export_contract.py` 由 Agent 以临时 child runner/HTTP server 执行，`10/10 PASS` 覆盖 Help、dry-run 零调用、参数门禁、创建/查询不可信、pending、明确失败、仅返回 URL、原子落盘和覆盖保护；它不证明真实租户任务、下载 URL 权限或远端文件完整性。
- 当前 `skills/mono/scripts/` 中共有 35 个 Python 文件，其中 32 个是含
  `if __name__ == "__main__"` 的 Agent 入口，另有 3 个内部模块；对 32 个入口逐个执行
  `python3 <script> --help`，实测 32/32 声明脚本级 `--dry-run`、32/32 声明脚本级
  `--format`，且 Help 非零为 0。由此确认统计必须区分文件数、入口数和 Help 可观测能力，
  不能在 Skill 顶层把内部模块算作脚本入口，也不能仅凭参数存在宣称副作用安全。
- 上述“三十二者均接受脚本级 `--format`/`--dry-run`”只适用于已完成审计的
  `skills/mono/scripts/` 32 个 Agent 入口，不适用于 Multi Skill。此前把 Mono
  迁移清单混写成全仓结论是口径错误，已由独立 Multi 扫描纠正。
- Multi 的 Agent 扫描见 `docs/agent-scans/skill-contract-audit-20260809.md`：
  57 个 Python 文件、42 个入口，当前 42/42 `--help` 成功；Help 文本已有 31/42
  提及 `--dry-run`、24/42 提及脚本级 `--format`（这不是副作用安全证明）。这不是要求强行给所有工具增加两个参数，
  （本轮为 5 个 AITable 写/上传/导出入口补齐脚本级 dry-run，其中 export 同时接入格式与任务结果边界），但仍不是要求 Skill 对固定输出、
  内部检查器和真正的 Agent 契约强行统一；各类能力必须分别声明，不能笼统宣称“所有脚本统一支持”。
- 本轮修复了 Multi 9 个入口把 `--help` 当业务参数导致的非零返回；这只证明 Help
  可观测，不证明 dry-run 没有远端副作用。深层门控型脚本仍需受控 Agent 探针或隔离
  环境逐项取证，计划型脚本才可直接宣称零写入。
- 四个 AITable 写/上传入口已由 `scripts/agent/probe_multi_aitable_dry_run.py`
  使用临时 fixture 和 sentinel `dws` 做受控探针，4/4 通过且未观察到 dws/OSS 写入；
  探针结果见 `docs/agent-scans/multi-aitable-dry-run-probe-20260808.md`。
- Agent 语义复核还发现 AITable Skill 曾把 `bulk_add_fields.py` 和
  `upload_attachment.py` 误写成选项式调用；已按脚本真实位置参数契约修正，并把
  该类“Python 脚本参数不受 CLI 命令完整性检查覆盖”的风险加入 Multi 扫描台账。
- 同一 Agent 扫描进一步发现 Minutes 深层 recipe 将脚本自身的 `--max` 误写成
  `--limit`；已改回 `--max`，并把 Python 脚本调用的 flag 对拍纳入扫描器，当前
  `Documented Python-script flag mismatches = 0`。
- Mono 的同类深层 Minutes recipe 也发现同一偏移，已同步改回脚本参数 `--max`。
- Mono Agent 扫描器现同时对拍 `skills/mono/**/*.md` 中正向 Python 脚本调用的 flags；使用
  `--strict-rfc --strict-flags` 时当前 RFC 统计和深层脚本参数偏移均为 0，避免只检查
  命令路径而漏掉脚本参数漂移。
- 已把 Skill 的规则改为：终结型 dws 命令按 leaf Help 使用 `--format json`；无限
  `event consume` 使用 `--format ndjson`，只有有界消费才可选择 `json/pretty`；脚本
级参数以各脚本 Help 为准，脚本内部调用 dws 时再传递格式。
- “所有脚本”只指面向 Agent 的可执行入口，不包括 `attendance_report_common.py`、
  `minutes_list_parse.py` 等内部 helper。复杂报表入口必须先证明 dry-run 的远端只读
  边界、图片/Excel 处理无副作用，以及 stdout 完全可解析；不能仅添加 argparse 参数。
- `devdoc article search` 路由本身是有效能力，保留该入口；文档说明已弱化为“只返回
  搜索结果和官方链接，不能据此声称链接内容已被 CLI 读取或验证”，并明确 `--page`/
  `--size` 的当前类型以 Help 为准。
- Minutes 深层 reference 也完成了一次参数对照：Agent-facing 文档统一使用当前 Help
  的 `--limit` / `--cursor`；旧的 `--max` / `--next-token` 只保留在兼容别名说明和
  脚本自身的 Help 示例中，避免把隐藏别名继续扩散成 canonical 指令。
- 复核 Drive 的 `drive_tree_list.py` 时发现 Mono/Multi 都只读取每个目录第一页，
  且把调用失败、未知响应和真实空目录统一压成 `[]`；Mono 机器模式还会重新执行一次
  递归，Multi 则教 Agent 使用隐藏的 `--parent-id`。现两入口共享同一只读遍历边界：
  使用 canonical `drive list --folder/--limit/--cursor`，逐目录耗尽分页，机器/文本只执行
  一次遍历；第一页失败为 typed failure，后续页、子目录、游标循环和 `hasMore=true`
  缺 token 均保留已读树并返回 `partial_failure`/rc=7。只有
  `meta.pagination.endpoint_exhausted:true` 才表示请求深度内读取完成，历史
  `--parent-id` 仅保留为隐藏兼容别名。`drive-tree-contract-20260809.md` 以临时 child
  runner 对拍 Mono/Multi，26/26 PASS；不证明真实账号目录可见范围或服务端终态。
- 考勤默认只读脚本 `attendance_my_record.py` / `attendance_team_shift.py` 原先也把
  child 非零、超时、坏 JSON 和真实空结果统一压成“未查到”，且 Multi 两入口没有稳定
  机器格式。现 Mono/Multi 四入口统一通过共享 child 结果边界：当前用户必须投影出稳定
  `userId`，个人考勤只接受明确的详情对象/数组/null，团队班次只接受已知列表容器且每项
  必须有 `userId`；child typed error 与投影漂移均为 failure，不再伪装空结果。团队输入
  在调用前完成去重、有效日期和 7 天上限校验，dry-run 零 child。专项 Agent 探针
  `attendance-basic-read-contract-20260809.md` 为 30/30 PASS；它不证明真实考勤权限、
  组织覆盖或服务端终态。
- 考勤报表公共 child 边界此前只解包旧 `{success,result,error}`；当底层命令迁入统一
  `{ok,outcome,data,error}` 后，成功信封会被当作业务对象继续传递，而 nonzero typed
  error 又在 JSON 解码前被降成 stderr 文本。现 Mono/Multi 公共模块同时严格支持统一、
  旧和 bare JSON：`ok/outcome` 必须类型与语义一致，success 只向上返回 `data`，
  pending/partial/failure 均保留 typed error 并停止报表投影，字符串布尔值和
  “非零退出+成功信封”按协议矛盾拒绝。`attendance-report-child-contract-20260809.md`
  以注入式 subprocess 对拍为 22/22 PASS。随后明细、签到、每日和月度四类报表的
  Mono/Multi 八个入口进一步共用逐批 ledger：只有已知记录容器才算成功，未知字段名、
  非法行或读取错误不会再降成空批次；混合结果保留成功批次并以
  `partial_failure`/rc 7 输出 `succeeded[]/failed[]/unknown[]` 和不完整的报表摘要，
  权限拒绝继续保持 `authorization/permission_denied` 而不是被顶层异常边界降成普通
  validation；全批失败则在本地 Excel 写入前返回 typed failure。产品 Skill 明确禁止把部分文件描述
  为完整报表或重做已成功批次。`attendance-report-partial-contract-20260809.md` 受控对拍
  为 44/44 PASS；它不创建 Excel，也不证明真实考勤权限、图片下载或后端覆盖。
- 第五类 `attendance_report_record.py` 此前仍在 Mono 静默跳过审批列表/详情/打卡结果/
  考勤组失败，Multi 入口甚至没有脚本级 format、dry-run 和异常边界。现两入口共享同一
  实现：审批列表只接受已知容器，详情必须是对象，缺失 `originId` 或无法投影记录均进入
  typed ledger；权限失败保留 authorization。完整成功保持原 data 形状，部分读取仍可
  交付只含成功记录的文件但返回 `partial_failure`/rc 7，全批失败或零可交付行且存在失败
  时在 Excel 写入前返回 typed failure。`attendance-record-contract-20260809.md` 受控对拍
  为 16/16 PASS；它不创建 Excel，也不证明真实审批内容、考勤组覆盖或后端终态。
- 正式路由中的 `attendance_schedule_export.py` 与 `attendance_vacation_balance.py` 在
  Multi 侧此前没有机器格式或 dry-run，Mono 侧又会把排班批次失败压成“未查询到记录”、
  把余额失败落成默认“不适用”单元格后继续 success。现两入口在 Mono/Multi 共用统一
  结果边界：已知空与未知投影分离；排班按用户批次、假期余额按规则+用户批次记账；
  明确的“不限制余额/不适用/外部规则暂无余额”仍是已识别业务结果，其他失败进入 typed
  ledger。混合结果生成仅含成功批次的 Excel 并返回 `partial_failure`/rc 7，全部业务批次
  失败则在本地写入前返回 failure。`attendance-read-exports-20260809.md` 为 24/24 PASS；
  受控探针不创建 Excel，也不证明真实组织覆盖或服务端数据正确性。
- 发现 Mono `url-patterns.md` 与 `doc.md` 仍声称在线表格导出未暴露；当前 Help 已提供
  `dws sheet export --node <ID或URL> [--output <path>]`，已改为正确路由，避免 Agent
  把可用能力错误降级为“只能在客户端手动导出”。
- Agent 深层语义复核还发现 `doc.md`、Sheet 文件分流、URL 分流与 Minutes 的文档兜底
  同时保留了已迁移的 `doc upload/download/copy/move/rename/delete/list/search` 正向 recipe；
  顶层“已迁移”提示会被这些后续 recipe 覆盖。现已将默认路径统一为公开的 `drive`
  命令（文档正文 read/create/update/export/block/media 仍属于 `doc`），并新增
  `scan_deprecated_doc_routes.py` 作为 Agent Markdown 审阅。扫描覆盖 Mono/Multi，基于邻近
  上下文区分旧命令对照表与正向教程，当前为 PASS；它不接入 CI，也不把“未命中旧路由”扩大为服务端能力证明。

这项扫描发现的是 Agent 可执行语义漂移，不是 CI 路径缺失；后续每次 Skill 变更都应
继续对 Help、参数、结果形状和安全语义做逐条 Agent 复核。

## 下一批优先级

1. 真实环境复验 contact 投影 `8/5/1`、`wiki +node-list`、event stop、todo participant、approval create-instance。
2. 在隔离数据上完成 sheet 二次回滚官方自证。
3. 继续复验 DevApp 权限受限、其他账号、异常响应及 `dev` 原子入口；终页位置 cursor
   不得输出为 `next_token`，当前结构计数不得扩大成企业应用目录或权限覆盖完整。
4. 对保留的 CLI 生命周期与 Agoal 产品边界 exclusion 做发布级复核，确保没有新的业务
   leaf 被误归入边界组；新增待审业务命令必须在同一变更中完成 Contract/Safety 审阅。
