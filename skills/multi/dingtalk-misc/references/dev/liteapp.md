# 轻应用命令参考（dws dev liteapp）

六个子命令对应轻应用全生命周期。调用身份由系统上下文注入，仅能操作当前调用人创建的
轻应用。写操作需 `--yes` 确认，首次先 `--dry-run` 预览。

## 核心概念

轻应用 = "创建即发布"的组织内部 H5 微应用。一次创建自动完成四件事：微应用落库
（status 直接 PUBLISHED，无草稿/审核）、挂载到工作台"我的"分组、注册统一应用域
（企业内部应用，响应回填 `unifiedAppId`）、签发 OAuth 凭证（appKey/secret，可登记
https 回调地址）。

- **无版本**：没有 `dws dev app` 的 INIT→AUDIT→RELEASE 版本机制，创建即终态，
  更新直接生效。
- **数据本质**：底层就是 OrgMicroApp + extension JSON 标记 `lite-app=1`——服务端
  识别轻应用的唯一权威依据。权限：除 delete 放宽到"创建者或组织管理员"外，
  update/detail/credential 仅创建者本人；无权限与不存在统一返回 E_NOT_FOUND
  （防探测）。
- **身份模型**：corpId/userId 由系统上下文注入，只能操作当前调用人自己创建的
  轻应用。

### 与相邻品类的边界

- **快捷应用**（工作台快捷入口，`quick-link-app=1` 标记）：个人向、每人上限 5 个、
  无 OAuth 凭证、走另一条链路；轻应用是组织级正式应用。
- **普通企业内部应用**（`dws dev app`）：完整应用生命周期（版本、审核、发布）；
  轻应用砍掉版本流换秒建秒用。
- 统一应用的权限点申请、版本发布、事件订阅**不属于**本命令组，走
  `dws mcp published`（按 unifiedAppId）。

### 生命周期与可靠性

- **生命周期**：创建 → 更新 → 删除。删除为 24 小时软删：期内仍占配额、列表可见
  （条目带 timeToDel）、不可恢复，到期物理删除。注意 timeToDel 在 list 条目里
  （软删状态属性）与 delete 响应里（本次删除截止时间戳）同名不同义。
- **幂等**：`requestId` 为幂等键。同键同参重试返回首次结果（不回显 secret）；同键
  不同参拒绝（E_IDEMPOTENT_CONFLICT）；首次仍在处理返回 E_IDEMPOTENT_PROCESSING
  （同键稍后重试，不要换键）。
- **降级**：创建是多步编排，单步失败不阻塞整体；响应 `warning` 非空即"成功但有
  降级"（如工作台挂载失败、统一应用注册失败），必须原样转述给用户，不能当纯
  成功处理。
- **配额**：50 个 /（组织, 用户），软删期仍计入；配额校验 fail-closed（查询失败
  拒绝创建，不会放行）。
- **凭证安全**：明文 secret 只出现在 create 响应和 credential 命令；detail/list
  永远只有掩码。防泄露：不落日志、文档、仓库。

## 何时用哪个

- 用户问「我建过哪些轻应用」→ list；「第2页有哪些」→ list 加 `--offset`
- 「看下这个应用的状态/配置/回调」→ detail（无 secret 明文）
- 「给我 appKey/secret 做服务端接入」→ credential（返回明文，注意防泄露）
- 「改名称/描述/首页/回调地址」→ update；「不再需要了」→ delete（先与用户确认）

## create

```bash
dws dev liteapp create --name 周报助手 --homepage-url https://example.com \
  --pc-url https://example.com/pc --desc 描述 --request-id 6f1c2b3a-uuid --yes
```

- `--name`/`--homepage-url` 必填；`--pc-url` 缺省取移动端首页。
- `--icon-media-id` 收 media_id（由媒体上传 OpenAPI 生成，非 http 地址）；
  不传自动按应用名生成默认图标。
- `--request-id` 幂等键（建议 UUID）：同键同参重试返回首次结果（含原 appId、
  appKey、sdkSnippet，不重复下发 secret），参数变化拒绝；首次仍在处理中返回
  E_IDEMPOTENT_PROCESSING，持同一 requestId 稍后重试而非换键。
- result：appId、appKey、secret（明文）、secretMask、unifiedAppId、sdkSnippet、
  status、requestId、warning。凭证生成降级时凭证字段为 null：告知用户稍后在
  开发者后台重试，再用 `credential <appId>` 补查。

## list

```bash
dws dev liteapp list --size 10 --offset 0 --format json   # 首查：前 10 条
dws dev liteapp list --size 10 --offset 10 --format json  # 翻页：第 11-20 条
dws dev liteapp list --size 50 --offset 0 --format json   # 边界：单页上限 50
```

- 触发场景：用户问「我建过哪些轻应用」「第2页有哪些」「还剩多少配额」。
- `total` 与配额口径一致（软删期仍计入）；按 createdAt 倒序；不含 secret 字段。
- 条目 `timeToDel` 仅表示该应用当前处于软删期（列表属性），不是删除操作的结果。

## detail

```bash
dws dev liteapp detail <appId> --format json
```

返回基础信息、appKey、secretMask、redirectUris、unifiedAppId、状态；**无 secret
明文**——需要明文接入时改用 credential。查状态/配置/回调一律用 detail。
无权限或不存在统一返回 E_NOT_FOUND（防探测）。

## update

```bash
dws dev liteapp update <appId> --name 新名称 --homepage-url https://m.example.com/ \
  --redirect-uris https://a.example.com/cb,https://b.example.com/cb --yes   # 混合更新
```

- 仅创建者本人；所有字段不传=保持原值，传空串一律拒绝（防误清空）。
- 回调地址必须 https（http:// 会被拒绝）；`--redirect-uris` 逗号分隔，
  传入即全量覆盖登记，上限 10 条。

## credential

```bash
dws dev liteapp credential <appId> --format json
```

- 返回 appKey 明文 + secret 明文与掩码，可持续获取；仅用于取密钥，
  查状态/配置/回调请用 detail，避免不必要的明文暴露。
- 防泄露：不写入日志、文档、邮件、群聊、代码仓库。重置需开发者后台人工完成。

## delete

```bash
dws dev liteapp delete <appId> --yes
```

- 仅创建者或组织管理员；24 小时软删，期内占配额、列表可见，不可恢复。
- `result` 的 timeToDel 是本次删除生成的软删截止时间戳（毫秒）——与 list
  条目里的 timeToDel（软删期状态属性）同名不同义。

## 错误码

| errorCode | 含义 | 处理 |
|---|---|---|
| E_PARAM_INVALID | 参数缺失/非法（含空串更新、回调超 10 条） | 按 message 修正 |
| E_NOT_LOGIN | 登录态缺失 | `dws auth login` |
| E_ORG_INVALID / E_ORG_BLACKLISTED | 组织无效/黑名单 | 核对 corpId 或联系管理员 |
| E_CONTENT_REJECTED | 内容安全拒绝 | 修改名称/描述 |
| E_USER_QUOTA_EXCEEDED | 配额满（50/组织/用户） | 删除不再使用的轻应用 |
| E_NOT_FOUND | 应用不存在或无权限（防探测） | 核对 appId 与当前账号 |
| E_IDEMPOTENT_CONFLICT | 同 requestId 参数变化 | 更换 requestId |
| E_IDEMPOTENT_PROCESSING | 首次请求仍在处理中 | 同 requestId 稍后重试 |
| E_DEPENDENCY_FAILED | 下游依赖失败 | 稍后重试 |

## MCP 服务

市场服务「钉钉开放平台应用管理」（预发 mcpId=10357）下的六个 HSF 工具
（create/update/delete/list/get_lite_app_detail/get_lite_app_credentials）。
mcpId 默认 10357；覆盖可传 `--mcp-id`。

- 排障：`endpoint_not_resolved` / `published_mcp_tool_error` 时用
  `dws mcp url get 10357` 验证端点，必要时 `--mcp-id` 显式指定，不要反复重试。
- 边界：权限点申请、版本发布、事件订阅等统一应用能力不在本命令组范围，
  走 `dws mcp published` 工具（按 `unifiedAppId` 定位，见 mcp.md）；
  普通企业内部应用管理走 `dws dev app`。
