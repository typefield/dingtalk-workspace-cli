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
| 12 条读命令 12 种 JSON 信封、字符串布尔值 | **框架已接入，渐进迁移中** | Framework 2.0 已提供统一 `ok/outcome/data/error/meta`、强类型布尔、单 writer、partial/pending；不公开协议选择参数，不输出版本标记 | legacy 命令按 terminal command 继续迁移；不得把迁移责任交给 Agent |
| `retryable` 与实际重试反向、超时盲重放 | **框架已修，待产品回归** | transport 对执行状态不明确的写调用不再自动重放；错误恢复元数据和 context deadline 已进入统一路径 | 对幂等读、限流、写后超时分别做真实链路复验 |
| `wiki +node-list` 挂起 | **未完全关闭** | 路由修复已有单测，但当前 `schema --cli-path 'wiki +node-list'` 仍返回 unknown；命令仍不在稳定 Agent surface | 完成 Contract/Safety 审阅并加入公开目录，随后做无网络路由测试和真实读取复验 |
| `drive list --pattern` 失效 | **代码已修** | `drive list` 已公开 `--pattern`；测试覆盖 pattern 投影、JSON 可解析以及与 `--versions` 的冲突 | 发布后二进制用真实目录做 3 组正反例复验 |
| `drive download --format json` 惰性、stdout 日志污染 | **代码已修** | 下载成功与 dry-run JSON 路径均有测试；Framework writer 统一 stdout/stderr 与 format 分发 | 发布后二进制执行成功/失败两条 `jq` 管道复验 |
| `doc version revert --dry-run` 对不存在版本也放行 | **代码已修** | 针对版本 `999` 的预校验回归测试已存在；dry-run 不触发写请求 | 真实文档分别验证存在/不存在版本号 |
| `todo +add-participant` 报错但已写入 | **代码已修，待真实环境复验** | 参加人写入增加幂等/结果核验路径及专项测试 | 真实任务执行“成功、服务端报错但已写、明确失败”三态复验 |
| IM 零结果却扩大成 `complete:true` | **CLI 语义已修，索引健康未解决** | 输出改为 `endpointExhausted` 与 `indexCoverageKnown:false`，不再把分页耗尽解释为业务全量完整 | 服务端需要提供索引覆盖/健康证据，CLI 不能自动推断 |
| AITable/Base 目录死条目与假阴 | **CLI 已 fail-closed，服务端未关闭** | 列表/搜索结果显式标记发现来源、分页已知性和索引覆盖未知，不再承诺权威全量目录 | 后端治理死条目；真实账号复测精度和召回率 |
| event 停机契约 | **安全入口已修，真实停机待复验** | `event stop` 为 destructive/high/user_required；无 `--yes` 拦截，`--dry-run` 不停订阅 | 真实订阅执行 stop 后验证进程、订阅和本地状态三者终态一致 |
| sheet 二次回滚 bricking | **未关闭** | 破坏性问题不能以普通单测或无账号 dry-run 证明消失 | 隔离表格、备份和服务端协同下做官方自证；失败时保留可恢复证据 |
| approval 真实提单 | **未关闭** | Schema 与三件套能力存在，但原报告缺真实创建成功证据 | 使用获授权测试审批模板完成一次真实创建并清理测试实例 |

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

## 下一批优先级

1. 完成 `wiki +node-list` 的公开 Contract 和无挂起验证。
2. 对剩余 shortcut 跟进项按产品做 Agent 审阅，优先处理写命令和报告点名命令。
3. 真实环境复验 event stop、todo participant、approval create-instance。
4. 在隔离数据上完成 sheet 二次回滚官方自证。
