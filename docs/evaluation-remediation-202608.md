# 2026-08 CLI 评测问题整改台账

## 口径

本台账对应以下两份外部评测快照：

- `CLI评测-v3完整评测报告-20260728晚.md`
- `CLI评测-v3完整评测报告-20260806.md`

状态只按当前代码和可复现证据认定：

- **已关闭**：代码、CLI 行为和覆盖范围一致，且有针对性测试或本地安全实跑证据。
- **代码已修，待真实环境复验**：本地契约已收口，但服务端终态、真实账号权限或异步投递仍需真实环境证明。
- **未关闭**：仍能复现，或当前证据不足以证明报告中的完整问题已经消失。

单元测试通过不等同于服务端问题关闭；Schema 可发现也不等同于真实调用成功。

## 当前结论

| 报告问题 | 当前状态 | 当前证据 | 剩余动作 |
|---|---|---|---|
| attendance 影子 shortcut：15 条不可发现、9 条写命令无门禁 | **已关闭（当前工作树）** | attendance 包内 33/33 shortcut 均公开并带 Contract；加 2 条 smart shortcut 后 `shortcut list --service attendance` 为 35。9/9 写命令均为 `confirmation:user_required`；`+boss-check` 非交互缺 `--yes` 返回 rc=3；`--dry-run` 只返回 `executed:false` | 合入独立提交；发布后二进制复验 |
| 全域影子 shortcut 扫描 | **未关闭** | attendance 已完成，但真实测试跟进清单仍有 156 条；其中包含真实后端报错，也包含尚未完成稳定接口/安全审阅的命令，不能一键公开。公开状态与真实调用结果是两个维度，不能因完成 Contract 审阅而删除失败证据 | 按产品逐条 Agent 审阅；写命令先审 Safety/dry-run，再决定公开或删除 |
| mail Schema 仅 41/68，缺不可逆命令 | **代码已修** | 当前 `schema mail` 返回 73 个工具，包含 `mail.recall_sent_message` 与 `mail.trash_mailbox_thread`；全局 Schema/Help/运行时门禁同源检查通过 | 以当前版本重新做“可执行叶子 vs Schema”全量对拍，避免沿用旧 68 基线 |
| 12 条读命令 12 种 JSON 信封 | **框架已接入，渐进迁移中** | Framework 2.0 已提供统一 `ok/outcome/data/error/meta`、强类型布尔、单 writer、partial/pending；不公开协议选择参数，不输出版本标记 | legacy 命令按 terminal command 继续迁移；不得把迁移责任交给 Agent |
| mail `success` 字段 string/bool 分裂 | **已关闭（JSON 输出路径）** | Mail JSON 输出会递归把键名严格等于 `success` 的 `"true"`/`"false"` 归一为 Go bool；不转换其他键或普通文本。端到端 helper 输出测试覆盖顶层、嵌套和数组 | 发布后二进制抽取报告同组 14 个样本，确认后端新增响应形状仍经过该 JSON 出口 |
| mail `--size` 等隐藏别名在 Skill 中被当作公开契约 | **已关闭（当前工作树）** | Help/Schema/Skill 按命令统一：消息列表公开 `--folder-id`，文件夹列表公开 `--folder`，相关命令公开 `--limit`/`--content`/`--from`；`--size`/`--page-size`/`--body`/`--sender` 及反向 folder 别名仅作隐藏兼容。mono/multi 脚本对子进程传 canonical flag，且可解包 legacy/v2 结果容器 | 发布前用 Agent 语义扫描继续检查其他 Mail 兼容 flag，不将隐参数重新暴露给 Agent |
| `retryable` 与实际重试反向、超时盲重放 | **框架已修，待产品回归** | transport 对执行状态不明确的写调用不再自动重放；错误恢复元数据和 context deadline 已进入统一路径 | 对幂等读、限流、写后超时分别做真实链路复验 |
| `wiki +node-list` 挂起 | **代码已修，待真实环境复验** | 已发布 canonical `wiki.shortcut_node_list` 及 read/low/not_required/idempotent Safety；无网络测试证明只调用一次 `doc/list_nodes`。未知响应形状不再伪装为空列表，分页矛盾会 typed failure，无分页证据时 `paginationKnown:false` | 发布后二进制执行真实根目录、空目录、分页三组读取，确认服务端不再挂起且字段形状与投影一致 |
| `drive list --pattern` 失效 | **代码已修** | `drive list` 已公开 `--pattern`；测试覆盖 pattern 投影、JSON 可解析以及与 `--versions` 的冲突 | 发布后二进制用真实目录做 3 组正反例复验 |
| `drive download --format json` 惰性、stdout 日志污染 | **代码已修** | 下载成功与 dry-run JSON 路径均有测试；Framework writer 统一 stdout/stderr 与 format 分发 | 发布后二进制执行成功/失败两条 `jq` 管道复验 |
| `drive delete --dry-run` 与确认门禁耦合、EOF 取消仍 rc=0 | **已关闭（当前工作树）** | `drive delete` 声明 destructive/high/user_required；专项测试证明 `--dry-run` 产生请求预览且写调用为 0，关闭 stdin 且无 `--yes` 时返回 typed `confirmation_required`、写调用仍为 0 | 发布后二进制复验 dry-run/EOF/`--yes` 三条路径 |
| `doc version revert --dry-run` 对不存在版本也放行 | **代码已修** | 针对版本 `999` 的预校验回归测试已存在；dry-run 不触发写请求 | 真实文档分别验证存在/不存在版本号 |
| `todo +add-participant` 报错但已写入 | **代码已修，待真实环境复验** | 参加人写入增加幂等/结果核验路径及专项测试 | 真实任务执行“成功、服务端报错但已写、明确失败”三态复验 |
| IM 零结果却扩大成 `complete:true` | **CLI 语义已修，索引健康未解决** | 输出改为 `endpointExhausted` 与 `indexCoverageKnown:false`，不再把分页耗尽解释为业务全量完整 | 服务端需要提供索引覆盖/健康证据，CLI 不能自动推断 |
| AITable/Base 目录死条目与假阴 | **CLI 已 fail-closed，服务端未关闭** | 列表/搜索结果显式标记发现来源、分页已知性和索引覆盖未知，不再承诺权威全量目录 | 后端治理死条目；真实账号复测精度和召回率 |
| event 停机契约 | **安全入口已修，真实停机待复验** | `event stop` 为 destructive/high/user_required；无 `--yes` 拦截，`--dry-run` 不停订阅 | 真实订阅执行 stop 后验证进程、订阅和本地状态三者终态一致 |
| contact 投影层 `8/5/1` 数据丢失 | **代码已修，待真实环境复验** | `+list-roles` 已拍平分组 `labels[]`；角色/部门成员识别 `labelUserList`/`deptUserList` 并解包 `userInfo`。回归测试锁定报告中的 8/5/1 下层计数；任一非空条目无法投影时整体 fail-closed，不再返回成功空/残缺列表 | 用原评测账号复跑三条命令并对拍 lower/upper 数量及稳定 ID |
| Skill/Help/Schema 指令偏移 | **本轮已关闭已知项，持续审阅** | Agent 对拍发现部门查询隐藏别名、AITable 写示例缺身份参数、考勤/听记/会议室脚本隐藏 flag、mono devdoc 类型漂移；已同步修正代码声明、Help、Schema examples、mono/multi Skill 和脚本 | 发布前继续按产品执行 Agent 语义扫描；CI 路径检查不能代替 flag/结果/安全审阅 |
| public leaf 依赖 Schema exclusion | **渐进收敛中** | `contact label list/get/list-members` 已补齐 Identity、MCP binding、参数、read/low/idempotent Safety 与 Agent 选型说明，并从 reviewed exclusion 移除；运行时 Schema 工具数由 1000 增至 1003。当前仍有 35 条 CLI 生命周期控制、16 条未开放 Agoal、45 条待审兼容 helper | 只按 terminal command 在完成接口、参数和安全审阅后移除；CLI 控制面和明确未开放产品不为追求数字清零而伪装成 Agent 工具 |
| sheet 二次回滚 bricking | **未关闭** | 破坏性问题不能以普通单测或无账号 dry-run 证明消失 | 隔离表格、备份和服务端协同下做官方自证；失败时保留可恢复证据 |
| approval 真实提单 | **未关闭** | Schema 与三件套能力存在，但原报告缺真实创建成功证据 | 使用获授权测试审批模板完成一次真实创建并清理测试实例 |
| `dws api` 默认 MCP 登录态不可用 | **未关闭（需要后端能力）** | 默认登录保存的是只能由 MCP 网关解密/代理的 token，不是可直接发给 `api.dingtalk.com` 的 access token；CLI 当前正确 fail-closed 并返回 typed auth 错误 | 后端提供受限 raw-API proxy/capability 才能让默认身份使用；在此之前仍要求自有 AppKey/AppSecret，禁止 CLI 伪造或转发不适用的密文 token |

## attendance 本轮验收证据

```text
shortcut count:      35
hidden:               0
write shortcuts:      9
ungated writes:       0
boss-check no --yes:  rc=3 confirmation_required
boss-check dry-run:   rc=0, executed=false
Schema canonical:     attendance.shortcut_boss_check
```

新增的域级门禁会同时校验：

1. 33 条 attendance 包内 shortcut 无 `Hidden`。
2. 33 条全部存在于公开目录。
3. 每条都有完整 Agent Contract。
4. 读命令必须是 read/not_required。
5. 写命令必须要求 user_required。

## Wiki node-list 本轮验收证据

```text
Schema canonical:            wiki.shortcut_node_list
Safety:                      read/low/not_required/idempotent
backend route:               doc/list_nodes
backend calls per execution: 1
unknown projection:          typed projection_unknown
pagination contradiction:    typed pagination_inconsistent
pagination without evidence: paginationKnown=false
```

这次只关闭 CLI 路由、可发现性和输出真实性问题，不以单元测试替代真实
Wiki 服务端读取。真实账号复验仍保留在发布验收中。

## Contact 投影层本轮验收证据

```text
contact +list-roles:        lower=8, projected=8
contact +list-role-members: lower=5, projected=5
contact +list-dept-members: lower=1, projected=1
unknown non-empty row:      projection_unknown（拒绝成功空/部分列表）
```

这里的“投影层”是把 MCP/API 原始响应转换成 Agent 直接消费的精简字段。
本轮不仅补齐报告命中的真实容器和 `userInfo` 包装，还要求投影后的行数与
下层行数一致；容器名已知但字段形状未知同样不能冒充成功。

## 下一批优先级

1. 对剩余 shortcut 跟进项按产品做 Agent 审阅，优先处理写命令和报告点名命令。
2. 真实环境复验 contact 投影 `8/5/1`、`wiki +node-list`、event stop、todo participant、approval create-instance。
3. 在隔离数据上完成 sheet 二次回滚官方自证。
