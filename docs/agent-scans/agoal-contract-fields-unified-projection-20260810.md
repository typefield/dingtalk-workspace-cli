# Agoal contract fields Agent active 审阅

扫描日期：2026-08-10

> Agent 从当前源码临时构建并审阅 Help、Runtime Schema 与可选真实响应；字段 ID、code、标题和值只在内存中处理，不写入本报告，也不接入 CI / policy。

## Result: PASS

| 检查项 | 结果 | 脱敏证据 |
|---|---|---|
| Contract/Safety/严格投影与 exclusion 回归 | PASS | `rc=0` |
| 临时构建当前源码 | PASS | `rc=0` |
| 无业务参数 Help 发现 | PASS | `rc=0, request_id=yes` |
| Runtime Schema 从精确 exclusion 进入公开面 | PASS | `rc=0, parameters=1` |
| Agoal 仍按叶渐进公开 | PASS | `public_agoal_tools=4` |
| 真实统一字段摘要 | PASS | `fields=11, unique_ids=yes, unique_codes=yes` |

## 结论

- 结果只发布稳定 fieldId、code、标题、类别、类型和四个激活/必填布尔值；presentation-only scheme 与当前恒空 source 不进入 Agent 摘要。
- 字段 ID/code 重复、未知字段、字符串布尔、非整数布局宽度或 source 形状漂移均 fail-closed 为不可重试的 projection_unknown。
- `fieldCoverageKnown:false` 表示服务端没有给出分页或权威目录覆盖事实；空数组不能扩大为组织没有经营合约字段。
- 当前处于 unified_active：普通 `--format json` 直接得到 `ok/outcome/data/meta`，不含协议选择参数或版本标记。
