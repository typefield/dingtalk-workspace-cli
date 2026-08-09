# Mono Todo 只读分页结果契约 Agent 探针

扫描日期：2026-08-09

> 使用临时假 dws 验证聚合边界；不保存 JSON fixture，也不替代真实服务端分页取证。

| 检查 | 结果 | 证据 |
|---|---|---|
| todo_daily_summary: 短页成功保留 child meta | PASS | rc=0 |
| todo_daily_summary: 后续页失败不伪装完整成功 | PASS | rc=7 |
| todo_daily_summary: text 模式保持 partial rc=7 | PASS | rc=7 |
| todo_daily_summary: 达到硬上限不宣称完整 | PASS | rc=7 |
| todo_daily_summary: 首页 typed failure 原样分类 | PASS | rc=1 |
| todo_daily_summary: dry-run 不启动 child dws | PASS | rc=0 |
| todo_overdue_check: 短页成功保留 child meta | PASS | rc=0 |
| todo_overdue_check: 后续页失败不伪装完整成功 | PASS | rc=7 |
| todo_overdue_check: text 模式保持 partial rc=7 | PASS | rc=7 |
| todo_overdue_check: 达到硬上限不宣称完整 | PASS | rc=7 |
| todo_overdue_check: 首页 typed failure 原样分类 | PASS | rc=1 |
| todo_overdue_check: dry-run 不启动 child dws | PASS | rc=0 |

结论：**12/12 PASS**。

范围：证明两个 Mono Todo 汇总入口不会把后续页失败压成完整成功，并保留 child meta；端点真实耗尽、数据覆盖率与服务端终态仍需 live evidence。
