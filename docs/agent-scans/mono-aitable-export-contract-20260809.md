# Mono AITable 异步导出 Agent 语义探针

临时 child runner 与本地 HTTP server 只验证脚本编排、任务状态表达和原子落盘；不保存 JSON fixture，也不证明真实租户导出终态。

| 检查 | 结果 | 证据 |
|---|---|---|
| Help 可发现任务/输出安全参数 | PASS | rc=0 |
| dry-run 不创建任务或落盘 | PASS | rc=0; ok; calls=False |
| scope 缺 tableId 在调用前拒绝 | PASS | rc=1; ok |
| 创建任务结果不可信不建议重复创建 | PASS | rc=1; ok |
| 任务未完成返回可续 pending | PASS | rc=0; ok |
| 轮询不可信不伪造任务失败 | PASS | rc=0; ok |
| 任务明确失败返回 typed failure | PASS | rc=1; ok |
| 完成任务可只返回下载地址 | PASS | rc=0; ok |
| 下载原子落盘且不夸大远端完整性 | PASS | rc=0; ok |
| 已有输出须显式允许覆盖 | PASS | rc=1; ok |

结论：**10/10 PASS**。

范围：验证 Help、预览、参数门禁、异步 pending、任务终态、下载原子性和覆盖保护；真实导出内容、下载 URL 权限与远端校验和仍须隔离账号复验。
