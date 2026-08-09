# wiki +resolve-space dual Agent 审阅

扫描日期：2026-08-10

> 当前源码临时构建；空间名称、workspaceId 和分页 token 只在内存中用于独立对拍。本文件不保存查询词、名称、ID、token 或原始 JSON，也不接入 CI / policy。

## Result: PASS

| 检查项 | 结果 | 脱敏证据 |
|---|---|---|
| 严格投影、分页与 rollout 回归 | PASS | `rc=0` |
| 临时构建当前源码 | PASS | `rc=0` |
| Runtime Schema 固定消歧结果 | PASS | `properties=3, rc=0` |
| 真实组织知识库完整目录消歧对拍 | PASS | `rc=0, legacy=yes, source_pages=3, candidates=1, stable_ids=yes` |
| rollout 单步迁移 | PASS | `legacy_only -> dual_validate` |

## 结论

- resolver 使用 `list_wikiSpaces` 逐页耗尽当前身份可访问的组织知识库目录，不再把无覆盖事实的搜索首屏扩大为唯一结果。
- 候选稳定 ID 只接受真实服务字段 `workspaceId`；未知容器、非法条目、空/重复 ID 与未知字段均 fail-closed。
- `hasMore/nextPageToken` 缺失、矛盾、重复或超过安全页数均返回稳定分页错误，不会发布 `resolved:true`。
- `--format json` 是唯一 Agent 输出入口；没有协议选择参数或版本字段。
