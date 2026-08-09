# Chat `+my-groups` 统一分页 Agent 审阅

扫描日期：2026-08-09

> 本审只读取当前声明并运行本地 fixture；不调用 DingTalk 租户、不保存群名、conversationId 或 JSON fixture，也不是 CI / policy gate。

| 检查 | 结果 | 证据 |
|---|---|---|
| 单命令进入 unified_active，未让 Agent 选择协议 | PASS | 公开调用只使用 `--format json`；每次调用只有一个 active 结果契约。 |
| 同一次读取进入 PageLedger 候选结果 | PASS | 候选输出从实际读取派生，不得为双 renderer 重新执行业务请求。 |
| 完整性和部分失败由框架语义表达 | PASS | 后续页失败保留已读页；游标矛盾不宣称 endpoint exhausted。 |
| 迁移测试仍锁定 legacy/dual 字节结果 | PASS | 迁移期不要求下游 Agent 或脚本改 argv / 改解析器。 |
| active 路径不泄漏版本或 legacy 分页字段 | PASS | active probe 只用 `--format json`，检查 partial_failure/rc=7。 |
| active 群句柄与 IM 后续命令对齐 | PASS | 兼容期仍保留 legacy conversationId；统一候选只发布可直接传入 IM 后续命令的 openConversationId。 |
| fixture-backed dual/active 候选语义 | PASS | rc=0 |

结论：**7/7 PASS**。`chat +my-groups` 已进入 unified_active：普通 `--format json` 直接由 PageLedger 表达 endpoint 耗尽、可续页、未知边界与后续页 `partial_failure`。Agent 不传协议版本或 rollout 参数。

边界：真实账号已观察到可续页形状，但空群、完整跨页、游标冲突、可见范围和网关异常形状仍由 fixture 或后续隔离账号复验覆盖；active 的 endpoint exhaustion 不代表租户目录完整。
