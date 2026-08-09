# aitable +resolve-base dual Agent 审阅

扫描日期：2026-08-10

> 当前源码临时构建；Base 名称、baseId、查询词与原始 JSON 只在内存中用于独立对拍。本文件不保存名称、ID 或 JSON fixture，也不接入 CI / policy。

## Result: PASS

| 检查项 | 结果 | 脱敏证据 |
|---|---|---|
| 严格候选/分页投影与 rollout 回归 | PASS | `rc=0` |
| 临时构建当前源码 | PASS | `rc=0` |
| Runtime Schema 固定唯一候选与索引边界 | PASS | `properties=8, rc=0` |
| 真实名称搜索终态或 fail-closed 对拍 | PASS | `rc=nonzero, subtype=pagination_inconsistent, retryable_true=no, selected=no` |

## 结论

- resolver 只接受稳定 `baseId/baseName`，逐页核对 `hasMore/nextCursor`，重复、不前进或相互矛盾的游标均失败关闭。
- 只有搜索端点明确耗尽后才允许唯一精确/显式 fuzzy 选择；`endpoint_exhausted` 不扩大成索引完整。
- 本次真实只读响应的游标发生停滞；命令正确返回 typed `pagination_inconsistent` 且不发布候选，没有把第一页伪装成完整目录。
- `--format json` 是唯一 Agent 输出入口；没有协议选择参数或版本字段。
