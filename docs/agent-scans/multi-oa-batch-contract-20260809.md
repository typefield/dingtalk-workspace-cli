# Multi OA 批量审批 Agent 语义探针

临时 child runner 仅用于本地分类与安全语义；本报告不保存 JSON fixture，也不证明真实审批终态。

| 检查 | 结果 | 证据 |
|---|---|---|
| 脚本 Help 可发现 format/dry-run/yes | PASS | rc=0 |
| dry-run 零写入且输出单信封 | PASS | rc=0; ok; sentinel=False |
| 非确认路径 fail-closed | PASS | rc=1; ok |
| 审批请求成功与任务查询未知不互相压缩 | PASS | rc=7; ok |
| 多个 taskId 不任选其一执行 | PASS | rc=1; ok |

结论：**5/5 PASS**。

范围：验证 Help、确认门禁、零写预览、唯一 taskId 选择和部分结果表达；审批动作已受理与实例终态仍分层表达，真实审批终态须用隔离账号复验。
