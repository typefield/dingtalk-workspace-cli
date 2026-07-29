# DWS 框架层面优化点

> 基于代码库深度审计（2026-07-29），对比 lark-cli 与现代 CLI 框架实践。

---

## 1. 双框架分裂：LeafSpec vs Shortcut

**现状**：两套并行的命令构建体系。

| | LeafSpec (`internal/helpers/leaf.go`) | Shortcut (`internal/shortcut/`) |
|---|---|---|
| 命令形态 | `dws <product> <cmd>` | `dws <product> +<cmd>` |
| 约束声明 | 无（靠 `Validate` hook） | 声明式 `at_least_one` / `exactly_one` / `mutually_exclusive` |
| Flag 类型 | string / int | string / bool / int / string-slice |
| 多步编排 | 不支持 | `CallMCPData` / `CallMCPWriteData` |
| 迁移状态 | 29 处调用（主要 devapp） | 366 个 shortcut |

**问题**：
- 两套框架各自演进，约束系统不统一
- LeafSpec 的 flag 回退链（主 flag → 别名 → env → default）是 shortcut 没有的
- Shortcut 的声明式约束是 LeafSpec 没有的
- 47 个 helpers 文件仍为手写 `cobra.Command{}`，boilerplate 重复

**建议**：
1. 把 shortcut 的 `Constraint` 系统下沉到 LeafSpec，支持 `RequireOneOf` / `MutuallyExclusive` 声明
2. 给 LeafSpec 补 `LeafBool` / `LeafStringSlice` / `LeafDuration` kind
3. 统一 flag 注册 + 约束校验基础设施，区别只在 dispatch 路径（单步 MCP vs 多步编排）
4. 逐步将 47 个手写 helpers 文件迁移到 LeafSpec

**预期收益**：减少 ~40% 手写 boilerplate，约束声明统一，新命令开发时间从 ~2h 降到 ~30min。

---

## 2. Transport 层缺 Circuit Breaker 和 Streaming

**现状**（`internal/transport/client.go`）：
- 有连接池（100 idle / 10 per-host）、HTTP/2、指数退避重试（1 次，500ms base）
- 无 circuit breaker — 某个 MCP server 持续 5xx 时，每次调用仍会尝试
- 无 streaming — 全量 buffer 响应体后才格式化输出
- `ndjson` format 只是格式化选项，底层仍是全量读取

**对比 lark-cli**：lark 的 transport 同样没有 circuit breaker，但 `+meeting-events` 等命令用了 NDJSON 逐行输出。

**建议**：

### Circuit Breaker（~100 行）
```
按 endpoint 维度跟踪连续失败次数
超过阈值（如 5 次 / 30s 窗口）后短路 10s
半开状态放行 1 个请求探测恢复
加在 doWithRetry 外层
```

### Streaming（~200 行）
```
对 tools/call 响应支持 Transfer-Encoding: chunked 逐行解码
配合 ndjson format 实现真正的流式输出
优先场景：minutes 转写、event bus、长列表分页
```

**预期收益**：避免单 server 故障雪崩；长响应场景首字节时间从 ~5s 降到 <500ms。

---

## 3. Schema 生成是全量重建，非增量

**现状**（`internal/generator/`）：
- 6 个输入 → 全量 resolve → 全量写 shard
- CI 每次 PR 都跑完整生成 + drift check（~2 分钟）
- 改一个产品的 hint 文件也要重建全部 26 个 shard
- `outputguard.Validate()` 的 byte guard 是全局的

**建议**：
1. Shard 已按产品拆分（`tools/<product>.json`），生成器只重建变更产品对应的 shard
2. `outputguard.Validate()` 的 byte guard 缩小到变更 shard 级别
3. CI 中 `check-generated-drift.sh` 只 diff 变更 shard，未变更 shard 跳过

**预期收益**：单产品变更时 CI 生成时间从 ~2 分钟降到 ~15 秒。

---

## 4. Plugin 系统缺沙箱和热加载

**现状**（`internal/plugin/`）：
- stdio plugin 以用户权限直接运行，无隔离
- 启动时一次性加载，无 hot-reload
- 无 plugin 间依赖管理
- build 步骤执行任意 shell 命令（信任模型 = 用户显式安装）
- 有基础安全：symlink 跳过、path traversal guard、dangerousEnvVars blocklist、git install 拒绝 file://

**分阶段建议**：

| 阶段 | 措施 | 工作量 |
|---|---|---|
| 短期 | stdio plugin 加 `seccomp` profile（Linux）或 `sandbox-exec`（macOS），限制网络/文件访问范围到 plugin 目录 | ~200 行 |
| 中期 | 支持 `DWS_PLUGIN_WATCH=1` 热加载（fsnotify 监听 plugin 目录），开发时不用重启 CLI | ~150 行 |
| 长期 | plugin 间依赖声明（`"requires": ["other-plugin@^1.0"]`）+ 拓扑排序加载 | ~300 行 |

---

## 5. Safety 元数据是静态的，无运行时升级

**现状**：
- `CommandSafety`（effect / risk / confirmation / idempotency）在 catalog 生成时固定
- 确认是二元的（yes/no），无 typed confirmation
- 无基于参数的运行时风险升级（如 `--force` 删除 1000 条记录 vs 1 条）
- Content scanner 是 regex 匹配，无 ML/heuristic 检测

**建议**：
1. 在 `LeafSpec.Validate` / shortcut `Validate` 阶段，根据实际参数值动态调整 risk level
   - 例：`record delete --count=500` 升级为 high + 要求 typed confirmation
2. 对 destructive 命令支持 `--confirm-text=DELETE` typed 确认
3. Content scanner 增加基于 entropy 的检测（高熵字符串可能是注入 payload）

**预期收益**：AI agent 场景下减少误操作风险；用户信任度提升。

---

## 6. Error Model 的 Actions 是建议性的，不可执行

**现状**（`internal/errors/errors.go`）：
- `Error.Actions` 是 `[]string`（如 `"run 'dws auth login'"`），纯文本建议
- Agent 需要自己解析 hint 文本来决定下一步
- 有 `ServerDiagnostics`（trace_id, server_error_code, friendly_hint, action_url）但仅展示

**建议**：把 `Actions` 升级为结构化的 `RecoveryAction`：

```go
type RecoveryAction struct {
    Type    string            // "retry" | "reauth" | "rerun_with" | "docs"
    Command string            // 可执行的 CLI 命令
    Params  map[string]string // 修正后的参数
    DocURL  string            // 相关文档链接
}
```

- `retry`：自动重试（配合 circuit breaker 的半开状态）
- `reauth`：触发 `dws auth login` 流程
- `rerun_with`：用修正后的参数重新执行（如缺少 required flag 时自动补全）
- `docs`：打开相关文档

**预期收益**：AI agent 可直接执行恢复动作，错误恢复时间从 ~30s（解析 + 决策 + 执行）降到 <5s。

---

## 7. 无统一分页抽象

**现状**：每个命令各自处理 cursor/offset 翻页。LeafSpec 和 Shortcut 都没有分页层——每个 smart shortcut 手写 while 循环。lark-cli 有统一的 `--page-all` / `--limit` / `--auto-paginate` 层，框架自动循环直到耗尽或达到 limit。

**建议**：在 transport 或 output 层加 `PaginatedCall(ctx, tool, args, pageSize) → chan Result`，命令层只需声明 `Paginate: true`。全局 flag `--page-all` / `--limit N` 由框架统一消费。

**预期收益**：消除所有 list 类命令中的重复翻页循环代码（当前至少 15 处手写 while）。

---

## 8. 无通用批量/分片执行器

**现状**：`+record-share-links` 手写了 >20 去重 + 分片 + fanout + 合并；`+replace-batch` 手写了逐组聚合。42 个 gap-buildable 中至少 8 个需要分片编排。每个实现都是独立的 ad-hoc 代码。

**建议**：框架级 `BatchExecutor`：

```go
type BatchExecutor struct {
    ChunkSize      int
    Dedupe         bool
    Concurrency    int
    MergeFunc      func(results []any) any
    OnPartialFail  func(chunk int, err error) // 容错策略
}
```

Shortcut 只声明分片策略，框架负责 chunk → fan-out → merge → partial-failure 报告。

**预期收益**：新批量 shortcut 开发从 ~3h 降到 ~30min；行为一致性有框架保证。

---

## 9. 无响应缓存

**现状**：contact 解析、base 元数据、schema 查询等高频读操作每次都走网络。AI agent 同一会话中可能重复解析同一个 contact 5-10 次。lark-cli 对 contact 有 TTL 缓存（默认 5min）。

**建议**：transport 层加 LRU + TTL 缓存（按 tool+args hash），`CacheTTL` 在 LeafSpec/Shortcut 声明。写操作自动 invalidate 相关 key。`--no-cache` 全局 flag 绕过。

**预期收益**：AI agent 场景下减少 60-80% 重复网络调用；交互响应时间显著降低。

---

## 10. 无命令组合 / 结构化管道

**现状**：`dws contact search --name X` 输出 JSON，但 `dws chat send --to` 不能直接消费上一个命令的 stdout。用户必须手动 `--jq` 提取再传参。

**建议**：
- 支持 `--to -`（从 stdin 读 JSON，按 bind key 自动映射）
- 或 `dws pipe "contact search --name X" "chat send --to $.userId"` 子命令
- 输出支持 `--format=env`（`KEY=VALUE` 格式，方便 `eval $(dws ...)`）

**预期收益**：Power user 和 shell 脚本场景效率提升；减少中间变量和 jq 依赖。

---

## 11. 无 dry-run diff 预览

**现状**：`--dry-run` 只打印 tool name + args。对 write 操作，用户看不到"执行后世界会变成什么样"。

**建议**：对支持 `get` 的资源，dry-run 先 get 当前状态，再展示 before/after diff（类似 terraform plan）。在 LeafSpec/Shortcut 声明 `DryRunGetTool: "get_record"` 即可启用。

**预期收益**：write 操作信心提升；AI agent 可在执行前验证预期结果。

---

## 12. 无多组织 / 多环境 profile

**现状**：auth 是全局单例（keychain 里一个 token）。企业用户经常在多个组织间切换，当前必须 `dws auth logout` + `dws auth login` 来回切。

**建议**：
- `dws profile create/use/list/delete` 子命令
- 每个 profile 独立 keychain entry + config（`~/.dws/profiles/<name>/`）
- `DWS_PROFILE=org-b dws ...` 环境变量切换
- `dws auth login --profile org-b` 登录到指定 profile

**预期收益**：企业多组织用户刚需；ISV 开发/测试/生产环境隔离。

---

## 13. 无动态补全

**现状**：Cobra 静态补全已有（`dws completion bash/zsh/fish`），但无动态补全。补全 baseId、chatId 等高频 ID 参数时，用户只能手动复制粘贴。lark-cli 对高频 ID 参数有 shell completion hook。

**建议**：注册 `ValidArgsFunction`，对声明了 `Completable: true` 的 flag，运行时查询最近使用记录（audit log）或 API 候选列表。缓存 5min 避免补全延迟。

**预期收益**：DX 锦上添花；减少 ID 输入错误。

---

## 14. 无 i18n 框架

**现状**：所有 usage / hint / error 文案硬编码中文。开源社区贡献者无法阅读。`--help` 输出、error hint、safety annotation 全部是中文。

**建议**：
- `internal/i18n` 包 + `go:embed` 语言文件（`locales/zh.json`, `locales/en.json`）
- 默认中文、英文 fallback
- `DWS_LANG=en` 或 `--lang en` 切换
- 新命令的 usage 文案通过 key 引用，不直接写字符串

**预期收益**：开源社区国际化；为 lark-cli 英文用户迁移降低门槛。

---

## 优先级总览

| 优先级 | 优化点 | 影响面 | 工作量 |
|---|---|---|---|
| **P0** | #1 双框架统一 | 所有新命令开发 | 大（渐进迁移） |
| **P0** | #2 Circuit Breaker + Streaming | 所有 MCP 调用 | 中（~300 行） |
| **P0** | #7 统一分页抽象 | 所有 list 类命令 | 中（~200 行） |
| **P0** | #8 批量/分片执行器 | 42 个 gap-buildable 中 8+ 个 | 中（~250 行） |
| **P1** | #3 增量 Schema 生成 | CI 效率 | 小（~100 行） |
| **P1** | #6 可执行 RecoveryAction | AI agent 体验 | 小（~150 行） |
| **P1** | #9 响应缓存 | AI agent 重复调用 | 中（~200 行） |
| **P1** | #12 多组织 profile | 企业用户刚需 | 中（~300 行） |
| **P2** | #4 Plugin 沙箱 + 热加载 | 安全性 + DX | 中（分阶段） |
| **P2** | #5 运行时 Safety 升级 | 安全性 | 中（~200 行） |
| **P2** | #10 命令管道 | Power user 效率 | 中（~200 行） |
| **P2** | #11 dry-run diff | write 操作信心 | 小（~150 行） |
| **P3** | #13 动态补全 | DX 锦上添花 | 小（~100 行） |
| **P3** | #14 i18n 框架 | 开源国际化 | 中（~300 行） |