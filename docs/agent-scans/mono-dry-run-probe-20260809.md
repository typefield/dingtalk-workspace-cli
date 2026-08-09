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
