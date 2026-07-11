---
name: dingtalk-chat
description: 钉钉群聊与消息。Use when 用户提到 发消息/单聊/群聊/建群/拉人进群/改群名/搜索群/群成员管理/@消息/撤回消息/机器人群发/Webhook通知/发图片或文件到群/标记未读/清除红点/置顶消息/全部群列表。不做紧急 DING/短信/电话（走 dingtalk-ding）、邮件（走 dingtalk-mail）、班级群（走 dingtalk-edu-group）。命令前缀：dws chat。
cli_version: ">=0.2.14"
metadata:
  category: product
  requires:
    bins:
      - dws
---

# 钉钉群聊 / 消息 Skill

## 前置条件 — 执行操作前必读

> **`use_skill(dws-shared)`** — 认证、全局参数（`--format json` / `--yes`）、错误码、URL 模板、跨产品消歧、安全规则与 capability 边界。**执行任何 `dws` 命令前先读；** 单产品的清晰命令可直接用本 skill。

<!-- SAFETY_PREAMBLE_INJECT -->

> 命令参考：[chat.md](references/chat.md)；表情：[chat-emoji-list.md](references/chat-emoji-list.md)；剧本：[01-messaging.md](references/01-messaging.md)。

## 意图表

| 用户说 | 命令 |
|--------|------|
| "发消息给张三" | `dws chat message send --open-dingtalk-id <id> --text "<内容>"` |
| "发到XX群" | `dws chat search --query "<群名>"` → `dws chat message send --group <openConversationId> --text "<内容>"` |
| "建群" / "拉人进群" | `dws chat group create` / `dws chat group members add` |
| "改群名" / "踢人" | `dws chat group rename` / `dws chat group members remove` |
| "@我消息" / "查群聊记录" | `dws chat message list` |
| "用机器人发消息" | `dws chat message send-by-bot --robot-code <code> --group <id>` |
| "Webhook 推一条" | `dws chat message send-by-webhook --token <token>` |
| "撤回消息" | `dws chat message recall --client-msg-id <id>` |
| "标记未读 / 清除红点 / 全部已读" | `dws chat mark-unread` / `dws chat clear-red-point` / `dws chat clear-all-red-point` |
| "置顶某条消息 / 取消消息置顶" | `dws chat message set-top-msg` / `dws chat message unset-top-msg` |
| "我加入的所有群 / 全部群列表" | `dws chat group list-all` |

## 标准 SOP（必遵流程）

> 命中以下意图**必须**按对应 SOP 顺序执行；**禁止**跳步、替换命令、编造 ID。每条命令必须带 `--format json`，执行后必须按"解析"步取真实字段（`openDingTalkId` / `openConversationId` / `clientMsgId`）。

### SOP-1 发消息（send-message）

**触发**：发消息/单聊/通知某人/发到群里。

1. **解析收件人（必须）**：人名 → 先 `dws aisearch person --keyword "<姓名>" --dimension name --format json` 取 `openDingTalkId`（优先）或 `userId`；群名 → 先 `dws chat search --query "<群名>" --format json` 取 `openConversationId`。
2. **执行（必须）**：单聊 `dws chat message send --open-dingtalk-id <openDingTalkId> --text "<内容>" --format json`（只有拿不到 `openDingTalkId` 时才用 `--user <userId>`）；群聊 `dws chat message send --group <openConversationId> --text "<内容>" --format json`。
3. **验证（必须）**：从返回取 `messageId` / `openMessageId` / `clientMsgId` 备用（撤回要用）；返回非 `success` 必须如实报错，不要谎报已发。

**禁止**：把人名/群名直接当 ID 传入、跳过 `aisearch person`/`chat search` 解析、跳过 `--format json`、未发送成功就答复"已发送"。

### SOP-2 建群（create-group）

**触发**：建群/拉人进群/新建讨论组。

1. **解析成员（必须）**：对每个成员 `dws aisearch person --keyword "<姓名>" --dimension name --format json` 取 `userId`，多人英文逗号拼接。
2. **执行（必须）**：`dws chat group create --name "<群名>" --users <userId1,userId2,...> --format json`；外部群加 `--type EXTERNAL`，话题圈加 `--thread`。
3. **验证（必须）**：从返回取 `openConversationId`，可用 `dws chat search --query "<群名>" --format json` 复核。

**禁止**：跳过成员 userId 解析直接传姓名、编造 `openConversationId`。

### SOP-3 Webhook 推送（send-by-webhook）

**触发**：用机器人群 webhook 推一条消息。

1. **执行（必须）**：`dws chat message send-by-webhook --token <webhookToken> --title "<标题>" --text "<内容>" --format json`。
2. **@ 人（必须）**：需要 @ 时，`--text` 中**必须**先包含对应 `@userId` / `@手机号` / `@10`，再配合 `--at-users` / `--at-mobiles` / `--at-all`；否则 @ 不生效。

**禁止**：只传 `--at-users` 而 `--text` 里不含 `@<标识>`。

### SOP-4 共同群查询（search-common-group）

**触发**："我和 XX 的共同群"。

1. **取昵称（必须）**：先 `dws contact user get-self --format json` 取自己昵称；对方昵称从历史/上下文取，拿不到必须先问用户。
2. **执行（必须）**：`dws chat search-common --nicks "<昵称1>,<昵称2>" --limit 20 --cursor 0 --format json`；`hasMore=true` 时**必须**用 `nextCursor` 翻页，不要停在第一页。

**禁止**：跳过昵称解析、忽略 `hasMore` 不翻页。

### SOP-5 红点 / 未读管理（manage-red-point）

**触发**：标记未读/清除红点/全部已读。

1. **执行（必须）**：标记某会话未读 `dws chat mark-unread --conversation-id <openConversationId> --format json`；清除某会话红点 `dws chat clear-red-point --conversation-id <openConversationId> --format json`；全部已读 `dws chat clear-all-red-point --format json`。
2. **取会话 ID（必须）**：`openConversationId` 拿不准时先 `dws chat group list-all --format json` 或 `dws chat search --query "<群名>" --format json`，**禁止**编造。

**禁止**：未确认会话就批量"全部已读"（破坏性，必须先与用户确认）。

### SOP-6 特别关注消息（focus-messages）

**触发**："特别关注的人最近发了什么/聊了什么"。

1. **执行（必须）**：`dws chat message list-focused --limit 50 --format json`，直接基于返回答复。
2. **边界（必须）**：只有用户终点是"我关注了谁"这种**人员列表**时，才切 `dingtalk-contact` 关系查询。

**禁止**：用普通 `message list` 冒充 focused、把人员列表需求硬塞进 chat。

## 跨产品协作

- 收件人是人名 → 先用 `dingtalk-contact` 或 `dingtalk-aisearch` 拿 `openDingTalkId` / `userId`
- 要发图片/文件 → 先 `dt_media_upload` 上传 → `python scripts/extract_media_id.py "<URL>"` 提取 mediaId → 再用 `--media-id`
- 紧急升级（应用内/短信/电话）→ 切到 `dingtalk-ding`
- 发邮件 → 切到 `dingtalk-mail`
## 局部意图与 Recipe

- [局部意图消歧](references/intent-guide.md)；[Lite Recipe](references/lite-recipes.md)。

