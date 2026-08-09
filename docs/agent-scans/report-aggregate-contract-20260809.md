# Report 聚合脚本结果契约 Agent 探针

扫描日期：2026-08-09

> 临时假 dws 验证两个兼容入口；不保存 JSON fixture，不证明真实日志可见性或服务端终态。

| 检查 | 结果 | 证据 |
|---|---|---|
| mono-inbox: 两页耗尽后才 success | PASS | rc=0 |
| mono-inbox: 首页失败不伪装空列表 | PASS | rc=1 |
| mono-inbox: 后续页失败保留第一页 | PASS | rc=7 |
| mono-inbox: hasMore 无 cursor fail-closed | PASS | rc=1 |
| mono-inbox: JSON detail 单项失败为 partial | PASS | rc=7 |
| mono-inbox: dry-run 零 child 进程 | PASS | rc=0 |
| mono-received: 两页耗尽后才 success | PASS | rc=0 |
| mono-received: 首页失败不伪装空列表 | PASS | rc=1 |
| mono-received: 后续页失败保留第一页 | PASS | rc=7 |
| mono-received: hasMore 无 cursor fail-closed | PASS | rc=1 |
| mono-received: JSON detail 单项失败为 partial | PASS | rc=7 |
| mono-received: dry-run 零 child 进程 | PASS | rc=0 |
| multi-misc-received: 两页耗尽后才 success | PASS | rc=0 |
| multi-misc-received: 首页失败不伪装空列表 | PASS | rc=1 |
| multi-misc-received: 后续页失败保留第一页 | PASS | rc=7 |
| multi-misc-received: hasMore 无 cursor fail-closed | PASS | rc=1 |
| multi-misc-received: JSON detail 单项失败为 partial | PASS | rc=7 |
| multi-misc-received: dry-run 零 child 进程 | PASS | rc=0 |

结论：**18/18 PASS**。

范围：证明分页耗尽、前后页失败、分页矛盾、JSON detail partial 与 dry-run 本地边界；真实账号数据仍需 live evidence。
