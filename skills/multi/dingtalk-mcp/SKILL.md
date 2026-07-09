---
name: dingtalk-mcp
description: 钉钉 MCP 开发平台服务与工具管理：MCP 服务创建/查询/更新/删除、工具创建/读取/更新/调试/发布/删除/版本历史、获取 MCP 接入地址。Use when 用户说 MCP服务/MCP工具/MCP开发脚手架/MCP发布/MCP调试/获取MCP地址/mcpId/actionId/versionId/mcp_tool/mcp_service。命令前缀：dws connector mcp。
cli_version: "1.0.40+"
metadata:
  category: product
  stability: experimental
  requires:
    bins:
      - dws
---

# 钉钉 MCP 开发管理 Skill

> 🧪 **EXPERIMENTAL · 试验版 / Preview** — multi 模式当前未达 stable 标准；接口、命名、跨 skill 引用后续可能调整。生产 / 共享环境请优先使用 mono 模式（`dws skill setup --mode mono`）。

> **PREREQUISITE:** Read the `dws-shared` skill first for auth, global flags, product routing, URL preflight, error codes, and safety rules. The `dws` binary must be on PATH.

<!-- SAFETY_PREAMBLE_INJECT -->

> ⚠️ **命令可用性以当前 dws 二进制为准**。服务发现已下线，本文档随内置 skill 发布；如果 `dws <cmd> --help` 不存在，说明当前版本未暴露该命令。若命令存在但调用失败，请按错误中的 endpoint 或 tool 提示确认静态端点目录和后端工具注册。实际调用前可用 `dws <cmd> --help` 或 `--dry-run` 验证。

## MUST DO

1. 执行前先看 `dws connector mcp --help`、目标分组 `--help`、叶子命令 `--help`，按当前二进制 flag 构造命令。
2. 所有命令加 `--format json`。
3. 创建、更新、删除、调试、发布都先 `--dry-run --format json` 预览，再在用户明确确认后加 `--yes --format json`。
4. `mcpId`、`actionId`、`versionId` 必须来自 `service list/get`、`tool list/get`、`tool versions` 或上游命令返回，禁止猜。
5. 发布前必须先 `tool debug` 调试通过；调试草稿必须显式传草稿 `--version-id`。
6. 接入地址里的 `?key=` 是敏感凭证，不写入文档、代码、日志或回答全文。
7. 删除服务前必须先 `service get` + `tool list` 核对；服务下还有工具时先逐个删除工具。

## 领域模型

```
MCP 开发脚手架（mcpdev 管理面）
├── MCP 服务 Service（mcpId）
│   ├── 服务元信息：name / description / icon / introduction
│   ├── 接入地址：mcpURL / mcpJSON（PUBLISHED / MARKET，可能含敏感 ?key=）
│   └── MCP 工具 Tool（actionId）
│       ├── 身份字段：name / title / description / status
│       ├── HTTP 适配：method / url / auth
│       ├── 三段式定义：apiInputs/apiOutputs + toolInputs/toolOutputs + mappings
│       └── 工具版本 Version（versionId / versionNo / status）
├── 调试 Debug：用 value 真实执行工具，草稿调试必须显式传 versionId
├── 发布 Publish：草稿转正，发布后使用方可调用
└── DWS 动态命令面：发现已发布 MCP 后生成 <service-or-tool> <tool>，同时保留 connector mcp published <service-or-tool> <tool> 调试路径
```

- 服务是工具容器，先建服务再建工具。
- `mcpId` 定位服务；`actionId` 定位工具；`versionId` 定位工具版本。
- 工具定义是三段式：真实 HTTP 入参出参、暴露给 LLM 的入参出参、两者之间的映射规则。
- 工具创建或更新后只是草稿，必须调试通过并发布后才对使用方生效；更新工具是全量提交语义，漏字段会被清空。
- `draft` 表示只有草稿；`published` 表示只有线上版本；`published_with_draft` 表示线上已有发布版，同时存在更新草稿。
- `tool debug` 不传 `versionId` 时，已发布工具默认调线上版本；调试草稿必须传草稿 `versionId`。
- DWS 动态命令来自已发布 MCP 的发现结果或接入地址的 `tools/list`，一级命令优先用 `serverName`，缺失时用 MCP 服务 `name`，再缺失时退到工具 `name`；不要凭 `mcpId` 手拼接入地址。

## Shortcut

按用户目标直接走下面的快捷方案；每一步仍要遵守 dry-run、确认和回读规则。

| 目标 | 快捷方案 |
|------|----------|
| 从零创建 MCP 服务 | `service create --dry-run` → 用户确认 → `service create --yes` → 记录返回 `mcpId` |
| 给服务新增 HTTP 工具 | `tool create --dry-run` → 用户确认 → `tool create --yes` → `tool list` 或 `tool get` 取 `actionId/versionId` |
| 验证草稿工具能跑 | `tool get` 取草稿 `versionId` → `tool debug --version-id <versionId> --dry-run` → 用户确认 → `tool debug --version-id <versionId> --yes` |
| 发布工具并可调用 | 确认最近一次 debug 成功 → `tool publish --dry-run` → 明确说明发布后使用方可调用 → 用户明确确认 → `tool publish --yes` → `tool get` 回读状态 |
| 获取客户端接入地址 | 已发布未上架用 `url get --source PUBLISHED`；已上架市场用 `url get --source MARKET`；输出中的 `?key=` 只脱敏展示 |
| 生成/刷新 DWS 动态命令 | 工具发布后执行 `dws connector mcp refresh --format json` → 检查 `dws <service-or-tool> --help` 和 `dws connector mcp published --help` → 优先使用 `dws <service-or-tool> <tool>`，必要时用 `dws connector mcp published <service-or-tool> <tool>` 调试 |
| 续作已有服务/工具 | `service list --keyword` 找 `mcpId` → `tool list --mcp-id` 找 `actionId` → 再执行目标操作 |
| 编辑已发布工具 | `tool get` 读当前定义 → 按三段式全量构造 `tool update --dry-run` → 调试草稿 versionId → 用户确认后发布 |
| 删除工具或服务 | 先 `tool get` 或 `service get` + `tool list` 核对影响面 → 用户明确确认 → 删除命令加 `--yes` |

## 意图表

| 用户说 | 命令 |
|--------|------|
| "列出 MCP 服务 / 找回 mcpId" | `dws connector mcp service list --keyword <关键词> --format json` |
| "查看 MCP 服务详情" | `dws connector mcp service get --mcp-id <mcpId> --format json` |
| "创建 MCP 服务" | `dws connector mcp service create --name <名称> --description <描述> --dry-run --format json` |
| "列出某服务工具 / 找回 actionId" | `dws connector mcp tool list --mcp-id <mcpId> --page-size 100 --format json` |
| "读取工具定义 / 找 versionId" | `dws connector mcp tool get --mcp-id <mcpId> --action-id <actionId> --format json` |
| "调试工具草稿" | `dws connector mcp tool debug --mcp-id <mcpId> --action-id <actionId> --version-id <versionId> --value '{}' --dry-run --format json` |
| "发布工具" | `dws connector mcp tool publish --mcp-id <mcpId> --action-id <actionId> --dry-run --format json` |
| "获取 MCP 接入地址" | `dws connector mcp url get --mcp-id <mcpId> --source PUBLISHED --format json` |

## 详细参考

按任务读取 [mcp.md](references/mcp.md)。
