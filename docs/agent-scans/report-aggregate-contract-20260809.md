# Report 聚合脚本结果契约 Agent 探针

扫描日期：2026-08-09

> 临时假 dws 验证两个兼容入口；不保存 JSON fixture，不证明真实日志可见性或服务端终态。

| 检查 | 结果 | 证据 |
|---|---|---|
| report_inbox_today: 两页耗尽后才 success | PASS | rc=0 |
| report_inbox_today: 首页失败不伪装空列表 | PASS | rc=1 |
| report_inbox_today: 后续页失败保留第一页 | PASS | rc=7 |
| report_inbox_today: hasMore 无 cursor fail-closed | PASS | rc=1 |
| report_inbox_today: JSON detail 单项失败为 partial | PASS | rc=7 |
| report_inbox_today: dry-run 零 child 进程 | PASS | rc=0 |
| report_received_today: 两页耗尽后才 success | PASS | rc=0 |
| report_received_today: 首页失败不伪装空列表 | PASS | rc=1 |
| report_received_today: 后续页失败保留第一页 | PASS | rc=7 |
| report_received_today: hasMore 无 cursor fail-closed | PASS | rc=1 |
| report_received_today: JSON detail 单项失败为 partial | PASS | rc=7 |
| report_received_today: dry-run 零 child 进程 | PASS | rc=0 |

结论：**12/12 PASS**。

范围：证明分页耗尽、前后页失败、分页矛盾、JSON detail partial 与 dry-run 本地边界；真实账号数据仍需 live evidence。
