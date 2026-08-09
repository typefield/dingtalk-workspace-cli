# MCP 服务端失败恢复 Agent 扫描

扫描日期：2026-08-09

> Agent 在当前工作树执行；它仅验证 runtime 分类器和本地 HTTP fixture，不调用真实 DingTalk，也不保存运行时 JSON fixture。

| 检查 | 结果 | 证据 |
|---|---|---|
| 标准 tools/call MCP / 业务失败进入统一分类器 | PASS | MCP isError 与成功传输中的业务 error 不能退回自由文本错误。 |
| 仅 queryToolMeta 前置失败允许安全重试 | PASS | 已知业务工具尚未执行时，服务端 retryable 才是可采纳的恢复建议。 |
| 不确定网络失败不把服务端 retryable 升格为安全重放 | PASS | 执行状态未知时保留诊断，但让 Agent 先核对，避免重复写入。 |
| 本地 fixture 覆盖两类恢复分支 | PASS | 同时验证 metadata retry 与 ambiguous write 的 no-retry 语义。 |
| fixture-backed recovery semantics | PASS | rc=0 |

结论：**5/5 PASS**。MCP metadata lookup 已明确失败且未执行业务工具时，Agent 可按 `retryable:true` 重试；其他网络错误即使服务端提示 retryable，仍视为业务效果未知，必须先核对目标状态。

未验证：真实网关的每个 error code 是否准确标注前置/执行中阶段，以及真实写请求的最终效果；这些需要隔离账号与故障注入环境复验。
