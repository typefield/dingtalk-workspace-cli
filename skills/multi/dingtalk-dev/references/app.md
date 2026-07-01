# 应用基础操作

> 操作的是「应用」容器本体（见 SKILL.md 概念地图）；启停/删除改的是应用 appStatus，不是版本 versionStatus。

应用列表查询、详情、创建、修改、生命周期启停和删除。参数查 `dws schema dev.app.<method>`（list / get / create / update / delete / disable / enable）。

`create` 的 `--name` 和 `--desc` 均为必填；`--app-type`（inner/personal）可选，默认 inner。

## 应用定位

所有单应用命令统一只用 `--unified-app-id`（全树主键）定位。`--app-key`/`--name` 只在 `dev app list` 里作列表过滤，不能定位单应用。拿到 appKey/agentId 时，只能做只读候选排查；写操作必须由用户或上游结果提供明确 `unifiedAppId`。

## 三方个人应用

三方个人应用是本命令组的受限支持范围，不等同于企业内部应用。当前只支持：

- `dev app create --app-type personal`：创建三方个人应用。
- `dev app list --app-type personal`：查询三方个人应用列表；按名称搜索时必须带 `--app-type personal`，否则默认查询企业内部应用。
- `dev app get --unified-app-id <id>`：查询三方个人应用基础信息。
- `dev app security config --unified-app-id <id> --redirect-urls <url>`：配置个人应用 OAuth 回调地址（只支持 `--redirect-urls`，不支持 `--ip-whitelist` / `--sso-urls`）。
- `dev app permission list/add/remove --unified-app-id <id>`：管理个人应用权限点。

三方个人应用列表只可靠支持 `--name` 模糊搜索、`--app-key` 精确搜索、`--page-size` 和 `--cursor` 分页。不要使用企业内部应用专属过滤条件（如机器人名称、分组、开发类型）来查询三方个人应用。

三方个人应用不支持：`update`、`delete`、`enable`、`disable`、`credentials get`、`webapp`、`robot`、`version`、`event`、`member`。

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
