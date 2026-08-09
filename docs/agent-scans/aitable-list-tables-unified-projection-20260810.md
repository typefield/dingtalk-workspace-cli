# aitable +list-tables active Agent 审阅

扫描日期：2026-08-10

> 当前源码临时构建；Base 名称、baseId、tableId、tableName 与原始 JSON 只在内存中用于独立对拍。本文件不保存名称、ID、token 或 JSON fixture，也不接入 CI / policy。

## Result: PASS

| 检查项 | 结果 | 脱敏证据 |
|---|---|---|
| 严格表目录投影与 rollout 回归 | PASS | `rc=0` |
| 临时构建当前源码 | PASS | `rc=0` |
| Runtime Schema 固定表目录结果 | PASS | `properties=1, rc=0` |
| 真实 Base 完整表目录独立对拍 | PASS | `source=1, projected=1, same_pairs=yes, wire=active` |
| rollout 单步迁移 | PASS | `dual_validate -> unified_active` |

## 结论

- `get_tables` 只接受实测的 `success -> data.tables[] -> tableId/tableName` 投影链路，不再猜测 `items/list/records/id/name`。
- 未知容器、非对象条目、空或重复稳定 ID、缺少名称与错误子集合类型均 fail-closed，不会变成空列表或原始成功结果。
- 真实 Base 的原子 `table get` 与 `+list-tables` 按稳定 ID/名称集合在内存中完全对拍；Markdown 只记录计数与布尔结论。
- 普通 `--format json` 直接输出 `ok/outcome/data/meta`；没有协议选择参数或版本字段。
