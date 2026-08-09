# 考勤基础只读脚本结果契约 Agent 探针

扫描日期：2026-08-09

> 临时假 dws 对拍 Mono/Multi 四入口；不保存 JSON fixture，不证明真实考勤权限、数据覆盖或服务端终态。

| 检查 | 结果 | 证据 |
|---|---|---|
| mono-my-record: 身份与考勤详情成功 | PASS | rc=0, calls=2 |
| mono-my-record: 服务端明确 null 才返回空 | PASS | rc=0 |
| mono-my-record: 当前用户失败不变成 validation/空 | PASS | rc=1 |
| mono-my-record: 当前用户缺 userId fail-closed | PASS | rc=1 |
| mono-my-record: 考勤 child 失败不伪装空 | PASS | rc=1 |
| mono-my-record: 考勤未知形状 fail-closed | PASS | rc=1 |
| mono-my-record: dry-run 零 child 进程 | PASS | rc=0, calls=0 |
| mono-my-record: 无效真实日期调用前拒绝 | PASS | rc=1 |
| multi-my-record: 身份与考勤详情成功 | PASS | rc=0, calls=2 |
| multi-my-record: 服务端明确 null 才返回空 | PASS | rc=0 |
| multi-my-record: 当前用户失败不变成 validation/空 | PASS | rc=1 |
| multi-my-record: 当前用户缺 userId fail-closed | PASS | rc=1 |
| multi-my-record: 考勤 child 失败不伪装空 | PASS | rc=1 |
| multi-my-record: 考勤未知形状 fail-closed | PASS | rc=1 |
| multi-my-record: dry-run 零 child 进程 | PASS | rc=0, calls=0 |
| multi-my-record: 无效真实日期调用前拒绝 | PASS | rc=1 |
| mono-team-shift: 有界班次成功且用户去重 | PASS | rc=0 |
| mono-team-shift: 已知空 items 成功 | PASS | rc=0 |
| mono-team-shift: child 失败不伪装空 | PASS | rc=1 |
| mono-team-shift: 未知容器 fail-closed | PASS | rc=1 |
| mono-team-shift: 缺 userId 不静默跳过 | PASS | rc=1 |
| mono-team-shift: dry-run 零 child 进程 | PASS | rc=0, calls=0 |
| mono-team-shift: 超 7 天调用前拒绝 | PASS | rc=1 |
| multi-team-shift: 有界班次成功且用户去重 | PASS | rc=0 |
| multi-team-shift: 已知空 items 成功 | PASS | rc=0 |
| multi-team-shift: child 失败不伪装空 | PASS | rc=1 |
| multi-team-shift: 未知容器 fail-closed | PASS | rc=1 |
| multi-team-shift: 缺 userId 不静默跳过 | PASS | rc=1 |
| multi-team-shift: dry-run 零 child 进程 | PASS | rc=0, calls=0 |
| multi-team-shift: 超 7 天调用前拒绝 | PASS | rc=1 |

结论：**30/30 PASS**。

范围：证明身份投影、个人考勤/团队班次已知空、child failure、投影漂移、参数校验、meta 和 dry-run；真实管理员权限与组织数据完整性仍需 live evidence。
