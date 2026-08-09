# aitable +base-get active Agent 审阅

扫描日期：2026-08-10

> 当前源码临时构建；Base 名称、baseId 与原始 JSON 只在内存中用于结构审阅。本文件仅保存脱敏字段类型与计数，不接入 CI / policy。

## Result: PASS

| 检查项 | 结果 | 脱敏证据 |
|---|---|---|
| 严格目录投影回归 | PASS | `rc=0` |
| 临时构建当前源码 | PASS | `rc=0` |
| 真实 Base 目录投影或未知形状 fail-closed | PASS | `rc=0, wire=active, tables=array:True, dashboards=array:True, documents=array:True` |

## 结论

- Base 目录只发布已取证的稳定资源 ID/名称；请求 ID 不一致、重复 ID、字段漂移与未取证的非空 documents 均失败关闭。
- `inventoryCoverageKnown:false` 明确表示这份响应不能扩大为用户所有 Base 或所有业务资源的权威清单。
- active 阶段由普通 `--format json` 直接返回统一结果；没有协议选择参数或版本字段。
