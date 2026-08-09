# 考勤记录报表结果契约 Agent 探针

扫描日期：2026-08-09

> 受控注入 Mono/Multi 的列表、详情与最终写入边界；不保存 JSON fixture，不创建 Excel，不证明真实审批/考勤权限或后端终态。

| 检查 | 结果 | 证据 |
|---|---|---|
| mono: Help 可发现 format/dry-run | PASS | rc=0 |
| mono: 明确空审批列表是成功空结果 | PASS | succeeded=1 failed=0 |
| mono: 未知审批列表投影不伪装为空 | PASS | succeeded=0 failed=1 |
| mono: 权限失败保留 authorization | PASS | authorization |
| mono: 非对象审批详情成为 projection failure | PASS | projection_unknown |
| mono: 完整结果保持 success 与旧 data 形状 | PASS | rc=0 writes=1 outcome=success |
| mono: 部分读取保留文件并返回 rc7 ledger | PASS | rc=7 writes=1 outcome=partial_failure |
| mono: 全批失败不写文件并返回 typed failure | PASS | rc=1 writes=0 outcome=failure |
| multi: Help 可发现 format/dry-run | PASS | rc=0 |
| multi: 明确空审批列表是成功空结果 | PASS | succeeded=1 failed=0 |
| multi: 未知审批列表投影不伪装为空 | PASS | succeeded=0 failed=1 |
| multi: 权限失败保留 authorization | PASS | authorization |
| multi: 非对象审批详情成为 projection failure | PASS | projection_unknown |
| multi: 完整结果保持 success 与旧 data 形状 | PASS | rc=0 writes=1 outcome=success |
| multi: 部分读取保留文件并返回 rc7 ledger | PASS | rc=7 writes=1 outcome=partial_failure |
| multi: 全批失败不写文件并返回 typed failure | PASS | rc=1 writes=0 outcome=failure |

结论：**16/16 PASS**。

范围：证明入口发现、已知空/未知投影、权限分类、完整 success、partial/rc7 和全失败零本地写；真实服务端与 Excel 内容仍需隔离/live evidence。
