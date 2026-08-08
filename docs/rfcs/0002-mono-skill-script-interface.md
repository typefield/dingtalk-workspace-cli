# RFC 0002：Mono Skill 脚本统一接口

状态：提案（渐进迁移）
范围：`skills/mono/scripts/*.py` 中被 Skill 正向引用的可执行脚本
非目标：修改 `dws` 主命令契约、引入公开协议选择参数、保存生成的 JSON 文件

## 背景

Mono Skill 目前把脚本当作可执行 Agent 入口，但脚本自身没有统一运行时接口。
当前工作树的 Agent 扫描结果（口径必须区分文件、入口和 Help 可观测能力）：

| 口径 | 数量 | 定义 |
|------|------|------|
| Python 文件 | 35 | `skills/mono/scripts/*.py` 全部文件，包含内部模块 |
| Agent 入口 | 32 | AST 中包含 `if __name__ == "__main__"` 的脚本 |
| 内部模块 | 3 | `_runtime.py`、`attendance_report_common.py`、`minutes_list_parse.py`，不应被当作 CLI 入口 |
| Help 暴露 `--dry-run` | 32 | 对 32 个入口逐个运行 `python <script> --help` 后扫描实际输出 |
| Help 暴露 `--format` | 32 | 同上；类型为 `text|json|ndjson` |
| Help 非零 | 0 | 当前版本在缺少业务参数和可选运行依赖时均可完成能力发现 |

作为历史基线，迁移前曾有 19 个入口暴露 `--dry-run`、仅 1 个入口暴露脚本级
`--format`，且存在 Help 非零脚本；这些数字不能继续作为当前状态。扫描必须记录
四层口径，不能用文件数代替入口数，也不能用源码是否调用 `add_contract_flags`
代替实际 Help 结果。
- 当前已迁移 `todo_batch_create.py`、`aitable_import_via_task.py`、
  `upload_attachment.py`、`doc_create_and_write.py`、`aitable_export_via_task.py`、
  `mail_unread_summary.py`、`contact_dept_members.py`、`report_received_today.py`、
  `oa_batch_approve.py`、`calendar_schedule_meeting.py`、`mail_send_with_cc.py`、
  `oa_pending_review.py`、`report_inbox_today.py`、`drive_tree_list.py`、
  `calendar_free_slot_finder.py`、`todo_overdue_check.py`、
  `minutes_recent_summary.py`、`minutes_extract_todos.py`、
  `calendar_today_agenda.py`、`attendance_team_shift.py`、
  `attendance_schedule_export.py`、`attendance_my_record.py`、
  `import_records.py`、`bulk_add_fields.py`、`todo_daily_summary.py`、
  `attendance_vacation_balance.py`、`attendance_report_record.py` 和
  `attendance_report_daily.py`、`attendance_report_monthly.py`、`attendance_report_detail.py` 和
  `attendance_report_checkin.py`、`attendance_schedule_import.py`，当前实际扫描结果为
  32 个 dry-run、32 个 format、0 个 help 非零脚本；
- 很多脚本虽然内部调用 `dws --format json`，但脚本外层仍输出人读文本和日志。

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

### format

- `json`：一次执行输出一个可解析的统一结果对象；业务结果放在 `data`，失败放在
  `error`，退出码与结果一致。
- `text`：面向用户的人读输出；日志必须走 stderr，stdout 只输出正文。
- `ndjson`：每条独立结果一行，仅用于流式或批量逐项输出。

脚本内部调用 `dws` 时必须使用同一格式并保留子命令的 `ok/outcome/error/meta`。
脚本不得把 `success` 字符串重新拼成自己的第二套信封，也不得把日志混入 JSON
stdout。

### dry-run

`--dry-run` 的最低保证是：

1. 不创建、修改或删除本地文件；
2. 不触发远端写请求、不创建任务、不发送消息；
3. 输出完整执行计划，包括目标、参数、预计步骤和可能的删除/覆盖范围；
4. 返回成功时明确标记 `dry_run: true`；不能把预览伪装成业务已完成。

只读脚本可以选择“请求预览”或“跳过远端读取”，但必须在 Help/Skill 中说明
`remote_reads` 语义。写脚本默认要求零远端调用；无法预览的脚本不得标称支持
`--dry-run`。

### 流式例外

`event consume` 等持续流命令默认使用 `ndjson`。只有显式设置 `--max-events`
或 `--duration`，才允许 `json`/`pretty`；无限流不能承诺单个 JSON 文档。

## 运行时实现

新增共享 Python 模块（随 Skill 一起发布，不保存运行结果）：

```text
scripts/_runtime.py
  add_script_flags(parser)
  run_child_dws(args, *, dry_run, format, remote_reads)
  emit_result(result, *, format, dry_run)
  emit_error(error, *, format)
```

模块只负责参数、stdout/stderr、退出码和子进程结果投影；每个脚本仍负责业务
参数校验、步骤编排和业务数据映射。脚本不得通过 `print()` 直接写机器输出，统一
调用 `emit_result`；诊断日志使用 `log()` 写 stderr。

## 渐进迁移

### 阶段一：高风险写脚本

先迁移 `aitable_import_via_task.py`、`aitable_export_via_task.py`、
`doc_create_and_write.py`、`upload_attachment.py`、`attendance_schedule_import.py`、
`oa_batch_approve.py`、`todo_batch_create.py`。

当前 pilot 已完成 32 个 Agent 入口；3 个内部模块不纳入脚本接口统计。入口数、Help
可观测数和 dry-run 副作用验证数必须分别记录，不能把“已接入参数”直接写成“已证明安全”。

`attendance_vacation_balance.py` 与排班导出一样，dry-run 会远端只读查询并构造内存中的
Excel 计划，但不会写本地文件。

`attendance_schedule_export.py` 属于远端只读校验型 dry-run：会查询排班并生成内存中的
预览，但不会写 Excel；Skill 不应把它描述成“零远端调用”。

验收重点是 dry-run 零写入、部分失败逐项保留、失败退出码和重试安全。

### 阶段二：复合读脚本

迁移考勤报表、听记摘要、邮件摘要、日程和通讯录脚本，统一 JSON 数据结构，
保留 text 展示为兼容模式。脚本级 `--format` 必须透传到所有子 dws 调用。

### 阶段三：流式/长任务脚本

迁移事件监听、轮询和导入任务脚本，明确 `ndjson`、`pending`、`next_command`
和超时语义；不得为了统一而破坏长连接消费。

每个阶段按单脚本发布，未迁移脚本继续按自身 Help 声明能力，不把未实现能力写入
Skill 总则。

## Agent 扫描验收

验收必须由 Agent 语义扫描完成，CI 只能作为辅助：

1. 对每个脚本运行 `--help`，确认两个 flag 的实际声明和类型；
2. 对写脚本执行 `--dry-run`，使用临时 HOME 和受控 child runner，证明零写入、
   零远端写请求；
3. 对 `--format json` 检查 stdout 是单个可解析对象、stderr 无业务数据、退出码
   与 `ok/outcome` 一致；
4. 注入一个成功、一个明确失败和一个结果不确定的步骤，确认
   `succeeded/failed/unknown` 不丢失；
5. 对流式脚本检查每行独立可解析且有界/无限模式语义不同；
6. 把扫描结果写入评测台账，禁止把生成的 JSON 结果作为仓库 fixture 保存。

仓库提供 `scripts/agent/scan_mono_script_contract.py` 作为可重复的 Agent 扫描器。
它只生成 Markdown：统计入口/内部模块、逐个运行 Help、核对实际 flags，并把未完成的
dry-run 副作用验证明确标为 `UNVERIFIED`；它不是 CI 门禁，也不把扫描结果伪装成测试
fixture。Agent 应在评测或发布前运行：

```bash
python3 scripts/agent/scan_mono_script_contract.py \
  --strict-rfc --strict-flags \
  --output docs/agent-scans/mono-script-contract-YYYYMMDD.md
```

对高风险写入口，另有 `scripts/agent/probe_mono_dry_run.py` 使用临时 HOME、临时工作区
和假的 `dws` 子进程做受控探针。它目前覆盖 7 个深层门控 fixture；报告中的 `PASS`
只证明这些 fixture，其他参数/异常/账号路径仍必须保持 `UNVERIFIED`：

```bash
python3 scripts/agent/probe_mono_dry_run.py \
  --output docs/agent-scans/mono-dry-run-probe-YYYYMMDD.md
```

## 兼容与回滚

迁移脚本保留原有业务参数和人读 `text` 输出；`json` 只在脚本明确宣布迁移后
成为 Agent 默认。回滚通过发布版本或内部脚本清单完成，不让 Agent 追加协议选择
参数，也不在同一脚本中同时暴露两套机器信封。

## 决策

“全部支持”是目标，但实现单位是脚本而不是一次性全量改造。先统一运行时和高风险
写脚本，证明 dry-run/部分失败/流向契约，再逐批覆盖其余脚本；在此之前，Skill
必须准确声明每个脚本当前实际支持的参数。
