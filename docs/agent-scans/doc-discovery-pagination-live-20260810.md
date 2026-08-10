# Doc 发现分页与投影 Agent 实测

扫描日期：2026-08-10

> 仅执行只读空间、文档列表与搜索调用。知识库、节点、标题、URL、查询词和 token 只在内存中使用；报告不保存原始 JSON，也不接入 CI。

## Result: REVIEW

| 检查项 | 结果 | 脱敏证据 |
|---|---|---|
| 可见知识库发现 | PASS | `workspace=yes` |
| Doc list 首屏续页与范围 | PASS | `count=1, endpoint_exhausted=false, next_token=yes, scope=requested_location` |
| Doc list 第二页可恢复且不重复 | PASS | `count=1, stable_ids=yes, pages_disjoint=yes` |
| Doc list 真实空文件夹终页 | REVIEW | `folder_candidates=33, likely_empty=0, bounded_probes=3, empty_found=no` |
| Doc search 首屏续页且不扩大索引覆盖 | PASS | `count=1, endpoint_exhausted=false, next_token=yes, index_coverage_known=false` |
| Doc search 第二页可恢复且不重复 | PASS | `count=1, stable_ids=yes, pages_disjoint=yes` |
| Doc search 无命中只声明 endpoint 终页 | PASS | `count=0, endpoint_exhausted=true, index_coverage_known=false` |
| Doc search 非法 limit 调用前失败 | PASS | `rc=3, subtype=invalid_flag_value` |
| Doc list 非法 limit 调用前失败 | PASS | `rc=3, subtype=invalid_flag_value` |

## 结论

- `hasMore:true + nextPageToken` 被投影为 `meta.pagination.endpoint_exhausted:false + next_token`；Agent 可以原样续页。
- 搜索 endpoint 耗尽不等于索引健康或全局召回完整，因此始终保留 `index_coverage_known:false`。
- 列表范围固定为 `requested_location`；它只表示请求位置的直接子节点，不扩大为递归目录或租户全量文档。
- 非法页大小在远端调用前返回 typed validation；未知容器、非法行、字段类型漂移及分页矛盾由 Go response seam fail-closed。

## 尚未验证

- 有界只读范围内未找到真实空文件夹时，只记录 REVIEW；不为通过测试而创建或修改业务资源。
