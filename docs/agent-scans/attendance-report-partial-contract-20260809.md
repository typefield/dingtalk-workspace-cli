# 考勤报表逐批 partial ledger Agent 探针

扫描日期：2026-08-09

> 受控注入逐个调用 Mono/Multi 八个报表入口的批次函数，并审阅本地写入门禁；不保存 JSON fixture，不创建 Excel，不证明真实考勤权限或服务端终态。

| 检查 | 结果 | 证据 |
|---|---|---|
| mono-checkin: 明确空批次记成功而非失败 | PASS | succeeded=1, failed=0 |
| mono-checkin: 未知投影记 typed failure | PASS | succeeded=0, failed=1 |
| mono-checkin: 权限失败保留 authorization 类型 | PASS | succeeded=0, failed=1 |
| mono-checkin: 全失败门禁位于本地写入前 | PASS | failure_pos=14064, write_pos=14349 |
| mono-checkin: 终态使用公共 report_result | PASS | shared-helper |
| mono-daily: 明确空批次记成功而非失败 | PASS | succeeded=1, failed=0 |
| mono-daily: 未知投影记 typed failure | PASS | succeeded=0, failed=1 |
| mono-daily: 权限失败保留 authorization 类型 | PASS | succeeded=0, failed=1 |
| mono-daily: 全失败门禁位于本地写入前 | PASS | failure_pos=18098, write_pos=18357 |
| mono-daily: 终态使用公共 report_result | PASS | shared-helper |
| mono-monthly: 明确空批次记成功而非失败 | PASS | succeeded=1, failed=0 |
| mono-monthly: 未知投影记 typed failure | PASS | succeeded=0, failed=1 |
| mono-monthly: 权限失败保留 authorization 类型 | PASS | succeeded=0, failed=1 |
| mono-monthly: 全失败门禁位于本地写入前 | PASS | failure_pos=25717, write_pos=25977 |
| mono-monthly: 终态使用公共 report_result | PASS | shared-helper |
| mono-detail: 明确空批次记成功而非失败 | PASS | succeeded=1, failed=0 |
| mono-detail: 未知投影记 typed failure | PASS | succeeded=0, failed=1 |
| mono-detail: 权限失败保留 authorization 类型 | PASS | succeeded=0, failed=1 |
| mono-detail: 全失败门禁位于本地写入前 | PASS | failure_pos=26474, write_pos=26759 |
| mono-detail: 终态使用公共 report_result | PASS | shared-helper |
| multi-checkin: 明确空批次记成功而非失败 | PASS | succeeded=1, failed=0 |
| multi-checkin: 未知投影记 typed failure | PASS | succeeded=0, failed=1 |
| multi-checkin: 权限失败保留 authorization 类型 | PASS | succeeded=0, failed=1 |
| multi-checkin: 全失败门禁位于本地写入前 | PASS | failure_pos=14027, write_pos=14312 |
| multi-checkin: 终态使用公共 report_result | PASS | shared-helper |
| multi-daily: 明确空批次记成功而非失败 | PASS | succeeded=1, failed=0 |
| multi-daily: 未知投影记 typed failure | PASS | succeeded=0, failed=1 |
| multi-daily: 权限失败保留 authorization 类型 | PASS | succeeded=0, failed=1 |
| multi-daily: 全失败门禁位于本地写入前 | PASS | failure_pos=18061, write_pos=18320 |
| multi-daily: 终态使用公共 report_result | PASS | shared-helper |
| multi-monthly: 明确空批次记成功而非失败 | PASS | succeeded=1, failed=0 |
| multi-monthly: 未知投影记 typed failure | PASS | succeeded=0, failed=1 |
| multi-monthly: 权限失败保留 authorization 类型 | PASS | succeeded=0, failed=1 |
| multi-monthly: 全失败门禁位于本地写入前 | PASS | failure_pos=25680, write_pos=25940 |
| multi-monthly: 终态使用公共 report_result | PASS | shared-helper |
| multi-detail: 明确空批次记成功而非失败 | PASS | succeeded=1, failed=0 |
| multi-detail: 未知投影记 typed failure | PASS | succeeded=0, failed=1 |
| multi-detail: 权限失败保留 authorization 类型 | PASS | succeeded=0, failed=1 |
| multi-detail: 全失败门禁位于本地写入前 | PASS | failure_pos=26437, write_pos=26722 |
| multi-detail: 终态使用公共 report_result | PASS | shared-helper |
| mono-common: 混合批次为 partial ledger | PASS | outcome=partial_failure |
| mono-common: 全批次失败为 typed failure | PASS | outcome=failure |
| multi-common: 混合批次为 partial ledger | PASS | outcome=partial_failure |
| multi-common: 全批次失败为 typed failure | PASS | outcome=failure |

结论：**44/44 PASS**。

范围：证明已知空、未知投影、逐批 ledger、partial/failure 推导及全失败写前门禁；真实 Excel 内容、图片下载与后端覆盖仍需隔离/live evidence。
