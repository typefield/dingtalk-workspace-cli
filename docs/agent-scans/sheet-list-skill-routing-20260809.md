# Sheet 工作表列表 Skill 路由 — Agent review

扫描时间：2026-08-09T15:39:04+08:00

> 本扫描由 Agent 在当前工作树执行，审阅实际 Skill 指令和本地 Sheet 回归；它只输出 Markdown，不是 CI / policy gate，也不保存 JSON fixture。

## Result: PASS

- 正向 `dws sheet +list-sheets` 示例：**97** 处，分布在 **42** 个文件
- 仍教学 legacy `dws sheet list` 的示例：**0** 处
- Sheet 焦点测试退出码：`0`

## Routing rule

当 Agent 需要取得真实 `sheetId` 供后续读写时，使用：

```sh
dws sheet +list-sheets --node <NODE_ID_OR_URL> --format json
```

旧 `sheet list` 保持 CLI 兼容，但 Skill 不再把它作为正向 Agent 路径。新路径是已审阅的统一结果入口：只接受明确 sheet ID、未知响应 fail-closed，且不伪造分页终态。

## Legacy hits

无

## Focused test transcript

```text
ok  	github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut/sheet	0.367s
```

## Boundary

这验证的是 Skill 指令与本地统一输出入口的一致性；不证明真实表格权限、服务端响应形状或写后效果。
