# 跨产品编排路由

仅在请求包含多个时序步骤、跨产品数据传递、批量采集、汇总分析或报告交付时读取。
当前发布包不包含独立 scenario skill；跨产品请求由本文件选择行动指南，再组合产品
skill 完成。

## 选择顺序

1. 按下表选择与用户目标最匹配的产品行动指南。
2. 显式读取流程涉及的产品 `SKILL.md`，不要预加载无关产品。
3. 前一步返回的真实 ID、URL 或结构化数据作为后一步输入。
4. 只读采集可并行；依赖上一步输出的步骤、写操作和验证步骤必须保持顺序。

## 产品行动指南

| 场景 | 典型目标 | 读取目标 |
|---|---|---|
| 消息沟通 | 发消息、查聊天、建群、机器人/Webhook 推送 | [`01-messaging.md`](../../dingtalk-chat/references/01-messaging.md) |
| 任务管理 | 创建/查询/完成待办、从来源提取任务 | [`02-task.md`](../../dingtalk-todo/references/02-task.md) |
| 会议日程 | 日程、会议室、闲忙、改期 | [`03-meeting.md`](../../dingtalk-calendar/references/03-meeting.md) |
| 文档知识 | 搜索、创建、编辑、分享文档和知识库分流 | [`04-document.md`](../../dingtalk-doc/references/04-document.md) |
| 工作汇报 | 日报周报、日志、多源会议或项目报告 | [`05-reporting.md`](../../dingtalk-misc/references/05-reporting.md) |
| 数据分析 | AI 表格读写、模板建表、统计报告 | [`06-data-analytics.md`](../../dingtalk-aitable/references/06-data-analytics.md) |
| 听记与会后 | 摘要、转写、会后待办和通知 | [`07-minutes.md`](../../dingtalk-minutes/references/07-minutes.md) |
| 通讯录 | 找人、部门、上下级、负责人和精确用户信息 | [`08-directory.md`](../../dingtalk-contact/references/08-directory.md) |

## 常用跨产品链路

共同规则：先解析人再执行写操作；同名多命中必须 `contact user get --ids` 消歧并由用户确认，禁止默认选第一个（详见 [`08-directory.md`](../../dingtalk-contact/references/08-directory.md)「多命中」）。

- **发邮件给同事**（aisearch → contact → mail）：`aisearch person` 取 userId → `contact user get` 取 email → `mail mailbox list` 取发件邮箱 → `mail message send`。`dingtalk-mail` 包内有发送辅助脚本。
- **创建日程并邀请同事**（aisearch → calendar）：`aisearch person` 取 userId → `calendar event create` → `calendar participant add --event <EVENT_ID> --users <USER_ID>`。`dingtalk-calendar` 包内有日程创建辅助脚本。
- **创建待办并指派**（aisearch → todo）：`aisearch person` 取 userId → `todo task create --title "..." --executors <USER_ID>`。

## 多场景消歧

- "催审批"是 OA 审批操作，走 `dingtalk-misc`，不是普通消息发送。
- "会后/听记生成待办"优先听记场景，不是普通待办创建。
- "安排日程/订会议室"走 calendar；"发起或预约视频会议"当前 CLI 不支持，请在钉钉客户端操作；"读取会后内容"走 minutes。
- "汇总多次会议/多个来源并生成报告"走工作汇报行动指南，不是单篇文档编辑。
- "整理项目全部讨论"是跨源汇总；"编辑指定文档"才是单产品 doc 操作。

批量采集与 ID 传递规则见 [conventions.md](conventions.md)；产品 skill 已内联完整步骤时直接执行其步骤，不再读取通用 recipe。

仍有歧义时，读取 [intent-guide.md](intent-guide.md) 的相关章节，不要全文加载。
