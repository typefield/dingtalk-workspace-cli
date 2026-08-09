# Drive `+list` unified-active — Agent review

扫描时间：2026-08-09T20:56:06+08:00

> 本审阅读实际 Skill、当前 CLI Help 和 Drive 回归，只保存 Markdown；它不是 CI / policy gate，也不保存运行时 JSON、文件名或 dentry ID。

## Result: PASS

- 正向教学 shortcut 路由 `dws drive +list`：**0** 处
- 路由表教学：**1** 处
- Drive 焦点测试退出码：`0`
- `+list --help` 声明 `--folder/--cursor/--limit`：`true`

## 当前路由与边界

`drive +list` 已进入 **unified_active**：普通 `--format json` 直接返回 `ok/outcome/data/meta`。Skill 可以把它作为指定位置目录浏览的默认 Agent 路由；不公开协议选择参数，也不输出版本标记。

结果边界是：它只列出**请求的 space/folder 的一页**。一致的 `hasMore + nextCursor` 或非空 token-only continuation 都可安全续页；token 缺失而没有显式终态布尔时必须标记为未知。无论哪种情况，都不能把结果扩大为“全部可访问钉盘文件”。跨目录按名称定位文件应使用 `dws drive +search --query "<关键词>" --format json`。

## Active shortcut route hits

无

## Route-table mentions

`skills/multi/dingtalk-drive/SKILL.md:41`

## Focused test transcript

```text
ok  	github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut/drive	0.317s
```

## Boundary

本审证明的是本地 CLI projection、Help 与 Skill 路由一致性；真实租户中的 token-only 续页形状由独立脱敏 live probe 记录。两者都不证明目录召回率、死条目治理、权限可见性或租户级目录完整。
