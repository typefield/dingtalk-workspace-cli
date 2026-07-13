# lark-cli Shortcut 深度对齐矩阵

> 12 个 agent 逐条深读 lark 每个 shortcut 的智能实现（Validate/DryRun/ID解析/投影/多步/分页），映射钉钉、标注保真度差距。

## 汇总（361 条 lark shortcut）

| dws_status | 数量 | 含义 |
|---|:---:|---|
| covered-1to1 | 144 | lark 组合在钉钉塌缩成 1:1，封装层已覆盖 |
| no-dingtalk-tool | 127 | 钉钉无对应工具，客观不可对齐 |
| **gap-buildable** | **42** | 钉钉有工具、值得补成智能 shortcut（**建设目标**）；已建 minutes `+detail`/`+replace-batch`、base `+record-share-links`/`+resolve-base`、im `+thread-replies`/`+chat-messages`、task `+related-tasks` |
| covered-smart | 48 | 已建智能 shortcut / 部分覆盖 |

## 🎯 gap-buildable 目标清单（原 49 条，已建 7 → 剩 42，按服务）

> 已落地：minutes `+detail`（✅ smart `+detail`）、minutes `+word-replace`（✅ smart `+replace-batch`，批量+去重）、base `+record-share-link-create`（✅ smart `+record-share-links`，>20 去重+分片+合并）、im `+threads-messages-list`（✅ smart `chat +thread-replies`，list_topic_replies + 投影）、task `+get-related-tasks`（✅ smart `todo +related-tasks`，三角色并集+去重+投影）。

### im → chat（6）

| lark 命令 | risk | 保真度差距（钉钉有 tool，缺什么智能） |
|---|---|---|
| `+chat-list` | read | dws 有 list-my-groups/list-all-conversations 原子 tool，但无 types 枚举+bot剥p2p降级、无 exclude-muted 客户端过滤、无字段投影 |
| `+chat-messages-list` ✅ | read | **已建 smart `chat +chat-messages`**：群/单聊 list_conversation_message_v2 / list_individual_chat_message 互斥 + sender/text/time 投影。剩余未做：reactions 富化、资源下载 |
| `+chat-search` | read | dws 无群名模糊搜索v2对应 tool(search_common_groups/find 语义不同)，缺 query规范化、mode映射、mute过滤、meta投影 |
| `+messages-resources-download` | write | dws download-media 走 get_resource_download_url 拿URL，缺分片Range下载/重试/扩展名推断/安全落盘路径校验 |
| `+messages-search` | read | dws 有 search_messages_by_keyword/by_time_range/by_sender/at_me 多个原子 tool，但各自单点，缺统一多维filter编排+mget+chat上下文富化+跨字段Validate |
| `+threads-messages-list` ✅ | read | **已建 smart `chat +thread-replies`**：list_topic_replies + sender/text/time 投影。剩余未做：reactions 富化、资源下载 |

### task → todo（3）

| lark 命令 | risk | 保真度差距（钉钉有 tool，缺什么智能） |
|---|---|---|
| `+reminder` | write | dws 有 add_todo_reminder/reset_todo_reminder 但无 lark 的先查现有再替换编排、相对时间(15m/1h)解析与互斥校验，值得补智能 shortcut |
| `+get-related-tasks` ✅ | read | **已建 smart `todo +related-tasks`**：creator+executor+participant 三角色并集 + taskId 去重 + 投影。剩余未做：followed-by-me 成员比对、subtask_count/tasklists 富投影 |
| `+upload-attachment` | write | dws add-attachment 走 init→PUT→commit 三步 MCP 上传(能力更重)，但无 50MB/regular 校验、applink 提取与 dry-run 计划展示；可对齐成更智能 shortcut |

### calendar → calendar（1）

| lark 命令 | risk | 保真度差距（钉钉有 tool，缺什么智能） |
|---|---|---|
| `+room-find` | read | dws 有 room search(query_available_meeting_room 按单一时间段+过滤)和 busy search，但无多slot并发room_find聚合、无city/building/floor/capacity维度过滤、无按attendee推荐可用室，值得补成智能 shortcut 但未建 |

### doc (docs) → doc（2）

| lark 命令 | risk | 保真度差距（钉钉有 tool，缺什么智能） |
|---|---|---|
| `+media-insert` | write | dws doc media insert 为3步(取凭证→PUT→insert_document_block)无回滚、无selection定位、无剪贴板、无宽高比补算、无wiki解析；可补成带回滚的智能shortcut |
| `+media-download` | read | dws doc media download 走resourceId→downloadUrl两段,缺whiteboard导图分支、自动扩展名、路径安全、overwrite防护；media分支可对齐，whiteboard无工具 |

### drive → drive（1）

| lark 命令 | risk | 保真度差距（钉钉有 tool，缺什么智能） |
|---|---|---|
| `+import` | write | dws drive upload 有 --workspace --convert 可转在线文档，但缺按目标类型(docx/sheet/bitable/slides)导入、缺 target-token 挂载与异步轮询 |

### mail → mail（4）

| lark 命令 | risk | 保真度差距（钉钉有 tool，缺什么智能） |
|---|---|---|
| `+reply` | write | dws reply 走 create_reply_draft+send_draft 两步、附件仅上传会话，缺 EML 线程头构造、签名自动注入、模板合并、HTML lint、读回执、send-time 定时、跨字段校验 |
| `+reply-all` | write | dws reply-all 两步且收件人由服务端决定，缺原文收件人抽取去重排己、线程头、签名/模板/lint/定时等编排保真 |
| `+send` | write | dws send_email 单步(附件时先 create_draft 再传再 send)，缺签名/模板/lint/日历内嵌/定时发送/发件人profile解析/跨字段校验 |
| `+forward` | write | dws forward 走 create_forward_draft+send_draft，缺 Fw:主题/引用块/原附件转载 EML 构建、签名/模板/lint/定时保真 |

### wiki → wiki（1）

| lark 命令 | risk | 保真度差距（钉钉有 tool，缺什么智能） |
|---|---|---|
| `+node-get` | read | dws 无 get_node 对应 tool(proxy wiki doc read 读的是文档正文而非节点元数据/space解析)；缺 token/obj_token/URL→node 解析、obj_type推断、space交叉校验——是值得补的智能 shortcut 缺口 |

### minutes → minutes（4）

| lark 命令 | risk | 保真度差距（钉钉有 tool，缺什么智能） |
|---|---|---|
| `+search` | read | dws list_by_keyword_and_time_range 只按 keyword+时间+归属(created/shared)过滤，缺 owner/participant 的 me 解析与筛选、缺 query 长度与跨字段互斥校验、缺输出投影与去头像 |
| `+download` | read | dws 只有 query_minutes_audio_url 返回 OSS 地址(相当于 --url-only 单条)，缺真正落盘下载、批量 fanout+限速+去重、文件名推断、SSRF 防护与覆盖保护 |
| `+word-replace` ✅ | write | **已建 smart `+replace-batch`**：多组 `原文=>替换` 批量替换 + 去重校验 + 逐组结果聚合（补齐 1:1 `+word-replace` 的单组限制）。剩余未做：@file/stdin 输入 |
| `+detail` ✅ | read | **已建 smart `+detail`**：单命令按 `--artifacts` fanout basic/summary/keywords/transcript/todos + partial-failure 容错 + rt.Output 投影。剩余未做：wait-ready 轮询、transcript 落盘 |

### base → aitable（10）

| lark 命令 | risk | 保真度差距（钉钉有 tool，缺什么智能） |
|---|---|---|
| `+title-resolve` ✅ | read | **已建 smart `aitable +resolve-base`**：search_bases 按名解析 baseId + 0/1/多候选消歧投影。剩余未做：Drive doc_wiki 全文搜索 |
| `+field-create` | write | dws create_fields 支持批量,但缺 formula/lookup guide-ack 门禁与逐字段节流,可补智能 shortcut |
| `+field-update` | write | dws update_field 缺 formula/lookup guide-ack 保护 |
| `+record-share-link-create` ✅ | read | **已建 smart `+record-share-links`**：>20 条记录去重 + 分片(≤20/批) + 跨 aitable-helper server fanout + 合并 {recordId,shareUrl}，补齐单批 20 条上限 |
| `+record-upload-attachment` | write | dws 只有 prepare_attachment_upload(拿上传凭证),缺 分片上传编排+append_attachments 回填单元格的完整链路 |
| `+dashboard-block-list` | read | dws 仪表盘块是 chart(create/get/update/delete_chart),缺通用 block list,可对齐补 |
| `+dashboard-block-get` | read | dws get_chart 覆盖 chart 类块,缺通用 block get |
| `+dashboard-block-create` | write | dws create_chart 覆盖图表块,缺其他 block 类型的通用创建 |
| `+dashboard-block-update` | write | dws update_chart 覆盖图表块更新 |
| `+dashboard-block-delete` | high-risk-write | dws delete_chart 覆盖图表块删除 |

### sheets → sheet（14）

| lark 命令 | risk | 保真度差距（钉钉有 tool，缺什么智能） |
|---|---|---|
| `+sheet-hide` | write | dws update_sheet可能含hidden属性但未见独立hide命令,需确认 |
| `+sheet-unhide` | write | 同上,dws无独立unhide命令 |
| `+sheet-set-tab-color` | write | dws update_sheet或可设tab色但无独立命令 |
| `+sheet-show-gridline` | write | dws无网格线显隐命令 |
| `+sheet-hide-gridline` | write | dws无网格线显隐命令 |
| `+workbook-create` | write | dws有create_workspace_sheet但仅建空表,缺typed一步建表+填充+样式+partial回滚编排 |
| `+dim-hide` | write | dws update-dimension或含hidden但无独立hide命令 |
| `+dim-unhide` | write | 同上,dws无独立unhide命令 |
| `+dim-freeze` | write | dws update-dimension可能含frozen但无独立freeze命令 |
| `+cells-get` | read | dws range read存在但缺include样式/公式投影统一封装 |
| `+table-get` | read | dws缺typed table读回+列类型推断+多sheet编排,只有裸csv/range读 |
| `+table-put` | write | dws有append/set_cell_range但缺typed多sheet分块写+建缺失sheet+样式+partial回滚编排 |
| `+rows-resize` | write | dws update-dimension可调尺寸但无独立rows-resize+size/type互斥校验 |
| `+cols-resize` | write | dws update-dimension可调尺寸但无独立cols-resize+互斥校验 |

### apps → devapp（3）

| lark 命令 | risk | 保真度差距（钉钉有 tool，缺什么智能） |
|---|---|---|
| `+release-create` | write | dws 有 create_dev_app_version(开放平台版本)可类比，但妙搭 release 是低代码应用发布、语义与产物不同 |
| `+release-get` | read | dws 有 get_dev_app_version_detail 可类比但产品域(开放平台vs妙搭)不同 |
| `+release-list` | read | dws 有 list_dev_app_versions 可类比但无 status 枚举过滤且产品域不同 |

## 已建智能 shortcut（covered-smart，48）— 可继续升级保真度

- **im**: +chat-members-list +messages-send +threads-messages-list
- **task**: +complete +assign +get-my-tasks +get-related-tasks
- **contact**: +search-user
- **calendar**: +agenda +create +update +freebusy +suggestion
- **doc (docs)**: +history-revert
- **drive**: +upload +search +inspect
- **mail**: +triage
- **minutes**: +upload +latest-minutes +action-items +transcript +minutes-search +detail +replace-batch
- **base**: +table-get +table-create +view-create +view-get-filter +view-set-filter +view-get-visible-fields +view-set-visible-fields +view-get-group +view-set-group +view-get-sort +view-set-sort +view-get-timebar +view-set-timebar +view-get-card +view-set-card +record-list +record-search +record-get +record-upsert +base-create +workflow-list +form-create +form-list +form-get +record-share-link-create