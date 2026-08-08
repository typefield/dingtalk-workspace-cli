# Shortcut exclusion Agent scan

扫描日期：2026-08-08

> 本报告由 Agent 从当前 Runtime Schema 生成；不修改公开目录，不保存运行时 JSON。`public=false` 只表示未进入 Agent 选择面，不代表命令不存在。

## 汇总

| 指标 | 数量 |
|---|---:|
| 运行时 shortcut 总数 | 415 |
| public=true | 376 |
| exclusion（public=false） | 39 |
| 已 review 的 exclusion | 4 |
| 未 review 的 exclusion | 35 |

## 逐条队列

| service | command | risk | confirmation | reviewed | next decision |
|---|---|---|---|:---:|---|
| `calendar` | `+find-room` | `read` | `not_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `calendar` | `+respond-event` | `write` | `user_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `calendar` | `+room-find` | `read` | `not_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `chat` | `+conversation-mute-at-all` | `write` | `user_required` | yes | 已审阅：保留隐藏，需保留原因 |
| `chat` | `+conversation-mute-red-envelope` | `write` | `user_required` | yes | 已审阅：保留隐藏，需保留原因 |
| `contact` | `+get-roster` | `read` | `not_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `contact` | `+list-roster-fields` | `read` | `not_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `devapp` | `+event-subscribe` | `write` | `user_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `devapp` | `+event-unsubscribe` | `high-risk-write` | `user_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `devapp` | `+permission-add` | `write` | `user_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `devapp` | `+permission-remove` | `high-risk-write` | `user_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `devapp` | `+robot-config` | `write` | `user_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `devapp` | `+robot-disable` | `high-risk-write` | `user_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `devapp` | `+robot-enable` | `write` | `user_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `devapp` | `+security-config` | `write` | `user_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `devapp` | `+version-create` | `write` | `user_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `devapp` | `+version-publish` | `write` | `user_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `ding` | `+send-by-message` | `write` | `user_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `doc` | `+comment-create-inline` | `write` | `user_required` | yes | 已审阅：保留隐藏，需保留原因 |
| `doc` | `+template-apply` | `write` | `user_required` | yes | 已审阅：保留隐藏，需保留原因 |
| `drive` | `+download` | `read` | `not_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `drive` | `+list` | `read` | `not_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `minutes` | `+action-items` | `read` | `not_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `minutes` | `+latest-minutes` | `read` | `not_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `minutes` | `+record-pause` | `write` | `user_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `minutes` | `+record-resume` | `write` | `user_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `minutes` | `+record-stop` | `write` | `user_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `minutes` | `+transcript` | `read` | `not_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `oa` | `+approve-by` | `write` | `user_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `oa` | `+done-approvals` | `read` | `not_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `oa` | `+pending` | `read` | `not_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `report` | `+report-latest` | `read` | `not_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `todo` | `+due-today` | `read` | `not_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `todo` | `+related-tasks` | `read` | `not_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `wiki` | `+node-copy` | `write` | `user_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `wiki` | `+node-move` | `write` | `user_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `wiki` | `+resolve-space` | `read` | `not_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `wiki` | `+space-list` | `read` | `not_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |
| `wiki` | `+wiki-new-doc` | `write` | `user_required` | no | 待 Agent 审阅：公开 / 删除 / 保留并写原因 |

## 规则

- 只有完成 Contract、Safety、Help/Schema、dry-run 或只读证明，并写入审阅理由后，才允许从 exclusion 移入 public。
- 写/高风险 shortcut 不因“可执行”自动公开；无稳定结果投影或真实后端证据时应继续隐藏。
- 该扫描是 Agent review queue，不是 CI 门禁；每次评测或发布前重新运行。
