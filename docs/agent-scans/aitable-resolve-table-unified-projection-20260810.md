# aitable +resolve-table active Agent 审阅

扫描日期：2026-08-10

> 当前源码临时构建；Base 名称、baseId、tableId、tableName 与原始 JSON 只在内存中用于独立对拍。本文件不保存名称、ID 或 JSON fixture，也不接入 CI / policy。

## Result: PASS

| 检查项 | 结果 | 脱敏证据 |
|---|---|---|
| 严格消歧投影与 rollout 回归 | PASS | `rc=0` |
| 临时构建当前源码 | PASS | `rc=0` |
| Runtime Schema 固定唯一候选结果 | PASS | `properties=4, rc=0` |
| 真实表目录唯一候选独立对拍 | PASS | `rc=0, exact=yes, stable_id=yes, wire=active` |
| rollout 单步迁移 | PASS | `dual_validate -> unified_active` |

## 结论

- resolver 与 `+list-tables` 复用同一个严格完整表目录投影，不再维护独立的容器/ID/名称猜测器。
- 只有完整目录中的大小写不敏感精确名称可以默认成功；包含匹配仍要求显式 `--fuzzy`，零/多候选不猜选。
- 真实候选从原子 `table get` 在内存中派生，resolver 的稳定 ID/名称与之完全一致，Markdown 不保存业务值。
- 普通 `--format json` 直接输出 `ok/outcome/data/meta`；没有协议选择参数或版本字段。
