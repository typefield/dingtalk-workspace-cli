# Calendar 只读结果真实性 Agent 探针

扫描日期：2026-08-09

> 临时假 dws 对拍 Mono/Multi 日程与忙闲入口；不保存 JSON fixture，不证明真实日历覆盖。

| 检查 | 结果 | 证据 |
|---|---|---|
| mono-agenda: 已知 event 结构与 meta | PASS | rc=0 |
| mono-agenda: 已知空 events 才返回空日程 | PASS | rc=0 |
| mono-agenda: child failure 不伪装空日程 | PASS | rc=1 |
| mono-agenda: 未知形状 fail-closed | PASS | rc=1 |
| mono-agenda: 非法时间不静默丢项 | PASS | rc=1 |
| mono-agenda: dry-run 零 child 进程 | PASS | rc=0 |
| multi-agenda: 已知 event 结构与 meta | PASS | rc=0 |
| multi-agenda: 已知空 events 才返回空日程 | PASS | rc=0 |
| multi-agenda: child failure 不伪装空日程 | PASS | rc=1 |
| multi-agenda: 未知形状 fail-closed | PASS | rc=1 |
| multi-agenda: 非法时间不静默丢项 | PASS | rc=1 |
| multi-agenda: dry-run 零 child 进程 | PASS | rc=0 |
| mono-free: 全参与人覆盖后才推荐 | PASS | rc=0 |
| mono-free: 忙时段参与计算 | PASS | rc=0 |
| mono-free: 缺参与人覆盖拒绝推荐 | PASS | rc=1 |
| mono-free: 非法忙时段拒绝全天空闲 | PASS | rc=1 |
| mono-free: child failure 保留 typed error | PASS | rc=1 |
| mono-free: dry-run 零 child 进程 | PASS | rc=0 |
| mono-free: 非法工作时间调用前校验 | PASS | rc=1 |
| multi-free: 全参与人覆盖后才推荐 | PASS | rc=0 |
| multi-free: 忙时段参与计算 | PASS | rc=0 |
| multi-free: 缺参与人覆盖拒绝推荐 | PASS | rc=1 |
| multi-free: 非法忙时段拒绝全天空闲 | PASS | rc=1 |
| multi-free: child failure 保留 typed error | PASS | rc=1 |
| multi-free: dry-run 零 child 进程 | PASS | rc=0 |
| multi-free: 非法工作时间调用前校验 | PASS | rc=1 |

结论：**26/26 PASS**。

范围：证明已知空、typed failure、投影漂移、参与人覆盖、时间校验、meta 与 dry-run 的本地编排；真实日历权限、数据覆盖和服务端终态仍需 live evidence。
