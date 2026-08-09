# Chat 群目录统一结果 Agent 实测

扫描日期：2026-08-09

> 本探针只读当前用户已加入群的一页；群名、群 ID 与原始 JSON 都不落盘。它是 Agent 审阅证据，不接入 CI。

## 结果

**PASS**

## 可观测事实

- `chat +my-groups` 直接返回统一 `ok/outcome/data/meta`：true。
- 投影数量与 `meta.count` 一致：1。
- 所有投影群均有稳定 `openConversationId`：true。
- 真实单页明确可续：`endpoint_exhausted:false + next_token`：true。
- 未暴露协议版本或 legacy 分页字段，stderr 为空：true。
- `chat +chat-list-all` 直接返回统一 `ok/outcome/data/meta`：true。
- 投影数量与 `meta.count` 一致：1。
- 所有投影群均有稳定 `openConversationId`：true。
- 真实单页明确可续：`endpoint_exhausted:false + next_token`：true。
- 未暴露协议版本或 legacy 分页字段，stderr 为空：true。

## 边界

本次真实账号只验证两个入口的正常可续单页；未证明空群、完整跨页、游标冲突、网关异常形状或租户目录覆盖范围。这些边界继续由 fixture 和后续隔离账号复验覆盖；endpoint exhausted 只表示该接口分页耗尽。
