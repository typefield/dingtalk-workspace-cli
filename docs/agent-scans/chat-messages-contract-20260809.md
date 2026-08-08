# Chat +chat-messages 统一结果契约 Agent 扫描

扫描日期：2026-08-09

> Agent 在当前工作树执行此扫描；它只验证声明、输出形状和本地分页 fixture，不调用真实 DingTalk，也不保存 JSON fixture。

| 检查 | 结果 | 证据 |
|---|---|---|
| 迁移状态为 unified_active | PASS | 此命令的公开 JSON 只有统一信封；Agent 始终不传协议选择参数。 |
| 读取成功后本地导出失败进入部分成功账本 | PASS | 成功消息页必须保留，导出失败不得压缩为丢失结果的普通错误。 |
| 失败项保留可恢复的本地导出事实 | PASS | 失败详情应标记操作、来源和请求的本地目标，而不把它伪装成远端读取失败。 |
| 统一结果只使用 --format json 且不泄漏版本标记 | PASS | 协议迁移由发布控制，Agent 始终只用公开 format 参数。 |
| 迁移历史保留 dual legacy 字节 golden | PASS | 历史双验证断言仍保留，证明晋级前未静默改变旧消费者输出。 |
| 发送者筛选未执行不误报成功 | PASS | 请求的 sender-query 未完成时保留消息页并返回 typed partial_failure。 |
| fixture-backed shadow/active 结果语义 | PASS | rc=0 |

结论：**7/7 PASS**。`chat +chat-messages` 已处于 `unified_active`：终页零游标可正确耗尽；范围已完成但源端未穷尽时不伪称 endpoint exhausted；后续读取、发送者筛选、资源下载和本地导出失败均保留成功页并返回 `partial_failure`/rc=7。Agent 只使用 `--format json`，不选择协议版本。

未验证：真实账号的消息可见范围、网关分页响应、本地文件系统实际故障和服务端终态；这些仍需要评测账号与受控环境复验。
