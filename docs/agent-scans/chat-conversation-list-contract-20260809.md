# Chat +conversation-list 分页契约 Agent 扫描

扫描日期：2026-08-09

> Agent 在当前工作树执行此扫描；它只验证声明、输出形状和本地分页 fixture，不调用真实 DingTalk，也不保存 JSON fixture。

| 检查 | 结果 | 证据 |
|---|---|---|
| 公开命令只使用一个 active 契约 | PASS | ConversationList 必须直接使用 unified_active；Agent 不选择协议版本。 |
| 终页零游标不被误报为矛盾 | PASS | hasMore=false + nextCursor=0 是 API 的终止哨兵，必须输出 endpoint_exhausted=true。 |
| 分页账本决定统一结果 | PASS | 业务读取一次后由 PageLedger 生成 CommandResult，再走单一输出出口。 |
| Agent 调用形式覆盖 --format json | PASS | promotion probe 只使用公开 format 参数，不存在协议选择 flag。 |
| 不暴露版本选择标记 | PASS | 测试明确拒绝在结果信封中泄漏 contract_version。 |
| fixture-backed active 结果语义 | PASS | rc=0 |

结论：**6/6 PASS**。`chat +conversation-list --format json` 已使用统一结果：`hasMore=false + nextCursor=0` 正确声明 endpoint 已耗尽；本地页上限返回可恢复 token；未知边界不声明耗尽；后续页读取失败保留成功页并返回 `partial_failure`/rc=7。

未验证：真实账号会话可见范围、网关实际响应形状和服务端分页终态；这些仍需要评测账号复验。
