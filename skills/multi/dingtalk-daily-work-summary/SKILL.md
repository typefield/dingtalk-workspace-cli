---
name: dingtalk-daily-work-summary
version: 1.0.0
description: >
  生成今日工作全景摘要：汇总日程、待办、待处理审批、会后待办等多维度工作数据，输出结构化日报。
  Use when user mentions "今日工作总结", "今天有什么安排", "生成今日日报", "今日事项汇总",
  "早报摘要", "开工摘要", "standup report", "今天要做什么".
  Do NOT use for 单独查看日程（用日历技能）、单独查看待办（用待办技能）、
  单独查看审批（用 OA 技能）、提交日志（用 report 技能）.
  Distinct from dingtalk-calendar(单独查日程), dingtalk-todo(单独查待办),
  and dingtalk-report(日志提交/查询).
metadata:
  category: scenario
  stability: experimental
  requires:
    bins:
      - dws
    skills:
      - dingtalk-calendar
      - dingtalk-todo
      - dingtalk-oa
      - dingtalk-minutes
  cliHelp: "dws calendar +agenda --help && dws todo +get-my-tasks --help"
---

# 今日工作总结（L2 场景技能）

## 前置条件 — 执行操作前必读

> 本配方用 `dws` CLI 编排多个产品。**使用 Read 工具按需读取下列 skill**（`dws-shared` 必读，产品 skill 按实际步骤读取，无需一次性全读）：
>
> 1. **MUST Read [`dws-shared`](../dws-shared/SKILL.md)** — 全局执行契约、安全底线。**所有操作通用，第一条必读。**
> 2. **Read [`dingtalk-calendar`](../dingtalk-calendar/SKILL.md)** — 日程查询的命令与参数细节
> 3. **Read [`dingtalk-todo`](../dingtalk-todo/SKILL.md)** — 待办查询的命令与参数细节
> 4. **Read [`dingtalk-oa`](../dingtalk-oa/SKILL.md)** — 待处理审批查询的命令与参数细节
> 5. **Read [`dingtalk-minutes`](../dingtalk-minutes/SKILL.md)** — 听记待办提取的命令与参数细节
>
> 本配方「执行流程」已**内联各步骤的 `dws` 命令**，可直接执行；上面的产品 skill 仅作参数全集与边界约束的**按需参考**——遇到本文未覆盖的参数/字段再去读。

## 身份

本技能**仅支持 user 身份**（`dws auth login` 后的个人登录态）。所有查询均返回当前登录用户自己的数据。不支持 bot 身份——bot 没有日程、待办、审批等个人资源。

```bash
dws auth status    # 确认 token_valid=true 且 identity=user
```

通过 `dws` 命令聚合钉钉多产品数据，生成结构化工作日报。

## 严格禁止 (NEVER DO)

- 不要在模块查询失败时终止流程，应跳过继续执行
- 不要编造数据，所有数据必须来自实际查询结果
- 不要输出空模块，无数据的模块直接跳过不展示
- 不要把已完成待办当成「待办」展示进摘要

## 严格要求 (MUST DO)

- 所有命令必须加 `--format json` 以获取可解析输出
- 每个模块独立查询，互不影响
- 时间参数统一使用今日 00:00:00 至 23:59:59；calendar 用 ISO-8601，todo 用 Unix 毫秒时间戳
- 最终输出必须按照输出模板格式化

## 涉及产品

| 产品 | 用途 | 对应命令 | 安全等级 |
|------|------|---------|----------|
| 日历 (calendar) | 获取今日日程 | `dws calendar +agenda` | 只读 |
| 待办 (todo) | 获取未完成待办 | `dws todo +get-my-tasks --status false` | 只读 |
| OA 审批 (oa) | 获取待我处理的审批 | `dws oa +list-pending` | 只读 |
| 听记 (minutes) | 获取最新会后待办 | `dws minutes +action-items` | 只读 |

## 能力清单

| 动作 | 命令 | 必填参数 | 安全等级 | 边界与注意事项 |
|------|------|----------|----------|----------------|
| 获取今日日程 | `dws calendar +agenda` | -- | 只读 | 不传时间默认今天 00:00~23:59；`--start/--end` 仅 ISO-8601，不支持 "tomorrow"；已取消日程自动过滤；超 40 天自动拆分查询 |
| 获取未完成待办 | `dws todo +get-my-tasks --status false` | --status | 只读 | 不带 `--status` 会同时返回已完成+未完成；`--plan-finish-end` 是 Unix 毫秒（非 ISO）；`--role-types` 默认 executor |
| 待处理审批 | `dws oa +list-pending` | -- | 只读 | 返回待当前用户审批的流程；无待审批时返回空列表（非报错） |
| 最新听记待办 | `dws minutes +action-items` | -- | 只读 | 自动取最新一条听记；无听记时返回「暂无妙记」（非报错）；仅覆盖最新一条，多条听记需逐条 `+detail` |

## 意图判断

| 用户说 | 线索 | 对应动作 |
|--------|------|---------|
| "今天有什么安排" | 今天 + 安排 | 完整流程：日程 + 待办 + 审批 |
| "生成今日日报" | 日报 | 完整流程 + 输出模板 |
| "今天要开什么会" | 今天 + 会 | 仅日程模块 |
| "开会说了啥要做的" | 会议 + 要做的 | 仅听记待办模块 |
| "有什么审批要我处理" | 审批 + 处理 | 仅审批模块 |

## 易混淆场景

| 用户说 | 线索 | 应路由到 | 原因 |
|--------|------|---------|------|
| "帮我提交今天的日报" | 提交 + 日报 | dingtalk-report | 提交日志是写操作，不是生成摘要 |
| "明天有什么会" | 明天 + 会 | dingtalk-calendar (+agenda --start/--end) | 单产品查询，无需编排 |
| "创建一个待办" | 创建 + 待办 | dingtalk-todo | 写操作，不是汇总 |
| "把听记待办转成钉钉待办" | 听记 + 转 + 待办 | dingtalk-minutes + dingtalk-todo（写） | 是转换流程，不是只读摘要 |

## 执行流程

详细阶段说明（含校验点、决策点、失败处理）见
[references/workflow-details.md](references/workflow-details.md)。

概览：

```
阶段 1: calendar +agenda                    ──► 今日日程
阶段 2: todo +get-my-tasks --status false   ──► 未完成待办
阶段 3: oa +list-pending                    ──► 待处理审批
阶段 4: minutes +action-items               ──► 最新会后待办（可选）
阶段 5: AI 汇总                              ──► 结构化日报
```

## 输出模板

```
## 今日工作摘要（YYYY-MM-DD 星期X）

### 📅 日程安排（N 场）
- HH:MM–HH:MM 「标题」 [地点]（时间重叠标 ⚠️ 冲突）

### ✅ 未完成待办（M 项）
- [P1] 任务标题（截止 MM-DD）

### 📋 待处理审批（K 件）
- 「流程标题」 来自 {发起人}

### 🎙 会后待办（最新听记）
- 待办内容（参与人）

### 💡 提示
- （可选）日程密集/空闲时段、已逾期待办
```
