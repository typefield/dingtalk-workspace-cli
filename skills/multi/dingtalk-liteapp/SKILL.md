---
name: dingtalk-liteapp
description: 钉钉轻应用（快捷应用）：创建、更新、删除、列表、详情、凭证查询，创建即发布挂工作台"我的"分组并注册统一应用。Use when 用户要创建一个能直接上工作台的轻应用/快捷入口、给应用配 OAuth 回调地址、查 appKey/secret 凭证、管理自己创建的轻应用配额。涉及开放平台权限点申请、版本发布、事件订阅等统一应用能力时走 dingtalk-misc 的 mcp published 工具；普通企业内部应用管理走 dingtalk-misc。命令前缀：dws dev liteapp（应用开发组，与 dev app 同级）。
metadata:
  cli_version: ">=1.0.62"
  category: product
  requires:
    bins:
      - dws
---

# 钉钉轻应用 Skill

轻应用 = 创建即发布挂工作台"我的"分组的快捷入口，附带 OAuth 凭证（appKey/secret）
并同步注册统一应用（企业内部应用，返回 `unifiedAppId`）。命令挂在「应用开发」
`dws dev` 之下（`dws dev liteapp`，与 `dev app` 同级）。底层调用已发布的
「钉钉开放平台应用管理」MCP 服务（预发 mcpId=10357）下的六个轻应用 HSF 工具，
调用身份由系统上下文注入（corpId/userId），只能操作当前调用人创建的轻应用。

## MUST DO

1. 写操作（create/update/delete）真实执行一律需要 `--yes`；首次请求先加
   `--dry-run` 预览将发送的参数，向用户展示并确认后再去掉 `--dry-run` 加 `--yes` 执行。
2. secret 明文会随 create 响应与 `credential` 子命令返回：注意防泄露，
   不得写入日志、文档、邮件、群聊或代码仓库。应用详情与列表永远只有掩码。
3. 删除是 24 小时软删：软删期内仍占用配额、列表可见（条目带 `timeToDel`）、不可恢复。
4. 底层 MCP 服务默认固定为「钉钉开放平台应用管理」（预发 mcpId=10357），
   一般无需指定；如需覆盖用 `--mcp-id <市场 mcpId>` 显式指定。

## 命令

```bash
# 创建（name/homepageUrl 必填；requestId 幂等键建议传 UUID）
dws dev liteapp create --name 周报助手 --homepage-url https://example.com \
  --desc 可选描述 --request-id $(uuidgen) --dry-run --format json
dws dev liteapp create --name 周报助手 --homepage-url https://example.com \
  --request-id $(uuidgen) --yes --format json

# 列表 / 详情（无 secret 明文）
dws dev liteapp list --size 20 --format json
dws dev liteapp detail <appId> --format json

# 更新（不传=不修改；传空串会被拒绝；redirectUris 传入即整体覆盖）
dws dev liteapp update <appId> --desc 新描述 --yes --format json
dws dev liteapp update <appId> \
  --redirect-uris https://a.example.com/cb,https://b.example.com/cb --yes --format json

# 凭证（appKey 明文 + secret 明文与掩码；重置需在开发者后台人工完成）
dws dev liteapp credential <appId> --format json

# 删除（24 小时软删，返回 timeToDel；仅创建者或组织管理员）
dws dev liteapp delete <appId> --yes --format json
```

## 返回契约

- 全部命令返回 ServiceResult 信封：`success / errorCode / errorMsg / result`。
- `create` 的 `result` 含 `appId、appKey、secret、secretMask、unifiedAppId、sdkSnippet、
  status（创建即 PUBLISHED）、warning`。
- `warning` 非空表示创建成功但有降级（如工作台入口挂载失败、统一应用注册失败），
  需原样转述给用户。
- 业务失败不抛传输错误：检查 `success=false` 时的 `errorCode/errorMsg`
  （如 `E_USER_QUOTA_EXCEEDED` 配额满、`E_IDEMPOTENT_CONFLICT` 幂同键参数变化）。

## 错误与边界

- `endpoint_not_resolved` / `published_mcp_tool_error`：网关端点或工具暂不可用，
  先用 `dws mcp url get 10357` 验证端点，必要时用 `--mcp-id` 显式指定服务，
  不要反复重试。
- 权限点申请、版本发布、事件订阅不在本 Skill 范围；统一应用域工具见
  dingtalk-misc 的 `mcp published`（按 `unifiedAppId` 定位）。
- 默认权限点（qyapi_base 等）创建时自动开通；更多权限点的申请能力待后续版本。
