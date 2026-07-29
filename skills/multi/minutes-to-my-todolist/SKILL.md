---
name: minutes-to-my-todolist
version: 1.0.0
description: >
  从听记中提取待办事项，筛选出用户需要负责推进的任务，转换为钉钉待办。
  Use when user mentions "听记待办", "我的听记待办", "会议待办", "把听记里的待办加到待办",
  "帮我整理今天会议的待办", "听记里有什么要做的", "同步听记待办",
  "今天开会说了啥要做的", "会议任务转待办",
  or asks to "帮我把听记待办转成钉钉待办".
  Do NOT use for 查看听记摘要、手动创建待办、查看已有待办列表、修改或删除待办.
  Distinct from dingtalk-minutes(查看听记内容) and dingtalk-todo(手动创建/查询普通待办).
metadata:
  category: scenario
  stability: experimental
  requires:
    bins:
      - dws
    skills:
      - dingtalk-minutes
      - dingtalk-todo
      - dingtalk-contact
  cliHelp: "dws minutes +action-items --help && dws todo task create --help"
---

# 听记待办转钉钉待办（L2 场景技能）

## 前置条件 — 执行操作前必读

> 本配方用 `dws` CLI 编排多个产品。**使用 Read 工具按需读取下列 skill**（`dws-shared` 必读，产品 skill 按实际步骤读取，无需一次性全读）：
>
> 1. **MUST Read [`dws-shared`](../dws-shared/SKILL.md)** — 全局执行契约、安全底线。**所有操作通用，第一条必读。**
> 2. **Read [`dingtalk-minutes`](../dingtalk-minutes/SKILL.md)** — 听记定位 / 待办提取的命令与参数细节
> 3. **Read [`dingtalk-todo`](../dingtalk-todo/SKILL.md)** — 待办创建的命令与参数细节
> 4. **Read [`dingtalk-contact`](../dingtalk-contact/SKILL.md)** — 当前用户信息的命令与参数细节
>
> 本配方「执行流程」已**内联各步骤的 `dws` 命令**，可直接执行；产品 skill 仅作按需参考。

## 身份

本技能**仅支持 user 身份**。听记和待办都是个人资源，bot 无法访问。

```bash
dws auth status    # 确认 token_valid=true 且 identity=user
```

从钉钉听记中提取待办事项，识别用户需要负责推进的任务，确认后创建为钉钉待办。

## 涉及产品

| 产品 | 用途 | 对应命令 | 安全等级 |
|------|------|---------|----------|
| 听记 (minutes) | 定位听记、提取待办 | `dws minutes +list-mine` / `+detail --artifacts todos` / `+action-items` | 只读 |
| 待办 (todo) | 创建待办任务 | `dws todo task create` | 写入-需确认 |
| 通讯录 (contact) | 获取当前用户 userId | `dws contact user get-self` | 只读 |

## 能力清单

| 动作 | 命令 | 必填参数 | 安全等级 | 边界与注意事项 |
|------|------|----------|----------|----------------|
| 获取当前用户 | `dws contact user get-self` | -- | 只读 | 返回 userId + name；别名 self/me/whoami/current 等价；禁止用 `get --ids me` 代替（返回空数据的假成功） |
| 最新听记待办（快捷） | `dws minutes +action-items` | -- | 只读 | 自动取最新一条；无听记返回「暂无妙记」（非报错）；仅覆盖最新一条 |
| 按关键词找听记 | `dws minutes +list-mine --query <kw>` | -- | 只读 | `--limit` 默认 10；多条匹配时列出标题+时间请用户选择，不要自动取第一条 |
| 提取指定听记待办 | `dws minutes +detail --id <taskUuid> --artifacts todos` | --id | 只读 | 用 `--artifacts todos` 只拉待办，不要默认全量（会连逐字稿一起拉，浪费 context） |
| 创建待办 | `dws todo task create --title <t> --executors <userId>` | --title, --executors | 写入-需确认 | `--executors` 必须是真实 userId（来自 get-self 或前序返回）；`--due` 是 ISO-8601；单批不超 20 条 |

## 意图判断

| 用户说 | 线索 | 对应动作 |
|--------|------|---------|
| "把最新会议的待办加到我的待办" | 最新 + 待办 + 转换 | `+action-items` → 筛选 → 创建 |
| "把周会听记里的待办转成待办" | 指定听记 + 转换 | `+list-mine --query 周会` → `+detail --artifacts todos` → 创建 |
| "看看今天开会有什么要做的" | 会议 + 要做的 | 仅提取展示，**不创建** |
| "把听记里我负责的转成待办" | 我负责 | 提取 → 按 userId/姓名筛选 → 创建 |

## 易混淆场景

| 用户说 | 线索 | 应路由到 | 原因 |
|--------|------|---------|------|
| "看一下上次会议讲了什么" | 会议 + 内容 | dingtalk-minutes（+detail --artifacts summary） | 要摘要，不是待办 |
| "创建一个待办：明天提交报告" | 创建 + 具体内容 | dingtalk-todo（task create） | 手动创建，不涉及听记 |
| "我有哪些待办没做完" | 待办 + 未完成 | dingtalk-todo（+get-my-tasks --status false） | 查已有待办 |

## 操作指引

1. MUST: 所有命令加 `--format json`
2. MUST: 创建待办前展示待创建列表并获得用户确认；确认后才执行 `dws todo task create`
3. MUST: `--executors` 使用阶段 1 获取的真实 userId，禁止编造
4. 单次批量创建不超过 20 条；超过时分批并逐批确认

## 执行流程

详细阶段说明（含校验点、决策点、失败处理）见
[references/workflow-details.md](references/workflow-details.md)。

概览：

```
阶段 1: contact user get-self                       ──► userId
阶段 2: 定位听记（+action-items 或 +list-mine → +detail）──► 待办原始列表
阶段 3: 筛选我负责的待办（userId/姓名匹配）           ──► 待创建列表
阶段 4: 用户确认                                     ──► 逐条 todo task create
阶段 5: 汇总报告（成功 N 条 / 跳过 M 条）
```
