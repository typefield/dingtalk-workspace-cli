# OA 待审聚合结果契约 Agent 探针

扫描日期：2026-08-09

> 临时假 dws 对拍 Mono/Multi 两入口；不保存 JSON fixture，不证明真实审批权限或服务端终态。

| 检查 | 结果 | 证据 |
|---|---|---|
| mono-oa: 列表与两详情成功 | PASS | rc=0 |
| mono-oa: 已知空列表成功 | PASS | rc=0 |
| mono-oa: 列表 typed failure | PASS | rc=1 |
| mono-oa: 列表未知形状 fail-closed | PASS | rc=1 |
| mono-oa: 缺 processInstanceId 不跳过 | PASS | rc=1 |
| mono-oa: 单详情失败保留成功项 | PASS | rc=7 |
| mono-oa: 单详情投影漂移为 partial | PASS | rc=7 |
| mono-oa: 全详情失败为 failure | PASS | rc=1 |
| mono-oa: dry-run 零 child 进程 | PASS | rc=0 |
| mono-oa: 非法 days 调用前校验 | PASS | rc=1 |
| multi-oa: 列表与两详情成功 | PASS | rc=0 |
| multi-oa: 已知空列表成功 | PASS | rc=0 |
| multi-oa: 列表 typed failure | PASS | rc=1 |
| multi-oa: 列表未知形状 fail-closed | PASS | rc=1 |
| multi-oa: 缺 processInstanceId 不跳过 | PASS | rc=1 |
| multi-oa: 单详情失败保留成功项 | PASS | rc=7 |
| multi-oa: 单详情投影漂移为 partial | PASS | rc=7 |
| multi-oa: 全详情失败为 failure | PASS | rc=1 |
| multi-oa: dry-run 零 child 进程 | PASS | rc=0 |
| multi-oa: 非法 days 调用前校验 | PASS | rc=1 |

结论：**20/20 PASS**。

范围：证明列表、稳定实例 ID、逐详情 partial、投影漂移、已知空、meta、dry-run 与参数校验；审批可见范围和真实状态仍需 live evidence。
