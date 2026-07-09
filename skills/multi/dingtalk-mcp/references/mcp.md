# MCP 服务与工具管理

命令入口：

```bash
dws connector mcp --help
dws connector mcp service --help
dws connector mcp tool --help
dws connector mcp url --help
```

## 领域模型

这套能力分成两层，不要混用：

- `mcpdev` 管理面：`dws connector mcp service/tool/url ...`，用于创建、编辑、调试、发布 MCP 服务和工具。
- 已发布 MCP 调用面：服务发布后，DWS 通过发现接口或真实接入地址的 `tools/list` 生成固定 DWS 动态命令，同时保留 `dws connector mcp published <service-or-tool> <tool>` 调试路径。

| 对象 | 主键 | 说明 |
|------|------|------|
| MCP 服务 Service | `mcpId` | 工具容器，承载服务名称、描述、图标、详情介绍和接入地址 |
| MCP 工具 Tool | `actionId` | 服务下的单个工具，包含工具身份、HTTP 定义、LLM 入参出参和映射规则 |
| 工具版本 Version | `versionId` | 草稿、发布版或历史版本；调试草稿、回滚验证时必须明确版本 |
| 工具定义 Definition | 无独立主键 | 三段式定义：`apiInputs/apiOutputs`、`toolInputs/toolOutputs`、`inputMappings/outputMappings` |
| 调试记录 Debug | 无稳定主键 | 用一组 `value` 真实执行工具；写操作必须用测试数据 |
| 接入地址 Endpoint | `mcpURL` / `mcpJSON` | 客户端配置入口，可能含敏感 `?key=`；只脱敏展示 |
| DWS 动态命令 | 命令路径 | 由已发布 MCP 的 `tools/list` 投影成 `dws <service-or-tool> <tool>`；同时保留 `dws connector mcp published <service-or-tool> <tool>` |

对象关系：

```text
MCP Service (mcpId)
├── service metadata
├── Endpoint(PUBLISHED / MARKET)
└── Tool(actionId)
    ├── identity: name / title / description / status
    ├── HTTP adapter: method / url / auth
    ├── Definition
    │   ├── apiInputs / apiOutputs       # 真实 HTTP 参数和响应
    │   ├── toolInputs / toolOutputs     # 暴露给 LLM 的参数和响应
    │   └── inputMappings / outputMappings
    └── Version(versionId, versionNo, status)
```

工具生命周期：

```text
service create
  → tool create/update
  → draft
  → tool debug --version-id <draftVersionId>
  → tool publish
  → published
  → url get --source PUBLISHED
  → connector mcp refresh
  → <service-or-tool> <tool>
  → connector mcp published <service-or-tool> <tool>
```

`published_with_draft` 是最容易误判的状态：它表示线上仍是旧发布版，同时有一份新草稿。不传 `--version-id` 调试时会命中线上旧版本；要验证正在编辑的内容，必须从 `tool get` 取草稿 `versionId` 后传给 `tool debug`。

### 三段式工具定义

`tool create` / `tool update` 用同一套三段式入参。`tool update` 是全量提交语义，漏传字段等于清空。

| 段 | 作用 | 例子 |
|----|------|------|
| `apiInputs` / `apiOutputs` | HTTP 接口真实参数和响应字段 | `query.wd`、`headers.User-Agent`、`body.items` |
| `toolInputs` / `toolOutputs` | 暴露给 LLM 的参数和响应字段，可裁剪、改名、防呆 | `keyword`、`city`、`raw` |
| `inputMappings` / `outputMappings` | LLM 字段和 HTTP 字段之间的映射 | `$.node_start.keyword` → `$.Query.wd` |

字段结构统一用 `{key,title,type,required,description,children}`。`array` 的 `children` 固定为一项 `key="items"`。

映射规则：

- `reference`：引用工具入参或系统参数，例如 `$.node_start.keyword`。
- `fixed`：固定常量，例如把 `$.Query.prod` 固定为 `pc`。
- `express`：表达式映射，只有确实需要转换时使用。
- target 分组用 Pascal：`$.Head.*`、`$.Query.*`、`$.Body.*`、`$.Path.*`。不要写 `$.query.*` 或 `$.QUERY.*`。
- `corpId` / `userId` 等调用者身份由系统注入，不要在映射里额外传。

### 发现与动态命令

动态命令不是 mcpdev 管理命令。它来自已发布 MCP 的 `tools/list`，由 DWS 投影成固定命令：

```text
dws <service-or-tool-slug> <tool-slug> [flags]
```

一级命令命名优先级：`serverName` → MCP 服务 `name` → 工具 `name`。当前发现接口尚未稳定返回 `serverName` 时，先用 MCP 服务名；如果服务名也缺失，再退到工具名。DWS 同时保留调试路径：

```text
dws connector mcp published <service-or-tool-slug> <tool-slug> [flags]
```

刷新命令：

```bash
dws connector mcp refresh --format json
dws <service-or-tool-slug> --help
dws connector mcp published --help
```

缓存规则：

- DWS 会缓存已发布 MCP 的工具描述，TTL 10 分钟。
- `refresh` 会主动拉取预发/线上 Portal 发现接口并重建缓存。
- 发布或更新工具后，可以立即 `refresh`，再看 `published --help`。
- 不要根据 `mcpId` 自行猜 `/server/org-{mcpId}`；应使用发现接口返回的 `mcpUrl`，或 `url get --source PUBLISHED` 返回的真实接入地址。

## Shortcut

### 从零创建并发布一个只读 HTTP 工具

```text
1. service create --dry-run
2. service create --yes
3. tool create --dry-run
4. tool create --yes
5. tool get 或 tool list 取 actionId/versionId
6. tool debug --version-id <草稿versionId> --dry-run
7. tool debug --version-id <草稿versionId> --yes
8. tool publish --dry-run
9. 用户明确确认“发布后使用方可调用”
10. tool publish --yes
11. tool get 回读状态
12. url get --source PUBLISHED 获取接入地址
13. connector mcp refresh --format json
14. <service-or-tool> --help 和 connector mcp published --help 确认动态命令出现
```

### 只更新一个已有工具草稿

```text
1. tool list --mcp-id <mcpId> 找 actionId
2. tool get --mcp-id <mcpId> --action-id <actionId> 读取当前定义和草稿 versionId
3. tool update --dry-run（全量提交，别漏字段）
4. tool update --yes
5. tool get 取新草稿 versionId
6. tool debug --version-id <草稿versionId> --yes
7. 用户确认后 tool publish --yes
```

注意：第 2 步读回的是平台底层存储结构，不能直接原样回填 `tool update`；要翻译回三段式参数后全量提交。

### 只验证线上工具

```text
1. tool list --mcp-id <mcpId> 找 actionId
2. tool debug --mcp-id <mcpId> --action-id <actionId> --value '{}' --dry-run
3. 确认后 tool debug --mcp-id <mcpId> --action-id <actionId> --value '{}' --yes
```

线上验证可以不传 `--version-id`；草稿验证必须传。

### 发布后生成 DWS 动态命令

```text
1. tool publish --yes 成功
2. url get --mcp-id <mcpId> --source PUBLISHED 确认服务已有真实接入地址
3. connector mcp refresh --format json
4. <service-or-tool> --help 和 connector mcp published --help
5. 优先使用 <service-or-tool> <tool> --format json 调用；必要时使用 connector mcp published <service-or-tool> <tool> --format json 调试
```

如果 `refresh` 返回 `count=0` 或 `published --help` 没有子命令，先确认 Portal 发现接口是否返回该企业已发布 MCP；不要改用猜测的 `org-{mcpId}` 地址。

### 找回接入地址

```text
1. service list --keyword <服务名关键词> 找 mcpId
2. url get --mcp-id <mcpId> --source PUBLISHED
3. 若服务已上架市场，改用 --source MARKET
```

返回值含 `?key=`，回答里只说“已获取”或脱敏展示。

### 跨会话续作

```text
1. service list --keyword <服务名关键词> --format json
2. tool list --mcp-id <mcpId> --page-size 100 --format json
3. tool get --mcp-id <mcpId> --action-id <actionId> --format json
4. 根据 status 决定调试草稿、验证线上、发布或更新
```

禁止凭记忆使用 `mcpId` / `actionId` / `versionId`。

### 故障定位

```text
1. schema connector.mcp.<leaf> 看当前 MCP tools/list 暴露的参数说明
2. 目标命令 --help 看当前二进制 flag
3. 加 --dry-run 看实际 tools/call 参数
4. 加 --verbose 重试一次获取 trace_id / technical_detail
5. service get / tool get / tool versions 回读平台状态
```

常见定位结论：

- `mcp_not_found`：当前 mcpdev endpoint 或当前登录组织下没有该 `mcpId`。
- `no_draft_to_publish`：没有可发布草稿，先 `tool update` 或确认 actionId。
- 调试通过但发布后调用旧逻辑：调试时漏传草稿 `versionId`，实际测了线上旧版本。
- 动态命令没出现：发现接口未返回 MCP，或接入地址 `tools/list` 不可访问。

## 服务

```bash
# 列服务 / 查服务
dws connector mcp service list --keyword <关键词> --format json
dws connector mcp service get --mcp-id <mcpId> --format json

# 创建 / 修改 / 删除服务
dws connector mcp service create --name <服务名> --description <描述> --dry-run --format json
dws connector mcp service update --mcp-id <mcpId> --description <新描述> --dry-run --format json
dws connector mcp service delete --mcp-id <mcpId> --dry-run --format json
```

服务名用业务语义命名且组织内唯一，不要用 test、临时 等占位名。删除服务不可恢复，删除前先 `service get` 和 `tool list` 核对；服务下还有工具时会被拒绝。

## 工具

```bash
# 列工具 / 读工具 / 版本历史
dws connector mcp tool list --mcp-id <mcpId> --page-size 100 --format json
dws connector mcp tool get --mcp-id <mcpId> --action-id <actionId> --format json
dws connector mcp tool versions --mcp-id <mcpId> --action-id <actionId> --format json

# 创建 / 更新工具：复杂字段使用 JSON 字符串
dws connector mcp tool create --mcp-id <mcpId> --name <snake_case_name> --title <标题> --description <描述> --http '{"method":"GET","url":"https://example.com","auth":{"type":"NO_AUTH"}}' --dry-run --format json
dws connector mcp tool update --mcp-id <mcpId> --action-id <actionId> --name <snake_case_name> --title <标题> --description <描述> --http '{"method":"GET","url":"https://example.com","auth":{"type":"NO_AUTH"}}' --dry-run --format json

# 调试 / 发布 / 删除
dws connector mcp tool debug --mcp-id <mcpId> --action-id <actionId> --version-id <versionId> --value '{}' --dry-run --format json
dws connector mcp tool publish --mcp-id <mcpId> --action-id <actionId> --dry-run --format json
dws connector mcp tool delete --mcp-id <mcpId> --action-id <actionId> --dry-run --format json
```

工具定义是三段式：

- `apiInputs` / `apiOutputs`：HTTP 接口真实入参和出参。
- `toolInputs` / `toolOutputs`：暴露给 LLM 的入参和出参，可以裁剪、改名、写防呆描述。
- `inputMappings` / `outputMappings`：工具字段与真实接口字段的映射。
- 创建/更新工具前，把 `name` / `title` / `description` 和 `toolInputs` 展示给用户复核。

复杂字段直接传 JSON：

| flag | 类型 | MCP 入参 |
|------|------|----------|
| `--http` | object | `http` |
| `--api-inputs` | object | `apiInputs` |
| `--api-outputs` | object | `apiOutputs` |
| `--tool-inputs` | array | `toolInputs` |
| `--tool-outputs` | array | `toolOutputs` |
| `--input-mappings` | array | `inputMappings` |
| `--output-mappings` | array | `outputMappings` |
| `--value` | object | `value` |

## 调试与发布

- `tool debug` 会真实执行目标接口；写接口必须用测试数据。
- 不传 `--version-id` 时，如果工具曾发布过，会调已发布版本，不会自动调最新草稿。
- 要调试正在编辑的草稿，必须从 `tool get` 取草稿 `versionId` 后显式传 `--version-id`。
- 发布前必须至少调试通过一次。
- 发布前向用户复述工具名与作用，并明确说明发布后使用方可调用；用户明确同意后再加 `--yes`。
- 发布后立即 `tool get` 回读 `status`，必要时 `tool debug` 不传 `versionId` 验证线上版本。

## 接入地址

```bash
dws connector mcp url get --mcp-id <mcpId> --source PUBLISHED --format json
dws connector mcp url get --mcp-id <mcpId> --source MARKET --format json
```

- 新建服务发布后用于验证通常传 `--source PUBLISHED`。
- 已上架市场传 `--source MARKET`。
- 返回的 URL 或 JSON config 含 `?key=`，按敏感凭证处理，只能给当前用户本地配置使用。
- 代码或命令需要接入地址时优先使用 `url get` 或发现接口返回值，不要自行拼接服务 URL。

## Schema

helper-only 命令需要确认 JSON 参数结构时用：

```bash
dws schema connector.mcp.service.create --format json
dws schema connector.mcp.tool.create --format json
dws schema connector.mcp.tool.update --format json
dws schema connector.mcp.tool.debug --format json
```
