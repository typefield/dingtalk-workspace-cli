# 轻应用命令参考（dws dev liteapp）

六个子命令对应轻应用全生命周期。调用身份由系统上下文注入，仅能操作当前调用人创建的
轻应用。写操作需 `--yes` 确认，首次先 `--dry-run` 预览。

## 操作语义

- 创建即发布：`create` 返回 `status=PUBLISHED` 就是已上线终态，没有草稿/审核/版本
  流；要版本与发布能力的是 `dws dev app`（[version.md](version.md)），不要混用。
- `warning` 非空 = 成功但有降级（工作台挂载失败、统一应用注册失败等）：按成功
  继续后续步骤，但必须把 warning 原样转述给用户，不要自行判断"没影响"而省略。
- `success=false` 原样报告 `errorCode/errorMsg`（错误码表见下方）；不要按传输层
  "调用没报错"就当成功。
- 幂等只在传了 requestId 时生效（schema 里它是可选）：不传 requestId 的重试会
  创建出重复应用，网络重试/用户重发场景务必带键。同键同参重试返回首次结果
  （secret 不回显，用 `credential` 补查）；同键不同参拒绝（E_IDEMPOTENT_CONFLICT）；
  首次在途（E_IDEMPOTENT_PROCESSING）用同一 requestId 稍后重试，不换键。四个
  易误判点：参数/内容校验先于幂等判重——新参数不过校验时报校验错误码（如
  E_CONTENT_REJECTED）且不消耗幂等键，修正后同键可直接重试，CONFLICT 只在新
  参数通过校验后才会出现；首次创建落库失败会释放幂等键，同键重试是一次全新
  创建，不返回 DUPLICATE/CONFLICT；幂等服务自身读写失败返回 E_DEPENDENCY_FAILED
  （不是幂等冲突，同键稍后重试即可）；首次中断后的重放会逐项补齐缺失副作用
  （创建者/负责人登记、OAuth 凭证与映射、默认权限、发布、统一应用），补不齐的
  项以 warning 告知，secret 明文仍只经 create 响应或 `credential` 查询获取；
  幂等记录 TTL 24h，过期后同 requestId 视为新请求。
- 删除是 24 小时软删：期内仍占配额、list 可见且条目带 `timeToDel`；软删不可恢复，
  删除放在全部依赖步骤的最后。list 条目的 timeToDel（软删状态属性）与 delete
  返回的 timeToDel（本次删除截止时间戳）同名不同义，不要混用。
- 明文 secret 只出现在 `create` 响应和 `credential`；detail/list 永远只有掩码；
  不回显给用户以外的目的地，不落日志、文档、仓库。
- 无权限与不存在统一返回 E_NOT_FOUND，不据此推断应用是否存在，也不要反复换
  参数重试探权限。
- 配额 50 /（组织, 用户），软删期仍计入；E_USER_QUOTA_EXCEEDED 时引导用户删除
  不再使用的轻应用，不要自动批量删。

## 边界

liteapp 只管"创建即发布的轻应用本体 + 凭证"；轻应用创建时同步注册的统一应用
（`unifiedAppId`）此后就是普通统一应用——轻应用和统一应用是分离的两个面。后续
高级能力拿 `unifiedAppId` 走 `dws dev` 对应能力组，文档与 `dws dev app` 完全一致：

- 版本历史 / 待发布版本 / 发布 → `dws dev app version`（[version.md](version.md)）
- 权限点申请 → [permission.md](permission.md)
- 事件订阅 → [event.md](event.md)
- 机器人线上配置 → [robot.md](robot.md)；管理成员 → [member.md](member.md)

要"完整生命周期的企业内部应用"（草稿、审核、版本发布流程）时，直接用
`dws dev app`（[app.md](app.md)）创建，不要先建 liteapp 再补版本；只有
"秒建秒用、挂工作台、拿凭证"诉求才用 liteapp。

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
| E_DEPENDENCY_FAILED | 下游依赖失败（含幂等服务暂不可用） | 稍后重试；带 requestId 时保持同键 |

## MCP 服务

市场服务「钉钉开放平台应用管理」（预发 mcpId=10357）下的六个 HSF 工具
（create/update/delete/list/get_lite_app_detail/get_lite_app_credentials）。
mcpId 默认 10357；覆盖可传 `--mcp-id`。

- 排障：`endpoint_not_resolved` / `published_mcp_tool_error` 时用
  `dws mcp url get 10357` 验证端点，必要时 `--mcp-id` 显式指定，不要反复重试。
- 边界：权限点申请、版本发布、事件订阅等统一应用能力不在本命令组范围，
  拿 `unifiedAppId` 走 `dws dev` 对应能力组（version/permission/event，见上方
  「边界」一节）；普通企业内部应用管理走 `dws dev app`。
