# DWS Gap-Buildable 清单（42 条）

> 钉钉有底层 MCP tool、值得补成智能 shortcut 的条目。原 49 条已建 7 条，剩 42 条。
>
> 对齐基线：lark-cli `main@e96c4fa5` / DWS `feature/shortcut@b7c14c1`
>
> 生成日期：2026-07-29

## 已落地（7 条）

| lark 命令 | DWS smart shortcut | 说明 |
|---|---|---|
| `minutes +detail` | `minutes +detail` | fanout basic/summary/keywords/transcript/todos + partial-failure 容错 |
| `minutes +word-replace` | `minutes +replace-batch` | 多组批量替换 + 去重校验 + 逐组聚合 |
| `base +record-share-link-create` | `aitable +record-share-links` | >20 去重 + 分片 + 跨 server fanout + 合并 |
| `base +title-resolve` | `aitable +resolve-base` | search_bases 按名解析 + 0/1/多候选消歧 |
| `im +threads-messages-list` | `chat +thread-replies` | list_topic_replies + sender/text/time 投影 |
| `im +chat-messages-list` | `chat +chat-messages` | 群/单聊互斥 + sender/text/time 投影 |
| `task +get-related-tasks` | `todo +related-tasks` | 三角色并集 + taskId 去重 + 投影 |

---

## 待建设（42 条，按服务分组）

### im → chat（4 条）

| # | lark 命令 | risk | 保真度差距 |
|---|---|---|---|
| 1 | `+chat-list` | read | dws 有 list-my-groups/list-all-conversations 原子 tool，但无 types 枚举 + bot 剥 p2p 降级、无 exclude-muted 客户端过滤、无字段投影 |
| 2 | `+chat-search` | read | dws 无群名模糊搜索 v2 对应 tool（search_common_groups/find 语义不同），缺 query 规范化、mode 映射、mute 过滤、meta 投影 |
| 3 | `+messages-resources-download` | write | dws download-media 走 get_resource_download_url 拿 URL，缺分片 Range 下载 / 重试 / 扩展名推断 / 安全落盘路径校验 |
| 4 | `+messages-search` | read | dws 有 search_messages_by_keyword/by_time_range/by_sender/at_me 多个原子 tool，但各自单点，缺统一多维 filter 编排 + mget + chat 上下文富化 + 跨字段 Validate |

### task → todo（2 条）

| # | lark 命令 | risk | 保真度差距 |
|---|---|---|---|
| 5 | `+reminder` | write | dws 有 add_todo_reminder/reset_todo_reminder 但无先查现有再替换编排、相对时间（15m/1h）解析与互斥校验 |
| 6 | `+upload-attachment` | write | dws add-attachment 走 init→PUT→commit 三步 MCP 上传，但无 50MB/regular 校验、applink 提取与 dry-run 计划展示 |

### calendar → calendar（1 条）

| # | lark 命令 | risk | 保真度差距 |
|---|---|---|---|
| 7 | `+room-find` | read | dws 有 room search（query_available_meeting_room 按单一时间段 + 过滤）和 busy search，但无多 slot 并发 room_find 聚合、无 city/building/floor/capacity 维度过滤、无按 attendee 推荐可用室 |

### doc → doc（2 条）

| # | lark 命令 | risk | 保真度差距 |
|---|---|---|---|
| 8 | `+media-insert` | write | dws doc media insert 为 3 步（取凭证→PUT→insert_document_block）无回滚、无 selection 定位、无剪贴板、无宽高比补算、无 wiki 解析 |
| 9 | `+media-download` | read | dws doc media download 走 resourceId→downloadUrl 两段，缺 whiteboard 导图分支、自动扩展名、路径安全、overwrite 防护 |

### drive → drive（1 条）

| # | lark 命令 | risk | 保真度差距 |
|---|---|---|---|
| 10 | `+import` | write | dws drive upload 有 --workspace --convert 可转在线文档，但缺按目标类型（docx/sheet/bitable/slides）导入、缺 target-token 挂载与异步轮询 |

### mail → mail（4 条）

| # | lark 命令 | risk | 保真度差距 |
|---|---|---|---|
| 11 | `+reply` | write | dws reply 走 create_reply_draft + send_draft 两步，缺 EML 线程头构造、签名自动注入、模板合并、HTML lint、读回执、send-time 定时、跨字段校验 |
| 12 | `+reply-all` | write | dws reply-all 两步且收件人由服务端决定，缺原文收件人抽取去重排己、线程头、签名/模板/lint/定时等编排保真 |
| 13 | `+send` | write | dws send_email 单步（附件时先 create_draft 再传再 send），缺签名/模板/lint/日历内嵌/定时发送/发件人 profile 解析/跨字段校验 |
| 14 | `+forward` | write | dws forward 走 create_forward_draft + send_draft，缺 Fw: 主题/引用块/原附件转载 EML 构建、签名/模板/lint/定时保真 |

### wiki → wiki（1 条）

| # | lark 命令 | risk | 保真度差距 |
|---|---|---|---|
| 15 | `+node-get` | read | dws 无 get_node 对应 tool（proxy wiki doc read 读的是文档正文而非节点元数据/space 解析），缺 token/obj_token/URL→node 解析、obj_type 推断、space 交叉校验 |

### minutes → minutes（2 条）

| # | lark 命令 | risk | 保真度差距 |
|---|---|---|---|
| 16 | `+search` | read | dws list_by_keyword_and_time_range 只按 keyword + 时间 + 归属（created/shared）过滤，缺 owner/participant 的 me 解析与筛选、缺 query 长度与跨字段互斥校验、缺输出投影与去头像 |
| 17 | `+download` | read | dws 只有 query_minutes_audio_url 返回 OSS 地址（相当于 --url-only 单条），缺真正落盘下载、批量 fanout + 限速 + 去重、文件名推断、SSRF 防护与覆盖保护 |

### base → aitable（8 条）

| # | lark 命令 | risk | 保真度差距 |
|---|---|---|---|
| 18 | `+field-create` | write | dws create_fields 支持批量，但缺 formula/lookup guide-ack 门禁与逐字段节流 |
| 19 | `+field-update` | write | dws update_field 缺 formula/lookup guide-ack 保护 |
| 20 | `+record-upload-attachment` | write | dws 只有 prepare_attachment_upload（拿上传凭证），缺分片上传编排 + append_attachments 回填单元格的完整链路 |
| 21 | `+dashboard-block-list` | read | dws 仪表盘块是 chart（create/get/update/delete_chart），缺通用 block list |
| 22 | `+dashboard-block-get` | read | dws get_chart 覆盖 chart 类块，缺通用 block get |
| 23 | `+dashboard-block-create` | write | dws create_chart 覆盖图表块，缺其他 block 类型的通用创建 |
| 24 | `+dashboard-block-update` | write | dws update_chart 覆盖图表块更新 |
| 25 | `+dashboard-block-delete` | high-risk-write | dws delete_chart 覆盖图表块删除 |

### sheets → sheet（14 条）

| # | lark 命令 | risk | 保真度差距 |
|---|---|---|---|
| 26 | `+sheet-hide` | write | dws update_sheet 可能含 hidden 属性但未见独立 hide 命令 |
| 27 | `+sheet-unhide` | write | dws 无独立 unhide 命令 |
| 28 | `+sheet-set-tab-color` | write | dws update_sheet 或可设 tab 色但无独立命令 |
| 29 | `+sheet-show-gridline` | write | dws 无网格线显隐命令 |
| 30 | `+sheet-hide-gridline` | write | dws 无网格线显隐命令 |
| 31 | `+workbook-create` | write | dws 有 create_workspace_sheet 但仅建空表，缺 typed 一步建表 + 填充 + 样式 + partial 回滚编排 |
| 32 | `+dim-hide` | write | dws update-dimension 或含 hidden 但无独立 hide 命令 |
| 33 | `+dim-unhide` | write | dws 无独立 unhide 命令 |
| 34 | `+dim-freeze` | write | dws update-dimension 可能含 frozen 但无独立 freeze 命令 |
| 35 | `+cells-get` | read | dws range read 存在但缺 include 样式/公式投影统一封装 |
| 36 | `+table-get` | read | dws 缺 typed table 读回 + 列类型推断 + 多 sheet 编排，只有裸 csv/range 读 |
| 37 | `+table-put` | write | dws 有 append/set_cell_range 但缺 typed 多 sheet 分块写 + 建缺失 sheet + 样式 + partial 回滚编排 |
| 38 | `+rows-resize` | write | dws update-dimension 可调尺寸但无独立 rows-resize + size/type 互斥校验 |
| 39 | `+cols-resize` | write | dws update-dimension 可调尺寸但无独立 cols-resize + 互斥校验 |

### apps → devapp（3 条）

| # | lark 命令 | risk | 保真度差距 |
|---|---|---|---|
| 40 | `+release-create` | write | dws 有 create_dev_app_version（开放平台版本）可类比，但妙搭 release 是低代码应用发布、语义与产物不同 |
| 41 | `+release-get` | read | dws 有 get_dev_app_version_detail 可类比但产品域（开放平台 vs 妙搭）不同 |
| 42 | `+release-list` | read | dws 有 list_dev_app_versions 可类比但无 status 枚举过滤且产品域不同 |

---

## 按优先级分组

### P0 — 用户感知最强（18 条）

| 领域 | 条目 | 理由 |
|---|---|---|
| **Sheets typed workflow** | #26–#39（14 条） | 最大可建设缺口；typed table 读写、批量样式、维度操作、workbook 编排 |
| **Drive 本地同步** | #10（1 条） | 缺完整目录同步和可恢复批处理 |
| **统一消息搜索** | #4（1 条） | 多个原子 tool 单点，缺统一多维 filter 编排 |
| **Mail 高保真写链路** | #11–#14（4 条） | 缺签名/模板/lint/线程头/定时编排 |

### P1 — 体验提升（14 条）

| 领域 | 条目 | 理由 |
|---|---|---|
| **消息资源与列表** | #1, #3（2 条） | chat-list 缺过滤投影，资源下载缺分片/安全落盘 |
| **文档资源保真度** | #8, #9（2 条） | 缺统一引用解析、路径门禁、资源回写编排 |
| **Base 字段与附件** | #18, #19, #20（3 条） | 缺 formula/lookup 门禁、分片上传完整链路 |
| **Base 仪表盘** | #21–#25（5 条） | 缺通用 block CRUD（当前仅 chart） |
| **Minutes 搜索与下载** | #16, #17（2 条） | 缺 me 解析、批量落盘、SSRF 防护 |

### P2 — 可延后（10 条）

| 领域 | 条目 | 理由 |
|---|---|---|
| **Todo 提醒与附件** | #5, #6（2 条） | 有底层能力，缺智能编排 |
| **Calendar 会议室** | #7（1 条） | 缺多 slot 聚合与维度过滤 |
| **Wiki 节点** | #15（1 条） | 缺 node 元数据解析 |
| **群搜索** | #2（1 条） | 缺对应底层 tool |
| **Apps 发布** | #40–#42（3 条） | 产品域差异（开放平台 vs 妙搭），语义不完全对应 |

---

## 不建议机械追平

- lark Apps DB、Spark 发布、Lark Drive/Wiki 特有对象模型属于平台差异
- DWS 的 attendance、DING、OA、report、agoal 和 event bus 是钉钉侧差异化能力
- DWS 已具备按姓名解析、跨产品智能编排、失败回滚和 usage→自定义 shortcut 沉淀闭环