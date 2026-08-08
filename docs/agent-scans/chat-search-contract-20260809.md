# Chat +chat-search 分页契约 Agent 扫描

扫描日期：2026-08-09

> Agent 在当前工作树执行此扫描；它只验证声明、输出形状和本地分页 fixture，不调用真实 DingTalk，也不保存 JSON fixture。

| 检查 | 结果 | 证据 |
|---|---|---|
| 公开命令只使用一个 active 契约 | PASS | ChatSearch 必须直接使用 unified_active；Agent 不选择协议版本。 |
| 最大窗口与分页账本共同决定结果 | PASS | 满页但无续页 token 的探测不能绕过 PageLedger。 |
| Agent 调用形式覆盖 --format json | PASS | promotion probe 走公开 format 参数，不存在协议选择 flag。 |
| 不暴露版本选择标记 | PASS | 测试明确拒绝 contract_version 出现在结果信封。 |
| fixture-backed active 结果语义 | PASS | rc=0 |

结论：**5/5 PASS**。`chat +chat-search --format json` 已使用统一结果：续页返回可恢复 token；未知边界不声明耗尽；后续读取失败、矛盾 cursor 和最大窗口仍满页均返回 `partial_failure`/rc=7。

未验证：真实群搜索索引覆盖、网关实际响应形状和账号可见范围；这些仍需要评测账号复验。
