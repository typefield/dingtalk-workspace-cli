# Chat `+chat-list-all` 统一分页 Agent 审阅

扫描日期：2026-08-09

> 本审只读取当前声明并运行本地 fixture；不调用 DingTalk 租户、不保存群名、conversationId 或 JSON fixture，也不是 CI / policy gate。

| 检查 | 结果 | 证据 |
|---|---|---|
| 单命令进入 unified_active，Agent 不选择协议 | PASS | 公开调用只使用 `--format json`；每次调用只有一个 active 结果契约。 |
| 同一次读取映射至 PageLedger 候选结果 | PASS | 候选结果来自原业务请求，禁止为了输出再次执行目录读取。 |
| 无稳定群 ID 的展示行 fail-closed | PASS | 只有可继续操作的 openConversationId 才会进入群目录结果。 |
| 分页边界/后续页失败具备统一结果语义 | PASS | 已读页保留为 succeeded；游标矛盾不冒充 endpoint 耗尽。 |
| 迁移测试锁定 legacy/dual 字节兼容 | PASS | 单页和 --page-all 均不要求既有消费者更改 argv 或解析器。 |
| active 结果不泄漏版本或 legacy 分页字段 | PASS | active probe 只使用 `--format json`，检查 endpoint 终态与 partial_failure/rc=7。 |
| fixture-backed dual/active 候选语义 | PASS | rc=0 |

结论：**7/7 PASS**。`chat +chat-list-all` 已进入 unified_active：普通 `--format json` 直接由 PageLedger 表达 endpoint 耗尽、可续页、未知边界和后续页 `partial_failure`。Agent 不传协议版本或 rollout 参数。

边界：真实账号已观察到可续页形状，但空群、完整多页、游标冲突、可见范围和网关异常形状仍由 fixture 或后续隔离账号复验覆盖；active 的 endpoint exhaustion 不代表租户目录完整。
