# DWS Skill 调度侧优化点

> 基于代码库深度审计（2026-07-29），聚焦 skill 系统的结构、路由、生成与调度。

---

## 当前 Skill 架构

```
skills/
├── mono/SKILL.md              ← 单文件全产品（大 context agent 用）
│   ├── references/            ← 32 产品文档 + 11 best practices + 跨切面文档
│   │   ├── products/          ← 按产品的命令参考（aitable/ ~25 文件, doc/, sheet/ ~17 文件）
│   │   ├── best_practices/    ← 01-messaging … 11-minutes-speaker-correct + lite-recipes
│   │   └── (root)             ← error-codes, recovery-guide, intent-guide, url-patterns 等
│   └── scripts/               ← ~40 Python helper（aitable_export, calendar_schedule 等）
├── multi/                     ← 28 产品目录（选择性加载）
│   ├── dws-shared/            ← 强制前置（auth/全局规则/错误处理/多组织）
│   └── dingtalk-<product>/    ← SKILL.md + references/<product>.md + 可选 scripts/
├── embed.go                   ← go:embed（疑似死代码，见 #7）
└── (root) skills_embed.go     ← 实际消费的 embed（dws.EmbeddedSkills）

安装：dws skill setup --mode mono|multi → 16 个 agent 目录
路由：纯 prose（意图决策树 + frontmatter description）
生成：仅 shortcut 表格自动生成（gen_skill_shortcut_sections.py），其余手写
校验：skill-command-integrity CI gate + skill_verify build tag 测试
```

### 支持的 Agent 目录（16 个）

`.agents`, `.claude`, `.cursor`, `.qoder`, `.qoderwork`, `.gemini`, `.codex`, `.github`, `.windsurf`, `.augment`, `.cline`, `.amp`, `.kiro`, `.trae`, `.openclaw`, `.hermes`

（`dws skill install` 额外支持 `opencode`；plugin sync 仅覆盖其中 5 个）

---

## 缺失的调度能力

### 1. 无机器可读的 Skill Manifest / Registry

**现状**：skill 发现完全靠文件系统约定（目录里有 `SKILL.md` 就是一个 skill）。路由靠 LLM 读 prose 自行判断。无 index、无依赖声明、无能力标签。

**问题**：
- Agent runtime 无法程序化地选择 skill（只能全量加载或靠 description 模糊匹配）
- 无 `requires` / `conflicts` / `provides` 语义
- `dws-shared` 是硬编码特例（`ensureMandatorySharedSkill`），不是通用依赖机制
- 新增 skill 后无自动注册，agent 不知道它的存在

**建议**：每个 skill 目录加 `manifest.json`：

```json
{
  "name": "dingtalk-calendar",
  "version": "1.2.0",
  "requires": ["dws-shared@>=1.0"],
  "provides": ["calendar", "meeting", "room", "freebusy"],
  "triggers": ["日程", "会议", "calendar", "schedule", "会议室"],
  "cli_version": ">=1.0.40",
  "context_budget": 8000,
  "priority": 10,
  "stability": "stable"
}
```

框架生成 `skills/index.json`（全量 registry），agent runtime 按 trigger 关键词 + context budget 选择性加载。

**预期收益**：skill 发现从 O(N) 全量读取降到 O(1) 索引查询；依赖关系有框架保证。

---

### 2. 无 Skill 组合 / 链式调度

**现状**：跨产品任务（如"查日程 → 找空闲人 → 建会议 → 发群通知"）需要 agent 自己串联 4 个 product skill。无框架级编排。

**问题**：
- 每个 product skill 只知道自己产品的命令
- 跨产品协作靠 mono skill 的 prose 描述或 agent 自行推理
- 无 "workflow skill"（多产品编排模板）
- 无 skill 间的输入/输出契约（calendar 输出的 userId 如何传给 chat）

**建议**：
- 新增 `skills/workflows/` 目录，存放跨产品编排 skill
- Workflow skill 声明 `uses: [dingtalk-calendar, dingtalk-contact, dingtalk-chat]`
- 框架在加载 workflow skill 时自动拉取依赖 product skill
- 定义 skill 间的数据契约（`outputs: {userId: string}` / `inputs: {userId: string}`）

**示例**（`skills/workflows/schedule-meeting.md`）：
```yaml
---
name: schedule-meeting
uses: [dingtalk-calendar, dingtalk-contact, dingtalk-chat]
triggers: ["约会议", "订会议室", "schedule meeting"]
---
## 流程
1. dingtalk-contact: 解析参会人 → userId[]
2. dingtalk-calendar: 查空闲 → 推荐时间段
3. dingtalk-calendar: 创建日程 + 预订会议室
4. dingtalk-chat: 发群通知
```

**预期收益**：跨产品任务从 "agent 自行推理 4 步" 降到 "加载 1 个 workflow skill 按流程执行"。

---

### 3. 无 Context Budget 管理

**现状**：mono skill 全量加载 ~50K tokens（SKILL.md + references）。multi 模式 28 个 product skill 全装 ~80K。Agent 的 context window 是有限资源。

**问题**：
- 无按任务动态加载/卸载 skill 的机制
- references/ 是全量嵌入还是按需读取，取决于 agent runtime（不受 DWS 控制）
- 无 "lite" 模式（只加载命令表，不加载 best practices）
- 小 context agent（如 8K window）无法使用 DWS skill

**建议**：
- manifest 声明 `context_budget`（预估 token 数）
- `dws skill setup --budget 20000` 按 budget 裁剪（只装 top-N 相关 skill）
- references 分层：
  - `core`（必加载：命令表 + safety + 意图表）
  - `extended`（按需：best practices + 详细参数）
  - `scripts`（仅执行时读：Python helper）
- `dws skill context "<intent>"` 输出该意图所需的最小 skill 集合 + 预估 token

**预期收益**：小 context agent 可用；大 context agent 减少噪音；token 成本降低 40-60%。

---

### 4. Skill 生成覆盖率不足

**现状**：只有 shortcut 表格是自动生成的（`gen_skill_shortcut_sections.py`）。产品总览表、意图表、危险操作表、命令参考全部手写。

**问题**：
- 手写内容与 Cobra 树 / Schema catalog 会 drift
- CI 的 `skill-command-integrity` gate 只检查命令路径存在性，不检查参数/描述一致性
- 新增命令后 skill 文档不会自动更新
- `references/products/*.md` 与 `schema_catalog/tools/*.json` 信息重复但格式不同

**建议**：
- 从 SchemaRegistry 自动生成：
  - 产品总览表（命令列表 + safety + 一句话描述）
  - 命令参数表（flag name + type + required + default + usage）
  - safety 注解表（effect / risk / confirmation）
- `references/products/<product>.md` 改为模板 + 生成（类似 `schema_catalog` 的 shard 模式）
- CI gate 升级为 "skill 内容与 catalog 零 drift"（类似 `check-generated-drift.sh`）
- 保留手写部分：意图决策树、best practices、跨产品协作指南（这些是 prose，无法自动生成）

**预期收益**：新命令上线后 skill 文档自动同步；消除 "命令存在但 skill 没写" 的 gap。

---

### 5. 无运行时 Skill 选择 / 路由

**现状**：skill 安装是静态的（`dws skill setup` 一次性写入 agent 目录）。运行时 agent 自己决定读哪个 skill。DWS 无 "根据用户输入推荐 skill" 的能力。

**问题**：
- 16 个 agent 目录的 skill 格式不统一（有的读 SKILL.md，有的读 system prompt）
- 无 `dws skill recommend "帮我订明天的会议室"` → 输出应加载的 skill 列表
- Plugin skill 只同步到 5 个 agent 目录（hardcoded），与 core skill 的 16 个不一致
- 无 skill 使用统计（哪些 skill 被频繁加载、哪些从未使用）

**建议**：
- `dws skill recommend "<intent>"` 子命令：基于 manifest triggers + schema selection metadata 输出推荐 skill 排序
- 统一 agent 目录列表为单一 source of truth（`internal/app/agent_dirs.go`），所有消费方引用
- Plugin skill sync 使用同一列表
- 可选：`dws skill stats` 从 audit log 统计 skill 使用频率

**预期收益**：多 skill 场景下选择效率提升；plugin skill 覆盖所有 agent。

---

### 6. 无 Skill 版本兼容性校验

**现状**：frontmatter `cli_version` 是声明性的，安装时不校验。有一个非法值 `1.0.37+`（不是合法 semver range）。

**问题**：
- 旧 CLI + 新 skill（或反过来）可能引用不存在的命令
- 无 install-time 兼容性检查
- 无 skill 自身的 changelog / migration guide

**建议**：
- `dws skill setup` 时校验 `cli_version` 约束，不满足则 warn（不 block，因为 skill 可能仍部分可用）
- 修复非法 semver 值（`1.0.37+` → `>=1.0.37`）
- CI 加 lint：所有 SKILL.md 的 `cli_version` 必须是合法 semver range
- 可选：skill 目录加 `CHANGELOG.md`，`dws skill setup --diff` 展示变更

**预期收益**：防御性保障；减少 "skill 引用了不存在的命令" 的运行时错误。

---

### 7. 死代码 / 不一致

| 问题 | 位置 | 影响 | 建议 |
|---|---|---|---|
| 双重 embed | `skills/embed.go`（`skills.FS`）+ `skills_embed.go`（`dws.EmbeddedSkills`） | 二进制膨胀；`skills.FS` 无消费者 | 删除 `skills/embed.go` |
| 悬挂注入标记 | 24 个 product skill 的 `<!-- SAFETY_PREAMBLE_INJECT -->` | 无生成器消费，永远不被替换 | 实现注入或删除标记 |
| 死目录 | `skills/multi/dingtalk-devapp/`（无 SKILL.md） | 永远不会被安装 | 删除或补充 SKILL.md |
| 三套 agent 目录列表 | setup(16) / command(17) / upgrade(16) / plugin sync(5) | 静默 drift | 统一为单一 `agent_dirs.go` |
| `cli_version` 非法值 | 某 SKILL.md 的 `1.0.37+` | 无法被 semver 库解析 | 修复为 `>=1.0.37` |

---

## 与 lark-cli 对比

| 维度 | DWS | lark-cli |
|---|---|---|
| Skill 格式 | SKILL.md（prose + frontmatter） | 无独立 skill 系统；命令 metadata 内嵌于 Go 代码 |
| 路由 | 纯 prose 意图决策树 | 无（agent 直接读 --help） |
| 生成 | 仅 shortcut 表格 | 无（全手写） |
| 多 agent 支持 | 16 个 agent 目录 | 无（仅 CLI 自身） |
| Plugin skill | 有（5 个 agent 目录） | 无 |
| 版本管理 | 二进制 embed + setup 命令 | 无 |

DWS 的 skill 系统在 AI agent 生态覆盖上领先 lark-cli，但在机器可读性、自动生成、动态路由方面仍有显著差距。

---

## 优先级总览

| 优先级 | 优化点 | 影响面 | 工作量 |
|---|---|---|---|
| **P0** | #1 Skill Manifest + Registry | 所有后续调度能力的基础 | 中（~300 行 + 28 个 manifest.json） |
| **P0** | #4 生成覆盖率提升 | 防止 skill ↔ catalog drift | 中（~400 行生成器） |
| **P1** | #3 Context Budget 管理 | AI agent token 成本 | 中（~200 行） |
| **P1** | #5 运行时 Skill 推荐 | 多 skill 选择效率 | 小（~150 行） |
| **P1** | #7 死代码清理 | 减少混淆 + 二进制体积 | 小（~50 行删除） |
| **P2** | #2 Workflow Skill | 跨产品编排 UX | 中（新目录 + 格式设计） |
| **P2** | #6 版本兼容性校验 | 防御性保障 | 小（~80 行） |

---

## 多 Skill 架构优化（参考 WorkBuddy 模式）

> 基于 GitHub 开源项目 [darker2016/workbuddy-skill-groups](https://github.com/darker2016/workbuddy-skill-groups)（39 个专家团、13 个一级分类）和 [Tugoukezhang/workbuddy-skills](https://github.com/Tugoukezhang/workbuddy-skills)（78 个独立 skill）的架构分析。

### WorkBuddy 核心模式

```
skill-group/
├── README.md                    ← 团队宣言：成员表 + 工作流概览
└── skills/
    ├── <lead>/SKILL.md          ← 主理人：意图识别 → 工作流选择 → 成员调度 → 汇编报告
    ├── <member-a>/SKILL.md      ← 专业成员：独立产出，不直连其他成员
    ├── <member-b>/SKILL.md
    └── ...
```

**三层结构**：
1. **主理人（Lead）**：不做具体产出，只做编排。识别意图 → 选择 SOP → 按阶段调度成员 → 收集去重合并 → 输出结构化报告。
2. **专业成员（Member）**：各自独立的 SKILL.md，可单独加载也可被主理人调度。有稳定 Agent ID（如 `code-reviewer`、`architect`）。
3. **工作流 SOP**：主理人 SKILL.md 内声明多个命名工作流（如"全面代码审查"、"事故响应"），每个工作流是有序的成员调度步骤。

**协作铁律**：
- 主理人严禁代写成员产出
- 成员间严禁直连通信，所有跨成员信息流经主理人中转
- 团队创建必须且只能由主理人执行
- 成员结论为准，主理人只做编排与汇编

### DWS 现状 vs WorkBuddy

| 维度 | WorkBuddy | DWS 现状 | 差距 |
|---|---|---|---|
| 组织形态 | 域团队（1 lead + 5-6 members） | 28 个扁平 product skill + 1 shared | 无层次 |
| 路由机制 | Lead SKILL.md 内的 SOP 决策树 | 无（靠 agent 自行推理读哪个 skill） | 无路由 |
| 跨域编排 | 主理人中转 + 工作流阶段 | 无（跨产品任务靠 mono skill prose） | 无编排 |
| 成员调度 | 显式 Agent ID + subagent_type | 无 | 无调度协议 |
| 协作约束 | 铁律（不代写/不直连/中转） | 无 | 无约束 |
| 可组合性 | 成员可独立使用也可被编排 | product skill 独立但无组合协议 | 无契约 |
| 工作流声明 | 命名 SOP（步骤 + 成员 + 产出） | 无 | 无 SOP |

### DWS 多 Skill 优化方案

#### 方案：域团队 + 路由 Lead + 工作流 SOP

将 28 个 product skill 重组为 **6 个域团队**，每个团队有一个路由 Lead：

```
skills/multi/
├── dws-shared/                      ← 全局前置（不变）
├── team-collaboration/              ← 协作沟通域
│   ├── SKILL.md                     ← Lead：路由 chat/calendar/contact/todo/mail
│   └── members/
│       ├── dingtalk-chat/
│       ├── dingtalk-calendar/
│       ├── dingtalk-contact/
│       ├── dingtalk-todo/
│       └── dingtalk-mail/
├── team-knowledge/                  ← 文档知识域
│   ├── SKILL.md                     ← Lead：路由 doc/drive/wiki/minutes
│   └── members/
│       ├── dingtalk-doc/
│       ├── dingtalk-drive/
│       ├── dingtalk-wiki/
│       └── dingtalk-minutes/
├── team-data/                       ← 数据表格域
│   ├── SKILL.md                     ← Lead：路由 aitable/sheet/report
│   └── members/
│       ├── dingtalk-aitable/
│       ├── dingtalk-sheet/
│       └── dingtalk-report/
├── team-devops/                     ← 开发运维域
│   ├── SKILL.md                     ← Lead：路由 dev/devdoc/event/pat
│   └── members/
│       ├── dingtalk-dev/
│       ├── dingtalk-devdoc/
│       ├── dingtalk-event/
│       └── dingtalk-pat/
├── team-admin/                      ← 行政 HR 域
│   ├── SKILL.md                     ← Lead：路由 attendance/hrbrain/oa/ding
│   └── members/
│       ├── dingtalk-attendance/
│       ├── dingtalk-hrbrain/
│       ├── dingtalk-oa/
│       └── dingtalk-ding/
└── team-media/                      ← 音视频域
    ├── SKILL.md                     ← Lead：路由 live/markdown
    └── members/
        ├── dingtalk-live/
        └── dingtalk-markdown/
```

#### Lead SKILL.md 模板

```markdown
---
name: team-collaboration
description: 协作沟通域主理人。路由 chat/calendar/contact/todo/mail 五个产品，
  处理消息发送、日程管理、联系人解析、任务分配、邮件收发等协作场景。
cli_version: ">=1.0.40"
metadata:
  category: collaboration
  stability: stable
  members: [dingtalk-chat, dingtalk-calendar, dingtalk-contact, dingtalk-todo, dingtalk-mail]
---

# 协作沟通域 - 主理人

你是协作沟通域的路由主理人。你不直接执行产品命令，而是：
1. 识别用户意图，选择目标产品成员
2. 跨产品任务按工作流 SOP 编排多成员
3. 收集成员产出，合并输出

## 成员表

| Agent ID | 产品 | 专长 |
|---|---|---|
| `dingtalk-chat` | 群聊/IM | 消息发送、群管理、消息搜索、资源下载 |
| `dingtalk-calendar` | 日历 | 日程 CRUD、会议室预订、空闲查询 |
| `dingtalk-contact` | 通讯录 | 人员搜索、部门查询、userId 解析 |
| `dingtalk-todo` | 待办 | 任务创建、分配、完成、提醒 |
| `dingtalk-mail` | 邮件 | 收发、草稿、附件、分类 |

## 路由决策树

- 涉及"消息/群/聊天/发送" → `dingtalk-chat`
- 涉及"日程/会议/日历/预订" → `dingtalk-calendar`
- 涉及"找人/通讯录/userId" → `dingtalk-contact`
- 涉及"待办/任务/提醒" → `dingtalk-todo`
- 涉及"邮件/收发/附件" → `dingtalk-mail`
- 跨产品 → 走工作流 SOP

## 工作流 SOP

### SOP-1: 约会议（calendar + contact + chat）

1. `dingtalk-contact`：解析参会人姓名 → userId[]
2. `dingtalk-calendar`：查空闲 → 推荐时间段 + 预订会议室
3. `dingtalk-calendar`：创建日程（attendees = userId[]）
4. `dingtalk-chat`：发群通知（日程链接 + 时间地点）

### SOP-2: 任务跟进（todo + chat + contact）

1. `dingtalk-contact`：解析负责人 → userId
2. `dingtalk-todo`：创建待办（executor = userId）
3. `dingtalk-chat`：@负责人通知

### SOP-3: 邮件转任务（mail + todo）

1. `dingtalk-mail`：读取邮件内容
2. `dingtalk-todo`：提取 action items → 创建待办
3. `dingtalk-mail`：回复确认

## 协作铁律

- 主理人不直接执行 `dws` 命令，只路由到成员
- 成员间不直连，跨成员数据流经主理人中转
- 每个成员的产出独立，主理人只做合并
```

#### 与现有架构的兼容

| 现有 | 变化 | 兼容策略 |
|---|---|---|
| `dws skill setup --mode multi` | 新增 `--mode team` | team 模式安装 6 个 Lead + 按需 members |
| 28 个 product skill 目录 | 移入 `team-*/members/` | 保留独立 SKILL.md，仍可单独安装 |
| `dws-shared` | 不变 | 所有 Lead 和 member 仍声明 PREREQUISITE |
| mono 模式 | 不变 | 大 context agent 继续用全量 mono |
| `gen_skill_shortcut_sections.py` | 扩展 | 为每个 Lead 生成成员 shortcut 汇总表 |
| CI `skill-command-integrity` | 扩展 | 校验 Lead 路由表中的命令与 catalog 一致 |

#### 预期收益

1. **路由效率**：agent 只需加载 1 个 Lead（~3K tokens）即可路由到正确产品，无需读 28 个 description
2. **跨产品编排**：SOP 声明式编排，agent 按步骤执行而非自行推理
3. **Context 节省**：按需加载 member（Lead 判断后才拉取），而非全量 28 个
4. **可组合性**：member 仍可独立使用（向后兼容），也可被 Lead 编排
5. **可扩展性**：新增产品只需加入对应 team 的 members/ + 更新 Lead 路由表

#### 实施步骤

| 阶段 | 内容 | 工作量 |
|---|---|---|
| Phase 1 | 设计 team 目录结构 + Lead SKILL.md 模板 + manifest.json | 1 周 |
| Phase 2 | 实现 `dws skill setup --mode team` + Lead 生成器 | 1 周 |
| Phase 3 | 迁移 28 个 product skill 到 6 个 team（保留独立可用） | 1 周 |
| Phase 4 | 编写 6 个 Lead 的路由决策树 + 3-5 个跨域 SOP | 1 周 |
| Phase 5 | CI gate 扩展 + E2E 测试（Lead 路由正确性） | 3 天 |