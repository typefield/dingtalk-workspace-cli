# Chat +flag-list 分页契约 Agent 扫描

扫描日期：2026-08-09

> 本扫描由 Agent 发起，读取当前命令声明并运行本地 fixture probe。它只验证 CLI 的声明、分页结果和输出形状；不调用真实 DingTalk，不保存 JSON fixture，也不能证明服务端索引覆盖或真实分页终态。

| 检查 | 结果 | 证据 |
|---|---|---|
| 公开命令只使用一个 active 契约 | PASS | FlagList 的 rollout 必须是 unified_active；不接受 Agent/用户选择协议。 |
| 分页账本生成统一结果 | PASS | 业务读取一次后由 PageLedger 生成 CommandResult，再经单一输出出口传递。 |
| Agent 调用形式覆盖 --format json | PASS | promotion probe 必须使用公开的格式参数，而不是隐藏协议开关。 |
| 不暴露版本选择标记 | PASS | 测试明确拒绝在结果信封中泄漏 contract_version。 |
| fixture-backed active 结果语义 | PASS | rc=0 |

结论：**5/5 PASS**。`chat +flag-list --format json` 已是 active 统一结果试点：续页只声明 endpoint 可续，边界未知不声明耗尽，后续页失败保留成功页并返回 `partial_failure`/rc=7。

未验证：真实账号的收藏列表召回、网关响应形状和索引健康；这些仍需评测账号复验。
