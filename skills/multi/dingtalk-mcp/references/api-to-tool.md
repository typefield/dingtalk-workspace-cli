# 从 API 材料到工具定义

用户给 API 材料（OpenAPI / Postman / curl / 接口文档）要求「做成 MCP / 给 agent 用」时，按本文档把材料拆成 `tool create` 的三段式入参。三段式字段结构见 `mcp.md` §三段式工具定义；映射格式见 `mapping-rules.md`。

## 1. 信息对齐（缺就问，禁止猜）

动手前收齐三件事：

1. **API 材料**：OpenAPI/Postman/curl/文档，至少一种；
2. **业务目标**：这些接口给 agent 干什么——决定工具怎么拆、description 怎么写；
3. **鉴权方式**：不确定就问，别默认 NO_AUTH 蒙混。`http-info.auth` 三型：无鉴权 `{"type":"NO_AUTH"}`；Basic `{"type":"BASIC","username":"..","password":".."}`；API Secret `{"type":"API_SECRET","apiSecret":".."}`。

## 2. 从不同材料提取

| 材料 | 提取规则 |
|------|----------|
| **OpenAPI/Swagger** | `paths.{path}.{method}` → 候选工具；`operationId` → name（不合 snake_case 动词开头就改名）；`summary/description` → title/description 素材；`parameters` 按 in=query/path/header 分组进 apiInputs 对应组 + `requestBody` → apiInputs.body；`responses.200.schema` → apiOutputs.body；`servers[0].url` + path → http-info.url |
| **Postman Collection** | `item[].request` → method/url/headers/body；`item[].name` → title 素材；`response`/example → 测试入参 + apiOutputs 素材 |
| **curl 样例** | `-X` → method；URL 的 query 串拆进 apiInputs.query；`-H` → headers；`-d/--data/--form` → body |
| **文档文本** | 逐接口提取；字段含义不确定处**问用户，禁止编** |

**建议真跑取样**：只读接口先真实请求一次，拿真实响应反推 apiOutputs 结构 + 生成 `tool debug` 用的测试入参——比看文档猜可靠得多。

## 3. 工具侧加工（toolInputs 是面向 LLM 的投影，不是 apiInputs 照搬）

- **裁剪**：分页游标、固定控制位、冗余参数不暴露给 LLM，用 `fixed` 映射写死（mapping-rules §4）；
- **改名**：接口字段名对 LLM 不友好时改语义名（如接口 `name` → 工具 `city_name`），靠 inputMappings 连回去；
- **补约束**：平台不支持 enum/default/example 属性 → 枚举/默认值/示例写进字段 `description` 文本（规范见 mapping-rules §8）；
- **身份字段**：接口要 userId/corpId 的走系统参数注入（mapping-rules §6），不做成 toolInput。

## 4. 拆分粒度

**一个语义动作一个工具**，不是一个 endpoint 机械翻译成一个工具：

- 同一 endpoint 的两种典型用法可拆两个工具（各自 description 更聚焦）；
- 纯运维 / 冗余 / 内部接口可以不建；
- 有依赖关系的工具（先 A 拿 ID 再 B 用）在各自 description 写清依赖，让 agent 会编排。

## 5. 设计整表先过目

每个工具产出：name/title/description、http-info（CLI 自动附加 toolType:"http"）、apiInputs、toolInputs、inputMappings、outputMappings + **一组建议测试入参**（从材料示例值来，debug 用）。**整表给用户过一遍再动手建**——此时改成本最低。
