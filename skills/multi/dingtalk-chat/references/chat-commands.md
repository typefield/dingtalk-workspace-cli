# 命令总览

## 命令总览

### group (群组管理)

#### 创建群 — 支持内部群/外部群/普通群/话题圈，当前登录用户自动成为群主

创建一个群聊，支持指定群名称、初始成员列表、群类型和话题模式。默认创建内部群。

```
Usage:
  dws chat group create [flags]
Example:
  dws chat group create --name "Q1 项目冲刺群" --users userId1,userId2,userId3
  dws chat group create --name "外部合作群" --users userId1,userId2 --type EXTERNAL
  dws chat group create --name "话题圈" --users userId1,userId2 --thread
  # 指定他人为群主
  dws chat group create --name "项目群" --users userId1,userId2 --owner <openDingTalkId>
Flags:
      --name string     群名称 (必填)
      --users string    成员 userId 或 openDingTalkId（可混传），用户本身会自动加入无需包含，逗号分隔，不超过20个 (必填)
      --type string     群类型: INTERNAL(内部群,默认)/EXTERNAL(外部群)/NORMAL(普通群)
      --thread          开启话题模式，将创建话题圈
      --owner string    群主 openDingTalkId（可选，未指定时默认操作者为群主）
```

#### 查看群成员列表 — 分页查询指定群聊的成员
```
Usage:
  dws chat group members [flags]
Example:
  dws chat group members --id <openconversation_id>
Flags:
      --cursor string   分页游标，首次从 0 开始
      --id string       群 ID / openconversation_id (必填)
```

#### 添加群成员 — 向指定群聊添加成员，需传入群 ID 与用户 ID 列表
```
Usage:
  dws chat group members add [flags]
Example:
  dws chat group members add --id <openconversation_id> --users userId1,userId2
Flags:
      --id string      群 ID / openconversation_id (必填)
      --users string   要添加的用户 userId 列表，逗号分隔 (必填)
```

#### 移除群成员 — 从指定群聊中移除成员，需传入群 ID 与待移除的用户 ID 列表
```
Usage:
  dws chat group members remove [flags]
Example:
  dws chat group members remove --id <openconversation_id> --users userId1,userId2
Flags:
      --id string      群 ID / openconversation_id (必填)
      --users string   要移除的用户 userId 列表，逗号分隔 (必填)
```

#### 将机器人添加到群中 — 将自定义机器人添加到当前用户有管理权限的群聊中，如果没有权限则会报错
```
Usage:
  dws chat group members add-bot [flags]
Example:
  dws chat group members add-bot --robot-code <robot-code> --id <openconversation_id>
Flags:
      --id string           群聊 openConversationId (必填)
      --robot-code string   机器人 Code (必填)
```

#### 从群内移除机器人 — 将指定机器人从群聊中移除，需要群管理员或群主权限
```
Usage:
  dws chat group members remove-bot [flags]
Example:
  dws chat group members remove-bot --id <openConversationId> --bot-id <openBotId>
  # 查询群 ID: dws chat search --query "群名"
  # 查询群内机器人: dws chat group bots --group <openConversationId>
Flags:
      --id string       群聊 openConversationId (必填)
      --bot-id string   机器人 openBotId (必填)
```

#### 批量查看群成员详情 — 根据成员 openDingTalkId 列表批量查询群成员详情信息
```
Usage:
  dws chat group members list-by-ids [flags]
Example:
  dws chat group members list-by-ids --id <openConversationId> --users openDingTalkId1,openDingTalkId2
  # 查询群 ID: dws chat search --query "群名"
  # 查询 openDingTalkId: dws contact user search --query "姓名"
Flags:
      --id string      群 ID / openConversationId (必填)
      --users string   成员 openDingTalkId 列表，逗号分隔 (必填)
```

#### 更新群名称
```
Usage:
  dws chat group rename [flags]
Example:
  dws chat group rename --id <openconversation_id> --name "新群名"
Flags:
      --id string     群 ID / openconversation_id (必填)
      --name string   修改后的群名称 (必填)
```

#### 根据群号获取群聊信息 — 当用户只提供了数字群号而非 openConversationId 时，用此命令转换
```
Usage:
  dws chat group get-by-group-id [flags]
Example:
  dws chat group get-by-group-id --group-id 12345678
  # 群号为数字类型的群ID
Flags:
      --group-id int   群号 (必填，数字类型)
```

#### 转让群主 — 将群主身份转让给群内其他成员
```
Usage:
  dws chat group transfer-owner [flags]
Example:
  dws chat group transfer-owner --group <openConversationId> --new-owner <openDingTalkId>
  dws chat group transfer-owner --group <openConversationId> --user <userId>
  # 查询群 ID: dws chat search --query "群名"
  # 查询人员: dws contact user search --keyword "姓名" --format json
Flags:
      --group string       群聊 openConversationId (必填)
      --new-owner string   新群主 openDingTalkId
      --user string        新群主 userId
```

#### 获取群邀请链接 — 获取指定群聊的邀请加入链接

可选 --expires-seconds 指定链接有效期（秒），0 表示永久有效，不传则使用服务端默认值。
```
Usage:
  dws chat group invite-url [flags]
Example:
  dws chat group invite-url --group <openConversationId>
  dws chat group invite-url --group <openConversationId> --expires-seconds 86400
  dws chat group invite-url --group <openConversationId> --expires-seconds 0
  # 查询群 ID: dws chat search --query "群名"
Flags:
      --group string            群聊 openConversationId (必填)
      --expires-seconds int64   链接有效期（秒），0 表示永久有效，不传使用服务端默认值
```

#### 分享群聊链接到会话 — 将指定群的邀请链接分享到另一个会话或单聊用户

--target 和 --receiver 二选一：--target 指定目标会话，--receiver 指定单聊用户。
```
Usage:
  dws chat group share-invite [flags]
Example:
  dws chat group share-invite --source <被分享群openConversationId> --target <目标会话openConversationId>
  dws chat group share-invite --source <被分享群openConversationId> --receiver <接收者openDingTalkId>
  dws chat group share-invite --source <openConversationId> --target <openConversationId> --expires-seconds 86400
  # 查询群 ID: dws chat search --query "群名"
Flags:
      --source string            被分享群的 openConversationId (必填)
      --target string            接收分享消息的会话 openConversationId（与 --receiver 二选一）
      --receiver string          接收分享消息的单聊用户 openDingTalkId（与 --target 二选一）
      --expires-seconds int64    链接有效期（秒），0 表示永久有效，不传使用服务端默认值
      --uuid string              消息幂等键（可选）
```

#### 退出群聊 — 当前用户退出指定群聊
```
Usage:
  dws chat group quit [flags]
Example:
  dws chat group quit --group <openConversationId>
  # 查询群 ID: dws chat search --query "群名"
Flags:
      --group string   群聊 openConversationId (必填)
```

#### 更新群头像 — 更新指定群聊的群头像
```
Usage:
  dws chat group update-icon [flags]
Example:
  dws chat group update-icon --group <openConversationId> --icon-media-id <mediaId>
  # 查询群 ID: dws chat search --query "群名"
Flags:
      --group string          群聊 openConversationId (必填)
      --icon-media-id string  群头像 mediaId (必填)
```

#### 更新群设置 — 更新指定群聊的设置项

--setting-key 指定设置项，--status 指定值（0=关闭，1=开启）。

支持的 settingKey:
  authority、joinValidation、onlyAdminCanAtAll、searchable、addFriendForbidden、
  toolbarStatus、pluginCustomizeVerify、onlyAdminCanDING、allMembersCanCreateMcsConf、
  onlyAdminCanSetMsgTop、onlyAdminCanPinMsg、onlyAdminCanSendFile、
  allMembersCanCreateCalendar、groupEmailDisabled、groupRedEnvelopeSwitch、
  groupLiveAuthority、groupBillAuthority
```
Usage:
  dws chat group update-settings [flags]
Example:
  dws chat group update-settings --group <openConversationId> --setting-key searchable --status 1
  dws chat group update-settings --group <openConversationId> --setting-key onlyAdminCanAtAll --status 0
  # 查询群 ID: dws chat search --query "群名"
Flags:
      --group string        群聊 openConversationId (必填)
      --setting-key string  群设置项 key (必填)
      --status int          设置值: 0=关闭, 1=开启 (必填)
```

#### 设置用户在群内的群昵称 — 设置当前用户在指定群聊内的个人群昵称
```
Usage:
  dws chat group update-nick [flags]
Example:
  dws chat group update-nick --group <openConversationId> --nick "我的群昵称"
  # 查询群 ID: dws chat search --query "群名"
Flags:
      --group string   群聊 openConversationId (必填)
      --nick string    个人群昵称 (必填)
```

#### 设置群备注 — 设置当前用户对指定群聊的备注名称（仅自己可见）
```
Usage:
  dws chat group update-alias [flags]
Example:
  dws chat group update-alias --group <openConversationId> --alias-title "项目A群"
  # 查询群 ID: dws chat search --query "群名"
Flags:
      --group string         群聊 openConversationId (必填)
      --alias-title string   群备注标题 (必填)
```

#### 查看群内所有机器人 — 获取指定群聊中的所有机器人列表
```
Usage:
  dws chat group bots [flags]
Example:
  dws chat group bots --group <openConversationId>
  # 查询群 ID: dws chat search --query "群名"
Flags:
      --group string   群聊 openConversationId (必填)
```

#### 解散群聊 — 解散指定群聊，操作不可逆，需要群主权限
```
Usage:
  dws chat group dismiss [flags]
Example:
  dws chat group dismiss --group <openConversationId>
  # 查询群 ID: dws chat search --query "群名"
Flags:
      --group string   群聊 openConversationId (必填)
```

#### 设置新成员入群可查看历史消息选项 — 控制新加入成员可见的历史消息范围
```
Usage:
  dws chat group set-history [flags]
Example:
  dws chat group set-history --group <openConversationId> --option RECENT_100
  dws chat group set-history --group <openConversationId> --option FORBIDDEN
  # 查询群 ID: dws chat search --query "群名"
Flags:
      --group string    群聊 openConversationId (必填)
      --option string   可见范围: FORBIDDEN | RECENT_100 | ALL (必填)

注意:
  - FORBIDDEN：禁止查看历史消息（默认安全策略）
  - RECENT_100：可查看最近 100 条消息（最常用）
  - ALL：可查看全部历史消息（开放性最高）
```

#### 拉取我创建/管理的群 — 查询当前用户作为群主或管理员的群列表

> 重要限制：此命令只能拉取到用户作为「群主」或「管理员」的群，无法获取普通成员身份的群聊。如果用户要找的群不在结果中，必须改用 `dws chat search --query "关键词"` 按名称搜索。

可通过 --role 过滤角色：OWNER 仅群主、ADMIN 仅管理员，不传则返回全部。可通过 --limit 限制返回数量，不传则返回所有符合条件的群。
```
Usage:
  dws chat group list-my-groups [flags]
Example:
  dws chat group list-my-groups
  dws chat group list-my-groups --role OWNER
  dws chat group list-my-groups --role ADMIN --limit 10
Flags:
      --role string    角色过滤: OWNER(仅群主) / ADMIN(仅管理员)，不传返回全部
      --limit int      最多返回群数量，不传返回全部
      --exclude-muted  是否排除已设置免打扰的群聊（默认 false）

注意:
  - 底层先拉取最近 1000 条会话，剔除单聊和话题圈后筛选出群主/管理员的群
  - 内部群会校验 orgId 归属
  - 不传 --role 时返回群主 + 管理员的所有群
```

#### 发布群公告 — 在指定群聊中发布群公告，支持 Markdown、定时发布

正文为 Markdown 格式。支持标题、加粗、斜体、删除线、行内代码、链接、代码块、有序/无序/任务列表、表格、引用、分割线、图片、段落、换行。
```
Usage:
  dws chat group notice create [flags]
Example:
  dws chat group notice create --group <openConversationId> --content "今晚 22 点系统维护，请提前保存工作内容"
  dws chat group notice create --group <openConversationId> --content "# 重要通知\n请大家查收" --sticky --send-ding
  dws chat group notice create --group <openConversationId> --content "明早九点例会" --run-at "2026-07-03T09:00:00+08:00"
  # 查询群 ID: dws chat search --query "群名"
Flags:
      --group string       群聊 openConversationId (必填)
      --content string     公告正文，Markdown 格式 (必填)
      --sticky             是否吊顶置顶（默认 false）
      --send-ding          是否发 DING 提醒（默认 false）
      --run-at string      定时发布时间 ISO-8601（如 2026-07-03T09:00:00+08:00，传入则定时发布）

注意:
  - 下划线/字体色/背景色/字号为编辑器专属能力，Markdown 无对应语法，无法通过本命令表达
  - --run-at 建议带时区偏移，不带偏移时按北京时区处理
```

#### 修改群公告 — 整体替换指定群公告的内容
```
Usage:
  dws chat group notice edit [flags]
Example:
  dws chat group notice edit --group <openConversationId> --notice-id <dataId> --content "更新后的公告内容"
  dws chat group notice edit --group <openConversationId> --notice-id <dataId> --content "更新后的公告内容" --sticky --send-ding
  # 查询公告 ID: dws chat group notice list --group <openConversationId>
Flags:
      --group string       群聊 openConversationId (必填)
      --notice-id string   群公告 dataId (必填)
      --content string     公告新正文，Markdown 格式 (必填)
      --sticky             是否吊顶置顶（不传按 false 处理）
      --send-ding          是否发 DING 提醒（默认 false）

注意:
  - 修改会整体替换原公告正文，需传入完整的新内容
```

#### 查看群公告详情 — 查询单条群公告的详细信息
```
Usage:
  dws chat group notice get [flags]
Example:
  dws chat group notice get --group <openConversationId> --notice-id <dataId>
  # 查询公告 ID: dws chat group notice list --group <openConversationId>
Flags:
      --group string       群聊 openConversationId (必填)
      --notice-id string   群公告 dataId (必填)

注意:
  - 返回正文摘要、吊顶状态、发布者、已读人数/应收人数、点赞/评论数、是否可编辑、是否已读、是否定时公告等信息
```

#### 查看群公告列表 — 分页拉取指定群聊的群公告列表
```
Usage:
  dws chat group notice list [flags]
Example:
  dws chat group notice list --group <openConversationId>
  dws chat group notice list --group <openConversationId> --limit 20 --cursor <nextPageCursor>
  dws chat group notice list --group <openConversationId> --scheduled
  # 查询群 ID: dws chat search --query "群名"
Flags:
      --group string    群聊 openConversationId (必填)
      --limit int       每页返回数量（默认 10，最大 100）
      --cursor string   分页游标（首次不传，翻页传返回的 nextPageCursor）
      --scheduled       是否查询定时公告列表（默认 false，查询已发布公告）

注意:
  - 分页: hasMore=true 时用返回的 nextPageCursor 作为下次 --cursor
  - 默认查询已发布公告，传 --scheduled 查询尚未到发布时间的定时公告
```

### group-role (群身份管理)

#### 查看群身份列表 — 拉取指定群聊的自定义群身份列表
```
Usage:
  dws chat group-role list [flags]
Example:
  dws chat group-role list --group <openConversationId>
Flags:
      --group string   群聊 openConversationId (必填)
```

#### 添加群身份 — 在指定群中创建一个新的自定义群身份
```
Usage:
  dws chat group-role add [flags]
Example:
  dws chat group-role add --group <openConversationId> --name "管理员"
Flags:
      --group string   群聊 openConversationId (必填)
      --name string    群身份名称 (必填)
```

#### 更新群身份名称 — 修改指定群身份的名称
```
Usage:
  dws chat group-role update [flags]
Example:
  dws chat group-role update --group <openConversationId> --role-id <openRoleId> --name "新名称"
Flags:
      --group string     群聊 openConversationId (必填)
      --role-id string   群身份 openRoleId，由 group-role list 返回 (必填)
      --name string      群身份新名称 (必填)
```

#### 删除群身份 — 删除指定群聊中的某个自定义群身份
```
Usage:
  dws chat group-role remove [flags]
Example:
  dws chat group-role remove --group <openConversationId> --role-id <openRoleId>
Flags:
      --group string     群聊 openConversationId (必填)
      --role-id string   群身份 openRoleId，由 group-role list 返回 (必填)
```

#### 设置用户群身份 — 覆盖指定用户在群中的全部群身份（传空则清除所有身份）
```
Usage:
  dws chat group-role set-user [flags]
Example:
  dws chat group-role set-user --group <openConversationId> --user <userId> --role-ids roleId1,roleId2
  # 查询人员: dws contact user search --keyword "姓名" --format json
  # 查询 role-id: dws chat group-role list --group <openConversationId>
Flags:
      --group string      群聊 openConversationId (必填)
      --user string       用户 userId（必填）
      --role-ids string   群身份 openRoleId 列表，逗号分隔 (必填)，传空字符串则清除该用户所有群身份
```

#### 移除用户的指定群身份 — 从用户身上移除指定的群身份（不影响其他群身份）
```
Usage:
  dws chat group-role remove-user [flags]
Example:
  dws chat group-role remove-user --group <openConversationId> --user <userId> --role-ids roleId1,roleId2
Flags:
      --group string      群聊 openConversationId (必填)
      --user string       用户 userId（必填）
      --role-ids string   要移除的群身份 openRoleId 列表，逗号分隔 (必填)
```

#### 查询群成员的群身份 — 查询指定群成员当前持有的所有群身份
```
Usage:
  dws chat group-role query-user [flags]
Example:
  dws chat group-role query-user --group <openConversationId> --user <userId>
Flags:
      --group string   群聊 openConversationId (必填)
      --user string    用户 userId（必填）
```

### search (搜索群聊)

#### 根据关键词搜索群聊 — 分页返回匹配群聊列表

hasMore=true 时用返回的 nextCursor 作为下次 --cursor 继续翻页。

**注意：**
1. query 不要拆分得太细，应使用群名称中连续的核心词作为关键词（如群名"项目冲刺群"应搜"项目冲刺"而非拆成"项目"+"冲刺"分别搜索）。
2. 当搜索结果返回多个群聊时，应列出候选群让用户确认目标群聊，不要自行假定并直接进行后续操作。

```
Usage:
  dws chat search [flags]
Example:
  dws chat search --query "项目冲刺"
  dws chat search --query "项目冲刺" --limit 20 --cursor 0
Flags:
      --query string   搜索关键词 (必填)
      --limit int        每页返回数量（默认 20）
      --cursor string    分页游标（默认 "0"，翻页传 nextCursor）
      --exclude-muted    是否排除已设置免打扰的群聊（默认 false）
```

### message (会话消息管理)

#### 拉取会话消息内容 — 拉取指定群聊或单聊的会话消息内容

--group 指定群聊，--user 指定单聊用户（通过 userId），--open-dingtalk-id 指定单聊用户（通过 openDingTalkId），三者互斥。推荐使用 --direction 控制时间方向：newer 表示从给定时间往现在拉，older 表示从给定时间往以前拉。--forward 为旧兼容参数：true 等价 newer，false 等价 older。hasMore=true 时用结果中的边界 createTime 作为下次 --time 翻页。
```
Usage:
  dws chat message list [flags]
Example:
  dws chat message list --group <openconversation_id> --time "2025-03-01 00:00:00"
  dws chat message list --user <userId> --time "2025-03-01 00:00:00" --limit 50
  dws chat message list --open-dingtalk-id <openDingTalkId> --time "2025-03-01 00:00:00" --limit 50
  dws chat message list --group <openconversation_id> --time "2025-03-01 00:00:00" --direction newer
  dws chat message list --group <openconversation_id> --time "2025-03-01 00:00:00" --direction older
Flags:
      --direction string         时间方向: newer=从给定时间往现在拉，older=从给定时间往以前拉（推荐）
      --forward                  旧兼容参数: true 等价 --direction newer，false 等价 --direction older (default true)
      --group string             群聊 openconversation_id（群聊时必填）
      --limit int                返回数量，不传则不限制
      --time string              开始时间，格式: yyyy-MM-dd HH:mm:ss (必填)
      --user string              单聊用户 userId（单聊时与 --open-dingtalk-id 二选一）
      --open-dingtalk-id string  单聊用户 openDingTalkId（单聊时与 --user 二选一，适用于三方应用等无法获取 userId 的场景）

注意:
  - --group、--user、--open-dingtalk-id 三者互斥，只需指定其一：群聊用 --group，单聊用 --user 或 --open-dingtalk-id
  - --user 和 --open-dingtalk-id 都是发起单聊消息拉取，区别在于用不同格式的用户标识：
    - --user 传 userId（企业内部应用常用）
    - --open-dingtalk-id 传 openDingTalkId（三方应用或跨组织场景常用，无法获取 userId 时使用）
  - --group 的别名: --id, --chat, --conversation-id (均可替代 --group)
  - 时间方向：优先使用 --direction newer/older；仅为兼容旧调用保留 --forward。两者同时传入且语义冲突时会报错
  - 翻页：hasMore=true 时，用结果中的边界 createTime 作为下次 --time
  - 话题圈消息拉取流程：如果返回的会话消息中包含 openConvThreadId 字段，说明是话题类消息。要获取完整的话题内容，需要两步操作：(1) 先通过 dws chat message list 拉取话题主消息（即话题帖子本身）；(2) 再调用 dws chat message list-topic-replies --group <openConversationId> --topic-id <openConvThreadId> 分页拉取该话题下的所有回复消息。只有话题主消息 + 回复列表合在一起，才是一条话题的完整内容。
```

#### 以当前用户身份发送消息 — --group 群聊 / --user 或 --open-dingtalk-id 单聊

**重要：该接口会真实发送消息到目标会话，不可用于测试或试探性调用。调用前必须确认消息内容和接收对象无误。**

--group 指定群聊 openConversationId 发群消息；--user 指定用户 userId 发单聊；--open-dingtalk-id 指定用户 openDingTalkId 发单聊。三者只能选其一，不能同时指定。纯文本/Markdown 单聊传 --user 时直接走 userId 发送能力，不需要先手动查询 openDingTalkId。推荐使用 --text flag 传递消息内容（也支持位置参数）。可选 --title 作为消息标题。
若用户只提供了数字群号而非 openConversationId，需先调用 `chat group get-by-group-id` 将群号转为 openConversationId，再传入 --group。
--群聊时可选 --at-all @所有人，或 --at-open-dingtalk-ids 指定成员（仅群聊时生效）。
**发送图片或本地文件 — 一条命令直发**

收到本地路径就直接传给 `--file-path`，CLI 自动完成上传与发送，**无需任何前置工具调用**（不要再走 `dt_media_upload` / `extract_media_id.py` / `drive upload` 任何前置链路）。`.png/.jpg/.pdf/.mp4/.zip` 等任意类型一律：

```bash
# 群聊
dws chat message send --group <openConversationId> --msg-type file --file-path /local/path/to/file.png --format json
# 单聊（推荐 --open-dingtalk-id，userId 也可）
dws chat message send --open-dingtalk-id <openDingTalkId> --msg-type file --file-path /local/path/to/file.png --format json
```

> 单条命令完成上传 + 发送，没有"先 dt_media_upload 再 send"两步流程。带文字说明再额外发一条 `--text "..."` 即可。

```
Usage:
  dws chat message send [flags] [<text>]

Example:
  # 文本/Markdown
  dws chat message send --group <openconversation_id> --text "hello"
  dws chat message send --user <userId> --text "请查收"
  dws chat message send --open-dingtalk-id <openDingTalkId> --text "请查收"
  dws chat message send --group <openconversation_id> --title "周报提醒" --text "请大家本周五前提交周报"
  # 幂等发送（24h 内相同 uuid 不重复投递）
  dws chat message send --group <openconversation_id> --text "hello" --uuid "unique-id-123"
  # 话题回复（拉消息返回的 openConvThreadId 作为 --group 传入）
  dws chat message send --group <openConvThreadId> --text "回复话题内容"
  # @ 群成员
  dws chat message send --group <openconversation_id> --at-all "<@all> 请大家注意"
  dws chat message send --group <openconversation_id> --at-open-dingtalk-ids odt1,odt2 "<@odt1> <@odt2> 请查收"
  # 发送本地图片（直接传路径，CLI 自动上传并发送）
  dws chat message send --group <openconversation_id> --msg-type file --file-path ./screenshot.png
  # 发送本地文件（同一条命令，任意扩展名）
  dws chat message send --group <openconversation_id> --msg-type file --file-path ./report.pdf
  # 旧链路（仅当上游已用 dt_media_upload 拿到 mediaId 时使用）
  dws chat message send --group <openconversation_id> --msg-type image --media-id <mediaId>
  # 发送位置消息（需先通过 dt_media_upload 上传地图缩略图获取 mediaId）
  dws chat message send --group <openconversation_id> --msg-type location --latitude <纬度> --longitude <经度> --location-name <地址名称> --map-thumbnail-url "@mediaId"
  # 分享联系人名片
  dws chat message send --group <openconversation_id> --msg-type profile --contact-id <openDingTalkId>
Flags:
      --text string              消息内容（推荐使用，也可用位置参数）
      --group string             群聊 openconversation_id（群聊时必填）
      --user string              单聊接收人 userId（单聊时与 --open-dingtalk-id 二选一）
      --open-dingtalk-id string  单聊接收人 openDingTalkId（单聊时与 --user 二选一）
      --title string             消息标题（可选，默认「消息」）
      --at-all                   @所有人（仅群聊时生效，可选，默认 false）
      --at-open-dingtalk-ids string  @指定成员的 openDingTalkId 列表，逗号分隔（仅群聊时生效，可选）
      --media-id string          图片 mediaId（仅旧链路：`--msg-type image` 时使用；新场景请直接用 `--msg-type file --file-path`）
      --msg-type string          消息类型: image/file/location/profile（推荐统一用 `file --file-path`；image+media-id 仅作旧链路兼容）
      --dentry-id int64          旧链路兼容：文件 dentryId（与 --space-id 成对传入时跳过自动上传）
      --space-id int64           旧链路兼容：空间 ID（与 --dentry-id 成对传入时跳过自动上传）
      --file-name string         旧链路兼容：文件名
      --file-type string         旧链路兼容：文件类型/扩展名
      --file-path string         本地文件路径（msgType=file 时可直接上传发送；旧链路中作为 content.filePath）
      --file-size int64          旧链路兼容：文件大小，单位字节
      --uuid string             幂等 UUID，相同 uuid 在 24h 内不会重复发送（可选）
      --latitude string          纬度，如 30.271321（msgType=location 时必填）
      --longitude string         经度，如 120.007878（msgType=location 时必填）
      --location-name string     地址名称，如 阿里集团-钉钉（msgType=location 时必填）
      --map-thumbnail-url string 地图缩略图 mediaId，格式 @mediaId（msgType=location 时必填，需先通过 dt_media_upload 上传图片获取）
      --contact-id string       要分享的联系人 openDingTalkId（msgType=profile 时必填）

注意:
  - --text 和位置参数二选一，--text 优先
  - --group、--user、--open-dingtalk-id 三者互斥，只需指定其一：群聊用 --group，单聊用 --user 或 --open-dingtalk-id
  - **话题回复**：话题圈拉消息（`chat message list`）返回的 openConvThreadId 可直接作为 --group 的值传入，即可往该话题内发送回复消息。注意 openConvThreadId 仅从消息列表接口的返回值中获取，禁止自行拼接或猜测
  - 纯文本/Markdown 单聊发送时 `--user` 和 `--open-dingtalk-id` 都可用；传 `--user` 时直接走 userId 发送能力
  - --group 的别名: --id, --chat, --conversation-id (均可替代 --group)
  - --at-all 和 --at-open-dingtalk-ids 仅在 --group 群聊时生效，单聊时无效；当设置--at-all时，消息内容中一定要包含对应的占位符<@all>;当设置--at-open-dingtalk-ids openDingTalkId1,openDingTalkId2时，消息内容中一定要包含对应格式的占位符<@openDingTalkId1> <@openDingTalkId2>
  - **换行符**：消息内容按 Markdown 渲染，换行有两层要求，缺一不可：
    1. 必须使用**真实换行符**（Unicode `U+000A`），而非字面量字符串 `\n`（反斜杠 + 字母 n）。程序或大模型构造参数时，须确保已正确反转义；否则全部内容会渲染在同一行
    2. Markdown 规范下**单个换行不产生换行效果**。需要换行时请使用：段落分隔（连续两个真实换行符 `\n\n`）、行尾两个空格 + 真实换行符（硬换行 `<br>`），或直接写 HTML 的 `<br>` 标签
  - **发图片/文件统一走 `--msg-type file --file-path <本地路径>`**（任意扩展名 png/jpg/pdf/mp4/zip…），CLI 内部一条命令完成上传 + 发送，无需 `dt_media_upload` / `extract_media_id` / `drive upload` 等前置步骤
  - 旧链路兼容：仅当上游已经通过 `dt_media_upload` 拿到 `@lQL...` 形式的 mediaId 时，才使用 `--msg-type image --media-id`；新代码与新指引一律用 file-path 路径
  - 富媒体消息的单聊优先使用 `--open-dingtalk-id`；传 `--user` 时 CLI 会尝试解析成 openDingTalkId 后发送
  - --uuid 用于幂等发送，传入相同 uuid 在 24h 内不会重复投递消息（可选，群聊和单聊均支持）
  - **发送位置消息**：`--msg-type location --latitude <纬度> --longitude <经度> --location-name <地址名称> --map-thumbnail-url @mediaId`；地图缩略图需先通过 `dt_media_upload` 上传图片获取 mediaId
  - **分享联系人名片**：`--msg-type profile --contact-id <openDingTalkId>`；将指定联系人的名片分享到群聊或单聊
```

### file (会话文件上传)

#### 上传本地文件或 URL 文件到会话文件空间 — 不暴露 spaceId

上传文件到指定会话关联的文件空间。调用方只需要提供会话和文件来源，不需要先调用 conversation-info，也不需要传递 spaceId。若只是发送本地文件，优先使用 `dws chat message send --msg-type file --file-path <本地文件>`。
```
Usage:
  dws chat file upload [flags]
Example:
  # 本地文件：CLI 会初始化上传、直传文件内容并提交
  dws chat file upload --group <openConversationId> --file ./report.pdf --format json
  dws chat file upload --user <userId> --file ./report.pdf --format json
  dws chat file upload --open-dingtalk-id <openDingTalkId> --file ./report.pdf --format json

  # URL 文件：服务端拉取 URL 并上传到会话文件空间
  dws chat file upload --group <openConversationId> --url https://example.com/report.pdf --file-name report.pdf --format json
Flags:
      --group string              群聊 openConversationId（群聊时使用）
      --user string               单聊对方 userId（单聊时使用）
      --open-dingtalk-id string   单聊对方 openDingTalkId（单聊时使用）
      --file string               本地文件路径（与 --url 二选一）
      --url string                远程文件 URL（与 --file 二选一，服务端代传）
      --file-name string          文件名（可选，本地文件默认取文件名，URL 默认从 URL 推断）
      --md5 string                文件 MD5（可选，本地文件不传时自动计算）
      --uuid string               幂等 UUID（可选）

注意:
  - --group、--user、--open-dingtalk-id 互斥，必须且只能指定其一
  - --file 和 --url 互斥，必须且只能指定其一
  - 本地文件由一个命令内部完成：获取上传链接 → CLI 直传文件内容 → 提交上传
  - URL 文件走服务端代传：服务端自行解析会话空间并上传到会话文件空间
  - 若只是发送本地文件，直接使用 `dws chat message send --msg-type file --file-path <本地文件>`，CLI 会复用同一套上传逻辑
  - 发图片/文件优先走 `chat message send --msg-type file --file-path <本地路径>`，无需调用 `chat file upload`；本节只在以下场景使用：(a) URL 文件由服务端代传 (`--url`)；(b) 业务需要先拿到下载链接再以 Markdown 形式内嵌到文字消息中
  - **文字 + 文件双消息**（仅适用于非图片文件；图片直接 `--msg-type file --file-path` 单条消息即可，不要走双发）：先发一条 Markdown 文字消息引用下载链接，再对每个文件各补发一条 `--msg-type file` 文件消息，确保接收方既看到文字说明又能直接下载原始文件
```

#### 查询消息发送状态 — 查询以当前用户身份发送的消息的发送状态

查询以当前用户身份发送的消息的发送状态。需要传入发送消息时返回的 openTaskId。
```
Usage:
  dws chat message query-send-status [flags]
Example:
  dws chat message query-send-status --open-task-id <openTaskId>
  # openTaskId 由 dws chat message send 返回
Flags:
      --open-task-id string   消息发送任务 ID (必填)

注意:
  - openTaskId 由 `dws chat message send` 发送消息成功后返回
  - 用于确认消息是否已成功发送或获取发送失败的原因
```

#### 撤回消息 — 撤回自己或他人的消息

撤回消息。支持撤回自己发送的消息，也支持在群聊中以群主/管理员身份撤回他人的消息。与 `recall-by-bot` 的区别：本命令通过 IM 接口撤回用户消息，`recall-by-bot` 通过机器人接口撤回机器人发出的消息（需要 robot-code + processQueryKey）。
```
Usage:
  dws chat message recall [flags]
Example:
  dws chat message recall --conversation-id <openConversationId> --msg-id <openMessageId>
  # 群主/管理员撤回他人消息：直接传目标消息 ID 即可
  # 查询会话 ID: dws chat search --query "群名"
  # 消息 ID 可通过 dws chat message list 获取
Flags:
      --conversation-id string   会话 openConversationId (必填，支持单聊/群聊，别名: --group / --id / --chat)
      --msg-id string            消息 openMessageId (必填)

注意:
  - --conversation-id 的别名: --group, --id, --chat (均可替代 --conversation-id)
  - 消息 ID 可通过 `dws chat message list` 命令获取
  - 支持撤回自己发出的消息，也支持群主/管理员撤回群内他人的消息
  - 与 `recall-by-bot` 的区别：本命令通过 IM 接口撤回用户消息（需要 openConversationId + openMessageId），`recall-by-bot` 通过机器人接口撤回机器人发出的消息（需要 robot-code + processQueryKey）
```

#### 机器人发送消息（--group 群聊 / --users 单聊）

**重要：该接口会真实发送消息到目标会话，不可用于测试或试探性调用。调用前必须确认消息内容和接收对象无误。**

群聊：传 --group 指定群；单聊：传 --users 指定用户列表，二者只能选其一，不能同时指定。--text 支持 Markdown。群聊时可选 --at-user-ids @指定成员。

如果用户明确要求"用机器人/机器人身份/robot"发送，必须使用本命令，严禁改用 `chat message send` 以当前用户身份发送。

**重要**：机器人发群消息前，必须确认该机器人已在目标群中。若机器人不在群内会报错"机器人不存在"，需先执行 `dws chat group members add-bot --id <openConversationId> --robot-code <robot-code>` 将机器人加入群聊后再发送。
```
Usage:
  dws chat message send-by-bot [flags]
Example:
  dws chat message send-by-bot --robot-code <robot-code> --group <openconversation_id> --title "日报" --text "## 今日完成..."
  dws chat message send-by-bot --robot-code <robot-code> --users userId1,userId2 --title "提醒" --text "请提交周报"
  dws chat message send-by-bot --robot-code <robot-code> --open-dingtalk-ids openDingtalkId1,openDingtalkId2 --title "提醒" --text "请提交周报"
  dws chat message send-by-bot --robot-code <robot-code> --group <openconversation_id> --at-user-ids userId1,userId2 --title "提醒" --text "@userId1 @userId2 请查收本周报告"
  dws chat message send-by-bot --robot-code <robot-code> --group <openconversation_id> --at-open-dingtalk-ids openDingtalkId1,openDingtalkId2 --title "提醒" --text "@openDingtalkId1 @openDingtalkId2 请查收本周报告"
  dws chat message send-by-bot --robot-code <robot-code> --group <openconversation_id> --at-all --title "通知" --text "请所有人注意"
Flags:
      --group string                 群聊 openConversationId（群聊时必填）
      --robot-code string            机器人 Code (必填)
      --text string                  消息内容 Markdown (必填)
      --title string                 消息标题 (必填)
      --users string                 用户 userId 列表，逗号分隔，最多20个（单聊时必填）
      --open-dingtalk-ids string     用户 openDingtalkId 列表，逗号分隔（单聊时可替代 --users，可选）
      --at-user-ids string           @指定成员的 userId 列表，逗号分隔（仅群聊时生效，可选）
      --at-open-dingtalk-ids string  @指定成员的 openDingtalkId 列表，逗号分隔（仅群聊时生效，可选）
      --at-all                        @所有人（可选），服务端接收字符串 true/false

注意:
  - 用户明确要求机器人发送时，必须使用 `chat message send-by-bot`；严禁使用 `chat message send` 以用户身份代发
  - --group 与 --users/--open-dingtalk-ids 互斥，必须且只能指定其一
  - --group 的别名: --id, --chat, --conversation-id (均可替代 --group)
  - --at-user-ids 仅在 --group 群聊时生效，单聊时无效；设置时 --text 中需包含 @userId 对应文本
  - --at-open-dingtalk-ids 仅在 --group 群聊时生效，单聊时无效；设置时 --text 中需包含 @openDingtalkId 对应文本
  - --at-all @所有人，仅群聊时生效；只需带上 --at-all flag 即可，服务端会自动处理
  - userId 获取方式：`dws contact user search --query "姓名"` 搜人获取 userId
  - **换行符**：--text 按 Markdown 渲染，换行规则同 `chat message send`：
    1. 必须使用**真实换行符**（`U+000A`），而非字面量 `\n`，否则全部内容会渲染在同一行
    2. 单个换行不产生换行效果，需用空行（`\n\n`）做段落分隔，或行尾两空格 + 换行/`<br>` 做硬换行
```

#### 机器人撤回消息（--group 群聊 / 不传为单聊）

群聊：传 --group 与 --keys；单聊：仅传 --keys。--keys 为发送时返回的 processQueryKey 列表，逗号分隔。
```
Usage:
  dws chat message recall-by-bot [flags]
Example:
  dws chat message recall-by-bot --robot-code <robot-code> --group <openconversation_id> --keys <process-query-key>
  dws chat message recall-by-bot --robot-code <robot-code> --keys key1,key2
Flags:
      --group string         群聊 openConversationId（群聊撤回时必填）
      --keys string         消息 processQueryKey 列表，逗号分隔 (必填)
      --robot-code string   机器人 Code (必填)
```

#### 自定义机器人 Webhook 发送群消息

@ 人时需在 --text 中包含 @userId 或 @手机号，否则 @ 不生效；@所有人时需在 --text 中包含 @10 并带上 --at-all。
```
Usage:
  dws chat message send-by-webhook [flags]
Example:
  dws chat message send-by-webhook --token <webhook-token> --title "告警" --text "CPU 超 90% @10" --at-all
  dws chat message send-by-webhook --token <webhook-token> --title "test" --text "hi @118785" --at-users 118785
Flags:
      --at-all              @ 所有人（需在 --text 中包含 @10）
      --at-mobiles string   @ 指定手机号，逗号分隔
      --at-users string     @ 指定用户，逗号分隔（需在 text 中包含 @userId）
      --text string         消息内容 (必填)
      --title string        消息标题 (必填)
      --token string        Webhook Token (必填)

注意:
  - **换行符**：--text 按 Markdown 渲染，换行规则同 `chat message send`：
    1. 必须使用**真实换行符**（`U+000A`），而非字面量 `\n`，否则全部内容会渲染在同一行
    2. 单个换行不产生换行效果，需用空行（`\n\n`）做段落分隔，或行尾两空格 + 换行/`<br>` 做硬换行
```

#### 拉取群话题回复消息列表

查询指定群聊中某条话题消息的全部回复。--group 指定群会话 ID，--topic-id 指定话题 ID（由 dws chat message list 返回）。
```
Usage:
  dws chat message list-topic-replies [flags]
Example:
  dws chat message list-topic-replies --group <openconversation_id> --topic-id <topicId>
  dws chat message list-topic-replies --group <openconversation_id> --topic-id <topicId> --time "2025-03-01 00:00:00" --limit 20
  dws chat message list-topic-replies --group <openconversation_id> --topic-id <topicId> --time "2025-03-01 00:00:00" --direction newer
Flags:
      --group string      群会话 openconversationId (必填)
      --topic-id string   话题 ID，由 dws chat message list 返回 (必填)
      --time string       开始时间，格式: yyyy-MM-dd HH:mm:ss（可选）
      --limit int         返回数量（默认 50）
      --direction string  时间方向: newer=从给定时间往现在拉，older=从给定时间往以前拉（推荐，默认 older）
      --forward           旧兼容参数: true 等价 --direction newer，false 等价 --direction older（默认 false）
```

#### 拉取指定时间范围内当前用户的所有会话消息 — 分页拉取当前登录用户在指定时间范围内的所有会话消息

--start 和 --end 限定时间范围，--limit 指定每页数量，--cursor 传分页游标（首页传 "0"，后续从响应中的 nextCursor 获取）。服务端按 cursor 分页返回，hasMore=true 时用返回的 nextCursor 值作为下次 --cursor 继续翻页。
```
Usage:
  dws chat message list-all [flags]
Example:
  dws chat message list-all --start "2025-03-01 00:00:00" --end "2025-03-31 23:59:59" --limit 50
  dws chat message list-all --start "2025-03-01 00:00:00" --end "2025-03-31 23:59:59" --limit 50 --cursor "abc123token"
Flags:
      --start string         起始时间，格式: yyyy-MM-dd HH:mm:ss (必填)
      --end string           结束时间，格式: yyyy-MM-dd HH:mm:ss (必填)
      --limit int            每页返回数量（默认 50）
      --cursor string       分页游标（首页传 "0"，后续从响应中的 nextCursor 获取）

注意:
  - 四个参数每次请求都会传递给服务端，cursor 首页传 "0"
  - 与 chat message list 的区别：list 拉取指定单个会话（群聊或单聊）的消息，list-all 拉取当前用户所有会话的消息
  - 翻页：hasMore=true 时，用响应中的 nextCursor 值作为下次 --cursor 参数继续翻页
  - 时间格式统一为 yyyy-MM-dd HH:mm:ss
```

#### 拉取指定发送者的消息 — 搜索特定人发送给我的消息（包含单聊和群聊）

> 推荐优先使用 `chat message search-advanced --user/--users`（userId）或 `--sender-ids`（openDingTalkId），它还能叠加关键词/群/at 等过滤条件。本命令保留给需要旧 list-by-sender 返回结构的场景。

搜索特定人发送给我的消息，返回结果包含单聊和群聊标识。--sender-user-id 指定发送者 userId，--sender-open-dingtalk-id 指定发送者 openDingTalkId，二者互斥。分页参数 --limit（默认 50）和 --cursor（默认 "0"）始终传递；hasMore=true 时用返回的 nextCursor 作为下次 --cursor 继续翻页。
```
Usage:
  dws chat message list-by-sender [flags]
Example:
  dws chat message list-by-sender --sender-user-id <userId> --start "2026-03-10T00:00:00+08:00" --end "2026-03-11T00:00:00+08:00" --limit 50 --cursor 0
  dws chat message list-by-sender --sender-open-dingtalk-id <openDingTalkId> --start "2026-03-10T00:00:00+08:00" --end "2026-03-11T00:00:00+08:00" --limit 50 --cursor 0
  dws chat message list-by-sender --sender-user-id <userId> --start "2026-03-10T00:00:00+08:00" --end "2026-03-10T23:59:59+08:00" --limit 20 --cursor 0
  dws chat message list-by-sender --sender-open-dingtalk-id <openDingTalkId> --start "2026-03-10T00:00:00+08:00" --end "2026-03-11T00:00:00+08:00" --limit 50 --cursor <nextCursor>
Flags:
      --sender-user-id string                发送者 userId（与 --sender-open-dingtalk-id 二选一）
      --sender-open-dingtalk-id string        发送者 openDingTalkId（与 --sender-user-id 二选一，适用于无法获取 userId 的场景）
      --start string                          开始时间，ISO-8601 格式 (必填)
      --end string                            结束时间，ISO-8601 格式 (必填)
      --limit int                             每页返回数量（默认 50）
      --cursor string                         分页游标（默认 "0"，翻页传 nextCursor）

注意:
  - --sender-user-id 和 --sender-open-dingtalk-id 二者互斥，必须且只能指定其一：
    - --sender-user-id 传 userId（企业内部应用常用）
    - --sender-open-dingtalk-id 传 openDingTalkId（三方应用或跨组织场景常用，无法获取 userId 时使用）
  - openDingTalkId 获取方式见下方「openDingTalkId 获取方式」小节
  - 不需要指定单聊/群聊，返回结果自带会话类型标识
  - 时间支持多种 ISO-8601 格式，如 "2026-03-10T00:00:00+08:00"、"2026-03-10 14:00:00"、"2026-03-10" 等
  - 翻页：hasMore=true 时，用返回的 nextCursor 作为下次 --cursor
```

#### 拉取 @我 的消息 — 搜索时间范围内 @我 的消息

> 推荐使用 `chat message search-advanced --at-me`，它还能叠加关键词/群/发送者等过滤条件。本命令适用于仅需拉取 @我 消息的简单场景。

搜索时间范围内 @我 的消息，可选指定群聊。返回结果包含单聊和群聊标识。分页参数 --limit（默认 50）和 --cursor（默认 "0"）始终传递；hasMore=true 时用返回的 nextCursor 作为下次 --cursor 继续翻页。
```
Usage:
  dws chat message list-mentions [flags]
Example:
  dws chat message list-mentions --start "2026-03-10T00:00:00+08:00" --end "2026-03-11T00:00:00+08:00" --limit 50 --cursor 0
  dws chat message list-mentions --start "2026-04-01T00:00:00+08:00" --end "2026-04-14T00:00:00+08:00" --limit 20 --cursor 0
  dws chat message list-mentions --group <openconversation_id> --start "2026-03-10T00:00:00+08:00" --end "2026-03-11T00:00:00+08:00" --limit 50 --cursor 0
  dws chat message list-mentions --start "2026-03-10T00:00:00+08:00" --end "2026-03-11T00:00:00+08:00" --limit 50 --cursor <nextCursor>
Flags:
      --group string    群聊 openconversation_id（可选，不传则查全部）
      --start string    开始时间，ISO-8601 格式 (必填)
      --end string      结束时间，ISO-8601 格式 (必填)
      --limit int       每页返回数量（默认 50）
      --cursor string   分页游标（默认 "0"，翻页传 nextCursor）

注意:
  - --group 可选，不传则查询所有会话中 @我 的消息；传入则只查指定群聊
  - --group 的别名: --id, --chat, --conversation-id (均可替代 --group)
  - 时间支持多种 ISO-8601 格式，如 "2026-03-10T00:00:00+08:00"、"2026-03-10 14:00:00"、"2026-03-10" 等
  - 翻页：hasMore=true 时，用返回的 nextCursor 作为下次 --cursor
```

#### 拉取特别关注人的消息

拉取当前用户特别关注人的消息。分页参数 --limit 指定每页数量，--cursor 传分页游标（首次不传或传 0）。返回结果中 hasMore=true 时用 nextCursor 作为下次 --cursor 继续翻页。
```
Usage:
  dws chat message list-focused [flags]
Example:
  dws chat message list-focused --limit 50
  dws chat message list-focused --limit 20 --cursor <nextCursor>
Flags:
      --limit int       每页返回数量（默认 50）
      --cursor int64    分页游标（首次不传或传 0，翻页传 nextCursor）

注意:
  - 首次调用不传 --cursor 或传 0，后续翻页传 nextCursor
```

#### 获取未读会话列表

获取当前用户有未读消息的会话信息。可选通过 `--count` 限制返回条数，不传则使用服务端默认值。
```
Usage:
  dws chat message list-unread-conversations [flags]
Example:
  dws chat message list-unread-conversations
  dws chat message list-unread-conversations --count 20
Flags:
      --count int        返回未读会话条数（可选）
      --exclude-muted    是否排除已设置免打扰的会话（默认 false）
```

#### 查询消息的已读/未读状态

查询指定会话中消息的已读/未读状态（仅消息发送者可查询自己发出的消息）。--conversation-id 指定会话 openConversationId（群聊或单聊均可），--message-id 指定消息 ID（由 dws chat message list 返回的 openMessageId，必须是当前用户发送的消息）。目标用户 userId 使用 --user/--users；目标用户 openDingTalkId 使用 --target-open-dingtalk-ids；不传目标用户则返回所有接收者的状态。
```
Usage:
  dws chat message read-status [flags]
Example:
  dws chat message read-status --conversation-id <openConversationId> --message-id <openMessageId>
  dws chat message read-status --conversation-id <openConversationId> --message-id <openMessageId> --user userId1,userId2
  dws chat message read-status --conversation-id <openConversationId> --message-id <openMessageId> --users userId1,userId2
  dws chat message read-status --conversation-id <openConversationId> --message-id <openMessageId> --target-open-dingtalk-ids openDingTalkId1,openDingTalkId2
Flags:
      --conversation-id string              会话 openConversationId (必填，群聊或单聊均可)
      --message-id string                   消息 openMessageId，由 chat message list 返回 (必填，必须是当前用户发送的消息)
      --user string                         目标用户 userId，支持逗号分隔（可选，不传则查所有接收者）
      --users string                        目标用户 userId 列表，逗号分隔（可选，不传则查所有接收者）
      --target-open-dingtalk-ids string     目标用户 openDingTalkId 列表，逗号分隔（可选，不传则查所有接收者）

注意:
  - 仅消息发送者可查询自己发出的消息的已读/未读状态，查询他人发的消息会报错
  - --conversation-id 的别名: --group, --id, --chat (均可替代 --conversation-id)
  - --message-id 从 dws chat message list 返回的消息列表中获取（字段名 openMessageId）
  - --user / --users 传目标用户 userId
  - --target-open-dingtalk-ids 不传时返回该消息所有接收者的已读状态；传入则只返回指定 openDingTalkId 用户的状态
```

#### 按关键词搜索消息 — 在当前用户的会话中按关键词搜索消息

> 推荐优先使用 `chat message search-advanced`，它是本命令的严格超集：query 可选（非必填）、支持多个会话（非单个）、还能叠加发送者/at 等维度过滤。

按关键词搜索消息内容。--query 指定搜索关键词（必填）。可选 --group 限定搜索某个会话，不传则搜索所有会话。时间参数 --start/--end（ISO-8601）限定搜索时间范围。分页参数 --limit（默认 100）和 --cursor（默认 "0"）始终传递；hasMore=true 时用返回的 nextCursor 作为下次 --cursor 继续翻页。
```
Usage:
  dws chat message search [flags]
Example:
  dws chat message search --query "changefree" --start "2026-04-01T00:00:00+08:00" --end "2026-04-15T00:00:00+08:00" --limit 50 --cursor 0
  dws chat message search --query "codereview" --group <openconversation_id> --start "2026-04-01T00:00:00+08:00" --end "2026-04-15T00:00:00+08:00" --limit 100 --cursor 0
  dws chat message search --query "链接" --start "2026-04-15T00:00:00+08:00" --end "2026-04-16T00:00:00+08:00" --limit 100 --cursor <nextCursor>
Flags:
      --query string   搜索关键词 (必填)
      --group string     群聊 openconversation_id（可选，不传则搜索所有会话）
      --start string     开始时间，ISO-8601 格式 (必填)
      --end string       结束时间，ISO-8601 格式 (必填)
      --limit int        每页返回数量（默认 100）
      --cursor string    分页游标（默认 "0"，翻页传 nextCursor）

注意:
  - --group 可选，不传则搜索所有会话中的消息；传入则只搜索指定会话
  - --group 的别名: --id, --chat, --conversation-id (均可替代 --group)
  - 时间支持多种 ISO-8601 格式，如 "2026-03-10T00:00:00+08:00"、"2026-03-10 14:00:00"、"2026-03-10" 等
  - 翻页：hasMore=true 时，用返回的 nextCursor 作为下次 --cursor
```

#### 多维度搜索消息（推荐首选） — 支持按关键词、发送者、@我、@指定人、指定会话、时间范围、消息类型、会话类型等多维度搜索

> 推荐：这是消息搜索的首选接口。它可以完全替代 `chat message search`（query 可选 vs 必填，支持多个会话 vs 单个），大部分替代 `chat message list-by-sender`（通过 --user/--users 按 userId 搜索发送者，或通过 --sender-ids 按 openDingTalkId 搜索）和 `chat message list-mentions`（通过 --at-me 搜索@我的消息）。仅在拉取「特别关注人」消息时需要退回 `list-focused`。

支持按关键词、发送者、@我、@指定人、指定会话、时间范围、消息类型、会话类型等多维度搜索消息。发送者 userId 使用 --user/--users；发送者或 @ 人的 openDingTalkId 使用 --sender-ids/--at-ids。所有参数均为可选，至少指定一个搜索条件。
```
Usage:
  dws chat message search-advanced [flags]
Example:
  dws chat message search-advanced --query "周报" --start "2026-04-01T00:00:00+08:00" --end "2026-04-15T00:00:00+08:00"
  dws chat message search-advanced --user <userId> --start "2026-04-01T00:00:00+08:00" --end "2026-04-15T00:00:00+08:00"
  dws chat message search-advanced --users <userId1>,<userId2> --start "2026-04-01T00:00:00+08:00" --end "2026-04-15T00:00:00+08:00"
  dws chat message search-advanced --sender-ids <openDingTalkId1>,<openDingTalkId2> --start "2026-04-01T00:00:00+08:00" --end "2026-04-15T00:00:00+08:00"
  dws chat message search-advanced --at-me --start "2026-04-01T00:00:00+08:00" --end "2026-04-15T00:00:00+08:00"
  dws chat message search-advanced --at-ids <openDingTalkId1>,<openDingTalkId2> --conversation-ids <openConversationId1>,<openConversationId2> --limit 50 --cursor 0
  dws chat message search-advanced --conversation-ids <单聊openConversationId> --query "合同" --start "2026-04-01T00:00:00+08:00" --end "2026-04-15T00:00:00+08:00"
  dws chat message search-advanced --message-type file --search-conv-type group_chat --query "附件"
  dws chat message search-advanced --only-robot-messages --query "通知"
  # 查询群 ID: dws chat search --query "群名"
  # 查询单聊会话 ID: dws chat conversation-info --user <userId>
  # 查询人员: dws contact user search --keyword "姓名" --format json
Flags:
      --query string              搜索关键词（可选）
      --user string                 发送者 userId，支持逗号分隔（可选）
      --users string                发送者 userId 列表，逗号分隔（可选）
      --sender-ids string           发送者 openDingTalkId 列表，逗号分隔（可选）
      --at-me                       只搜索 @我 的消息（可选，默认 false）
      --at-ids string               @指定人的 openDingTalkId 列表，逗号分隔（可选）
      --conversation-ids string     会话 openConversationId 列表，逗号分隔（可选，群聊或单聊均可，不传则搜索所有会话）
      --message-type string          消息类型筛选（可选，支持 file/image/video/audio/link）
      --search-conv-type string      会话类型筛选（可选，single_chat=单聊, group_chat=群聊）
      --start string                开始时间，ISO-8601 格式（可选）
      --end string                  结束时间，ISO-8601 格式（可选）
      --cursor string               分页游标（默认 "0"）
      --limit int                   每页返回数量（默认 100）
      --only-robot-messages          仅搜索机器人消息（可选，默认 false）
      --conversation-ids 的别名: --groups

注意:
  - 所有参数均为可选，但至少需要指定一个搜索条件
  - --user / --users 传发送者 userId
  - --sender-ids 和 --at-ids 传 openDingTalkId
  - --conversation-ids 可指定多个会话 ID（群聊或单聊均可），逗号分隔，不传则搜索所有会话
  - 群聊 openConversationId 通过 `dws chat search --query "群名"` 获取
  - 单聊 openConversationId 通过 `dws chat conversation-info --user <userId>` 或 `--open-dingtalk-id <openDingTalkId>` 获取
  - 时间支持多种 ISO-8601 格式，如 "2026-03-10T00:00:00+08:00"、"2026-03-10 14:00:00"、"2026-03-10" 等
  - 翻页：hasMore=true 时，用返回的 nextCursor 作为下次 --cursor
  - 替代关系：完全替代 search（严格超集）；大部分替代 list-by-sender（--user 覆盖按 userId 搜索发送者，--sender-ids 覆盖按 openDingTalkId 搜索）和 list-mentions（--at-me 覆盖核心功能）；不能替代 list-focused（「特别关注」是独立维度）
```

#### 根据消息 ID 批量查询消息
```
Usage:
  dws chat message list-by-ids [flags]
Example:
  dws chat message list-by-ids --msg-ids msgId1,msgId2,msgId3
  # 最多传 50 条消息 ID
Flags:
      --msg-ids string   消息 ID 列表，逗号分隔，最多 50 条 (必填)
```

#### 表情回应选择策略

> 贴表情时，优先查 [chat-emoji-list.md](chat-emoji-list.md) 中的默认表情名称（共 199 个，如「赞」「鼓掌」「感谢」等）：
> - 命中 → 使用 `add-emoji --emoji <name>`（直接贴 emoji）
> - 未命中 → 先 `create-text-emotion` 创建文字表情获取 emotionId，再 `add-text-emotion` 贴文字表情

#### 对消息添加 emoji 表情回应
```
Usage:
  dws chat message add-emoji [flags]
Example:
  dws chat message add-emoji --conversation-id <openConversationId> --msg-id <openMsgId> --emoji "赞"
  dws chat message add-emoji --conversation-id <openConversationId> --msg-id <openMsgId> --emoji "鼓掌"
  # --emoji 的值必须是 chat-emoji-list.md 中的 name（中文名），如：赞、鼓掌、感谢、微笑 等
  # 查询会话 ID: dws chat search --query "群名"
Flags:
      --conversation-id string   会话 openConversationId (必填，支持单聊/群聊，别名: --group / --id / --chat)
      --msg-id string   消息 openMsgId (必填)
      --emoji string    emoji 表情名称，必须是默认表情列表中的 name 值 (必填，参见 chat-emoji-list.md)
```

#### 移除消息的 emoji 表情回应
```
Usage:
  dws chat message remove-emoji [flags]
Example:
  dws chat message remove-emoji --conversation-id <openConversationId> --msg-id <openMsgId> --emoji "赞"
  # 查询会话 ID: dws chat search --query "群名"
Flags:
      --conversation-id string   会话 openConversationId (必填，支持单聊/群聊，别名: --group / --id / --chat)
      --msg-id string   消息 openMsgId (必填)
      --emoji string    emoji 表情名称，必须是默认表情列表中的 name 值 (必填，参见 chat-emoji-list.md)
```

#### 对消息添加文字表情回应（当默认表情列表中没有所需表情时使用）
```
Usage:
  dws chat message add-text-emotion [flags]
Example:
  dws chat message add-text-emotion --conversation-id <openConversationId> --msg-id <openMsgId> --emotion-id <emotionId> --emotion-name "赞" --text "nice" --background-id im_bg_5
Flags:
      --conversation-id string   会话 openConversationId (必填，支持单聊/群聊，别名: --group / --id / --chat)
      --msg-id string          消息 openMsgId (必填)
      --emotion-id string      表情 ID (必填，通过 create-text-emotion 或已知表情获取)
      --emotion-name string    表情名称 (必填)
      --text string            文字内容 (必填)
      --background-id string   背景 ID (必填)
```

#### 移除消息的文字表情回应
```
Usage:
  dws chat message remove-text-emotion [flags]
Example:
  dws chat message remove-text-emotion --conversation-id <openConversationId> --msg-id <openMsgId> --emotion-id <emotionId> --emotion-name "赞" --text "nice" --background-id <backgroundId>
Flags:
      --conversation-id string   会话 openConversationId (必填，支持单聊/群聊，别名: --group / --id / --chat)
      --msg-id string          消息 openMsgId (必填)
      --emotion-id string      表情 ID (必填)
      --emotion-name string    表情名称 (必填)
      --text string            文字内容 (必填)
      --background-id string   背景 ID (必填)
```

#### 创建文字表情（获取 emotionId）— 当 chat-emoji-list.md 中没有所需表情时，先创建再贴
```
Usage:
  dws chat message create-text-emotion [flags]
Example:
  dws chat message create-text-emotion --emotion-name "赞" --text "nice"
  dws chat message create-text-emotion --emotion-name "感谢" --text "感谢" --background-id im_bg_5
Flags:
      --emotion-name string    表情名称 (必填)
      --text string            文字内容 (必填)
      --background-id string   背景 ID（可选，不传则由服务端默认分配）

注意:
  - 创建后返回 emotionId，可用于 add-text-emotion 命令
  - 如果已有合适的表情，无需创建新的
```

#### 批量拉取消息的表情回复和文字回复

根据消息 ID 列表批量查询消息的表情回复和文字回复信息。
```
Usage:
  dws chat message list-emotion-replies [flags]
Example:
  dws chat message list-emotion-replies --msg-ids msgId1,msgId2,msgId3
  # 消息 ID 可通过 dws chat message list 获取
Flags:
      --msg-ids string   消息 ID 列表，逗号分隔 (必填)

注意:
  - 支持批量查询多条消息的表情和文字回复
  - 消息 ID 可通过 `dws chat message list` 命令获取
```

### list-top-conversations (置顶会话)

#### 拉取置顶会话列表

拉取当前用户的置顶会话列表。分页参数 --limit 指定每页数量，--cursor 传分页游标（首次不传或传 0）。返回结果中 hasMore=true 时用 nextCursor 作为下次 --cursor 继续翻页。
```
Usage:
  dws chat list-top-conversations [flags]
Example:
  dws chat list-top-conversations --limit 1000
  dws chat list-top-conversations --limit 1000 --cursor <nextCursor>
Flags:
      --limit int        每页返回数量（默认 1000）
      --cursor int64     分页游标（首次不传或传 0，翻页传 nextCursor）
      --exclude-muted    是否排除已设置免打扰的会话（默认 false）

注意:
  - 用户询问"置顶会话"时，直接调用此命令返回置顶会话列表即可
  - 用户询问"置顶消息"时，需两步：先调用此命令拉取置顶会话列表获取各会话的 openConversationId，再用 `chat message list --group <openConversationId>` 分别拉取每个会话内的消息
  - 翻页：hasMore=true 时，用返回的 nextCursor 作为下次 --cursor
```

### download-media (下载消息资源)

#### 下载消息中的资源（图片/视频/语音等）到本地

下载聊天消息中的图片、视频、语音等资源到本地文件。流程：先获取下载 URL，再 HTTP GET 下载。
```
Usage:
  dws chat message download-media [flags]
Example:
  dws chat message download-media --type mediaId --resource-id <mediaId> --message-id <openMessageId> --open-conversation-id <openConversationId> --output ./downloads/
  dws chat message download-media --type mediaId --resource-id <mediaId> --message-id <openMessageId> --open-conversation-id <openConversationId> --output ./photo.jpg
Flags:
      --type string                  资源类型: mediaId (必填)
      --resource-id string           资源 ID，mediaId 类型时为消息中的 mediaId 值 (必填)
      --message-id string            消息 openMessageId (必填)
      --open-conversation-id string  会话 openConversationId (必填)
      --output string                本地保存路径，文件或目录 (必填)

注意:
  - resource-id 从 `dws chat message list` 返回的消息内容中获取 mediaId
  - message-id 从 `dws chat message list` 返回的 openMessageId
  - open-conversation-id 从 `dws chat search` 获取 openConversationId
  - --output 如果指定目录，文件名会从下载 URL 中自动推断
```

### search-common (搜索共同群)

#### 搜索共同群 — 查询指定人共同所在的群聊

根据昵称列表搜索共同群聊。--nicks 指定要搜索的人员昵称（逗号分隔，必填）。--match-mode 控制匹配模式：AND 表示所有人都在群里，OR 表示任一人在群里（默认 AND）。分页参数 --limit（默认 20）和 --cursor（默认 "0"）始终传递；hasMore=true 时用返回的 nextCursor 作为下次 --cursor 继续翻页。
```
Usage:
  dws chat search-common [flags]
Example:
  dws chat search-common --nicks "风雷,山乔" --limit 20 --cursor 0
  dws chat search-common --nicks "天鸡,乐函" --match-mode OR --limit 20 --cursor 0
  dws chat search-common --nicks "风雷,山乔,天鸡" --limit 10 --cursor <nextCursor>
Flags:
      --nicks string        要搜索的昵称列表，逗号分隔 (必填)
      --match-mode string   匹配模式：AND=所有人都在群里，OR=任一人在群里（默认 AND）
      --limit int           每页返回数量（默认 20）
      --cursor string       分页游标（默认 "0"，翻页传 nextCursor）
      --exclude-muted       是否排除已设置免打扰的群聊（默认 false）

注意:
  - --nicks 传人员昵称（花名），逗号分隔，如 "风雷,山乔"
  - --match-mode AND 表示群里必须包含所有指定的人；OR 表示包含任意一人即可
  - 翻页：hasMore=true 时，用返回的 nextCursor 作为下次 --cursor
```

### conversation-info (获取会话基础信息)

#### 获取会话基础信息

获取指定会话的基础信息。发送本地文件消息请优先使用 `dws chat message send --msg-type file --file-path <本地文件>`，CLI 不再要求调用方获取或传递 spaceId。
```
Usage:
  dws chat conversation-info [flags]
Example:
  dws chat conversation-info --group <openConversationId> --format json
  dws chat conversation-info --user <userId> --format json
  dws chat conversation-info --open-dingtalk-id <openDingTalkId> --format json
Flags:
      --group string              群聊 openConversationId（群聊时使用）
      --user string               单聊对方 userId（单聊时使用）
      --open-dingtalk-id string   单聊对方 openDingTalkId（单聊时使用）

注意:
  - --group、--user、--open-dingtalk-id 互斥，必须且只能指定其一
  - --group 的别名: --id, --chat, --conversation-id (均可替代 --group)
  - 文件发送不再依赖调用方读取 newCSpaceIdIM；使用 `dws chat message send --msg-type file --file-path <本地文件>` 会自动上传到会话文件空间
```

#### 合并转发多条消息 — 将多条消息合并后转发到目标会话（源/目标会话均支持单聊/群聊）
```
Usage:
  dws chat message combine-forward [flags]
Example:
  dws chat message combine-forward --src-conversation-id <srcOpenCid> --msg-ids <id1>,<id2>,<id3> --dest-conversation-id <destOpenCid>
  dws chat message combine-forward --src-conversation-id <srcOpenCid> --msg-ids <id1>,<id2> --dest-conversation-id <destOpenCid> --uuid <idempotencyKey>
Flags:
      --src-conversation-id string    源会话 openConversationId (必填)
      --msg-ids string                源消息 openMessageId 列表，逗号分隔 (必填)
      --dest-conversation-id string   目标会话 openConversationId (必填)
      --uuid string                   幂等键（可选）

注意:
  - 与 chat message forward 区别: forward 转单条，combine-forward 合并多条为一条转发
  - --msg-ids 多个消息 ID 用逗号分隔，无顺序要求
```

#### 转发话题消息 — 将话题消息从源会话转发到目标会话
```
Usage:
  dws chat message forward-topic [flags]
Example:
  dws chat message forward-topic --src-msg-id <srcOpenMessageId> --src-conversation-id <srcOpenConversationId> --src-thread-id <srcOpenConvThreadId> --dest-conversation-id <destOpenConversationId>
Flags:
      --src-msg-id string               源消息 openMessageId (必填，要转发的消息)
      --src-conversation-id string      源会话 openConversationId (必填，消息所在的会话)
      --src-thread-id string            话题 ID (必填，格式: convThread + 加密后的convThreadId)
      --dest-conversation-id string     目标会话 openConversationId (必填，转发到的会话)

注意:
  - 与 chat message forward 区别: forward 转发普通单条消息，forward-topic 专用于转发话题消息
  - --src-thread-id 格式为 "convThread" + 加密后的 convThreadId，可通过 dws chat message list 返回的话题信息获取
```

#### 钉住某条消息（Pin） — 将指定消息设置为钉住状态
```
Usage:
  dws chat message set-pin-msg [flags]
Example:
  dws chat message set-pin-msg --open-conversation-id <openConversationId> --msg-id <openMessageId>
Flags:
      --open-conversation-id string    (必填)会话 openConversationId（支持群聊/单聊）
      --msg-id string                  (必填)消息 openMessageId

注意:
  - 钉住消息后，会话成员均可在会话中看到被钉住的消息
```

#### 取消钉住某条消息（Unpin） — 取消指定消息的钉住状态
```
Usage:
  dws chat message unset-pin-msg [flags]
Example:
  dws chat message unset-pin-msg --open-conversation-id <openConversationId> --msg-id <openMessageId>
Flags:
      --open-conversation-id string    (必填)会话 openConversationId（支持群聊/单聊）
      --msg-id string                  (必填)消息 openMessageId

注意:
  - 取消钉住后消息仍保留在会话中，只是不再被标记为钉住状态
```

#### 拉取某个会话中钉住的消息列表 — 拉取指定会话中被钉住的消息列表
```
Usage:
  dws chat message list-pin-msg [flags]
Example:
  dws chat message list-pin-msg --open-conversation-id <openConversationId>
  dws chat message list-pin-msg --open-conversation-id <openConversationId> --size 50
  dws chat message list-pin-msg --open-conversation-id <openConversationId> --cursor <nextCursor> --size 20
Flags:
      --open-conversation-id string    (必填)会话 openConversationId（支持群聊/单聊）
      --cursor string   (选填)分页游标，首次不传，翻页时传上次返回的 nextCursor
      --size int        (选填)一次拉取的消息数量（默认 20，最大 100）

注意:
  - 与 `chat message list` 区别: list-pin-msg 只返回被钉住的消息；list 拉取全部消息
  - 分页: hasMore=true 时，用返回的 nextCursor 作为下次 --cursor 继续翻页
```

#### 置顶某条消息 — 将指定消息设置为置顶状态
```
Usage:
  dws chat message set-top-msg [flags]
Example:
  dws chat message set-top-msg --open-conversation-id <openConversationId> --msg-id <openMessageId>
Flags:
      --open-conversation-id string    (必填)会话 openConversationId（支持群聊/单聊）
      --msg-id string                  (必填)消息 openMessageId

注意:
  - 置顶消息后，会话成员可在会话顶部看到该条消息
  - 与 `chat set-top`（会话置顶）不同：set-top-msg 是将会话内某条消息置顶；set-top 是将整个会话在列表中置顶
```

#### 取消置顶某条消息 — 取消指定消息的置顶状态
```
Usage:
  dws chat message unset-top-msg [flags]
Example:
  dws chat message unset-top-msg --open-conversation-id <openConversationId> --msg-id <openMessageId>
Flags:
      --open-conversation-id string    (必填)会话 openConversationId（支持群聊/单聊）
      --msg-id string                  (必填)消息 openMessageId

注意:
  - 取消置顶后消息仍保留在会话中，只是不再被标记为置顶状态
```

### bot (机器人管理)

#### 搜索【我创建的】机器人 — 仅返回当前用户自己创建的机器人

范围: 仅限当前登录用户自己创建的机器人（不含他人创建、官方机器人）。
返回字段: 没有 openDingTalkId，如果需要给机器人发单聊消息请用 find。
典型触发词: "我创建的机器人""我的机器人""我自己的机器人""我做的机器人""查看我的机器人"。

```
Usage:
  dws chat bot search [flags]
Example:
  dws chat bot search --page 1
  dws chat bot search --page 1 --size 10 --name "日报"
Flags:
      --name string   按名称搜索
      --page int      页码，从1开始 (默认 1)
      --size int      每页条数 (默认 50)，别名: --limit
```

#### 搜索【全部可用】机器人 — 含他人创建/官方机器人，额外返回 openDingTalkId

范围: 当前用户可用的全部机器人（含他人创建、官方机器人）。
返回字段: 额外返回 openDingTalkId（可用于给机器人发单聊消息），search 没有此字段。
典型触发词: "搜索机器人""找一个机器人""帮我找 XXX 机器人""所有可用机器人""查机器人"。

```
Usage:
  dws chat bot find [flags]
Example:
  dws chat bot find --query "日报"
  dws chat bot find --query "日报" --limit 20
  dws chat bot find --query "日报" --limit 20 --cursor <上次返回的 nextCursor>
Flags:
      --query string   搜索关键词 (必填)
      --limit int        每页返回数量（默认 20）
      --cursor string    分页游标（首次调用不传，翻页时传上次返回的 nextCursor）

注意:
  - cursor 必须用上次返回的 nextCursor 字符串原值，不要传 "0" 或其他数字字面量
    （服务端 String 类型，但网关会把数字字符串 auto-coerce 回 Integer 导致 PARAM_ERROR）
```

search 与 find 选择指南:

| 维度 | `chat bot search` | `chat bot find` |
|------|-------------------|-----------------|
| 范围 | 仅我创建的机器人 | 全部可用机器人（含他人/官方） |
| 额外返回 openDingTalkId | 无 | 有（可用于给机器人发单聊消息） |
| 触发词 | "我创建的""我的""我自己的" | "搜索机器人""找机器人""查机器人" |

### category (会话分组管理)

#### 获取用户自定义会话分组
```
Usage:
  dws chat category list
Example:
  dws chat category list
  # 返回当前用户的所有自定义会话分组
```

#### 拉取指定分组下的会话列表
```
Usage:
  dws chat category list-conversations [flags]
Example:
  dws chat category list-conversations --category-id <分组ID>
  # 分组ID 可通过 dws chat category list 获取
Flags:
      --category-id int   会话分组 ID (必填)
      --exclude-muted     是否排除已设置免打扰的会话（默认 false）
```

#### 创建用户自定义会话分组
```
Usage:
  dws chat category create [flags]
Example:
  dws chat category create --title "工作群"
  dws chat category create --title "项目组"
Flags:
      --title string   分组名称 (必填)
```

#### 创建智能会话分组 — 可指定群名称关键词和群内成员作为匹配规则
```
Usage:
  dws chat category create-smart [flags]
Example:
  dws chat category create-smart --name "工作群"
  dws chat category create-smart --name "项目组" --keywords "项目,开发"
  dws chat category create-smart --name "团队群" --members openDingTalkId1,openDingTalkId2
  dws chat category create-smart --name "重点群" --keywords "重点" --members openDingTalkId1
Flags:
      --name string       分组名称 (必填)
      --keywords string   群名称关键词列表，逗号分隔（可选）
      --members string    群内成员 openDingTalkId 列表，逗号分隔（可选）
```

#### 删除用户自定义会话分组
```
Usage:
  dws chat category delete [flags]
Example:
  dws chat category delete --category-id <分组ID>
  # 分组ID 可通过 dws chat category list 获取
Flags:
      --category-id int   会话分组 ID (必填)
```

#### 更新用户自定义会话分组的名称
```
Usage:
  dws chat category rename [flags]
Example:
  dws chat category rename --category-id <分组ID> --title "新名称"
  # 分组ID 可通过 dws chat category list 获取
Flags:
      --category-id int   会话分组 ID (必填)
      --title string      新的分组名称 (必填)
```

#### 将会话移动到指定的自定义分组中
```
Usage:
  dws chat category add-conv [flags]
Example:
  dws chat category add-conv --group <openConversationId> --category-ids 123,456
  # 分组ID 可通过 dws chat category list 获取
  # 查询群 ID: dws chat search --query "群名"
Flags:
      --group string         会话 openConversationId (必填)
      --category-ids string  目标分组 ID 列表，逗号分隔 (必填)
```

#### 将会话从指定的自定义分组中移出
```
Usage:
  dws chat category remove-conv [flags]
Example:
  dws chat category remove-conv --group <openConversationId> --category-ids 123,456
  # 分组ID 可通过 dws chat category list 获取
  # 查询群 ID: dws chat search --query "群名"
Flags:
      --group string         会话 openConversationId (必填)
      --category-ids string  目标分组 ID 列表，逗号分隔 (必填)
```

### mute (会话免打扰)

#### 会话消息免打扰 — 开启或关闭会话消息免打扰（支持单聊和群聊）
```
Usage:
  dws chat mute [flags]
Example:
  dws chat mute --conversation-id <openConversationId>
  dws chat mute --conversation-id <openConversationId> --off
  # 查询群 ID: dws chat search --query "群名"
  # 查询单聊会话 ID: dws chat conversation-info --user <userId>
Flags:
      --conversation-id string   会话 openConversationId (必填，支持单聊/群聊)
      --id string                --conversation-id 的别名
      --chat string              --conversation-id 的别名
      --off                      关闭免打扰（不传则开启免打扰）

注意:
  - 默认行为是开启免打扰，传 --off 则关闭免打扰
  - 支持单聊和群聊，openConversationId 可通过 chat search（群聊）或 chat conversation-info（单聊）获取
```

### hide (隐藏会话)

#### 隐藏会话 — 在会话列表中隐藏指定会话（支持单聊/群聊），收到新消息时会重新出现
```
Usage:
  dws chat hide [flags]
Example:
  dws chat hide --conversation-id <openConversationId>
  dws chat hide --id <openConversationId>
  # 查询群 ID: dws chat search --query "群名"
  # 查询单聊会话 ID: dws chat conversation-info --user <userId>
Flags:
      --conversation-id string   会话 openConversationId (必填，支持单聊/群聊)
      --id string                --conversation-id 的别名
      --chat string              --conversation-id 的别名

注意:
  - 隐藏后会话不再显示在列表中，收到新消息时会重新出现
  - 支持单聊和群聊，openConversationId 可通过 chat search（群聊）或 chat conversation-info（单聊）获取
```


### mute-at-all (关闭@所有人通知)

#### 关闭/开启 @所有人消息提醒 — 关闭或开启会话中 @所有人的消息通知
```
Usage:
  dws chat mute-at-all [flags]
Example:
  dws chat mute-at-all --conversation-id <openConversationId>
  dws chat mute-at-all --conversation-id <openConversationId> --off
  # 查询群 ID: dws chat search --query "群名"
Flags:
      --conversation-id string   会话 openConversationId (必填，支持单聊/群聊)
      --id string                --conversation-id 的别名
      --chat string              --conversation-id 的别名
      --off                      恢复接收 @所有人通知（不传则关闭通知）

注意:
  - 默认行为是关闭 @所有人通知，传 --off 则恢复接收通知
  - 支持单聊和群聊，openConversationId 可通过 chat search（群聊）或 chat conversation-info（单聊）获取
```

### mute-red-envelope (关闭红包通知)

#### 关闭/开启红包消息提醒 — 关闭或开启会话中的红包消息通知
```
Usage:
  dws chat mute-red-envelope [flags]
Example:
  dws chat mute-red-envelope --conversation-id <openConversationId>
  dws chat mute-red-envelope --conversation-id <openConversationId> --off
  # 查询群 ID: dws chat search --query "群名"
Flags:
      --conversation-id string   会话 openConversationId (必填，支持单聊/群聊)
      --id string                --conversation-id 的别名
      --chat string              --conversation-id 的别名
      --off                      恢复接收红包通知（不传则关闭通知）

注意:
  - 默认行为是关闭红包通知，传 --off 则恢复接收通知
  - 支持单聊和群聊，openConversationId 可通过 chat search（群聊）或 chat conversation-info（单聊）获取
```

### mark-unread (标记会话为未读)

#### 标记会话为未读 — 将指定会话标记为未读状态
```
Usage:
  dws chat mark-unread [flags]
Example:
  dws chat mark-unread --conversation-id <openConversationId>
  dws chat mark-unread --id <openConversationId>
Flags:
      --conversation-id string   会话 openConversationId (必填，支持群聊/单聊)
      --id string                --conversation-id 的别名
      --chat string              --conversation-id 的别名

注意:
  - 支持群聊和单聊，openConversationId 可通过 chat search（群聊）或 chat conversation-info（单聊）获取
  - 标记未读后会话列表中会显示未读状态
```

### clear-red-point (清除会话红点)

#### 清除会话红点 — 清除指定会话的未读红点
```
Usage:
  dws chat clear-red-point [flags]
Example:
  dws chat clear-red-point --conversation-id <openConversationId>
  dws chat clear-red-point --id <openConversationId>
Flags:
      --conversation-id string   会话 openConversationId (必填，支持群聊/单聊)
      --id string                --conversation-id 的别名
      --chat string              --conversation-id 的别名

注意:
  - 支持群聊和单聊，openConversationId 可通过 chat search（群聊）或 chat conversation-info（单聊）获取
  - 清除红点后该会话不再显示未读标记
```

### clear-all-red-point (红点清零)

#### 清除所有会话红点 — 一键全部已读
```
Usage:
  dws chat clear-all-red-point
Example:
  dws chat clear-all-red-point

注意:
  - 无需任何参数，直接清除当前用户所有会话的未读红点
  - 等效于“全部已读”操作
```

### list-all-conversations (全部会话列表)

#### 分页获取全部会话列表 — 获取当前用户的所有会话
```
Usage:
  dws chat list-all-conversations [flags]
Example:
  dws chat list-all-conversations
  dws chat list-all-conversations --limit 50
  dws chat list-all-conversations --limit 100 --cursor <nextCursor>
  dws chat list-all-conversations --exclude-muted
Flags:
      --limit int        每页数量（默认 1000）
      --cursor int64     分页游标（首次不传或传 0，翻页传 nextCursor）
      --exclude-muted    是否排除已免打扰会话（默认 false）

注意:
  - 返回结果包含单聊和群聊，不区分会话类型
  - 翻页: hasMore=true 时用返回的 nextCursor 作为下次 --cursor
  - 与 list-top-conversations 的区别: 本命令返回全部会话（单聊+群聊），list-top-conversations 仅返回置顶会话
```

### clear-messages (清空会话聊天记录)

#### 清空会话聊天记录 — 清空当前用户指定会话的消息
```
Usage:
  dws chat clear-messages [flags]
Example:
  dws chat clear-messages --conversation-id <openConversationId>
  dws chat clear-messages --id <openConversationId>
Flags:
      --conversation-id string   会话 openConversationId (必填，支持群聊/单聊)
      --id string                --conversation-id 的别名
      --chat string              --conversation-id 的别名

注意:
  - 仅清空当前用户视角的消息，不影响其他成员
  - openConversationId 可通过 chat search（群聊）或 chat conversation-info（单聊）获取
```

### mark-read (标记消息已读)

#### 标记消息已读 — 将指定消息及之前的消息标记为已读
```
Usage:
  dws chat mark-read [flags]
Example:
  dws chat mark-read --conversation-id <openConversationId> --message-id <openMessageId>
  dws chat mark-read --id <openConversationId> --message-id <openMessageId>
Flags:
      --conversation-id string   会话 openConversationId (必填，支持群聊/单聊)
      --id string                --conversation-id 的别名
      --chat string              --conversation-id 的别名
      --message-id string        消息 openMessageId (必填)

注意:
  - 标记该消息及之前的所有消息为已读
  - openConversationId 可通过 chat search（群聊）或 chat conversation-info（单聊）获取
  - openMessageId 可通过 chat message list 获取
```

### group list-all (分页拉取所有群)

#### 分页拉取我所有群列表 — 获取当前用户加入的所有群聊
```
Usage:
  dws chat group list-all [flags]
Example:
  dws chat group list-all
  dws chat group list-all --limit 50
  dws chat group list-all --limit 100 --cursor <nextCursor>
Flags:
      --limit int       每页返回数量（默认 100，最大 200）
      --cursor string   分页游标（首次不传，翻页传返回的 nextCursor）

注意:
  - 与 `chat group list-my-groups` 区别: list-all 返回用户加入的所有群；list-my-groups 仅返回用户作为群主/管理员的群
  - 分页: hasMore=true 时用返回的 nextCursor 作为下次 --cursor
```

### group list-join-validations (分页拉取入群验证记录)

#### 分页拉取入群验证记录 — 获取当前用户的所有入群验证记录

包括自己被拒绝的记录以及作为审批者的记录。

```
Usage:
  dws chat group list-join-validations [flags]
Example:
  dws chat group list-join-validations
  dws chat group list-join-validations --limit 30
  dws chat group list-join-validations --limit 20 --cursor <nextCursor>
Flags:
      --limit int       单页数量（默认 20，最大 50）
      --cursor string   分页游标（首次不传，翻页传返回的 nextCursor）

注意:
  - 分页: hasMore=true 时用返回的 nextCursor 作为下次 --cursor
  - cursor 首次拉取不传或传 null 时从当前时间开始拉
```

### group audit-join-validation (审批入群验证)

#### 审批入群验证 — 通过、拒绝、删除单个审核

支持通过、拒绝、删除、忽略、拒绝并拉黑等操作。

```
Usage:
  dws chat group audit-join-validation [flags]
Example:
  dws chat group audit-join-validation --group <openConversationId> --record-id 123456 --applicant <openDingTalkId> --inviter <openDingTalkId> --status AuditApprove
  dws chat group audit-join-validation --group <openConversationId> --record-id 123456 --applicant <openDingTalkId> --inviter <openDingTalkId> --status AuditRefuse --description "不符合入群条件"
  # 查询入群验证记录: dws chat group list-join-validations
Flags:
      --group string        群 openConversationId (必填)
      --record-id int64     申请记录 ID (必填)
      --applicant string    申请人 openDingTalkId (必填)
      --inviter string      邀请人 openDingTalkId (必填)
      --status string       审批动作: AuditApprove/AuditDelete/AuditIgnore/AuditRefuse/AuditBlock (必填)
      --description string  审批说明（可选）

注意:
  - status 可选值: AuditApprove(通过), AuditDelete(删除), AuditIgnore(忽略), AuditRefuse(拒绝), AuditBlock(拒绝且拉黑)
  - record-id、applicant、inviter 可通过 dws chat group list-join-validations 查询获得
```
