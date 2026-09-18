# 轻应用命令参考（dws dev liteapp）

六个子命令对应轻应用全生命周期。调用身份由系统上下文注入（corpId/userId），
仅能操作当前调用人创建的轻应用。写操作需要 `--yes` 确认，首次请求先 `--dry-run` 预览。

## create

```bash
dws dev liteapp create --name 周报助手 --homepage-url https://example.com \
  --pc-url https://example.com/pc --desc 描述 --request-id 6f1c2b3a-uuid --yes
```

- `--name` / `--homepage-url` 必填；`--pc-url` 缺省取移动端首页。
- `--icon-media-id` 收 media_id（logoImg 格式），不接受 http 地址；不传自动生成默认图标。
- `--request-id` 幂等键：同键同参数重试返回首次结果（含原 appId、appKey、sdkSnippet，
  不重复下发 secret 明文），参数变化拒绝；首次请求仍在处理中返回 `E_IDEMPOTENT_PROCESSING`，
  应持同一 requestId 稍后重试而非换键。
- `result`：appId、appKey、secret（明文）、secretMask、unifiedAppId、sdkSnippet、
  status=PUBLISHED、requestId、warning。凭证生成降级时 appKey/secret/secretMask/sdkSnippet
  为 null，warning 给出提示。

## list

```bash
dws dev liteapp list --size 20 --offset 0 --format json
```

- `total` 与配额口径一致（软删期仍计入）；`apps[]` 按 createdAt 倒序，软删期条目带 `timeToDel`。
- 不含任何 secret 字段。

## detail

```bash
dws dev liteapp detail <appId> --format json
```

返回基础信息、appKey、secretMask、redirectUris、unifiedAppId、状态；无 secret 明文。
无权限或不存在统一返回 `E_NOT_FOUND`（防探测）。

## update

```bash
dws dev liteapp update <appId> --desc 新描述 --yes
dws dev liteapp update <appId> --redirect-uris https://a.example.com/cb,https://b.example.com/cb --yes
```

- 仅创建者本人；字段不传=不修改，传空串拒绝（防误清空）。
- `--redirect-uris` 逗号分隔，传入即整体覆盖登记。

## credential

```bash
dws dev liteapp credential <appId> --format json
```

- 返回 appKey 明文 + secret 明文与掩码，可持续获取。
- 防泄露底线：不写入日志、文档、邮件、群聊、代码仓库。
- 重置需在开发者后台人工完成，本命令不含重置。

## delete

```bash
dws dev liteapp delete <appId> --yes
```

- 仅创建者或组织管理员；24 小时软删，期内仍占配额、不可恢复。
- `result` 为软删截止时间 `timeToDel`（毫秒时间戳）。

## 错误码

| errorCode | 含义 | 处理 |
|---|---|---|
| E_PARAM_INVALID | 参数缺失/非法（含空串更新） | 按 message 修正 |
| E_NOT_LOGIN | 登录态缺失 | `dws auth login` |
| E_ORG_INVALID / E_ORG_BLACKLISTED | 组织无效/黑名单 | 核对 corpId 或联系管理员 |
| E_CONTENT_REJECTED | 内容安全拒绝 | 修改名称/描述 |
| E_USER_QUOTA_EXCEEDED | 配额满（50/组织/用户） | 删除不再使用的轻应用 |
| E_IDEMPOTENT_CONFLICT | 同 requestId 参数变化 | 更换 requestId |
| E_IDEMPOTENT_PROCESSING | 同 requestId 首次请求仍在处理中 | 用同一 requestId 稍后重试，不要换键 |
| E_DEPENDENCY_FAILED | 下游依赖失败 | 稍后重试 |

## MCP 服务

底层调用市场服务「钉钉开放平台应用管理」（预发 mcpId=10357）下的
`create_lite_app / update_lite_app / delete_lite_app / list_lite_apps /
get_lite_app_detail / get_lite_app_credentials` 六个 HSF 工具。
mcpId 默认 10357（钉钉开放平台应用管理），一般无需指定；
如需覆盖可对 `dws dev liteapp` 系列命令传 `--mcp-id` 显式指定。
