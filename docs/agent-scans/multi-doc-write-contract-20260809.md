# Multi 文档创建写入 Agent 语义探针

临时 child runner 和内容仅用于本次探针；报告不保存 JSON fixture，也不创建真实文档。

| 检查 | 结果 | 证据 |
|---|---|---|
| 脚本 Help 可发现 format/dry-run/yes | PASS | rc=0 |
| dry-run 零 child 调用且返回计划 | PASS | rc=0; ok; sentinel=False |
| 未确认创建写入 fail-closed | PASS | rc=1; ok; sentinel=False |
| 非幂等写入拒绝自动重试配置 | PASS | rc=1; ok; sentinel=False |
| 创建、写入、读回逐块对拍后才标 verified | PASS | rc=0; ok |
| 后续块未知保留前序成功且不重放 | PASS | rc=7; ok |

结论：**6/6 PASS**。

范围：验证 Help、确认门禁、零写预览、禁止自动重放、逐块 partial 与读回表达；真实文档服务终态仍须隔离账号复验。
