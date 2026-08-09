# RFC 0002：Mono Skill 脚本统一接口

状态：已实施（Help/输出层）；dry-run 副作用验证进行中
范围：`skills/mono/scripts/*.py` 中被 Skill 正向引用的可执行脚本
非目标：修改 `dws` 主命令契约、引入公开协议选择参数、保存生成的 JSON 文件

## 背景

Mono Skill 目前把脚本当作可执行 Agent 入口，但脚本自身没有统一运行时接口。
当前工作树的 Agent 扫描结果（2026-08-09；口径必须区分文件、入口和 Help 可观测能力）。这些数字是扫描快照，后续以 Agent 重跑结果为准：

| 口径 | 数量 | 定义 |
|------|------|------|
| Python 文件 | 35 | `skills/mono/scripts/*.py` 全部文件，包含内部模块 |
| Agent 入口 | 32 | AST 中包含 `if __name__ == "__main__"` 的脚本 |
| 内部模块 | 3 | `_runtime.py`、`attendance_report_common.py`、`minutes_list_parse.py`，不应被当作 CLI 入口 |
| Help 暴露 `--dry-run` | 32 | 对 32 个入口逐个运行 `python <script> --help` 后扫描实际输出 |
| Help 暴露 `--format` | 32 | 同上；类型为 `text|json|ndjson` |
| Help 非零 | 0 | 当前版本在缺少业务参数和可选运行依赖时均可完成能力发现 |

作为历史基线，迁移前曾有 19 个入口暴露 `--dry-run`、仅 1 个入口暴露脚本级
`--format`，且存在 Help 非零脚本；这些数字不能继续作为当前状态。当前 32 个入口的
具体清单与逐项 Help 结果由 [Agent 扫描报告](../agent-scans/mono-script-contract-20260809.md)
维护，避免在 RFC 正文复制一份会漂移的名单。扫描必须记录四层口径，不能用文件数代替
入口数，也不能用源码是否调用 `add_contract_flags` 代替实际 Help 结果。

因此当前可以准确宣称“32 个 Agent 入口在 Help 层声明了
`--dry-run/--format`”，但不能把这句话扩大为“32 个入口都已完成零副作用证明”。
Help 可观测性、JSON 流向和 dry-run 副作用是三个独立验收维度，长期目标是让 Agent
分别依赖每个维度的实测结果。

### 为什么不能按文件名一刀切

“全部支持”只适用于面向 Agent 的可执行入口，不适用于目录下所有 Python 文件，且不能
只添加两个 argparse 参数就宣称完成：

1. `attendance_report_common.py`、`minutes_list_parse.py` 是被其他脚本 import 的内部模块，
   没有独立业务生命周期，不应成为可选 CLI 入口。
2. `--dry-run` 必须证明零远端写入、零本地写入；报表类脚本还要明确“允许远端只读后在内存
   中生成预览”还是“完全跳过远端读取”。没有这一定义，参数只是安全假象。
3. `--format` 必须控制全部 stdout，包括进度、预览、异常和图片/Excel 处理结果；复杂脚本
   若仍有直接 `print()`，加参数反而会制造不可解析 JSON。
4. 分页、图片下载、认证探测和异步写入等脚本需要先定义 `pending`、失败、未知状态和
   资源副作用，不能沿用简单的“成功/失败”包装。

因此迁移单位是“一个 Agent 入口 + 一套可验证生命周期”，不是“所有 `.py` 文件”。
最终目标仍是覆盖所有真正的 Agent 入口；当前未迁移项必须在 Skill 中列为未支持，而不是
用全局承诺掩盖差异。

## 目标接口

所有迁移完成的脚本必须接受：

```text
--format text|json|ndjson
--dry-run
```

不增加 `--output-contract` 或 `contract_version`。Agent 只通过 `--format` 选择
表现形式，迁移状态由脚本发布版本决定。

会发起远端写入的复合脚本还必须接受 `--yes`。它不是全局脚本 flag：只读、查询和
本地导出入口不应为了形式统一而伪造确认门禁。对写入口，`--dry-run` 可免确认地生成
计划；缺少 `--yes` 的非 dry-run 调用必须在任何 child CLI、远端写入或本地输出写入前
返回 `policy/confirmation_required` 与 `execution_state=not_executed`。

### format

- `json`：一次执行输出一个可解析的统一结果对象；业务结果放在 `data`，失败放在
  `error`，退出码与结果一致。
- `text`：面向用户的人读输出；日志必须走 stderr，stdout 只输出正文。
- `ndjson`：每条独立结果一行，仅用于流式或批量逐项输出。

脚本内部调用 `dws` 时必须使用同一格式；在脚本需要映射或聚合子结果时，必须保留有意义的
`ok/outcome/error`，并将可用的分页、异步或传输事实透传到外层 `meta`。不得为了满足字段
形状伪造 `meta`，更不能把缺少 `meta` 扩大解释为数据完整或终态已验证。脚本不得把
`success` 字符串重新拼成自己的第二套信封，也不得把日志混入 JSON stdout。

机器 JSON 的 `failure` 必须携带非空字符串 `error.type`；否则统一出口拒绝原始 stdout，改发
typed internal failure。若出现，`meta` 必须是对象、`dry_run` 必须是 boolean；类型错误同样
不能绕过统一出口。`partial_failure` 的顶层 `error` 可以省略，但其业务数据必须按对应脚本的
三通道约定保留逐项事实。

### dry-run

`--dry-run` 的最低保证是：

1. 不创建、修改或删除本地文件；
2. 不触发远端写请求、不创建任务、不发送消息；
3. 输出完整执行计划，包括目标、参数、预计步骤和可能的删除/覆盖范围；
4. 返回成功时明确标记 `dry_run: true`；不能把预览伪装成业务已完成。

只读脚本可以选择“请求预览”或“跳过远端读取”，但必须在 Help/Skill 中说明是否会发生
远端只读探测。`remote_reads` 不是 `_runtime.py` 的参数或公共 API。写脚本默认要求
零远端写入；无法预览的脚本不得标称支持
`--dry-run`。

## 运行时实现

共享 Python 模块（随 Skill 一起发布，不保存运行结果）当前的真实公开函数为：

```text
scripts/_runtime.py
  add_contract_flags(parser, *, dry_run=True)
  add_write_confirmation_flag(parser)
  require_write_confirmation(*, fmt, confirmed, dry_run, operation, data=None)
  emit(*, fmt, outcome, data=None, error=None, meta=None, dry_run=False, text=None, items=None)
  failure(fmt, message, *, details=None, meta=None)
  run_child_dws(args, *, dry_run=False, timeout=60) -> ChildDWSResult
  batch_data(*, succeeded, failed, unknown, total=None, **extra)
  batch_outcome(data)
  run_main(main_fn)
```

模块只负责参数、stdout/stderr、结果信封与退出码；每个入口以
`sys.exit(run_main(main))` 进入统一边界。`run_main` 在 JSON/NDJSON 模式将未捕获的
`Exception` 映射为 `failure + error.type=internal + exit 1`，并将非零 `SystemExit`
映射为 validation failure；它不输出 traceback，也不回显原始异常消息。正常 Help 的
`SystemExit(0)` 和 text 模式保留原命令行行为。它还验证缓存后的机器 stdout：JSON 必须
恰好是一条具有 `ok:boolean` 与 `outcome:string` 的对象，并与 0/1/7 退出码一致；NDJSON 的每个非空行必须是对象。
出现遗留 `print()`、多行或非法 JSON 时，运行时拒绝原 stdout，向 stderr 写无敏感诊断，
并改发单一 typed internal failure，而不是把污染后的“成功”交给 Agent。每个脚本仍负责业务参数校验、步骤编排、
子 `dws` 调用和业务数据映射。`run_child_dws` 同时严格识别统一 `ok:false` 与旧信封的
布尔 `success:false`；任何顶层非布尔、矛盾或非字符串的 `ok`/`success`/`outcome` 组合都会作为
`untyped_status` 标为 `unknown`，绝不按 Python truthiness 或 `rc=0` 猜测成功。一个 coherent
`ok:true, outcome:pending` 也不能被脚本扩大为终态成功：运行时保留原 payload 与子 `meta`，并以
`operation_pending` 标为 `unknown`，由产品脚本决定是否能继续投影为顶层 `pending`。它是写编排的保守运输边界：只有稳定的
前置失败会标记为 `failed`；超时、非零退出、不可解析输出和未分类上游错误都标为
`unknown`，因为写入可能已经到达服务端。`batch_data` 固定保留
`succeeded[]/failed[]/unknown[]` 并校验逐项 ID、typed error、未知原因和总数；
`batch_outcome` 从三通道导出四态结果。当前 `todo_batch_create.py`、
`oa_batch_approve.py`、`doc_create_and_write.py`、`import_records.py`、`bulk_add_fields.py` 和
`aitable_import_via_task.py`、`upload_attachment.py` 已使用这条边界；其余脚本仍负责各自
业务映射。它没有 `emit_result`、`emit_error`、`remote_reads` 或 `log()` 参数；不得把这些未实现名称当成可依赖接口。脚本不得通过 `print()`
直接写机器输出，统一调用 `emit`/`failure`；需要诊断时显式 `print(..., file=sys.stderr)`。

## 实施状态与后续验证

脚本接口的渐进迁移已完成于 **Help/输出层**，不再把阶段表述为未来计划：

| 项目 | 当前状态 | 证据与边界 |
|---|---|---|
| 32 个 Agent 入口的 `--format` / `--dry-run` | 已实施 | 逐入口 `--help` 为 32/32、Help 非零为 0；这只证明能力可发现 |
| `text/json/ndjson` 输出函数 | 已实施 | `_runtime.py` 的 `emit` 负责 stdout 形状与成功/失败退出码 |
| 9 个复合远端写入口的 `--yes` 门禁 | 已受控探针验证 | `probe_mono_write_confirmation.py` 以临时 HOME、工作区和 sentinel `dws` 覆盖文档、邮件、日程、待办、审批、字段/记录/文件导入与附件上传；缺确认时为 typed policy failure，未观察 child 调用或新增本地文件；不证明确认后的真实服务端终态 |
| 10 个高风险深层门控 dry-run fixture | 已受控探针验证 | `probe_mono_dry_run.py` 使用临时 HOME、工作区与**假的** `dws` 子进程；它证明脚本在该夹具下不发子进程写调用，不证明真实后端零写 |
| 其余 22 个入口的 dry-run 副作用 | UNVERIFIED | 必须按真实参数、异常和账号路径另行 Agent 取证 |
| 三条写编排的 mixed result 映射 | 已受控 child-runner 验证 | 待办保留成功与未知写入；审批任务解析失败不会发送占位写入；文档写入失败只调用一次并标 `unknown`。假子进程只验证编排和信封，不证明真实后端终态 |
| 真实服务端零写与部分失败/不确定结果 | UNVERIFIED | 需要隔离账号或受控后端，不得由 Help、源码字符串或假子进程推断 |

入口数、Help 可观测数和 dry-run 副作用验证数必须分别记录，不能把“已接入参数”直接
写成“已证明安全”。例如，允许远端只读探测的脚本必须在自己的 Help/Skill 中逐条说明；
不允许把它宣传为“零远端调用”。后续只按单脚本补充真实副作用证据，不重新引入全局
“所有脚本已证明安全”的总则。

## Agent 扫描验收

验收必须由 Agent 语义扫描完成，CI 只能作为辅助：

1. 对每个脚本运行 `--help`，确认两个 flag 的实际声明和类型；
2. 对写脚本执行 `--dry-run`，使用临时 HOME 和受控 child runner，证明零写入、
   零远端写请求；
3. 对 `--format json` 检查 stdout 是单个可解析对象、stderr 无业务数据、退出码
   与 `ok/outcome` 一致；还要注入一个未捕获异常，确认仍输出
   `failure + error.type=internal` 而非 traceback，并确认 failure 缺 typed error、非法 `meta/dry_run` 都会被拒绝；同时确认子 `pending` 结果保留任务 meta、不会被投影为终态成功；
4. 注入一个成功、一个明确失败和一个结果不确定的步骤，确认
   `succeeded/failed/unknown` 不丢失；
5. 对流式脚本检查每行独立可解析且有界/无限模式语义不同；
6. 对每个复合写入口省略 `--yes` 执行，确认返回 `policy/confirmation_required`、
   `execution_state=not_executed`，且临时 child runner 未被调用；
7. 把扫描结果写入评测台账，禁止把生成的 JSON 结果作为仓库 fixture 保存。

仓库提供 `scripts/agent/scan_mono_script_contract.py` 作为可重复的 Agent 扫描器。
它只生成 Markdown：统计入口/内部模块、逐个运行 Help、核对实际 flags，并把未完成的
dry-run 副作用验证明确标为 `UNVERIFIED`；它不是 CI 门禁，也不把扫描结果伪装成测试
fixture。Agent 应在评测或发布前运行：

```bash
python3 scripts/agent/scan_mono_script_contract.py \
  --strict-rfc --strict-flags \
  --output docs/agent-scans/mono-script-contract-YYYYMMDD.md
```

结果错误路径与统一边界由下列 Agent 探针检查。它只保存 Markdown 证据：共享运行时的
未捕获异常、`SystemExit`、`partial_failure`、可选 `meta`，以及 `todo_batch_create.py`
的错误类型输入都必须保持一个可解析的结果对象。

```bash
python3 scripts/agent/probe_mono_result_contract.py \
  --output docs/agent-scans/mono-result-contract-YYYYMMDD.md
```

异步导出这类“任务完成”和“本地文件已安全落盘”分离的入口，应再运行专项 Agent
语义探针。探针使用临时 child runner 与本地 HTTP server，只保存 Markdown，不将
任何 JSON 结果作为仓库 fixture，也不替代真实租户验证：

```bash
python3 scripts/agent/probe_mono_aitable_export_contract.py \
  --output docs/agent-scans/mono-aitable-export-contract-YYYYMMDD.md
```

如需一次性复核 Mono、Multi、Shortcut surface 以及隐藏 shortcut exclusion 队列，可运行：

```bash
python3 scripts/agent/run_skill_contract_audit.py \
  --output docs/agent-scans/skill-contract-audit-YYYYMMDD.md
```

编排器只汇总可读 Markdown/text 证据，不接入 CI，也不保存 JSON fixture；它还会逐条
对拍 Skill 中的 `dws` 路径和显式 flags。Help 对账、CLI 参数对拍、运行时目录集合和
dry-run 副作用仍须在报告中分开标注，不能把总审计
通过解读成真实后端写入安全已被证明。

对高风险写入口，另有 `scripts/agent/probe_mono_dry_run.py` 使用临时 HOME、临时工作区
和假的 `dws` 子进程做受控探针。它目前覆盖 10 个深层门控 fixture；报告中的 `PASS`
只证明这些 fixture，其他参数/异常/账号路径仍必须保持 `UNVERIFIED`：

```bash
python3 scripts/agent/probe_mono_dry_run.py \
  --output docs/agent-scans/mono-dry-run-probe-YYYYMMDD.md
```

对复合写脚本的确认门禁，另有独立 Agent 探针。它故意省略 `--yes`，使用临时 HOME、
工作区和假的 `dws` 子进程确认入口在执行前停止；同样只保存 Markdown，不接入 CI：

```bash
python3 scripts/agent/probe_mono_write_confirmation.py \
  --output docs/agent-scans/mono-write-confirmation-YYYYMMDD.md
```

## 兼容与回滚

脚本保留原有业务参数和人读 `text` 输出；当前全部 Agent 入口都已接入 `--format`，
Agent 需要机器输出时显式传 `--format json`。回滚通过发布版本或脚本实现完成，不让
Agent 追加协议选择参数，也不在同一脚本中同时暴露两套机器信封。

## 决策

“全部入口的 Help/输出接口已实施”与“全部 dry-run 已证明安全”是两件事。后者仍以
脚本为单位渐进取证；在真实副作用证据补齐前，Skill 必须准确声明每个脚本当前实际支持
的参数和已验证边界。
