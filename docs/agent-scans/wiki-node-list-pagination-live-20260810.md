# Wiki node-list 分页边界 Agent 实测

扫描日期：2026-08-10

> 仅执行只读空间/节点列表调用。知识库、节点和标题标识只在内存中使用；报告不保存原始 JSON，也不接入 CI。

## Result: REVIEW

| 检查项 | 结果 | 脱敏证据 |
|---|---|---|
| 可见知识库发现 | PASS | `workspace=yes` |
| 真实首屏续页证据 | PASS | `count=1, endpoint_exhausted=false, next_token=yes` |
| 真实第二页可恢复且不重复首屏 | PASS | `count=1, stable_ids=yes, pages_disjoint=yes` |
| 真实空文件夹终页 | REVIEW | `folder_candidates=33, bounded_probes=20, empty_found=no` |

## 结论

- `hasMore:true + nextPageToken` 必须投影为 `endpoint_exhausted:false + next_token`；Agent 使用返回 token 恢复下一页，不猜测游标字段。
- 每页任一行缺稳定 string `nodeId` 时必须整体 fail-closed，不能静默丢行后仍返回 success。
- 空目录只有在真实空数组且 endpoint 明确耗尽时成立；没有空文件夹 fixture 只记为未验证，不构造资源或扩大结论。

## Findings

- bounded live scope did not contain a readable empty folder
