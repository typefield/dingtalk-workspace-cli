# Chat `+chat-list-all` dual-validate 分页 Agent 审阅

扫描日期：2026-08-09

> 本审只读取当前声明并运行本地 fixture；不调用 DingTalk 租户、不保存群名、conversationId 或 JSON fixture，也不是 CI / policy gate。

| 检查 | 结果 | 证据 |
|---|---|---|
| 单命令进入 dual_validate，Agent 不选择协议 | PASS | 公开调用仍只使用 `--format json`；外部输出维持 legacy，由发布评审决定后续晋级。 |
| 同一次读取映射至 PageLedger 候选结果 | PASS | 候选结果来自原业务请求，禁止为了输出再次执行目录读取。 |
| 无稳定群 ID 的展示行 fail-closed | PASS | 只有可继续操作的 openConversationId 才会进入群目录结果。 |
| 分页边界/后续页失败具备统一结果语义 | PASS | 已读页保留为 succeeded；游标矛盾不冒充 endpoint 耗尽。 |
| 双验证锁定 legacy 字节兼容 | PASS | 单页和 --page-all 均不要求既有消费者更改 argv 或解析器。 |
| 未来 active 结果不泄漏版本或 legacy 分页字段 | PASS | promotion probe 只使用 `--format json`，检查 endpoint 终态与 partial_failure/rc=7。 |
| fixture-backed dual/active 候选语义 | PASS | rc=0 |

结论：**7/7 PASS**。`chat +chat-list-all` 现从 legacy 进入 dual_validate：历史 JSON 仍原样输出；内部候选由 PageLedger 表达端点耗尽、可续页、未知边界和后续页 `partial_failure`。Agent 仍只传 `--format json`，不传协议版本或 rollout 参数。

未验证：真实账号的空群、多页、游标冲突、可见范围和网关异常形状。已存在正常单页只读证据不能替代这些边界；完成脱敏 live evidence 后才可按单命令评审进入 active。
