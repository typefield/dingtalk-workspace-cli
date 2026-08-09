# AITable Base 发现 unified-active — Agent review

扫描时间：2026-08-09T21:26:19+08:00

> 本审阅读取实际 Skill 语料、当前 CLI Help 和 AITable 回归，只保存 Markdown，不是 CI / policy gate，也不保存 JSON fixture。

## Result: PASS

- native Agent 路由 `dws aitable base list/search`：**0** 处
- shortcut catalog 提及（非工作流）：**3** 处
- active shortcut Agent 路由 `dws aitable +base-list/+base-search`：**6** 处
- AITable 焦点测试退出码：`0`
- `+base-search --help` 声明必填 `--query`：`true`
- `+base-list --help` 声明 `--limit/--cursor`：`true`

## 当前路由规则

有名称关键词时：

```sh
dws aitable +base-search --query "<名称>" --format json
```

只浏览最近访问时：

```sh
dws aitable +base-list --format json
```

二者都不是全量 Base 目录；`+base-search` 的零候选也不证明业务上不存在。没有可信候选时请求 URL 或 baseId，不得臆造标识符。Agent 只按顶层 `ok/outcome`、`data` 和 `meta.pagination` 分支；不再解析 historical 裸 payload。

## Native route hits

无

## Active shortcut route hits

`skills/mono/references/products/aitable.md:357`<br>`skills/mono/references/products/aitable/aitable-form.md:86`<br>`skills/multi/dingtalk-aitable/SKILL.md:147`<br>`skills/multi/dingtalk-aitable/SKILL.md:148`<br>`skills/multi/dingtalk-aitable/references/aitable.md:432`<br>`skills/multi/dingtalk-aitable/references/aitable/aitable-form.md:86`

## Catalog-only mentions

`skills/multi/dingtalk-aitable/SKILL.md:38`<br>`skills/multi/dingtalk-aitable/SKILL.md:40`<br>`skills/multi/dingtalk-aitable/SKILL.md:126`

## Focused test transcript

```text
ok  	github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut/aitable	0.562s
```

## Boundary

本审阅验证本地路由、Help 和投影契约；不证明最近访问列表的召回率、搜索索引健康、死条目治理或真实租户权限。
