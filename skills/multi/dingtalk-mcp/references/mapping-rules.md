# 映射规则（--input-mappings / --output-mappings）

工具的「入参映射」和「出参映射」把 LLM 可见的工具参数（toolInputs）与接口真实参数（apiInputs）连起来，是工具能不能跑通的核心。本文档是格式权威：`dws connector mcp tool create/update` 的 `--input-mappings` / `--output-mappings`（JSON 数组字符串）严格按此写。

> 这些规则用真实工具端到端实证过（映射样本比对 + open-meteo 全链路 + httpbin 回显）。**三个最大的坑都是「静默失效不报错」**：位置名大小写、express 字段用错、出参映射为空——只有 `tool debug` 真跑才暴露。

## 1. 一条映射规则的结构

```json
{ "type": "reference", "source": "$.node_start.city_name", "target": "$.Query.name" }
```

| 字段 | 含义 |
|------|------|
| `type` | 映射类型：`reference`（引用变量）/ `fixed`（固定常量）/ `express`（表达式） |
| `source` | 值从哪来（reference/fixed 用；**express 不用 source**，见 §4） |
| `expression` + `displayText` | express 专用：表达式 + 可读说明（见 §4） |
| `target` | 值放到哪个接口参数 |

`inputMappings` 是「工具入参 → 接口入参」的规则数组；`outputMappings` 是「接口出参 → 工具出参」的规则数组。

## 2. source / target 的 JSONPath 写法

**入参映射（inputMappings）**：
- `source`：`$.node_start.<toolInput 的 key>` —— 引用用户传给工具的参数。
- `target`：`$.<位置>.<接口字段>` —— 位置 = `Body` / `Query` / `Head` / `Path`（Pascal，见 §3）。

**出参映射（outputMappings）**：
- `source`：`$.node_service_activator.Body`（接口响应体）/ `$.node_service_activator.Headers`（响应头）。
- `target`：`$` 表示工具出参根，或 `$.<字段>` 指定子字段。

系统身份变量：`source` 用 `$.system_node.operateUserId` / `$.system_node.ddDataCorpId`（见 §6）。

## 3. 位置名必须 Pascal 大小写（最大的坑）

`target` 里的位置名**必须首字母大写**：`Body` / `Query` / `Head` / `Path`。

- ✅ 正确：`$.Query.name`、`$.Body.userId`
- ❌ 错误：`$.QUERY.name`（全大写）、`$.query.name`（全小写）——**静默失效**：不报错、值不流转、接口收到空参数。

> 注：`apiInputs` 分组的 key 是全小写（CLI 的 `--api-inputs` JSON 里写 `query`/`body`/`headers`/`path`），存储层是全大写，而映射 `target` 路径里用 Pascal。三处大小写不同，别混——`target` 只认 Pascal。

写错平台不报错，只有 `tool debug` 真跑才暴露（接口报缺参数/返回空）。建完工具必须 debug 就是为了抓这个。

## 4. 三种映射类型：reference / fixed / express

### reference（引用变量，最常用）
把工具入参透传到接口字段。`source` 指向 toolInput 的 key。
```json
{ "type": "reference", "source": "$.node_start.city_name", "target": "$.Query.name" }
```

### fixed（固定常量）
接口字段填写死的值，**不暴露给 LLM**。`source` 直接写常量值（不加 `$.`）。用来把接口的固定控制参数从 LLM 视野里裁掉。
```json
{ "type": "fixed", "source": "zh", "target": "$.Query.language" }
```

### express（表达式）
用表达式函数把值做变换/换算再送给接口。

> ⚠️ **字段用法与 reference/fixed 不同（最容易踩的坑）**：express 的表达式**必须放 `expression` 字段**（可读说明放 `displayText`），**不是 `source`**。放 `source` 会被服务端静默丢弃（存成 `{}`）且不报错。

```json
{ "type": "express",
  "expression": "GET(\"operateUserId\",${@(\"system_node/$\")})",
  "displayText": "GET(operateUserId,系统参数)",
  "target": "$.Query.p_expr" }
```

表达式语法要点：`${@("node_start/$/<字段>")}` 引用工具入参；`${@("system_node/$")}` 引用系统参数对象（配 `GET("key",…)` 取值）；函数可嵌套（`CONCATENATE` / `COALESCE` / `IF` 等）。

**「API → MCP」绝大多数场景用不到 express**：reference（用户传）+ fixed（常量）+ 系统注入（§6）就够了。只有值需要**运算/换算**才用，常见是身份换算（接口要 uid/unionId 而系统参数只有 userId）——⚠️ 换算函数以平台「推荐映射」为准（如内部函数 `USERID2UIDBYCORPID` 不在公开目录），建工具时用 `tool get` 读一个已有工具的 rules 参照，别凭空写函数名。

**完整函数目录**（7 组 82 个：集合/日期/逻辑/数学/字符串/JSON/系统，含 72 用例实测的序列化坑——date 出毫秒串、collection 直映丢数据须先 JOIN、double 带 `.0`）见 **[expression-functions.md](expression-functions.md)**，做复杂数据变换才翻。

## 5. 出参映射：整体透传

`outputMappings` **不能为空**（空了工具没有出参、返回空）。最简且推荐 = 响应体整体透传：

```json
[ { "type": "reference", "source": "$.node_service_activator.Body", "target": "$" } ]
```

工具出参 = 接口返回的完整 body，绝大多数只读接口这样就够。需要结构化裁剪/改名时才逐字段写（并配套 toolOutputs 定义结构）。

## 6. 系统参数注入（身份等）

接口需要调用者身份（userId / corpId）等运行时上下文时，**不要**做成 toolInput 让 LLM 传，用 `reference` 引用系统参数——平台运行时按「当前调用者」自动填充，权限跟人走：

```json
{ "type": "reference", "source": "$.system_node.operateUserId", "target": "$.Body.userId" }
{ "type": "reference", "source": "$.system_node.ddDataCorpId",  "target": "$.Body.corpId" }
```

### 系统参数全集（`$.system_node.*`）

**用 `key` 列写映射，不要用显示名**——多数 key 带 `deap` 前缀且与显示名不同，写错静默失效：

| key（写映射用） | 含义 |
|-----------------|------|
| `operateUserId` | 调用工具的用户 userId（最常用） |
| `ddDataCorpId` | 调用工具的组织 corpId（最常用） |
| `deapAgentCode` / `deapAgentName` | agentCode / agentName |
| `deapRunId` | 本次运行 runId |
| `deapClientSessionId` | sessionId |
| `deapScenarioCode` | scenarioCode |
| `deapParentAbilityCallSessionId` | 父能力调用 sessionId |

服务配了鉴权时另有 `$.system_node.AppKey` / `$.system_node.AppSecret`。

## 7. 数组字段的双规则

入参/出参是**数组**时，映射需要**两条一组**：整体一条 + 元素级一条（`[*]`，如 `$.node_start.ids[*]` → `$.Body.ids[*]`；缺 `[*]` 条 UI 显示未映射）。且 `apiInputs`/`toolInputs` 里该 array 字段必须带**非空 `items`**（items 用 object 型，避免误导 LLM）。

多数场景入参是标量、出参走整体透传（§5），用不到这条；遇到数组入参再查此节并读一个真实带数组的工具样本对齐。

## 8. description 写法规范（工具与参数）

description 喂给 agent，质量决定 agent 会不会用、会不会用对：

- **工具 description**：动词开头，50-200 字（列长上限约 700 字符，publish 才报错），四要素——功能 / 参数 / 输出 / 适用场景；写清「什么时候用」和前置工具依赖（如「latitude 可由 search_city 获得」）；破坏性/写操作显式注明影响面。
- **参数 description**（每个 toolInput）：标必填性 + 推荐格式 + 取值来源，带 ✅ GoodCase / ❌ BadCase：
  ```
  必填。要查询的城市中文名称。✅ 示例：北京 / 上海。❌ 不要传拼音或英文
  ```
- 平台不支持 enum/default/example 属性 → 枚举/默认值/示例写进 description 文本。

## 9. 完整示例

open-meteo 城市搜索工具 `search_city` 的映射（已实证跑通），即 `--input-mappings` / `--output-mappings` 的完整取值：

```json
{
  "inputMappings": [
    { "type": "reference", "source": "$.node_start.city_name", "target": "$.Query.name" },
    { "type": "fixed",     "source": "zh",                     "target": "$.Query.language" },
    { "type": "fixed",     "source": "10",                     "target": "$.Query.count" }
  ],
  "outputMappings": [
    { "type": "reference", "source": "$.node_service_activator.Body", "target": "$" }
  ]
}
```

- 用户只传 `city_name`（reference）；`language=zh`、`count=10` 用 fixed 固定不暴露；
- 出参整体透传，工具返回 open-meteo 的完整 `{results:[...]}`。
