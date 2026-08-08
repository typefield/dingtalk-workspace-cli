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
