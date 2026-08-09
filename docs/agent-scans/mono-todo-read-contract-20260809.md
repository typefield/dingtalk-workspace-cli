# Mono/Multi Todo 只读分页结果契约 Agent 探针

扫描日期：2026-08-09

> 使用临时假 dws 验证聚合边界；不保存 JSON fixture，也不替代真实服务端分页取证。

| 检查 | 结果 | 证据 |
|---|---|---|
| mono-daily: 短页成功保留 child meta | PASS | rc=0 |
| mono-daily: 后续页失败不伪装完整成功 | PASS | rc=7 |
| mono-daily: text 模式保持 partial rc=7 | PASS | rc=7 |
| mono-daily: 达到硬上限不宣称完整 | PASS | rc=7 |
| mono-daily: 首页 typed failure 原样分类 | PASS | rc=1 |
| mono-daily: dry-run 不启动 child dws | PASS | rc=0 |
| mono-overdue: 短页成功保留 child meta | PASS | rc=0 |
| mono-overdue: 后续页失败不伪装完整成功 | PASS | rc=7 |
| mono-overdue: text 模式保持 partial rc=7 | PASS | rc=7 |
| mono-overdue: 达到硬上限不宣称完整 | PASS | rc=7 |
| mono-overdue: 首页 typed failure 原样分类 | PASS | rc=1 |
| mono-overdue: dry-run 不启动 child dws | PASS | rc=0 |
| multi-daily: 短页成功保留 child meta | PASS | rc=0 |
| multi-daily: 后续页失败不伪装完整成功 | PASS | rc=7 |
| multi-daily: text 模式保持 partial rc=7 | PASS | rc=7 |
| multi-daily: 达到硬上限不宣称完整 | PASS | rc=7 |
| multi-daily: 首页 typed failure 原样分类 | PASS | rc=1 |
| multi-daily: dry-run 不启动 child dws | PASS | rc=0 |
| multi-overdue: 短页成功保留 child meta | PASS | rc=0 |
| multi-overdue: 后续页失败不伪装完整成功 | PASS | rc=7 |
| multi-overdue: text 模式保持 partial rc=7 | PASS | rc=7 |
| multi-overdue: 达到硬上限不宣称完整 | PASS | rc=7 |
| multi-overdue: 首页 typed failure 原样分类 | PASS | rc=1 |
| multi-overdue: dry-run 不启动 child dws | PASS | rc=0 |

结论：**24/24 PASS**。

范围：证明 Mono/Multi 四个 Todo 汇总入口不会把后续页失败压成完整成功，并保留 child meta；端点真实耗尽、数据覆盖率与服务端终态仍需 live evidence。
