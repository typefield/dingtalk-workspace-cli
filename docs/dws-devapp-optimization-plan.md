# DWS devapp（开放平台应用管理）优化方案

> 状态：实施记录（Phase 1/2 已分批落地，真实写终态仍在取证）
>
> 日期：2026-08-08
>
> DWS 口径：分支 `codex/eval-remediation-20260807`；以当前工作树和 `docs/evaluation-remediation-202608.md` 为准，不再固化易漂移的 HEAD。
>
> 对照基线：[`shortcut-lark-alignment.md`](shortcut-lark-alignment.md)（lark `apps` 服务）；
> 方法论基线：Multi IM 三层优化（执行/分页、路由/错误、skill/渐进路由）

## 1. 一句话结论

devapp 已经具备三层对齐的一半基础：skill 生成链路（`gen_skill_shortcut_sections.py`
覆盖 `devapp.md`）、概念地图/ID 体系文档、通用失败分类（`runner.go` 挂载的
`server_failure_classifier`）和 typed 分页错误（`helpers/devapp.go` 的
`pagination_conflict/incomplete/invalid`）都已存在。缺的是三件事：

1. **执行层深度**——分页保护只有 1 个证据型测试，没有硬页上限/去重/游标停滞的回归面；
2. **错误层闭环**——devapp 的分页 subtype 未进 RFC-0003 稳定 registry，
   classifier 的 PARAM_ERROR 模式全是 IM 特征（`opencid`/`openconversationid`），
   devapp 高频参数误用（unifiedAppId/appKey/agentId 混用）没有分类；
3. **真实性验证**——读取分页和成员列表已有 live evidence，核心写与删除已有零写/受控 caller 扫描；成员写与版本生命周期已建立 requested/not_verified、pending/unknown 契约，但 helper 真实成功响应、逐成员回读和版本发布终态仍未取证。

本方案不复制 chat 的实现，而是把同一套方法论落到 devapp 的现有机制上：
subtype 走 [RFC-0003](rfcs/0003-error-contract-subtype-governance.md) 治理流程，
skill 段落走现有生成器，真实性验证走 chat audit 脚本同款骨架。

## 2. 当前基础（已验证）

### 2.1 已经做对的部分

| 能力 | 现状 | 证据 |
|---|---|---|
| Skill 生成链路 | `devapp.md` shortcut 表由生成器维护（`VISIBLE_SHORTCUTS_START/END` marker，line 27） | `scripts/gen_skill_shortcut_sections.py:37`；devapp 独用「概念地图」锚点（line 254） |
| 领域文档 | 概念地图、ID 体系（unifiedAppId/appKey=clientId/agentId/robotCode）、生效模型（版本通道 + completionState 硬门禁）、边界与角色均已成文 | `skills/multi/dingtalk-misc/references/devapp.md` §概念地图 |
| 语义目录 | `semantic_catalog_devapp.json` 已存在（version/service/default_availability/shortcuts） | `internal/shortcut/semantic_catalog_devapp.json` |
| 通用失败分类 | classifier 挂在通用 runner 路径，devapp 调用自动受益（NETWORK_ERROR→backend_dependency_unavailable 等） | `internal/app/runner.go:724,749`；`internal/app/server_failure_classifier.go` |
| Typed 分页错误 | `devAppPaginationMeta`/`devAppPaginationError` 输出 `pagination_conflict/incomplete/invalid` | `internal/helpers/devapp.go:1957-2023`；台账 `docs/agent-scans/error-contract-inventory-20260808.md:110-112` |
| 分页证据测试 | `TestDevAppProjectedListsPreservePaginationEvidence` | `internal/shortcut/devapp/devapp_rollout_test.go:13` |
| 高危写门控 | 3 个 high-risk-write shortcut 已在 skill 表中标注风险级 | `devapp.md` §Shortcuts |
| 版本结果真实性 | create 要求稳定 versionId；publish 区分 pending、明确终态和 projection_unknown，且成功仍要求回读 | `helpers/devapp_version_result.go`；`agent-scans/devapp-version-lifecycle-contract-20260810.md` |

devapp 当前可见 shortcut 共 20 个：12 read、5 write、3 high-risk-write。

### 2.2 主要缺口

| 缺口 | 说明 | 对应层 |
|---|---|---|
| G1 分页回归面薄 | chat 有硬页上限（flagListHardPageLimit=500）、去重、stop-reason、游标停滞保护和 ~2000 行分页回归；devapp 只有 2 个 rollout 测试 | 执行层 |
| G2 分页 subtype 未稳定 | `pagination_conflict/incomplete/invalid` 是自由 `WithReason` 字符串，不在 RFC-0003 稳定 registry（当前仅 6 个审定 subtype），Agent 不能安全分支 | 错误层 |
| G3 classifier 无 devapp 模式 | PARAM_ERROR 只识别 IM 会话 ID 缺失；unifiedAppId/appKey/agentId 误用、版本审批未过等 devapp 高频错误落到 unclassified | 错误层 |
| G4 写终态 live audit 不完整 | DevApp 列表/成员读取已有真实只读证据；创建、启停、删除、成员写和版本写目前只有零写/受控 caller。版本写已能 fail-closed 表达未知 ACK 与审批 pending，但仍缺真实发布/拒绝/撤回和 status/get 回读 | 真实性层 |
| G5 Lark 面差 33 个 | devapp↔apps：DWS 30 vs Lark 63，同名仅 3 个，且均为妙搭 vs 开放平台的产品域错位（release-create/get/list） | 覆盖层（负面边界，见 §6） |
| G6 黄金路线无文档 | chat 有三篇设计文档 + `chat.json` 路由目录；devapp 没有等价物，路由知识只散落在 devapp.md 概念地图 | 路由层 |

## 3. Phase 1：执行/分页加固（栩朝层）

目标：devapp 的分页命令在异常输入下有与 chat 同级的保护，且保护行为被回归测试锁住。

覆盖命令：`+list`、`+version-list`、`+member-list`、`+permission-list`、`+event-list`
（所有走 `devAppPaginationMeta` 的分页面）。

交付：

1. **硬上限与停滞保护**：为 devapp 分页定义硬页上限（参照 chat 的
   `flagListHardPageLimit`/`chatListMaxWindowSize` 模式）和游标停滞检测
   （同 cursor 连续两页即停，输出 `pagination_conflict` 而非死循环）。
2. **去重与完整性证据**：投影列表保留稳定 ID 去重和 truncation/completeness 证据，
   与 `TestDevAppProjectedListsPreservePaginationEvidence` 的既有口径一致。
3. **回归测试扩面**：在 `internal/shortcut/devapp/` 增加分页回归
   （空页、重复 cursor、服务端 nextToken 缺失、超过硬上限截断），
   对齐 chat 分页测试的场景维度而非行数。

验收：

- `go test ./internal/shortcut/devapp/...` 覆盖上述 4 类异常场景且全绿；
- `+version-list --format json` 超页时输出显式截断证据，不静默丢页；
- 不引入自动重试：超限即停并给出 typed 错误（遵循 RFC-0003「CLI 不自动重放」原则）。

## 4. Phase 2：错误/路由闭环（Dennis 层）

目标：devapp 的高频失败有稳定、可分支的 subtype 和恢复提示；高危面有真实环境审计。

### 4.1 subtype 稳定化（走 RFC-0003 流程）

1. 将 `pagination_conflict/incomplete/invalid` 按 RFC-0003 §治理流程提名为稳定
   subtype（补 `SubtypeDescriptor`：恢复动作、retry policy、hint），
   或映射到已有的 `pagination_inconsistent`——二选一，由评审决定，不新增协议字段。
2. 迁移 `internal/helpers/devapp.go` 的自由 `WithReason` 调用点到
   `WithSubtype`（inventory 台账列出的 3 处：line 2000/2009/2016/2023）。

### 4.2 classifier 补 devapp 模式

`classifyServerFailure` 是服务无关的，只缺 devapp 词表。新增模式（每条需真实
badcase 支撑，参照 chat 的 `reviewed: true + review_reason` 机制）：

| 模式 | 触发特征（待定 badcase） | 分类 |
|---|---|---|
| 应用主键误用 | 用 agentId/appKey 做写操作定位，服务端报参数错误 | `invalid_request` + 指向 ID 体系表 |
| 审批未过 | 版本发布前置检查失败 / APPROVAL_REQUIRED | `precondition_unmet` + 指向 `+version-check-approval` |
| 版本未生效 | 机器人/权限「没生效」类错误的版本状态根因 | hint 指向 `+version-status` |

### 4.3 live audit 脚本

新增 `scripts/run_devapp_shortcut_live_audit.py`（骨架复制
`run_chat_shortcut_live_audit.py`）：

- 第一阶段只覆盖 12 个 read shortcut（`+list`→`+get` 的自然路由链）；
- 写面用 `scripts/run_chat_shortcut_write_dry_run_audit.py` 的 dry-run 门控模式，
  3 个高危写仅做 `--dry-run` 审计，不做真实写；
- 审计输出落 `docs/agent-scans/` 台账，与 error inventory 同格式。

验收：

- 稳定 subtype 进 registry 后，`scripts/agent/scan_error_contract.py` 复扫
  devapp 调用点无自由字符串残留；
- live audit 在真实账号上跑通 read 面并产出证据文件（没有真实账号时该步阻塞，
  不得用 mock 冒充，遵循 inventory 结论 §4）。

## 5. Phase 3：skill/语义对齐（瑞达层）

目标：Agent 在 devapp 上「选得对」，且 skill 文档与运行时同源。

1. **目标标识符消歧**：devapp.md 的 ID 体系表已有文档口径
   （写操作只用 `--unified-app-id`；agentId 只读返回；appKey=clientId），
   但 `semantic_catalog_devapp.json` 尚无对应的形态/消歧规则。参照克谨
   `param_concepts.json` 的词形机制，为四种 ID 增加消歧条目，
   使参数误用在 PreParse 阶段即被纠正，而不是等服务端报错。
2. **生成物同源验证**：跑 `gen_skill_shortcut_sections.py` 确认 `devapp.md`
   无 drift；把该检查并入现有 policy（`make policy` 已有 unified-result 侧的
   devapp 探针，见 `scripts/policy/unified-result-lib.sh:86`）。
3. **黄金路线小节**：把概念地图里的三条主链（建应用→配能力→发版本；
   机器人建联调试；权限/成员维护）固化为 devapp.md 的「典型任务」路由，
   等价于 chat 的 golden route 文档，但保留在 misc 包内、不拆独立 skill
   （devapp 体量不支持独立包，event 并入 misc 的先例同理）。

验收：

- 消歧规则有单测（错误 ID 类型在 PreParse 被拒并给出正确指引）；
- `make policy` 增加 devapp skill drift 检查项且通过。

## 6. 负面边界（不做的事）

- **不移植 lark `release-*` 三个 shortcut**：妙搭低代码发布与开放平台版本发布
  是不同产品域，`shortcut-lark-alignment.md` 已定性为「语义与产物不同」；
  如需覆盖妙搭，另立产品调研，不在本方案内伪造等价物。
- **不追求 Lark 63 个 shortcut 的数量对齐**：覆盖差（G5）优先用「typed 负面
  能力矩阵」表达（明确告知 Agent 该能力不存在及替代路径），而不是补齐数字。
- **不自动重试写操作**：RFC-0003 明确 CLI 不因 registry 自动重放；
  高危写的恢复提示只指向人工动作。
- **不拆独立 `dingtalk-devapp` skill 包**：当前 20 个 shortcut 的体量留在 misc
  更经济；拆包的触发条件（shortcut 数、独立生命周期需求）未达到。

## 7. 实施顺序与依赖

```text
Phase 1（分页加固，纯本地回归） ─┐
                                 ├→ Phase 2.1（subtype 稳定化，依赖 Phase 1 的错误语义定型）
Phase 2.2（classifier 词表）─────┘         ↓
Phase 3.1（消歧规则，独立）         Phase 2.3（live audit，依赖真实账号）
                                          ↓
                                 Phase 3.2/3.3（生成物同源 + 黄金路线收口）
```

Phase 1 与 Phase 3.1 无依赖，可并行；Phase 2.3 是唯一需要真实环境的阻塞点。

## 8. 台账登记

本方案的逐项交付应登记到
[`evaluation-remediation-202608.md`](evaluation-remediation-202608.md)
的 remediation ledger，与 error subtype governance（RFC-0003）共用审阅节奏：
每个 Phase 结束时更新本文档顶部「状态」行，并附 commit 范围。
