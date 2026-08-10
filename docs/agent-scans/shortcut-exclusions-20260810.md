# Shortcut exclusion Agent scan

扫描日期：2026-08-10

> 本报告由 Agent 从当前 Runtime Schema 生成；不修改公开目录，不保存运行时 JSON。`public=false` 只表示未进入 Agent 选择面，不代表命令不存在。

## 汇总

| 指标 | 数量 |
|---|---:|
| 运行时 shortcut 总数 | 415 |
| public=true | 383 |
| exclusion（public=false） | 32 |
| 已 review 的 exclusion | 32 |
| 未 review 的 exclusion | 0 |

## 逐条队列

| service | command | risk | confirmation | reviewed | decision / reason |
|---|---|---|---|:---:|---|
| `calendar` | `+respond-event` | `write` | `user_required` | yes | 保留隐藏：会改变他人日程响应状态；虽然有 user_required 门禁，但当前尚缺稳定的 Agent 结果投影与完整状态回读证据。 |
| `calendar` | `+room-find` | `read` | `not_required` | yes | 保留隐藏：旧版可用性搜索与已公开的 calendar +find-room 能力重叠，参数和返回投影不同；避免 Agent 在两个 canonical 入口间漂移。 |
| `chat` | `+conversation-mute-at-all` | `write` | `user_required` | yes | 保留隐藏：写操作会影响群成员提醒状态，当前 Agent 选择面暂不发布；运行时继续保留 user_required 门禁。 |
| `chat` | `+conversation-mute-red-envelope` | `write` | `user_required` | yes | 保留隐藏：写操作会影响群成员红包提醒状态，当前 Agent 选择面暂不发布；运行时继续保留 user_required 门禁。 |
| `contact` | `+get-roster` | `read` | `not_required` | yes | 保留隐藏：返回 HR 花名册、合同和银行卡等敏感个人档案；需要字段级授权、脱敏和审计语义后再进入通用 Agent 选择面。 |
| `contact` | `+list-roster-fields` | `read` | `not_required` | yes | 保留隐藏：该命令枚举敏感 HR 档案字段，会扩大 Agent 对个人数据的发现面；待权限/脱敏策略固化后再公开。 |
| `devapp` | `+event-subscribe` | `write` | `user_required` | yes | 保留隐藏：会创建长期事件订阅，属于有外部生命周期的写操作；当前缺少稳定 pending/stop 回收和结果投影证据。 |
| `devapp` | `+event-unsubscribe` | `high-risk-write` | `user_required` | yes | 保留隐藏：会删除事件订阅，属于不可逆或难恢复的写操作；需补回读、幂等和 dry-run 证据后再公开。 |
| `devapp` | `+permission-add` | `write` | `user_required` | yes | 保留隐藏：会扩大应用权限范围；当前需要逐权限风险解释、审批状态和失败后回读，不能仅凭 user_required 门禁公开。 |
| `devapp` | `+permission-remove` | `high-risk-write` | `user_required` | yes | 保留隐藏：会撤销应用权限并可能中断线上能力；缺少影响面预览和终态回读证据。 |
| `devapp` | `+robot-config` | `write` | `user_required` | yes | 保留隐藏：修改机器人线上配置，可能影响消息投递；需补配置差异预览、回滚和验证结果后再公开。 |
| `devapp` | `+robot-disable` | `high-risk-write` | `user_required` | yes | 保留隐藏：会使机器人下线；需确认门禁、恢复路径和 readiness 终态证据，当前不作为通用 Agent 入口。 |
| `devapp` | `+robot-enable` | `write` | `user_required` | yes | 保留隐藏：会恢复机器人线上流量；需补启用后健康检查和失败/未知状态表达。 |
| `devapp` | `+security-config` | `write` | `user_required` | yes | 保留隐藏：涉及应用安全策略与权限边界；字段级风险和回滚语义尚未完成审阅。 |
| `devapp` | `+version-create` | `write` | `user_required` | yes | 保留隐藏：ResultSpec 已要求稳定 versionId 并标 not_verified，但尚无隔离应用的真实创建、重复请求、回读与清理证据。 |
| `devapp` | `+version-publish` | `write` | `user_required` | yes | 保留隐藏：pending/unknown/终态投影已收口，但尚无真实直接发布、审批、拒绝/撤回、响应丢失与回读闭环证据。 |
| `ding` | `+send-by-message` | `write` | `user_required` | yes | 保留隐藏：会向外部收件人发送消息，涉及收件人消歧、内容确认和不可逆副作用；当前结果回执与重复发送保护不足。 |
| `doc` | `+comment-create-inline` | `write` | `user_required` | yes | 保留隐藏：文档内联评论写入需要文档位置和内容的额外语义审阅，暂不作为通用 Agent 入口。 |
| `doc` | `+template-apply` | `write` | `user_required` | yes | 保留隐藏：模板套用会对目标文档产生写入，待 dry-run/回滚和真实投影证据齐全后再决定公开。 |
| `drive` | `+download` | `read` | `not_required` | yes | 保留隐藏：旧下载入口与统一 file-transfer 路径重叠，输出格式、文件落盘和大文件副作用边界尚未形成稳定 Agent Contract。 |
| `drive` | `+list` | `read` | `not_required` | yes | 保留隐藏：旧目录入口存在死条目/分页召回风险，当前不对 Agent 承诺权威全量目录；使用已审阅的确定性查询入口。 |
| `minutes` | `+action-items` | `read` | `not_required` | yes | 保留隐藏：待办抽取结果依赖听记处理状态，可能是 pending/部分结果；当前缺少稳定分页和逐项结果契约。 |
| `minutes` | `+latest-minutes` | `read` | `not_required` | yes | 保留隐藏：本地目标选择已对稳定 taskUuid、非法/部分时间证据 fail-closed，但当前只读取首批 20 条且无 endpoint 覆盖证明；避免把首批最新扩大为全量最新。 |
| `minutes` | `+record-pause` | `write` | `user_required` | yes | 保留隐藏：会暂停正在进行的听记录音，属于有状态写操作；需补恢复/回读和用户确认语义。 |
| `minutes` | `+record-resume` | `write` | `user_required` | yes | 保留隐藏：会恢复正在进行的听记录音，需验证实际录音状态和重复调用幂等，当前不公开。 |
| `minutes` | `+record-stop` | `write` | `user_required` | yes | 保留隐藏：会终止听记录音且可能不可恢复；需补确认、终态回读和失败后状态核验。 |
| `minutes` | `+transcript` | `read` | `not_required` | yes | 保留隐藏：大体量转写读取涉及长响应和分页/异步处理；当前使用明确的 minutes get transcription 路径，避免重复 canonical。 |
| `oa` | `+approve-by` | `write` | `user_required` | yes | 保留隐藏：会代表用户执行审批动作，涉及审批人/实例消歧和不可逆业务变更；待完整审批结果与幂等证据。 |
| `report` | `+report-latest` | `read` | `not_required` | yes | 保留隐藏：本地最新项投影已 fail-closed，但旧聚合入口仍与 report outbox list + detail 规范路径重叠，且缺少真实账号详情样本；避免形成第二个 Agent canonical。 |
| `wiki` | `+node-copy` | `write` | `user_required` | yes | 保留隐藏：会复制知识库节点并产生新资源，需补目标空间确认、幂等和回滚/部分失败投影。 |
| `wiki` | `+node-move` | `write` | `user_required` | yes | 保留隐藏：会改变知识库节点归属，影响范围和回滚边界较大；待 dry-run、权限和终态回读证据。 |
| `wiki` | `+wiki-new-doc` | `write` | `user_required` | yes | 保留隐藏：会创建新文档资源，需补重复创建保护、失败后资源未知状态和清理动作。 |

## 规则

- 只有完成 Contract、Safety、Help/Schema、dry-run 或只读证明，并写入审阅理由后，才允许从 exclusion 移入 public。
- 写/高风险 shortcut 不因“可执行”自动公开；无稳定结果投影或真实后端证据时应继续隐藏。
- 该扫描是 Agent review queue，不是 CI 门禁；每次评测或发布前重新运行。
