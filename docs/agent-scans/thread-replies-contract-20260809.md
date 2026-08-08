# Chat +thread-replies 分页契约 Agent 扫描

扫描日期：2026-08-09

> Agent 在当前工作树执行此扫描；它只验证声明、输出形状和本地分页 fixture，不调用真实 DingTalk，也不保存 JSON fixture。

| 检查 | 结果 | 证据 |
|---|---|---|
| 公开命令只使用一个 active 契约 | PASS | Agent 不选择协议版本；`--format json` 直接取得统一结果。 |
| --page-all 缺失或矛盾边界保留已读页 | PASS | 无法证明全量结果时必须为 partial_failure，不能把当前页伪装成完整成功。 |
| 显式资源下载失败进入部分成功 | PASS | 消息页读取成功不掩盖请求的本地资源副作用失败。 |
| 终页零游标仍正确耗尽 | PASS | nextCursor=0 是 API 终态哨兵，不是继续游标或错误。 |
| 生产声明的集成测试走真实结果出口 | PASS | 测试不再把 active 命令按旧 payload 断言，覆盖 success 与 partial_failure。 |
| 不暴露协议版本标记 | PASS | 统一信封不得输出 contract_version 或让 Agent 协商版本。 |
| fixture-backed active 结果语义 | PASS | rc=0 |

结论：**7/7 PASS**。`chat +thread-replies --format json` 已使用统一结果：终页零游标声明 endpoint 已耗尽；有可靠续页时返回 token；缺失/矛盾分页证据、后续读取或资源下载失败都保留成功页并返回 `partial_failure`/rc=7。

未验证：真实账号话题回复可见范围、网关实际响应形状、本地资源下载和服务端分页终态；这些仍需要评测账号复验。
