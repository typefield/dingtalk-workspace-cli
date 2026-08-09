# 考勤报表 child 输出信封兼容 Agent 探针

扫描日期：2026-08-09

> 直接注入受控 subprocess 结果对拍 Mono/Multi 公共模块；不保存 JSON fixture，不调用 dws，不证明真实考勤数据或报表终态。

| 检查 | 结果 | 证据 |
|---|---|---|
| mono-report-common: 统一成功信封解到 data | PASS | ok |
| mono-report-common: 旧成功信封兼容 | PASS | ok |
| mono-report-common: bare 业务 JSON 兼容 | PASS | ok |
| mono-report-common: 非零统一错误保留 typed 信息 | PASS | scope_missing |
| mono-report-common: child partial 不伪装完整数据 | PASS | child_partial_failure |
| mono-report-common: child pending 不伪装终态数据 | PASS | operation_pending |
| mono-report-common: 字符串 success 被拒绝 | PASS | untyped_status |
| mono-report-common: 矛盾 ok/outcome 被拒绝 | PASS | untyped_status |
| mono-report-common: 非零退出与成功信封矛盾被拒绝 | PASS | exit_outcome_inconsistent |
| mono-report-common: rc0 非 JSON 被拒绝 | PASS | DwsCallError |
| mono-report-common: 非零文本错误保留权限分类 | PASS | DwsCallError |
| multi-report-common: 统一成功信封解到 data | PASS | ok |
| multi-report-common: 旧成功信封兼容 | PASS | ok |
| multi-report-common: bare 业务 JSON 兼容 | PASS | ok |
| multi-report-common: 非零统一错误保留 typed 信息 | PASS | scope_missing |
| multi-report-common: child partial 不伪装完整数据 | PASS | child_partial_failure |
| multi-report-common: child pending 不伪装终态数据 | PASS | operation_pending |
| multi-report-common: 字符串 success 被拒绝 | PASS | untyped_status |
| multi-report-common: 矛盾 ok/outcome 被拒绝 | PASS | untyped_status |
| multi-report-common: 非零退出与成功信封矛盾被拒绝 | PASS | exit_outcome_inconsistent |
| multi-report-common: rc0 非 JSON 被拒绝 | PASS | DwsCallError |
| multi-report-common: 非零文本错误保留权限分类 | PASS | DwsCallError |

结论：**22/22 PASS**。

范围：证明统一/旧/bare child 结果解包、typed error、pending/partial、字符串布尔值和退出码矛盾；各报表的逐批 partial ledger 仍需单独收敛。
