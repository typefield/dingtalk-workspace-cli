# Mono 复合写确认门禁 Agent 受控探针

> 调用均省略 `--yes` 且不使用 `--dry-run`。临时 HOME、工作目录和 sentinel `dws` 用于证明本地脚本在 child CLI/本地写入前停止；仅保存 Markdown 证据，不替代真实租户验证。

| 入口 | 结果 | 说明 |
|---|---|---|
| `doc_create_and_write.py` | PASS | ok |
| `mail_send_with_cc.py` | PASS | ok |
| `calendar_schedule_meeting.py` | PASS | ok |
| `todo_batch_create.py` | PASS | ok |
| `bulk_add_fields.py` | PASS | ok |
| `import_records.py` | PASS | ok |
| `aitable_import_via_task.py` | PASS | ok |
| `upload_attachment.py` | PASS | ok |
| `oa_batch_approve.py` | PASS | ok |

结果：9/9 通过

## 边界

- PASS 表示缺少确认时本地脚本返回 `policy/confirmation_required`、`execution_state=not_executed`，且 probe 未观察到子 dws 调用或新增本地文件。
- 不证明真实租户的权限、服务端写入终态或确认后的 exactly-once；这些仍需要隔离账号或受控后端证据。
