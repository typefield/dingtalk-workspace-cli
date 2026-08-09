# Chat `+my-groups` dual-validate 分页 Agent 审阅

扫描日期：2026-08-09

> 本审只读取当前声明并运行本地 fixture；不调用 DingTalk 租户、不保存群名、conversationId 或 JSON fixture，也不是 CI / policy gate。

| 检查 | 结果 | 证据 |
|---|---|---|
| 单命令进入 dual_validate，未让 Agent 选择协议 | PASS | 公开调用仍只使用 `--format json`；外部结果保持 legacy，发布控制负责后续晋级。 |
| 同一次读取进入 PageLedger 候选结果 | PASS | 候选输出从实际读取派生，不得为双 renderer 重新执行业务请求。 |
| 完整性和部分失败由框架语义表达 | PASS | 后续页失败保留已读页；游标矛盾不宣称 endpoint exhausted。 |
| 双验证保持 legacy 字节结果 | PASS | 迁移期不要求下游 Agent 或脚本改 argv / 改解析器。 |
| 未来 active 路径不泄漏版本或 legacy 分页字段 | PASS | promotion probe 只用 `--format json`，检查 partial_failure/rc=7。 |
| 未来 active 群句柄与 IM 后续命令对齐 | PASS | 兼容期仍保留 legacy conversationId；统一候选只发布可直接传入 IM 后续命令的 openConversationId。 |
| fixture-backed dual/active 候选语义 | PASS | rc=0 |

结论：**7/7 PASS**。`chat +my-groups` 现从 legacy 进入 dual_validate：外部 JSON 仍保持原 payload；候选结果以 PageLedger 表达 endpoint 耗尽、可续页、未知边界与后续页 `partial_failure`。Agent 仍只传 `--format json`，不传协议版本或 rollout 参数。

未验证：真实账号的空群、跨页、游标冲突、可见范围和网关异常形状。已有真实单页投影证据不能替代这些边界；完成脱敏 live evidence 后才可按单命令晋级 active。
