# Drive `+list` dual-validate — Agent review

扫描时间：2026-08-09T16:08:54+08:00

> 本审阅读实际 Skill、当前 CLI Help 和 Drive 回归，只保存 Markdown；它不是 CI / policy gate，也不保存运行时 JSON、文件名或 dentry ID。

## Result: PASS

- 过早教学 shortcut 路由 `dws drive +list`：**0** 处
- catalog-only 提及：**0** 处
- Drive 焦点测试退出码：`0`
- `+list --help` 声明 `--folder/--cursor/--limit`：`true`

## 当前路由与边界

本轮 `drive +list` 仅进入 **dual_validate**：外部仍是既有 legacy payload，框架在内部严格验证下一版候选结果。Skill 不应将该 shortcut 教成新的默认 Agent 路由。

候选结果的边界是：它只列出**请求的 space/folder 的一页**。当服务端给出一致的 `hasMore + nextCursor` 时，可继续翻页；没有分页证据时必须标记为未知。无论哪种情况，都不能把结果扩大为“全部可访问钉盘文件”。跨目录按名称定位文件应使用公开的 `dws drive search --query "<关键词>" --format json`。

## Premature shortcut route hits

无

## Catalog-only mentions

无

## Focused test transcript

```text
ok  	github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut/drive	0.358s
```

## Boundary

本审证明的是本地 CLI projection、Help 与 Skill 路由一致性；不证明真实租户中的目录召回率、死条目治理、权限可见性或服务端分页正确性。后者需要独立的脱敏 live evidence 才可晋级 active。
