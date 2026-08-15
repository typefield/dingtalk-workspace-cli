# 意图路由指南

当用户请求难以判断归属哪个产品时，只读取本指南的相关章节。产品内部的命令级消歧由各产品 skill 自己的 intent-guide / references 承载，不在本文件重复。

## 易混淆场景快速对照表

| 用户说... | 应该用 | 不要用 | 理由 |
|---|---|---|---|
| "搜一下 OAuth2 接入文档" | `devdoc` | `doc search` | 搜开放平台技术文档，不是钉钉内部内容 |
| "帮我建一个项目跟踪表" | `aitable` | `doc` / `sheet` | 结构化数据/行列操作，不是富文本或电子表格 |
| "帮我写个项目周报" | `doc` | `aitable` | 富文本内容创作，不是数据表 |
| "参照这个生成同样的" + 已有 alidocs URL | `drive copy + drive rename` → `doc block update`（`dingtalk-doc` 04-document.md 的 template-based-generation） | `doc read + doc create` 重写链 | adoc→markdown 是有损投影，copy 在 adoc 层保形复制后只改副本 |
| "创建一个电子表格" / "读 A1:D10" / "B2 写公式" | `sheet` | `aitable` | 单元格区域/公式/工作表，不是记录查询 |
| "读一下这个 xlsx 的数据" | `dws drive download --node` 后本地解析 | `sheet range read` | xlsx/xls/xlsm/csv 是上传的本地文件，sheet 只支持在线表格 |
| "把在线表格导出为 xlsx 文件" | `dws sheet export` | `dws drive download` | export 是 axls→xlsx 转换；download 只能下载已有 xlsx 节点 |
| "这个 alidocs 表格链接帮我看下" | 先 `dws drive info --node` 探测 `extension` 再路由 | 直接调 `sheet` | 禁止凭 URL 猜节点类型 |
| "帮我记一下明天要做的事" | `todo` | `doc` / `calendar` | 个人待办提醒 |
| "给自己留时间块 / 建个人日程" | `calendar event create` | `todo` | 个人 schedule 是日历事件 |
| "明早 9 点提醒我提交周报" | `todo`（先声明当前只支持 dueTime 截止时间） | `calendar` | todo 无独立精确 reminder |
| "把文件传到网盘 / 上传文件"（未指定目标） | `drive upload`（默认钉盘） | — | 提到"钉盘/网盘/我的文件"→ drive |
| "用本地文件覆盖钉盘/知识库里的文件" | 先 `drive info --node` 记录原 `name`；`extension=md` → `markdown overwrite --dry-run` 预览后 `--yes`；普通文件 → `drive upload --node --file-name "<原name>"`；`adoc`/`axls`/`able` 切对应内容 skill | 未探测类型就 `drive upload --node`，或普通文件覆盖时省略 `--file-name` | md 覆盖必须保留 diff 预览；省略 `--file-name` 会被隐式重命名 |
| "导入文件到我的文档" / "导入 Word/Excel" | `wiki space list --type myWikiSpace` 取 workspaceId → `doc import --workspace` | `drive upload`；`doc import` 不传目标参数 | doc import 是格式转换且必须传 `--folder`/`--workspace` 至少一个；upload 不转换 |
| "帮我看看知识库里的文件" / "在知识库里搜方案" | `wiki node list` / `wiki node search --workspace` | `drive list` / `drive search` | 明确空间上下文 → wiki |
| "搜一下有没有叫 XX 的文件" | `drive search` | `wiki node search` | 未指定空间 → 全局聚合搜索 |
| "列出钉盘团队空间" | `wiki space list --type orgSpace` | `drive list-spaces`（deprecated） | 空间管理归 wiki |
| "我的收藏 / 收藏这个 / 取消收藏" | `drive star list / add / remove` | — | 收藏管理归 drive |
| "在知识库里创建一个文档" | `wiki node create --type adoc` | `doc create` | 空间内建空文件实体归 wiki；doc create 是向已有文档写入内容 |
| "帮我看看收到的日报" | `report`（`dingtalk-misc`） | `doc` | 钉钉日志系统（日报/周报），不是文档 |
| "找一下张三 / 谁负责 XX / 工号查人 / 上下级" | `aisearch person` | `contact` | 语义找人走 aisearch，拿到 userId 后由 contact 补详情 |
| "13800138000 是谁"（完整手机号） | `contact user search-mobile` | `aisearch` | 完整手机号精确反查 |
| "研发部多少人 / 部门详情" | `contact dept get-info` | `contact dept list-members` | 人数/属性用 get-info；成员列表用 list-members |
| "搜一下智能化方案 / 我发给某人的邮件" | `aisearch enterprise / behavior` | 各产品自己的 search | 跨源企业内容与行为记录搜索 |
| "请假 / 加班 / 外出 / 出差 / 补卡 审批" | `attendance approve`（`dingtalk-misc`） | `oa approval` | 5 类考勤业务审批单，详见 misc 的 attendance-intent-guide |
| "报销 / 采购 / 用印 / 合同"、"我的待审批" | `oa approval`（`dingtalk-misc`） | `attendance approve` | 通用 OA 审批中心 |
| "发 DING / 紧急通知" | `ding`（`dingtalk-misc`） | `chat message send` | DING 是强提醒，独立顶层命令 |
| "把这段文字翻译成英文" | `chat text translate` | — | 纯文本翻译 |
| "把这个文档翻译成日文" | 先 `doc get` 再 `chat text translate` | 直接 translate 传文件 | translate 仅支持纯文本 |

---

## aitable vs doc vs sheet — 数据表格 vs 文档 vs 电子表格

- 有字段定义 / 记录增删改查 / 数据筛选 → `aitable`
- 纯文本 / Markdown / 富文本编辑 → `doc`
- 单元格区域读写 / 公式 / 多工作表 → `sheet`（位于 `dingtalk-misc`）

易误判：
- "在知识库中新建一个表格"（表格类型节点）→ `doc`/`wiki`，不是 `aitable`
- "帮我建个表记录项目进度"（结构化数据）→ `aitable`

## xlsx vs axls — 本地表格文件 vs 在线电子表格

alidocs 链接表面相同（`https://alidocs.dingtalk.com/i/nodes/{id}`），节点类型完全不同：

- 未知 alidocs URL → 必须先 `dws drive info --node <URL> --format json` 探测 `extension`
- `extension=axls` → `sheet`；`extension=xlsx/xls/xlsm/csv` → `dws drive download` 下载后本地解析
- 用户说"把在线表格导出为 xlsx" → `dws sheet export`（axls→xlsx 转换）
- 详见 [url-patterns.md](url-patterns.md) 与 `dingtalk-misc` 的 sheet reference

## devdoc vs drive search / wiki node search — 两种搜索

- 搜开放平台技术文档 / API 报错 / CLI 使用问题 → `devdoc`
- 搜用户自己的文档：不指定空间 → `drive search`（全局）；指定知识库 → `wiki node search --workspace`

## drive vs doc vs wiki — 存储层 vs 内容层 vs 空间管理层

**三层模型口诀**：操作换一种文件类型仍成立 → 存储层（`drive`）；只对特定格式有意义 → 内容层（`doc`/`sheet`）；对空间/节点的组织管理 → 空间管理层（`wiki`）。

- 搜索路由：不指定空间 → `drive search`；指定知识库 → `wiki node search`
- 创建路由：空间内建空节点 → `wiki node create`；直接写内容 → `doc create`；两者都要 → 先 `wiki node create` 再 `doc update`
- 列表路由：钉盘文件 → `drive list`；空间内文件 → `wiki node list` / `drive list --workspace`；空间本身 → `wiki space list`

## aisearch vs contact vs mail — 找人三件套

- 姓名/职责/上下级/工号/手机号线索的语义找人 → `aisearch person`
- 完整手机号精确反查、已有 userId 查详情/部门/角色 → `contact`
- 收件人不明确的邮件诉求 → 先解析人（aisearch/contact），再进 `mail`

## 视频会议已下线

发起会议、邀请入会、会中控制等 CLI 已下线，引导用户在钉钉客户端操作；涉及"约时间/订会议室/邀人"的诉求走 `calendar`。
