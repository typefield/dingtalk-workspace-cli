# Contact 部门成员聚合结果契约 Agent 探针

扫描日期：2026-08-09

> 临时假 dws 对拍 Mono/Multi 两入口；不保存 JSON fixture，不证明真实通讯录可见范围。

| 检查 | 结果 | 证据 |
|---|---|---|
| mono-contact: 两部门成功与 child meta | PASS | rc=0 |
| mono-contact: 已知空搜索结果成功 | PASS | rc=0 |
| mono-contact: 搜索 typed failure 原样分类 | PASS | rc=1 |
| mono-contact: 搜索未知形状 fail-closed | PASS | rc=1 |
| mono-contact: 缺 deptId 不静默跳过 | PASS | rc=1 |
| mono-contact: 单部门失败保留成功部门 | PASS | rc=7 |
| mono-contact: 单部门投影漂移为 partial | PASS | rc=7 |
| mono-contact: 全部门失败为 failure | PASS | rc=1 |
| mono-contact: 已知空成员数组保留部门 | PASS | rc=0 |
| mono-contact: dry-run 零 child 进程 | PASS | rc=0 |
| mono-contact: 空 query 调用前 validation | PASS | rc=1 |
| multi-contact: 两部门成功与 child meta | PASS | rc=0 |
| multi-contact: 已知空搜索结果成功 | PASS | rc=0 |
| multi-contact: 搜索 typed failure 原样分类 | PASS | rc=1 |
| multi-contact: 搜索未知形状 fail-closed | PASS | rc=1 |
| multi-contact: 缺 deptId 不静默跳过 | PASS | rc=1 |
| multi-contact: 单部门失败保留成功部门 | PASS | rc=7 |
| multi-contact: 单部门投影漂移为 partial | PASS | rc=7 |
| multi-contact: 全部门失败为 failure | PASS | rc=1 |
| multi-contact: 已知空成员数组保留部门 | PASS | rc=0 |
| multi-contact: dry-run 零 child 进程 | PASS | rc=0 |
| multi-contact: 空 query 调用前 validation | PASS | rc=1 |

结论：**22/22 PASS**。

范围：证明搜索、稳定 ID、逐部门 partial、投影漂移、已知空、meta、dry-run 与参数校验；通讯录权限、索引覆盖和跨层级完整性仍需 live evidence。
