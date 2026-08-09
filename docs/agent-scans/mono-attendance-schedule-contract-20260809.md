# Mono 考勤排班导入 Agent 语义探针

> 临时 child runner 只验证脚本的本地确认、参数委托与结果表达；不证明真实租户的权限、排班持久化或 exactly-once。仅保存 Markdown 证据。

| 检查 | 结果 | 证据 |
|---|---|---|
| Help 只公开脚本确认参数 --yes | PASS | rc=0 |
| 类型错误在任何 child 调用前返回 typed validation | PASS | rc=1; ok; child_calls=0 |
| 缺确认在任何 child 调用前 fail-closed | PASS | rc=1; ok; child_calls=0 |
| dry-run 可做只读校验但不会导入排班 | PASS | rc=0; ok; child_reads=3 |
| 确认后传底层 canonical --user-say-yes，且只报告请求受理 | PASS | rc=0; ok; import_calls=1 |
| 写请求异常保留 unknown、禁止把 retryable 透传为重放许可 | PASS | rc=1; ok |

结论：**6/6 PASS**。

边界：成功只表示 child API 接受请求，脚本明确标记 `verification.state=not_verified`；写请求异常只表示终态未知，Agent 必须先查询排班而不是重放导入。
