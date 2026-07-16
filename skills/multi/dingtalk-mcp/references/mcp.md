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
| MCP 工具 Tool | `toolId` | 服务下的单个工具（G-ACT- 开头；平台底层字段名 actionId，0714 契约起工具入参统一叫 toolId），包含工具身份、HTTP 定义、LLM 入参出参和映射规则 |
| 工具版本 Version | `versionId` | 草稿、发布版或历史版本；调试草稿、回滚验证时必须明确版本 |
| 工具定义 Definition | 无独立主键 | 三段式定义：`apiInputs/apiOutputs`、`toolInputs/toolOutputs`、`inputMappings/outputMappings` |
| 调试记录 Debug | 无稳定主键 | 用一组 `value` 真实执行工具；写操作必须用测试数据 |
| 接入地址 Endpoint | `mcpURL` / `mcpJSON` | 客户端配置入口，可能含敏感 `?key=`；只脱敏展示 |
| DWS 动态命令 | 命令路径 | 由已发布 MCP 的 `tools/list` 投影成 `dws <service-or-tool> <tool>`；同时保留 `dws connector mcp published <service-or-tool> <tool>` |

对象关系：

```text
MCP Service (mcpId)
├── service metadata
├── AuthConfig（下游鉴权说明书，不含凭证值）
├── Credential（凭证账号，密钥不回显）
├── Member（开发协作者 staffId）
├── Endpoint(PUBLISHED / MARKET)
└── Tool(toolId)
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
  → tool debug --version-id <draftVersionId>   # 真跑验证，返回真实业务数据才算通过
  → tool publish                                # publish = 企业内可用（≠ 上架市场）
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

字段结构统一用 `{key,title,type,required,description,children}`（type：string/number/boolean/object/array；object/array 的子结构放 `children` 递归；**array 字段的 items 不能为空**）。

- `toolInputs` 的 `key` 对应 `inputMappings.source`（`$.node_start.<key>`）；`apiInputs` 的字段位置对应 `inputMappings.target`（`$.<位置>.<key>`）。
- **平台不支持 enum/default/example 等 JSON Schema 标准属性**——枚举、默认值、示例一律写进字段 `description` 文本。
- create/update 必填集（V4 起）：`mcpId / name / title / description / --http-info / --api-inputs / --tool-inputs / --input-mappings` 全部必填（required 是 schema 语义约束，平台运行时不强拦缺失，但必须照填）；`toolType` 参数已移除（多传会被静默忽略）；旧键 `http` 仍报 `business_error_invalid_params`。`outputMappings` 建议显式配置（省略/传 []＝整包响应体外多包一层 Body，见下）。
- 工具 `description` 有 DB 列长限制（约 700 字符）：`tool create/update` 草稿不报错，**`tool publish` 才报 Data too long**——描述超长要在发布前收敛。

映射规则的完整格式（JSONPath 写法、Pascal 位置名、reference/fixed/express 三型、出参透传、系统参数、数组双规则）见 **[mapping-rules.md](mapping-rules.md)**，写 `--input-mappings` / `--output-mappings` 前必读。速记三条最大的坑：

- target 位置名必须 Pascal（`$.Query.*` / `$.Body.*`），全大写/全小写**静默失效**；
- `express` 类型的表达式必须放 `expression` 字段（不是 `source`），放错被静默存成 `{}`；
- `outputMappings` 显式二选一：整体透传 `{"type":"reference","source":"$.node_service_activator.Body","target":"$"}`，或字段级精修（裁剪/改名/[*]/嵌套/系统变量，见 mapping-rules.md §5）；⚠️省略或传 `[]`＝工具仍建成、运行时返回 `{"Body":{整包响应体}}` 包壳（不报错也不是返回空），不推荐。⚠️出参 rules 的 source 必须在 apiOutputs 声明范围内，否则 UI 标「变量已失效」（红线#13）。
- `corpId` / `userId` 等调用者身份由系统参数注入（`$.system_node.*`），不要做成 toolInput 让 LLM 传。

### 从 API 材料到工具定义

用户给 API 材料（OpenAPI/Postman/curl/文档）要求「做成 MCP / 给 agent 用」时，按 **[api-to-tool.md](api-to-tool.md)** 拆解：信息对齐（材料/业务目标/鉴权，缺就问）→ 按材料类型提取三段式 → 工具侧加工（裁剪/改名/约束写 desc/身份走注入）→ 一个语义动作一个工具 → 设计整表给用户过目再动手建。只读接口建议先真跑一次取真实响应（反推 apiOutputs + 生成 debug 测试入参）。

### 发现与动态命令

动态命令不是 mcpdev 管理命令。它来自已发布 MCP 的 `tools/list`，由 DWS 投影成固定命令：

```text
dws <service-or-tool-slug> <tool-slug> [flags]
```

一级命令命名优先级：合法 ASCII `serverName` → `mcp-<mcpId>` → 工具 `name`。中文服务名和非法 `serverName` 不进入命令路径；旧缓存中的中文一级路径会自动迁移为 `mcp-<mcpId>`。DWS 同时保留调试路径：

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
- `refresh` 会主动拉取预发/线上 Portal 发现接口并重建缓存；`--timeout` 同时作为发现请求和单个 MCP `tools/list` 的超时。
- MCP 工具发现使用有限并发和独立超时：单个服务失败不会取消后续服务；健康服务更新，失败服务保留上次缓存。
- 返回中的 `partial` 表示部分成功，`failedServices` 给出失败的 mcpId/脱敏端点/原因，`cacheUpdated` 表示合并后的缓存已持久化。
- 发布或更新工具后**最迟 10 分钟（缓存 TTL）自动生效**——「立即出现」只是缓存恰好过期的巧合，不是保证；需要立即可用就执行 `refresh`，再看 `published --help`。
- `service list` 返回顶层 `services[].serverName`（V4 起分页平铺，无 result 信封）；它是 kebab-case CLI 一级命令名，未设置时为空。动态发现必须优先使用该字段，不能始终从中文服务名推导。
- 所有刷新只使用发现接口返回的真实 hash `mcpUrl`；不会根据 `mcpId` 猜 `/server/org-{mcpId}`。需要找回地址时使用 `url get --source PUBLISHED`。

指定接入地址的只读探测：

```bash
DINGTALK_MCPDEV_MCP_URL='<含凭证的 MCP 地址>' dws connector mcp inspect --format json
```

`inspect` 依次执行 `initialize`、`notifications/initialized`、`tools/list`，返回协商协议版本、服务能力和完整工具 Schema，不调用任何业务工具。优先通过环境变量传入含 `?key=` 的地址，避免凭证出现在命令参数中；输出中的地址会自动脱敏。

### 鉴权、凭证与成员

```bash
# 服务级下游鉴权配置
dws connector mcp auth get --mcp-id <mcpId> --format json
dws connector mcp auth save --mcp-id <mcpId> --auth-type NO_AUTH --dry-run --format json
dws connector mcp auth save --mcp-id <mcpId> --auth-type TOKEN --token-auth-config '<JSON>' --dry-run --format json

# 凭证账号：密钥优先从文件或 stdin 读取，避免进入 shell history
dws connector mcp credential list --mcp-id <mcpId> --format json
dws connector mcp credential get --mcp-id <mcpId> --credential-id <id> --format json
dws connector mcp credential save --mcp-id <mcpId> --name <账号名> --content-file credentials.json --dry-run --format json
dws connector mcp credential debug --mcp-id <mcpId> --credential-id <id> --dry-run --format json
dws connector mcp credential bind --mcp-id <mcpId> --credential-id <id> --dry-run --format json
dws connector mcp credential delete --mcp-id <mcpId> --credential-id <id> --dry-run --format json

# 开发协作者，--user-ids 传 staffId，不传姓名
dws connector mcp member list --mcp-id <mcpId> --format json
dws connector mcp member add --mcp-id <mcpId> --user-ids <staffId1,staffId2> --dry-run --format json
dws connector mcp member remove --mcp-id <mcpId> --user-ids <staffId1,staffId2> --dry-run --format json
```

- `authType` 仅支持 `NO_AUTH`、`BASIC`、`API_SECRET`、`TOKEN`、`SIGNATURE`；只传当前类型对应的配置对象。auth save 存的是「说明书」（authFields 字段定义、换取与注入方式），不含密钥值。
- TOKEN 型注入位按下游要求三选一：`authHeaders`（token 放请求头）/ `authQuery`（token 放 query 参数）/ `authBody`。**query 位完整模板**（已实测跑通，按下游实际字段替换占位符）：

```json
{
  "authFields": [{"dataId":"appKey","type":"string","required":true},{"dataId":"appSecret","type":"password","required":true}],
  "fetchTokenRequest": {"method":"GET","url":"https://<换token接口>","query":[
    {"key":"<换token入参key名>","type":"authField","value":"#(\"appKey\")"},
    {"key":"<换token入参secret名>","type":"authField","value":"#(\"appSecret\")"}]},
  "authQuery": [{"key":"<业务接口的token参数名>","type":"text","value":"$.Body.<token在换token响应中的字段路径>"}],
  "tokenExpireRules": [{"func":"EQ(${@(\"Body/<错误码字段>\")},\"<token失效错误码>\")"}],
  "refreshToken": true,
  "testRequest": {"method":"POST","url":"https://<业务接口>"}
}
```

  要点：`authQuery[].value` 用 `$.Body.<字段>` 引用换 token 响应；失效判据按下游真实返回写——HTTP 状态码型用 `${@("$/statusCode")}`，业务错误码型用 `${@("Body/<字段>")}`。header 位注入同结构，把 `authQuery` 换成 `authHeaders` 即可。
- `credential save` 保存的是**开发者内置凭证**：归属「当前用户 + 当前 MCP」，密钥由开发者提供（不是使用者的凭证），配置调试与实例运行时两个场景都用它。`content` key 必须与 `auth get` 返回的 `authFields[].dataId` 一致；密钥不会回显，dry-run 也只显示脱敏占位。TOKEN 型 save 会真实调用换 token 接口验证密钥（密钥无效则 save 整体失败），其他类型纯落库。
- `credential debug` 会真实调用鉴权配置中的 `testRequest`，返回平铺结构 `{executeSuccess, executeErrorMsg, detail}`（V4 起 detail 在顶层）；**外层 success=true 只代表流程走完，不代表鉴权通过**——必须解析顶层 `detail.response`（statusCode 2xx 且业务无 errcode 才算真通过），由 AI 判读后给出明确结论，拿不准时人工复核。
- `credential bind` 只是「选用」：**仅对正式实例运行时生效**。`tool debug` 不使用 bind 绑定的凭证——调试带鉴权的工具必须在 `tool debug` 显式传 `--credential-id`，不传则按无凭证直调（TOKEN 注入配置原文、BASIC 静默不注入，报错全是误导性假症状）。
- 删除凭证前先 `credential get` 检查 `flowCount`；移除成员前先 `member list` 核对，二者都必须获得明确确认。

## Shortcut

### 从 API 材料一键建 MCP（终端用户最高频）

```text
1. 信息对齐：API 材料 / 业务目标 / 鉴权方式，缺就问
2. 按「从 API 材料到工具定义」拆三段式，映射按 mapping-rules.md 写
3. 设计整表给用户过目（工具清单/入参/映射/测试入参）
4. service create --dry-run → 确认 → --yes，记录 mcpId
5. 先建最简单的一个工具：tool create → tool get 读回核对（rules 位置名/条数）
6. 结构没问题再建其余；每建一个 tool list 反查一个，失败即停
7. 逐个 tool debug（草稿传 --version-id）：校验返回真实业务数据
8. debug 失败 → 对照 mapping-rules.md 修映射 → tool update → 再 debug（最多 2 轮，仍失败升级用户）
9. 用户明确确认后逐个 tool publish --yes
10. url get --source PUBLISHED 取接入地址（?key= 脱敏）
11. connector mcp refresh → 确认动态命令出现 → 真实调用验证一次
```

### 从零创建并发布一个只读 HTTP 工具

```text
1. service create --dry-run
2. service create --yes
3. tool create --dry-run
4. tool create --yes
5. tool get 或 tool list 取 toolId/versionId
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
1. tool list --mcp-id <mcpId> 找 toolId
2. tool get --mcp-id <mcpId> --tool-id <toolId> 读取当前定义和草稿 versionId
3. tool update --dry-run（全量提交，别漏字段）
4. tool update --yes
5. tool get 取新草稿 versionId
6. tool debug --version-id <草稿versionId> --yes
7. 用户确认后 tool publish --yes
```

注意：第 2 步读回的是平台底层存储结构（含组装后的 rules/schema JSON），**不能直接原样回填** `tool update`；要翻译回三段式参数后全量提交。翻译时以读回 rules 里的真实格式为参照（source/target/type 逐条还原到 inputMappings/outputMappings，schema 字段还原到 apiInputs/toolInputs）。

### 只验证线上工具

```text
1. tool list --mcp-id <mcpId> 找 toolId
2. tool get 读 toolInputs，构造真实测试入参（如 '{"city_name":"北京"}'；不要传空 {} 走过场）
3. tool debug --mcp-id <mcpId> --tool-id <toolId> --value '<测试入参JSON>' --dry-run
4. 确认后 tool debug --mcp-id <mcpId> --tool-id <toolId> --value '<测试入参JSON>' --yes

带鉴权的工具必须加 --credential-id <id>（debug 不使用 bind 绑定的凭证）。
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
3. tool get --mcp-id <mcpId> --tool-id <toolId> --format json
4. 根据 status 决定调试草稿、验证线上、发布或更新
```

禁止凭记忆使用 `mcpId` / `toolId` / `versionId`。

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
- `mcp_id_initializing`（MCP 数据初始化中）：存量数据补齐中，**等 10 秒重试一次**即可。
- `no_draft_to_publish`：没有可发布草稿，先 `tool update` 或确认 toolId。
- `business_error_invalid_params`：常见于旧契约调用——工具入参用了旧键 `actionId`（应为 `toolId`），或 create/update 用了旧键 `http`（应为 `httpInfo`）。升级 dws 后 CLI flag 对应 `--tool-id`/`--http-info`。
- `tool_already_listed_in_market`：已上架市场的工具不允许删除，需先在市场下架。
- 调试「成功」但接口报缺参/返回空：映射静默失效——位置名大小写（须 Pascal）/ express 用了 `source` 字段（须 `expression`）/ 漏映射，见 mapping-rules.md。
- 调试通过但发布后调用旧逻辑：调试时漏传草稿 `versionId`，实际测了线上旧版本。
- `service list` 偶发返回空列表：索引瞬态，重试一次即可。
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

列表类命令返回**顶层平铺**分页（V4 起，无 result/list 信封）：语义化数组 + 分页字段同级——`service list`→`services[]`、`tool list`→`tools[]`、`tool versions`→`versions[]`、`credential list`→`credentials[]`，旁边是 `hasMore/nextCursor/totalCount`。用 `hasMore` 判断翻页，不要用「返回条数==pageSize」启发式。

## 工具

```bash
# 列工具 / 读工具 / 版本历史
dws connector mcp tool list --mcp-id <mcpId> --page-size 100 --format json
dws connector mcp tool get --mcp-id <mcpId> --tool-id <toolId> --format json
dws connector mcp tool versions --mcp-id <mcpId> --tool-id <toolId> --format json

# 创建 / 更新工具：复杂字段使用 JSON 字符串
dws connector mcp tool create --mcp-id <mcpId> --name <snake_case_name> --title <标题> --description <描述> --http-info '{"method":"GET","url":"https://example.com","auth":{"type":"NO_AUTH"}}' --dry-run --format json
dws connector mcp tool update --mcp-id <mcpId> --tool-id <toolId> --name <snake_case_name> --title <标题> --description <描述> --http-info '{"method":"GET","url":"https://example.com","auth":{"type":"NO_AUTH"}}' --dry-run --format json

# 调试 / 发布 / 删除（value=符合 toolInputs 的测试入参，不要传空 {} 走过场）
dws connector mcp tool debug --mcp-id <mcpId> --tool-id <toolId> --version-id <versionId> --value '{"city_name":"北京"}' --credential-id <credentialId> --dry-run --format json
dws connector mcp tool publish --mcp-id <mcpId> --tool-id <toolId> --dry-run --format json
dws connector mcp tool delete --mcp-id <mcpId> --tool-id <toolId> --dry-run --format json
```

工具定义是三段式（结构详见上文「三段式工具定义」，映射详见 mapping-rules.md）：

- `apiInputs` / `apiOutputs`：HTTP 接口真实入参和出参。
- `toolInputs` / `toolOutputs`：暴露给 LLM 的入参和出参，可以裁剪、改名、写防呆描述。
- `inputMappings` / `outputMappings`：工具字段与真实接口字段的映射。
- 创建/更新工具前，把 `name` / `title` / `description` 和 `toolInputs` 展示给用户复核。

复杂字段直接传 JSON：

| flag | 类型 | MCP 入参 |
|------|------|----------|
| `--http-info` | object | `httpInfo` |
| `--api-inputs` | object | `apiInputs` |
| `--api-outputs` | object | `apiOutputs` |
| `--tool-inputs` | array | `toolInputs` |
| `--tool-outputs` | array | `toolOutputs` |
| `--input-mappings` | array | `inputMappings` |
| `--output-mappings` | array | `outputMappings` |
| `--value` | object | `value` |

## 调试与发布

- `tool debug` 会真实执行目标接口；写接口必须用测试数据（先让用户指定测试资源）。
- 调试带鉴权的工具必须显式传 `--credential-id`（debug 不使用 `credential bind` 绑定的凭证，不传按无凭证直调）。
- **通过标准 = 返回真实业务数据**（如查天气要真返回温度数值），不是「没报错」——映射静默失效时命令不报错但接口收到空参数。
- 调试返回为平铺结构（V4 起）：业务结果在**顶层 `toolOutput`**（不再嵌在 result.outputValue），同级有 `executeSuccess/toolInput/rawOutput/time`；出参精修是否生效直接看 `toolOutput` 形状。
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

- **publish ≠ 上架市场**：publish 后企业内即可用，`--source PUBLISHED` 直接取到「已发布未上架」服务的可用地址——无需上架、无需外部工具，全程自助闭环。
- 已上架市场传 `--source MARKET`。
- 返回的是按调用者**个人身份**生成的实例地址（非组织级公共地址），服务须已有已发布工具才有可用实例。
- 返回的 URL 或 JSON config 含个人化 `?key=`，按敏感凭证处理，勿外发共享，只能给当前用户本地配置使用。
- 代码或命令需要接入地址时优先使用 `url get` 或发现接口返回值，不要自行拼接服务 URL。

## Schema

helper-only 命令需要确认 JSON 参数结构时用：

```bash
dws schema connector.mcp.service.create --format json
dws schema connector.mcp.tool.create --format json
dws schema connector.mcp.tool.update --format json
dws schema connector.mcp.tool.debug --format json
```
