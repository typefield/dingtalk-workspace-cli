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

## 优先级总览

| 优先级 | 优化点 | 影响面 | 工作量 |
|---|---|---|---|
| **P0** | #1 双框架统一 | 所有新命令开发 | 大（渐进迁移） |
| **P0** | #2 Circuit Breaker + Streaming | 所有 MCP 调用 | 中（~300 行） |
| **P1** | #3 增量 Schema 生成 | CI 效率 | 小（~100 行） |
| **P1** | #6 可执行 RecoveryAction | AI agent 体验 | 小（~150 行） |
| **P2** | #4 Plugin 沙箱 + 热加载 | 安全性 + DX | 中（分阶段） |
| **P2** | #5 运行时 Safety 升级 | 安全性 | 中（~200 行） |