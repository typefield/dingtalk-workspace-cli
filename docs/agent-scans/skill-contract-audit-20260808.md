# Skill 契约 Agent 总审计

扫描日期：2026-08-08

> 本报告由 Agent 在当前源码上执行。它是评测证据，不是 CI 门禁；只保存 Markdown/text，不保存 JSON fixture。

## 结果摘要

| 审计项 | 结果 |
|---|---|
| Mono 脚本 Help/Skill 参数对账 | PASS |
| Multi 脚本 Help/Skill 参数对账 | PASS |
| Shortcut 运行时/目录/Skill 集合对账 | PASS |
| Shortcut exclusion 逐条审阅队列 | PASS |
| Skill CLI 路径/参数逐条对拍 | PASS |

## 原始 Agent 证据

### Mono 脚本 Help/Skill 参数对账

命令：`/Library/Developer/CommandLineTools/usr/bin/python3 scripts/agent/scan_mono_script_contract.py --strict-rfc --strict-flags`

```text
# Mono Skill 脚本契约 Agent 扫描

扫描日期：2026-08-08

> 本报告由 Agent 执行生成，仅记录 Help 可观测事实。dry-run 副作用需要受控 child-runner 或真实环境另行证明；本报告不会把未验证项写成通过，也不保存 JSON fixture。

## 统计口径

| 指标 | 数量 | 说明 |
|---|---:|---|
| Python 文件 | 35 | `skills/mono/scripts/*.py` 全部文件 |
| Agent 入口 | 32 | 含 `if __name__ == \"__main__\"` |
| 内部模块 | 3 | _runtime.py, attendance_report_common.py, minutes_list_parse.py |
| Help 暴露 `--dry-run` | 32/32 | 逐入口运行 `--help` |
| Help 暴露 `--format` | 32/32 | 逐入口运行 `--help` |
| Help 非零 | 0 | 退出码非 0 的入口 |

## RFC 对账

对账文件：`docs/rfcs/0002-mono-skill-script-interface.md`
状态：PASS

## 深层 Skill 脚本参数对拍

状态：PASS
正向 Python 脚本调用中的 Help 参数偏移：0

## 入口明细

| 脚本 | help rc | dry-run | format | 状态 |
|---|---:|:---:|:---:|---|
| `aitable_export_via_task.py` | 0 | yes | yes | PASS |
| `aitable_import_via_task.py` | 0 | yes | yes | PASS |
| `attendance_my_record.py` | 0 | yes | yes | PASS |
| `attendance_report_checkin.py` | 0 | yes | yes | PASS |
| `attendance_report_daily.py` | 0 | yes | yes | PASS |
| `attendance_report_detail.py` | 0 | yes | yes | PASS |
| `attendance_report_monthly.py` | 0 | yes | yes | PASS |
| `attendance_report_record.py` | 0 | yes | yes | PASS |
| `attendance_schedule_export.py` | 0 | yes | yes | PASS |
| `attendance_schedule_import.py` | 0 | yes | yes | PASS |
| `attendance_team_shift.py` | 0 | yes | yes | PASS |
| `attendance_vacation_balance.py` | 0 | yes | yes | PASS |
| `bulk_add_fields.py` | 0 | yes | yes | PASS |
| `calendar_free_slot_finder.py` | 0 | yes | yes | PASS |
| `calendar_schedule_meeting.py` | 0 | yes | yes | PASS |
| `calendar_today_agenda.py` | 0 | yes | yes | PASS |
| `contact_dept_members.py` | 0 | yes | yes | PASS |
| `doc_create_and_write.py` | 0 | yes | yes | PASS |
| `drive_tree_list.py` | 0 | yes | yes | PASS |
| `import_records.py` | 0 | yes | yes | PASS |
| `mail_send_with_cc.py` | 0 | yes | yes | PASS |
| `mail_unread_summary.py` | 0 | yes | yes | PASS |
| `minutes_extract_todos.py` | 0 | yes | yes | PASS |
| `minutes_recent_summary.py` | 0 | yes | yes | PASS |
| `oa_batch_approve.py` | 0 | yes | yes | PASS |
| `oa_pending_review.py` | 0 | yes | yes | PASS |
| `report_inbox_today.py` | 0 | yes | yes | PASS |
| `report_received_today.py` | 0 | yes | yes | PASS |
| `todo_batch_create.py` | 0 | yes | yes | PASS |
| `todo_daily_summary.py` | 0 | yes | yes | PASS |
| `todo_overdue_check.py` | 0 | yes | yes | PASS |
| `upload_attachment.py` | 0 | yes | yes | PASS |

## 尚未由本扫描证明的事项

- **UNVERIFIED**：`--dry-run` 是否对每个入口实现零远端写入、零本地写入；需要受控 child-runner、临时 HOME 和写请求计数器。
- **UNVERIFIED**：`--format json` 是否在成功、失败、部分成功和不确定结果下都保持单一可解析 stdout；需要注入式执行夹具。
- **UNVERIFIED**：脚本内部调用 `dws` 的 `remote_reads`、分页和重试语义；不能仅凭 flags 或源码字符串判定。

## 结论

当前 32 个 Agent 入口的 Help 可观测性为 32/32 dry-run、32/32 format、0 个 Help 非零；副作用和结果契约仍需单独实测。
```

### Multi 脚本 Help/Skill 参数对账

命令：`/Library/Developer/CommandLineTools/usr/bin/python3 scripts/agent/scan_multi_script_contract.py`

```text
multi Python files: 52
Agent entries: 42
Help nonzero: 0
Help text mentions --dry-run: 30/42
Help text mentions --format: 1/42

Nonzero help:

Entries without both flags (review, not automatic failures):
- skills/multi/dingtalk-aitable/scripts/aitable_export_via_task.py: rc=0, dry_run=False, format=False
- skills/multi/dingtalk-aitable/scripts/aitable_import_via_task.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-aitable/scripts/bulk_add_fields.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-aitable/scripts/import_records.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-calendar/scripts/calendar_free_slot_finder.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-calendar/scripts/calendar_schedule_meeting.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-calendar/scripts/calendar_today_agenda.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-contact/scripts/contact_dept_members.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-doc/scripts/doc_create_and_write.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-drive/scripts/drive_tree_list.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-mail/scripts/mail_send_with_cc.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-mail/scripts/mail_unread_summary.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-minutes/scripts/minutes_extract_todos.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-minutes/scripts/minutes_recent_summary.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-misc/scripts/aiapp_create_and_poll.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-misc/scripts/attendance_my_record.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-misc/scripts/attendance_report_checkin.py: rc=0, dry_run=False, format=False
- skills/multi/dingtalk-misc/scripts/attendance_report_daily.py: rc=0, dry_run=False, format=False
- skills/multi/dingtalk-misc/scripts/attendance_report_detail.py: rc=0, dry_run=False, format=False
- skills/multi/dingtalk-misc/scripts/attendance_report_monthly.py: rc=0, dry_run=False, format=False
- skills/multi/dingtalk-misc/scripts/attendance_report_record.py: rc=0, dry_run=False, format=False
- skills/multi/dingtalk-misc/scripts/attendance_schedule_export.py: rc=0, dry_run=False, format=False
- skills/multi/dingtalk-misc/scripts/attendance_schedule_import.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-misc/scripts/attendance_team_shift.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-misc/scripts/attendance_vacation_balance.py: rc=0, dry_run=False, format=False
- skills/multi/dingtalk-misc/scripts/finance_daily_cashflow.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-misc/scripts/finance_expense_flow.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-misc/scripts/oa_batch_approve.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-misc/scripts/oa_pending_review.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-misc/scripts/report_received_today.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-misc/scripts/yida_custom_page_update.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-misc/scripts/yida_form_inspector.py: rc=0, dry_run=False, format=False
- skills/multi/dingtalk-misc/scripts/yida_form_update.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-misc/scripts/yida_jsx_pipeline.py: rc=0, dry_run=False, format=False
- skills/multi/dingtalk-misc/scripts/yida_page_generate.py: rc=0, dry_run=False, format=False
- skills/multi/dingtalk-misc/scripts/yida_page_self_check.py: rc=0, dry_run=False, format=False
- skills/multi/dingtalk-misc/scripts/yida_process_update.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-misc/scripts/yida_report_update.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-todo/scripts/todo_batch_create.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-todo/scripts/todo_daily_summary.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-todo/scripts/todo_overdue_check.py: rc=0, dry_run=True, format=False

Documented Python-script flag mismatches: 0
```

### Shortcut 运行时/目录/Skill 集合对账

命令：`/Library/Developer/CommandLineTools/usr/bin/python3 scripts/agent/scan_shortcut_surface_alignment.py --output /var/folders/fj/17qvmrfd0b141s5cxshpt6lm0000gn/T/dws-skill-audit-zl49nsfq/shortcut-surface.md`

```text
# Shortcut surface alignment Agent scan

- generated_at: `2026-08-08T15:12:56`
- source: current `go run ./cmd shortcut list --all --mock --format json`
- fixture policy: runtime JSON is held in memory and not saved; this file is Markdown evidence only
- result: **PASS**

| surface | count |
|---|---:|
| runtime public | 376 |
| committed catalog | 376 |
| Mono Skill total | 376 |

| service | catalog | Skill |
|---|---:|---:|
| `aitable` | 92 | 92 |
| `attendance` | 35 | 35 |
| `calendar` | 20 | 20 |
| `chat` | 98 | 98 |
| `contact` | 14 | 14 |
| `devapp` | 20 | 20 |
| `ding` | 4 | 4 |
| `doc` | 45 | 45 |
| `drive` | 7 | 7 |
| `mail` | 10 | 10 |
| `minutes` | 7 | 7 |
| `oa` | 7 | 7 |
| `report` | 2 | 2 |
| `sheet` | 2 | 2 |
| `todo` | 11 | 11 |
| `wiki` | 2 | 2 |

## Findings

- runtime public set, committed catalog, and Skill counts are identical

```

### Shortcut exclusion 逐条审阅队列

命令：`/Library/Developer/CommandLineTools/usr/bin/python3 scripts/agent/scan_shortcut_exclusions.py --output /var/folders/fj/17qvmrfd0b141s5cxshpt6lm0000gn/T/dws-skill-audit-zl49nsfq/shortcut-exclusions.md`

```text
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

```

### Skill CLI 路径/参数逐条对拍

命令：`go run ./scripts/policy/skill-command-check`

```text
skill command integrity check: ok (1138 executable command paths)
```

## 解释边界

- Help 对账只证明参数可发现，不证明业务执行安全。
- CLI 路径/参数对拍只证明当前公开 Help 接受文档中的 flags；隐藏兼容别名是否应继续教学，仍需 Agent 语义审阅。
- dry-run 仍需由受控 child-runner、临时 HOME 和写请求计数器证明零写入。
- 集合对账只证明 Runtime、目录和 Skill 不漂移，不证明后端数据真实存在。
