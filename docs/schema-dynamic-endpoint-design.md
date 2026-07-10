# DWS Schema 功能设计文档（静态端点架构上的动态 Schema）

> 分支：`feat/schema-on-main`（基于 upstream `main` / v1.0.52 静态端点架构）
>
> 目标：在 upstream 已移除服务发现的静态端点架构上，重新为 `dws schema` 提供动态 Schema 能力。

## 1. 背景

upstream `main`（v1.0.52）将 CLI 切换为"静态端点模式（static endpoint mode）"，移除了服务发现与动态 Schema 生成体系：删除了 `internal/discovery`、`internal/ir`、`internal/generator`、`internal/compat`、`internal/cache`、`internal/market` 等包，`dws schema` 被降级为空壳 stub，仅返回 `{"kind":"schema","count":0,"products":[],"note":"static endpoint mode"}`，只保留 helper-only 子树查询。

本次工作在保留 upstream 静态端点架构（不恢复服务发现）的前提下，为 `dws schema` 重新实现动态 Schema 能力。

## 2. 设计原则

- 不恢复运行时服务发现（`tools/list`）；Schema 查询不发起网络请求。
- Schema 从**实际 Cobra 命令树**动态构建，反映当前二进制真实的产品、命令、flag、参数——与静态端点理念一致（事实来自 binary 本身）。
- 版本内嵌的 Agent/接口元数据作为语义补充，按命令路径匹配附加。
- 复用 upstream 保留的 helper-only 子树 Schema 能力。

## 3. 架构与数据流

```
dws schema [path]
  -> NewSchemaCommand (internal/cli/canonical.go)
     -> (helper-only 子树) renderHelperSchema(ctx, root, path, fetcher)
     -> runtimeSchemaPayload(root, args)         # 遍历实时 Cobra 命令树
        -> collectRuntimeSchemaEntries + walkLeafCommands
        -> 附加内嵌 Agent 元数据 (schema_agent_metadata/*.json)
        -> 附加内嵌接口元数据 (schema_mcp_metadata.json)
```

关键文件（运行时）：

- `internal/cli/canonical.go`：`NewSchemaCommand` 入口，从 stub 改为调用 `runtimeSchemaPayload`。
- `internal/cli/runtime_schema.go`：遍历实时命令树生成 Schema（总览 / 分组 / leaf detail）。
- `internal/cli/schema_catalog.go`：内嵌 Catalog 读取与构建期快照 `BuildSchemaCatalogSnapshot`。
- `internal/cli/schema_agent_metadata.go` + `schema_agent_metadata/*.json`：内嵌 Agent 语义元数据（21 产品 / 504 工具摘要）。
- `internal/cli/schema_hints*.go`：强类型参数/风险 Hint。
- `internal/cli/schema_support.go`：本次新增的自包含辅助（`walkLeafCommands`、`schemaCatalogToolCount`、`helperProductSummaries`）。
- `internal/ir/catalog.go`：保留 Catalog 数据结构；移除依赖已删 `internal/discovery` 的构建期函数 `BuildCatalog`。

## 4. 与 upstream 的适配点

1. `internal/ir/catalog.go`：删除 `BuildCatalog`（唯一依赖 `internal/discovery` 的构建期函数）与随之无用的 `sort` import；运行时数据结构全部保留。
2. `internal/cli/canonical.go`：保留 upstream 的 `NewSchemaCommand` 签名与 helper 分支；将末尾 stub 输出替换为 `runtimeSchemaPayload` 动态输出，并更新命令描述。
3. `internal/cli/dev_schema.go` / `schema_validate.go`：沿用 upstream 版本（提供 `renderHelperSchema` 4 参签名、`kebabCase`/`mcpJSONType`/`mcpDefault` 等辅助），避免与 upstream 重复声明。
4. `internal/cli/schema_catalog.go`：`schemaDetailFromRoot` 统一走 `runtimeSchemaPayload`，适配 upstream helper 渲染签名差异。

## 5. 测试与验证

- `go build ./...`：全量编译通过。
- `go test ./...`：44 个包通过；仅 `test/scripts` 的 `TestPostGoreleaser*` 3 个用例失败，原因为 macOS ad-hoc 签名阶段 `tar: Unrecognized archive format`，属 upstream 固有打包环境问题，与 Schema 改动无关（`package_script_test.go` 不涉及 schema）。
- 运行验证（总览）：`dws schema` 输出实时命令树共 14 产品 / 314 工具（如 aitable 114、sheet 69、attendance 38）。
- 运行验证（leaf detail）：`dws schema "calendar attendee delete"` 返回 `canonical_path=calendar.remove_calendar_participant`、`cli_path=calendar attendee delete`、`parameter_count=1`（`attendees`）、`effect=destructive`、`risk=high`、`confirmation=user_required`、`agent_summary=移除参与者`——证明参数取自实时命令树，且 Agent 语义（含 destructive 安全标注）按 canonical path 精确附着。`sheet delete-sheet` 同样返回 destructive/high/user_required。

## 6. 已知限制与后续工作

- **摘要头口径**：总览中的 `agent_metadata` / `interface_metadata` 块是**内嵌语义数据源的覆盖 provenance**（历史生成口径 21 产品 / 504 工具），与当前实时命令树的工具数（14 产品 / 314 工具，见顶层 `count` 与各产品 `tool_count`）是不同口径。二者并存不影响 leaf detail 查询的正确性（参数取自实时树，元数据按 canonical path 匹配附着）。若需两者口径统一，可在 upstream 命令树上重新生成内嵌 `schema_catalog.json` 与 `schema_agent_metadata/*.json`（见下）。
- **构建期生成器**：`internal/generator/*`（`cmd_schema_catalog`、`cmd_schema_agent_metadata`）依赖已删包，本次未移植；重新生成内嵌数据的能力作为后续工作。当前运行时功能不依赖它（直接遍历实时树 + 读内嵌摘要）。
- **helper-only 子树**：沿用 upstream 能力，未做增强。
