# Multi 日程创建 Agent 语义探针

临时 child runner 只验证脚本编排与结果表达；本报告不保存 JSON fixture，也不创建真实日程或预订会议室。

| 检查 | 结果 | 证据 |
|---|---|---|
| 脚本 Help 可发现 format/dry-run/yes | PASS | rc=0 |
| dry-run 不搜索会议室也不写入 | PASS | rc=0; ok; sentinel=False |
| 未确认日程创建 fail-closed | PASS | rc=1; ok; sentinel=False |
| 无效时间范围在任何调用前失败 | PASS | rc=1; ok; sentinel=False |
| 无可订会议室时不创建半成品日程 | PASS | rc=1; ok |
| 建日程、加人、订房与回读均成功才标 verified | PASS | rc=0; ok |
| 参会人写入未知时保留已创建日程并返回 rc=7 | PASS | rc=7; ok |

结论：**7/7 PASS**。

范围：验证 Help、确认门禁、零写预览、会议室预检、分步 partial 与 event get 回读；真实日程、参会人和会议室终态仍须隔离账号复验。
