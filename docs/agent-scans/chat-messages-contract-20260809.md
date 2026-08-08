# Chat +chat-messages 渐进结果契约 Agent 扫描

扫描日期：2026-08-09

> Agent 在当前工作树执行此扫描；它只验证声明、输出形状和本地分页 fixture，不调用真实 DingTalk，也不保存 JSON fixture。

| 检查 | 结果 | 证据 |
|---|---|---|
| 迁移状态保持 dual_validate | PASS | 当前仍保留 legacy stdout/错误行为；Agent 不传协议选择参数。 |
| 读取成功后本地导出失败进入部分成功账本 | PASS | 成功消息页必须保留，导出失败不得压缩为丢失结果的普通错误。 |
| 失败项保留可恢复的本地导出事实 | FAIL | 失败详情应标记操作、来源和请求的本地目标，而不把它伪装成远端读取失败。 |
| 激活态探针只使用 --format json 且不泄漏版本标记 | PASS | 协议迁移由发布控制，Agent 始终只用公开 format 参数。 |
| 双验证保持 legacy 字节输出 | PASS | 尚未激活时新增 shadow 结果不能改变消费者看到的历史输出。 |
| fixture-backed shadow/active 结果语义 | PASS | rc=0 |

结论：**5/6 PASS**。`chat +chat-messages` 仍处于 `dual_validate`，不改变 legacy stdout；但其激活态探针已证明：终页零游标可正确耗尽、后续读取/资源下载/本地导出失败均保留成功页并返回 `partial_failure`/rc=7。Agent 只使用 `--format json`，不选择协议版本。

未验证：真实账号的消息可见范围、网关分页响应、本地文件系统实际故障和服务端终态；这些仍需要评测账号与受控环境复验。
