---
name: dingtalk-mail
description: 钉钉邮箱读写、搜索、回复与转发。Use when 用户说发邮件/查邮件/回邮件/转发邮件/未读邮件/邮件搜索/邮箱附件。一句话发邮件时先用 dingtalk-contact 解析并确认收件人，再由本 skill 发送；不做钉钉消息（走 dingtalk-chat）、紧急通知（走 dingtalk-misc）。命令前缀：dws mail。
metadata:
  cli_version: ">=0.2.14"
  category: product
  requires:
    bins:
      - dws
---

# 钉钉邮箱 Skill

<!-- DWS_RUNTIME_CONTRACT_START -->
## 最小 DWS 执行契约

- 只通过 `dws` CLI 操作钉钉；结构化读取使用 `--format json`，按真实返回判断结果。
- 已知命令直接执行。Skill/reference 无法定位时才用 `dws schema search --query "<意图>" --limit 5`；选中后携带双 hash Inspect canonical，再按 `primary_cli_path` 执行。参数/安全语义或 Cobra flag 不确定时才补读精确 Schema/Help；不加载产品级 Catalog 代替选路。
- 不猜命令、flag、字段、ID、账号或时间。后续 ID 必须来自真实返回；零命中、多候选或类型不明时停止并消歧。
- 解析目标、读取上下文和最终执行必须使用同一 profile；不得跨组织复用 userId、openDingTalkId 或 openConversationId。多账号组织只使用明确的 `isOrgCurrent=true` 默认账号；没有默认账号时要求用户指定，禁止选择第一项、最近登录或最近使用账号。
- 不输出或记录 token、refresh token、appSecret、webhook token 等凭据；宿主已注入认证时不要索要凭据。
- 写操作必须符合用户明确意图。是否需要确认以最终 Runtime gate 和 Schema 为准；需要确认时先说明对象、动作与影响，再追加 `--yes`。
- 写后按任务结果契约验证；不能仅凭退出码宣称成功。部分结果、未知投递状态和失败项必须如实保留。
- 时间戳面向用户展示时转换为带时区的可读时间；默认使用当前会话时区，必要时同时保留原值。
- 遇到认证、权限、profile、confirmation 或未知错误时，只加载 `dingtalk-shared` 中对应 reference；不要连续猜测替代命令。
<!-- DWS_RUNTIME_CONTRACT_END -->

> 命令参考：[mail.md](references/mail.md)。复杂搜索、附件、批量处理、草稿等多步邮件场景参考：[09-mail.md](references/09-mail.md)。

<!-- VISIBLE_SHORTCUTS_START -->
## Shortcuts（无专用脚本/recipe 时优先）

以下 shortcut 同时进入公开 catalog 与 Runtime Schema。先按本 skill 的意图表、脚本和 recipe 路由：存在精确覆盖该场景的专用脚本/recipe 时按其执行；否则用户意图命中时，shortcut 优先于手写原子命令。命令已选中时直接执行；只在参数或安全语义不确定时读取 Agent leaf Schema（例如 `dws schema --cli-path "mail +<shortcut>" --compact --format json`），在当前 Cobra flags 不确定时读取 `dws mail <shortcut> --help`。只有参数映射、接口绑定或 provenance 审计才省略 `--compact`。仅当现有路由和 reference 都无法定位低频能力时，才用 `dws schema search --query "<用户意图>" --product mail --limit 5` 同时搜索原子命令和 shortcut。`dws shortcut list --service mail` 只作人工审计/兼容 fallback。

| Shortcut | 风险 | 适用场景 |
|---|---|---|
| `dws mail +contact-list` | read | 列出指定邮箱的所有邮件联系人 |
| `dws mail +find-mail-user` | read | 按关键词搜索邮箱联系人并投影列表（姓名/昵称/邮箱/工号等） |
| `dws mail +folder-list` | read | 列出顶层文件夹或指定父文件夹下的子文件夹 |
| `dws mail +recent-mail` | read | 列出收件箱近期邮件会话并投影列表（主题/发件人/时间/threadId） |
| `dws mail +search-mail` | read | 按 KQL 关键词搜索邮件并投影列表（主题/发件人/时间/messageId） |
| `dws mail +tag-list` | read | 列出指定邮箱下的所有邮件标签 |
| `dws mail +template-list` | read | 列出指定邮箱的所有邮件模板 |
| `dws mail +thread-list` | read | 列出指定邮箱文件夹下的邮件会话（thread） |
| `dws mail +unread-mail` | read | 列出未读邮件并投影列表（主题/发件人/时间/messageId） |
| `dws mail +user-search` | read | 按关键词或工号搜索邮箱用户（仅企业邮箱） |
<!-- VISIBLE_SHORTCUTS_END -->

## 意图表

| 用户说 | 命令 |
|--------|------|
| "发邮件给 a@b.com" | `dws mail mailbox list --format json` → `dws mail message send --from <邮箱> --to a@b.com --subject "<标题>" --content "<正文>" --format json` |
| "回复 / 全部回复 / 转发" | `dws mail message reply` / `reply-all` / `forward` |
| "今天未读邮件" | `python scripts/mail_unread_summary.py` |
| "带抄送发送" | `python scripts/mail_send_with_cc.py --to a@b.com --cc c@d.com --subject "<标题>" --body "<正文>"` |

## 标准 SOP（必遵流程）

> 命中以下意图**必须**按对应 SOP 顺序执行；**禁止**跳步、替换命令、编造 email/messageId。每条命令必须带 `--format json`。收件邮箱**必须**真实解析，**禁止**猜测。

### SOP-1 拿邮箱（get-mailbox）

**触发**：我的邮箱/发件需要邮箱/查邮件需要邮箱。

1. **执行（必须）**：`dws mail mailbox list --format json`，取自己的 `email`（默认选企业邮箱 `type:ORG`）；查**他人**邮箱用 `dws mail user search --keyword "<姓名>" --format json`，**禁止**用 `mailbox list` 查他人。

**禁止**：把 `mailbox list` 当作他人邮箱查询、猜测邮箱地址。

### SOP-2 查 / 搜邮件（search-mail）

**触发**：查邮件/搜邮件/某主题邮件/某人发的邮件。

1. **拿邮箱（必须）**：先按 SOP-1 取 `email`；用户已明确提供可跳过。
2. **执行（必须）**：浏览文件夹 `dws mail message list --email <邮箱> --limit <n> --format json`；KQL 搜索 `dws mail message search --email <邮箱> --query "<KQL>" --limit 20 --format json`（KQL 如 `subject:周报`、`from:alice@x.com`、`folderId:2`、`hasAttachments:true`，**只通过 `--query` 传**）。
3. **取正文（必须）**：`dws mail message get --email <邮箱> --id <messageId> --format json`；`messageId` 从列表/搜索结果取，**禁止**编造。

**禁止**：把 KQL 拆成多个 flag、跳过 `message list/search` 直接猜 messageId。

### SOP-3 发邮件（send-mail）

**触发**：发邮件/写邮件/群发。

1. **发件邮箱（必须）**：`dws mail mailbox list` 取自己邮箱。
2. **收件邮箱（必须）**：地址直接用；姓名按 [mail.md](references/mail.md) "查找他人邮箱地址"流程（`mail user search` 等）获取，**禁止**猜测。
3. **执行（必须）**：`dws mail message send --from <发件邮箱> --to <收件邮箱> --subject "<主题>" --content "<正文>" --format json`；按需 `--cc`/`--attachment`/`--inline-attachment`。
4. **验证（必须）**：从发送返回取真实 `internetMessageId`，执行 `dws mail message verify --email <发件邮箱> --internet-message-id <internetMessageId> --format json` 查发送状态；不要把普通 `messageId` 传给 verify。

**禁止**：猜测收件邮箱、发送后不确认状态就答复"已发送"。

### SOP-4 回复 / 转发（reply-forward）

**触发**：回复邮件/回复全部/转发。

1. **拿邮箱 + 原邮件（必须）**：SOP-1 取邮箱；用户未给 `messageId` 时**必须**先走 SOP-2 定位原邮件 `messageId`。
2. **执行（必须）**：回复 `dws mail message reply --from <邮箱> --id <messageId> --content "<正文>" --format json`；回复全部用 `reply-all`；转发 `dws mail message forward --from <邮箱> --to <收件邮箱> --id <messageId> --content "<附言>" --format json`。

**禁止**：未定位原邮件就回复/转发、编造 messageId。

## 高频硬约束

- 用户要"完整内容/看看这封邮件/正文"时，`message search` 命中后必须继续调用 `dws mail message get --email <邮箱> --id <messageId> --format json`；不要只列候选后停下。
- 搜到多封邮件时，若用户给了明确主题、附件名、发件人或时间线索，先选最匹配的一封执行 `message get`；只有同等候选无法判断时才询问用户。
- 附件链路固定三步：`message search` → `attachment list --email <邮箱> --id <messageId>` → `attachment download --email <邮箱> --message-id <messageId> --attachment-id <attachmentId> --name <文件名>`；不存在批量下载命令。
- 写入类操作（发送、回复、转发、删除、批量移动）按安全策略确认；只读查看、搜索、附件列表、下载不需要确认。
- 所有 `dws mail` 命令加 `--format json`，并复用同一封邮件的 `messageId`，不要重新搜索导致目标漂移。

## 跨产品协作

- 收件人是人名 → 先用 `dingtalk-contact` 取 `orgAuthEmail`
- 钉钉内消息 → 切到 `dingtalk-chat`
## 局部意图与短流程

- [局部意图消歧](references/intent-guide.md)；[短流程](references/lite-recipes.md)。
