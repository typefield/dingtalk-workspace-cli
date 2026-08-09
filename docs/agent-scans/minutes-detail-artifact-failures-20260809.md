# Minutes 详情 artifact 失败语义 — Agent review

扫描时间：2026-08-09T17:07:55+08:00

> 本扫描由 Agent 在当前工作树运行。它结合源码关系与内存 Go 测试生成 Markdown 证据；不是 CI / policy gate，也不保存服务端响应或 JSON fixture。

## Result: PASS

- `minutes +detail` 当前为 unified active：**yes**
- 部分失败保留 child 的 category / subtype / hint / actions / retry guidance：**yes**
- 全部 artifact 失败时在 aggregate error.details 保留逐项 typed error：**yes**
- 焦点测试：`TestMinutesDetail(Result|PreservesTyped)`
- 测试退出码：`0`

## Required behavior

1. 已成功 artifact 与失败 artifact 同时存在时，必须输出 `partial_failure` / rc=7；`failed[]` 中的每项保留稳定 ID 和 typed error。
2. 子错误已经提供 auth、validation、projection 或 retry 指引时，聚合层不得重写成笼统的 `api + retryable:true`。
3. 纯读取的、未分类的临时错误可以保留可重试建议；明确 `retryable:false` 必须保持为不鼓励重放。
4. 全部 artifact 失败必须是普通 `failure`，但 error.details 要保留每个 artifact 的错误事实，不能只丢一个名称列表。
5. 该本地证据不证明真实听记任务的 artifact 可读性、权限或服务端终态。

## Focused test transcript

```text
=== RUN   TestMinutesDetailResultPreservesPartialArtifactFacts
--- PASS: TestMinutesDetailResultPreservesPartialArtifactFacts (0.00s)
=== RUN   TestMinutesDetailResultUsesSuccessOrFailureForTerminalCases
--- PASS: TestMinutesDetailResultUsesSuccessOrFailureForTerminalCases (0.00s)
=== RUN   TestMinutesDetailPreservesTypedArtifactFailureGuidance
--- PASS: TestMinutesDetailPreservesTypedArtifactFailureGuidance (0.00s)
PASS
ok  	github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut/smart	0.400s
```
