# AITable Base discovery Agent 实测

扫描日期：2026-08-09

> 本探针只读取当前用户最近访问 Base 的一页，并在内存中取一个返回名称做搜索对拍；不保存 Base 名称、ID、查询词或原始 JSON，不接入 CI。

## 结果

**PASS**

## 可观测事实

- `recently_accessed` 已返回统一 success，count=1，稳定 baseId=true。
- `recently_accessed` 保留非权威目录/覆盖未知边界：true。
- `recently_accessed` 分页已知性为 False，只在 `meta.pagination` 表达续页事实：true。
- `recently_accessed` stderr 为空：true。
- `name_search_index` 已返回统一 success，count=1，稳定 baseId=true。
- `name_search_index` 保留非权威目录/覆盖未知边界：true。
- `name_search_index` 分页已知性为 True，只在 `meta.pagination` 表达续页事实：true。
- `name_search_index` stderr 为空：true。

## 边界

本次只验证一组真实正常列表/搜索响应，不证明最近访问列表等于所有可访问 Base，也不验证死条目、搜索召回率或服务端索引健康。分页矛盾/缺 continuation 的 fail-closed 行为由专项回归覆盖；搜索零命中仍只能表示当前索引返回为空，不能扩大成业务上不存在。
