# Dual-validate Legacy 输出兼容 Agent 审阅

扫描日期：2026-08-09

> 本记录由 Agent 在当前工作树以受控 MCP caller 执行。它只审阅 dual-validation 的
> 本地“单次调用 + legacy 字节不变”不变量；不接入 CI/policy，不保存运行 JSON fixture，
> 也不能证明真实账号、服务端终态或写入副作用。

## 审阅对象

`RuntimeContext.CallMCP` 的 `dual_validate` 分支曾在 shadow result 校验后改用
`output.WriteCommandPayload`。该 writer 属于 unified formatter，会禁用 legacy JSON 的
HTML escaping；例如 legacy `PrintJSON` 的 `&`、`<`、`>` 会分别编码为
`\u0026`、`\u003c`、`\u003e`。这会让尚未切 active 的命令在同一 argv 下发生 wire
漂移。

当前实现改为：MCP 只调用一次，先构造并校验 shadow `CommandResult`，再使用
`WriteMCPToolPayloadLegacy` 按原 formatter 写入。该兼容 seam 保留 legacy 的 JSON
escaping、mail/AITable JSON 归一化、`--fields/--jq`、表格/原文、空 text 与无 text
block 回退。

## 当前受控结果

执行：

```bash
DWS_PACKAGE_VERSION=0.0.0-test go test -count=1 -v ./internal/shortcut \
  -run 'TestDualValidate(StructuredJSONIsByteIdenticalToLegacy|NoTextResponseUsesLegacyFallback|EmptyTextResponseUsesLegacyRawFallback|PreservesLegacy)'

DWS_PACKAGE_VERSION=0.0.0-test go test -count=1 -v ./internal/helpers \
  -run 'Test(CrossPlatformCoverageMCP(ReturnTextClassification|OutputModesAndDevdocFormatting)|MailJSONOutputNormalizesSuccessStringBooleans|AitableJSONOutputPublishesNonAuthoritativeDiscoveryBoundary)'
```

| 审阅项 | 结果 | 观察 |
|---|---|---|
| raw JSON | PASS | dual 保持 raw 结果，单次 MCP 调用 |
| plain/table 文本 | PASS | 不被 JSON quoting |
| dry-run | PASS | 使用 legacy 预览，MCP 调用数为 0 |
| 含 `&<>` 的 JSON | PASS | legacy/dual stdout 与 stderr 逐字相同，确认保持 HTML escaping |
| 无 text block | PASS | 两路径均回退为 legacy `ToolResult` JSON |
| 空 text block | PASS | 两路径均保留 legacy 原文换行 |
| 业务错误分类 | PASS | gateway、auth、PAT、`success:false`、`error` 等仍在输出前分类 |
| 既有 JSON 归一化 | PASS | mail `success` 字符串布尔与 AITable discovery boundary 保持原行为 |

## 边界与后续

- 这是 Generic `CallMCP` passthrough 的兼容证据；复合 shortcut 的 `Output` 路径仍需按命令
  独立 golden 审阅。
- 成功的本地 renderer 对拍不等于真实服务端返回形状、分页完整性或终态写入已被验证。
- `dual_validate` 不应因为此报告自动晋级；发布前仍需从 live Cobra declaration 导出 rollout
  ledger，并审阅命令级风险、Schema、真实账号样本和回滚窗口。
