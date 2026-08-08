# Mono Skill 深层写脚本 dry-run 受控探针

> Agent probe：使用临时 HOME/工作区和假的 `dws` 子进程。`PASS` 只证明下面这些固定 fixture 的 dry-run 没有远端调用、没有临时工作区写入且 stdout 是统一 JSON；不替代真实账号、异常分支和全量业务验证。

| 脚本 | rc | 远端调用 | 临时区写入 | 结果 |
|---|---:|:---:|:---:|---|
| `doc_create_and_write` | 0 | no | no | PASS |
| `oa_batch_approve` | 0 | no | no | PASS |
| `todo_batch_create` | 0 | no | no | PASS |
| `bulk_add_fields` | 0 | no | no | PASS |
| `import_records` | 0 | no | no | PASS |
| `calendar_schedule_meeting` | 0 | no | no | PASS |
| `upload_attachment` | 0 | no | no | PASS |

结论：7/7 个受控 fixture 通过；剩余路径仍标记为 `UNVERIFIED`，不得据此扩大为全量安全承诺。
