# 考勤只读导出脚本结果契约 Agent 探针

扫描日期：2026-08-09

> 受控注入排班与假期余额 Mono/Multi 入口；不保存 JSON fixture，不创建 Excel，不证明真实考勤权限或后端终态。

| 检查 | 结果 | 证据 |
|---|---|---|
| mono-schedule: Help 可发现 format/dry-run | PASS | rc=0 |
| mono-vacation: Help 可发现 format/dry-run | PASS | rc=0 |
| mono-schedule: 明确空数组是成功空结果 | PASS | succeeded=1 failed=0 |
| mono-schedule: 未知投影不伪装空排班 | PASS | projection_unknown |
| mono-schedule: success 结果与写入边界 | PASS | rc=0 outcome=success writes=1 |
| mono-schedule: partial 结果与写入边界 | PASS | rc=7 outcome=partial_failure writes=1 |
| mono-schedule: failure 结果与写入边界 | PASS | rc=1 outcome=failure writes=0 |
| mono-vacation: 明确空规则数组是成功空结果 | PASS | succeeded=1 failed=0 |
| mono-vacation: 未知规则投影不伪装为空 | PASS | projection_unknown |
| mono-vacation: success 结果与写入边界 | PASS | rc=0 outcome=success writes=1 |
| mono-vacation: partial 结果与写入边界 | PASS | rc=7 outcome=partial_failure writes=1 |
| mono-vacation: failure 结果与写入边界 | PASS | rc=1 outcome=failure writes=0 |
| multi-schedule: Help 可发现 format/dry-run | PASS | rc=0 |
| multi-vacation: Help 可发现 format/dry-run | PASS | rc=0 |
| multi-schedule: 明确空数组是成功空结果 | PASS | succeeded=1 failed=0 |
| multi-schedule: 未知投影不伪装空排班 | PASS | projection_unknown |
| multi-schedule: success 结果与写入边界 | PASS | rc=0 outcome=success writes=1 |
| multi-schedule: partial 结果与写入边界 | PASS | rc=7 outcome=partial_failure writes=1 |
| multi-schedule: failure 结果与写入边界 | PASS | rc=1 outcome=failure writes=0 |
| multi-vacation: 明确空规则数组是成功空结果 | PASS | succeeded=1 failed=0 |
| multi-vacation: 未知规则投影不伪装为空 | PASS | projection_unknown |
| multi-vacation: success 结果与写入边界 | PASS | rc=0 outcome=success writes=1 |
| multi-vacation: partial 结果与写入边界 | PASS | rc=7 outcome=partial_failure writes=1 |
| multi-vacation: failure 结果与写入边界 | PASS | rc=1 outcome=failure writes=0 |

结论：**24/24 PASS**。

范围：证明 Help、已知空/未知投影、success/partial/failure、0/7/1 退出码和本地写入边界；真实数据内容与覆盖仍需隔离/live evidence。
