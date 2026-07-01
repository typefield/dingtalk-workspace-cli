# 应用基础操作

> 操作的是「应用」容器本体（见 SKILL.md 概念地图）；启停/删除改的是应用 appStatus，不是版本 versionStatus。

应用列表查询、详情、创建、修改、生命周期启停和删除。参数查 `dws schema dev.app.<method>`（list / get / create / update / delete / disable / enable）。

`create` 的 `--name` 和 `--desc` 均为必填；`--app-type`（inner/personal）可选，默认 inner。

## 应用定位

所有单应用命令统一只用 `--unified-app-id`（全树主键）定位。`--app-key`/`--name` 只在 `dev app list` 里作列表过滤，不能定位单应用。拿到 appKey/agentId 时，只能做只读候选排查；写操作必须由用户或上游结果提供明确 `unifiedAppId`。

## 应用类型 appType

开放平台应用分两类，`create` 和 `list` 用 `--app-type` 区分：`inner`（企业内部应用，默认）或 `personal`（三方个人应用/个人小程序应用）。

| | 企业内部应用 `inner`（默认） | 三方个人应用 `personal` |
|---|---|---|
| 归属 | 属于某个企业/组织，只在该组织内可用 | 属于开发者个人，跨组织分发 |
| 典型场景 | 组织内部 H5 微应用、小程序、机器人 | 个人开发者上架、面向多组织的应用 |
| 创建 | `create`（默认） | `create --app-type personal` |
| 列表 | `list`（默认 inner） | `list --app-type personal` |

**个人应用支持的命令：**

| 操作类型 | 支持的命令 | 说明 |
|---|---|---|
| **类型感知入口** | `create`、`list` | schema 显式带 `--app-type` 参数，`personal` 已在预发 MCP 验证 |
| **通用操作（按 unifiedAppId 定位）** | `get`、`update`、`delete`、`enable`、`disable` | 无类型参数；拿到个人应用 unifiedAppId 后用同一套命令操作 |
| **子资源管理** | `credentials get`、`permission list/add/remove`、`member list/add/remove`、`security config`、`event list/subscribe/unsubscribe` | 无类型参数，按 unifiedAppId 定位 |
| **版本发布** | `version create/list/detail/publish/status/check-approval` | 同上 |
| **不支持（企业内部应用专属）** | `webapp get/config`、`robot submit/result/config/enable/offline` | 三方个人应用不支持网页应用配置和机器人能力 |

**要点：**
- 只有 `create`/`list` 在参数层区分应用类型，是个人应用的唯一显式入口。
- 两类是分开的两套列表，`list` 不跨类型混返：查三方个人应用**必须**显式传 `--app-type personal`，查企业内部应用可省略。
- 其余子命令无类型参数，统一按 `--unified-app-id` 定位。服务端 schema 描述多写"企业内部应用"，但 unifiedAppId 是跨类型的全树主键，拿到个人应用 id 后同样用这些命令操作。
- **三方个人应用不支持网页应用（webapp）和机器人（robot）能力**，这两类是企业内部应用的专属能力。

## 应用状态 appStatus

`app get` 的 `appStatus` 是字符串，取值如 `normal`、`published`。`app list` 不回这个字段（恒 `null`），看应用状态以 `app get` 为准。应用状态 `appStatus` 和版本状态 `versionStatus` 是两套，别混。遇到没见过的 `appStatus` 值原样展示。

`app create`、`app update` 不返回状态字段；版本状态由 `version create` 返回的 `status`（值如 INIT）表达，见 version.md。

## 要点

- `get` 主要用于定位核验；若返回里带 `appSecret`，脱敏处理，不复制到回答；主动读凭证走 `credentials get`。
- `disable/enable` 成功返回 `{disabled:true}` / `{enabled:true}` + `message`，不回 `appStatus`；以这个布尔判操作成败。要确认最终生效态再 `get` 看 `appStatus` 字符串值。
- `delete` 前必须展示应用摘要；删除是异步，成功后延迟从列表消失。

## 错误处理

| 情况 | 处理 |
|------|------|
| 多应用命中 | 展示候选，停止写操作 |
| `ServiceResult.success=false` | 透传 `errorCode/errorMsg` |

## 发现命令

调用任何方法前先查清楚再敲：

```
# 浏览命令组下的子命令与 flag
dws dev app --help

# 查某方法的必填参数、类型、默认值
dws schema dev.app.<method>
```

按 `dws schema` 输出构造 `--flag`（flag 名 = schema 参数名）。
