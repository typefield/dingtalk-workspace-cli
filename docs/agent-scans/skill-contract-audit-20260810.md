# Skill 契约 Agent 总审计

扫描日期：2026-08-10

> 本报告由 Agent 在当前源码上执行。它是评测证据，不是 CI 门禁；只保存 Markdown/text，不保存 JSON fixture。

## 结果摘要

| 审计项 | 结果 |
|---|---|
| Mono 脚本 Help/Skill 参数对账 | PASS |
| Skill 已迁移 Doc 文件管理路由 | PASS |
| Mono 脚本结果/异常边界 | PASS |
| Mono 深层 dry-run 受控探针 | PASS |
| Mono 复合写确认门禁受控探针 | PASS |
| Mono 考勤排班写入委托与终态语义 | PASS |
| Multi 脚本 Help/Skill 参数对账 | PASS |
| Shortcut 运行时/目录/Skill 集合对账 | PASS |
| Shortcut exclusion 逐条审阅队列 | PASS |
| Skill CLI 路径/参数逐条对拍 | PASS |
| Skill 隐藏兼容 flag Agent 审阅 | PASS |

## 原始 Agent 证据

### Mono 脚本 Help/Skill 参数对账

命令：`/Library/Developer/CommandLineTools/usr/bin/python3 scripts/agent/scan_mono_script_contract.py --strict-rfc --strict-flags`

```text
# Mono Skill 脚本契约 Agent 扫描

扫描日期：2026-08-10

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

### Skill 已迁移 Doc 文件管理路由

命令：`/Library/Developer/CommandLineTools/usr/bin/python3 scripts/agent/scan_deprecated_doc_routes.py`

```text
# Skill 已迁移 Doc 文件管理路由 Agent 审阅

扫描日期：2026-08-10

> 本扫描检查 Agent 文档是否把旧 `dws doc` 文件管理入口当作正向路径。它是 Markdown 评测证据，不接入 CI，也不保存 JSON fixture。

## 结论

**PASS**：Mono/Multi Skill 均未发现未标注为迁移/兼容的 `doc upload/download/copy/move/rename/delete/list/search` 正向路由。

默认路由：文件发现、传输和节点管理使用 `dws drive`；文档正文读取、创建、块编辑、导出和媒体嵌入保留 `dws doc`。
```

### Mono 脚本结果/异常边界

命令：`/Library/Developer/CommandLineTools/usr/bin/python3 scripts/agent/probe_mono_result_contract.py`

```text
# Mono 脚本结果契约 Agent 探针

扫描日期：2026-08-10

> 本报告由 Agent 在当前工作树执行。它验证共享异常边界和代表性输入错误；不保存运行 JSON fixture，也不替代真实服务端副作用验证。

| 检查项 | 结果 | 说明 |
|---|---|---|
| 32 个入口统一异常边界 | PASS | 32/32 使用 run_main |
| 未捕获异常 JSON 兜底 | PASS | ok |
| 机器 stdout 污染拒绝 | PASS | ok |
| 机器结果与退出码一致性 | PASS | ok |
| SystemExit(0) 不可绕过机器输出契约 | PASS | ok |
| 显式 Help 保留 argparse 人读输出 | PASS | ok |
| 子 dws 严格布尔失败识别 | PASS | ok |
| 子 dws 非布尔状态不伪装成功 | PASS | ok |
| 子 dws 矛盾 ok/outcome 不伪装成功 | PASS | ok |
| 子 dws 非字符串 outcome 不泄漏异常或伪装成功 | PASS | ok |
| 子 dws pending 不伪装终态成功且保留任务 meta | PASS | ok |
| failure 缺 typed error 会被统一出口拒绝 | PASS | ok |
| meta/dry_run 非法类型会被统一出口拒绝 | PASS | ok |
| 非零 SystemExit JSON 兜底 | PASS | ok |
| 部分成功结果与退出码 | PASS | ok |
| 可选 meta 承载 | PASS | ok |
| 待办错误类型输入 | PASS | ok |
| 待办保留成功与未知写入 | PASS | ok |
| 审批任务解析失败不发送占位写入 | PASS | ok |
| 文档写入失败不自动重放且标记未知 | PASS | ok |
| 邮件旧业务失败不误报已发送 | PASS | ok |
| 日程后续写入失败保留部分结果 | PASS | ok |
| 记录导入保留成功与未知批次 | PASS | ok |
| 字段创建旧业务失败不误报成功 | PASS | ok |
| 文件导入不丢失各子步骤 meta | PASS | ok |
| 附件 PUT 未知不误报可用 | PASS | ok |

结果：26/26 通过

## 边界

- 本探针证明入口都接入共享异常边界，并证明该边界在机器格式下不会以 traceback 取代结果信封。
- 子 dws 探针覆盖待办、审批、文档、邮件、日程、记录导入、字段创建、文件导入任务和附件上传的代表性混合结果：成功、明确未执行和可能已执行不得压成布尔值；它不替代其他脚本和真实服务端终态验证。
- dry-run 零写、真实服务端终态和批量每项语义，仍按独立受控探针或真实环境证据标记。
```

### Mono 深层 dry-run 受控探针

命令：`/Library/Developer/CommandLineTools/usr/bin/python3 scripts/agent/probe_mono_dry_run.py`

```text
# Mono 深层 dry-run Agent 受控探针

> 临时 HOME、临时工作目录和 sentinel dws；不保存 JSON fixture。PASS 表示单一 JSON stdout、dry_run=true、无 dws 调用、无额外本地文件。

| 入口 | 结果 | 说明 |
|---|---|---|
| `doc_create_and_write.py` | PASS | ok |
| `oa_batch_approve.py` | PASS | ok |
| `todo_batch_create.py` | PASS | ok |
| `bulk_add_fields.py` | PASS | ok |
| `import_records.py` | PASS | ok |
| `aitable_import_via_task.py` | PASS | ok |
| `aitable_export_via_task.py` | PASS | ok |
| `calendar_schedule_meeting.py` | PASS | ok |
| `upload_attachment.py` | PASS | ok |
| `mail_send_with_cc.py` | PASS | ok |

结果：10/10 通过
```

### Mono 复合写确认门禁受控探针

命令：`/Library/Developer/CommandLineTools/usr/bin/python3 scripts/agent/probe_mono_write_confirmation.py`

```text
# Mono 复合写确认门禁 Agent 受控探针

> 调用均省略 `--yes` 且不使用 `--dry-run`。临时 HOME、工作目录和 sentinel `dws` 用于证明本地脚本在 child CLI/本地写入前停止；仅保存 Markdown 证据，不替代真实租户验证。

| 入口 | 结果 | 说明 |
|---|---|---|
| `doc_create_and_write.py` | PASS | ok |
| `mail_send_with_cc.py` | PASS | ok |
| `calendar_schedule_meeting.py` | PASS | ok |
| `attendance_schedule_import.py` | PASS | ok |
| `todo_batch_create.py` | PASS | ok |
| `bulk_add_fields.py` | PASS | ok |
| `import_records.py` | PASS | ok |
| `aitable_import_via_task.py` | PASS | ok |
| `upload_attachment.py` | PASS | ok |
| `oa_batch_approve.py` | PASS | ok |

结果：10/10 通过

## 边界

- PASS 表示缺少确认时本地脚本返回 `policy/confirmation_required`、`execution_state=not_executed`，且 probe 未观察到子 dws 调用或新增本地文件。
- 不证明真实租户的权限、服务端写入终态或确认后的 exactly-once；这些仍需要隔离账号或受控后端证据。
```

### Mono 考勤排班写入委托与终态语义

命令：`/Library/Developer/CommandLineTools/usr/bin/python3 scripts/agent/probe_mono_attendance_schedule_contract.py`

```text
# Mono 考勤排班导入 Agent 语义探针

> 临时 child runner 只验证脚本的本地确认、参数委托与结果表达；不证明真实租户的权限、排班持久化或 exactly-once。仅保存 Markdown 证据。

| 检查 | 结果 | 证据 |
|---|---|---|
| Help 只公开脚本确认参数 --yes | PASS | rc=0 |
| 类型错误在任何 child 调用前返回 typed validation | PASS | rc=1; ok; child_calls=0 |
| 缺确认在任何 child 调用前 fail-closed | PASS | rc=1; ok; child_calls=0 |
| dry-run 可做只读校验但不会导入排班 | PASS | rc=0; ok; child_reads=3 |
| 确认后传底层 canonical --user-say-yes，且只报告请求受理 | PASS | rc=0; ok; import_calls=1 |
| 写请求异常保留 unknown、禁止把 retryable 透传为重放许可 | PASS | rc=1; ok |

结论：**6/6 PASS**。

边界：成功只表示 child API 接受请求，脚本明确标记 `verification.state=not_verified`；写请求异常只表示终态未知，Agent 必须先查询排班而不是重放导入。
```

### Multi 脚本 Help/Skill 参数对账

命令：`/Library/Developer/CommandLineTools/usr/bin/python3 scripts/agent/scan_multi_script_contract.py`

```text
multi Python files: 57
Agent entries: 42
Help nonzero: 0
Help text mentions --dry-run: 38/42
Help text mentions --format: 31/42

Nonzero help:

Entries without both flags (review, not automatic failures):
- skills/multi/dingtalk-misc/scripts/aiapp_create_and_poll.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-misc/scripts/finance_daily_cashflow.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-misc/scripts/finance_expense_flow.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-misc/scripts/yida_custom_page_update.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-misc/scripts/yida_form_inspector.py: rc=0, dry_run=False, format=False
- skills/multi/dingtalk-misc/scripts/yida_form_update.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-misc/scripts/yida_jsx_pipeline.py: rc=0, dry_run=False, format=False
- skills/multi/dingtalk-misc/scripts/yida_page_generate.py: rc=0, dry_run=False, format=False
- skills/multi/dingtalk-misc/scripts/yida_page_self_check.py: rc=0, dry_run=False, format=False
- skills/multi/dingtalk-misc/scripts/yida_process_update.py: rc=0, dry_run=True, format=False
- skills/multi/dingtalk-misc/scripts/yida_report_update.py: rc=0, dry_run=True, format=False

Documented Python-script flag mismatches: 0
```

### Shortcut 运行时/目录/Skill 集合对账

命令：`/Library/Developer/CommandLineTools/usr/bin/python3 scripts/agent/scan_shortcut_surface_alignment.py --output <agent-temp>/shortcut-surface.md`

```text
# Shortcut surface alignment Agent scan

- generated_at: `2026-08-10T03:47:49`
- source: current `go run ./cmd shortcut list --all --mock --format json`
- fixture policy: runtime JSON is held in memory and not saved; this file is Markdown evidence only
- result: **PASS**

| surface | count |
|---|---:|
| runtime public | 383 |
| committed catalog | 383 |
| Mono Skill total | 383 |

| service | catalog | Skill |
|---|---:|---:|
| `aitable` | 92 | 92 |
| `attendance` | 35 | 35 |
| `calendar` | 21 | 21 |
| `chat` | 98 | 98 |
| `contact` | 14 | 14 |
| `devapp` | 20 | 20 |
| `ding` | 4 | 4 |
| `doc` | 45 | 45 |
| `drive` | 7 | 7 |
| `mail` | 10 | 10 |
| `minutes` | 7 | 7 |
| `oa` | 9 | 9 |
| `report` | 2 | 2 |
| `sheet` | 2 | 2 |
| `todo` | 13 | 13 |
| `wiki` | 4 | 4 |

## Findings

- runtime public set, committed catalog, and Skill counts are identical

```

### Shortcut exclusion 逐条审阅队列

命令：`/Library/Developer/CommandLineTools/usr/bin/python3 scripts/agent/scan_shortcut_exclusions.py --output <agent-temp>/shortcut-exclusions.md`

```text
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
| `devapp` | `+version-create` | `write` | `user_required` | yes | 保留隐藏：会创建应用版本并可能产生后续发布资源；当前 pending/清理和幂等证据不足。 |
| `devapp` | `+version-publish` | `write` | `user_required` | yes | 保留隐藏：会触发线上版本发布及审批链路；需完整 pending/approval/回滚验证后再公开。 |
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

```

### Skill CLI 路径/参数逐条对拍

命令：`go run ./scripts/policy/skill-command-check`

```text
skill command integrity check: ok (1141 executable command paths)
```

### Skill 隐藏兼容 flag Agent 审阅

命令：`go run ./scripts/policy/skill-command-check --agent-semantic`

```text
Agent semantic flag review: 0 hidden compatibility references
skill command integrity check: ok (1141 executable command paths)
```

## 解释边界

- Help 对账只证明参数可发现，不证明业务执行安全。
- CLI 路径/参数对拍只证明当前公开 Help 接受文档中的 flags；隐藏兼容别名是否应继续教学，仍需 Agent 语义审阅。
- 隐藏兼容 flag 审阅只把正向示例列为 REVIEW，不删除兼容 alias，也不作为 CI 阻断；应由 Agent 决定改 canonical 参数或保留历史说明。
- dry-run 与确认门禁的受控 probe 只能证明脚本在该夹具下未启动 child CLI/新增本地文件；真实租户的写入终态、权限和 exactly-once 仍需隔离账号或受控后端验证。
- 集合对账只证明 Runtime、目录和 Skill 不漂移，不证明后端数据真实存在。
