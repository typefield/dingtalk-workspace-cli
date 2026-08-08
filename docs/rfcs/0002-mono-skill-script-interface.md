# RFC 0002：Mono Skill 脚本统一接口

状态：提案（渐进迁移）
范围：`skills/mono/scripts/*.py` 中被 Skill 正向引用的可执行脚本
非目标：修改 `dws` 主命令契约、引入公开协议选择参数、保存生成的 JSON 文件

## 背景

Mono Skill 目前把脚本当作可执行 Agent 入口，但脚本自身没有统一运行时接口。
当前工作树的 Agent 扫描结果：

- 34 个可执行 Python 脚本逐个执行 `--help`；
- 第一阶段迁移前 19 个 Help 暴露脚本级 `--dry-run`，只有 1 个 Help 暴露脚本级
  `--format`，7 个脚本在没有业务参数时 `--help` 返回非零；
- 当前已迁移 `todo_batch_create.py`、`aitable_import_via_task.py`、
  `upload_attachment.py`、`doc_create_and_write.py` 和
  `aitable_export_via_task.py`、`mail_unread_summary.py` 和
  `contact_dept_members.py`、`report_received_today.py`，实际扫描结果为 23 个 dry-run、8 个 format、5 个
  help 非零脚本；
- 很多脚本虽然内部调用 `dws --format json`，但脚本外层仍输出人读文本和日志。

因此“所有脚本支持 `--dry-run/--format json`”不是当前事实。短期已删除这一
错误宣称；长期目标仍然是让 Agent 可依赖统一接口。

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

当前 pilot 已完成 `todo_batch_create.py`、`aitable_import_via_task.py`、
`upload_attachment.py`、`doc_create_and_write.py` 和
`aitable_export_via_task.py`、`mail_unread_summary.py` 和
`contact_dept_members.py`、`report_received_today.py`；其余脚本继续按阶段一逐个迁移。

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

## 兼容与回滚

迁移脚本保留原有业务参数和人读 `text` 输出；`json` 只在脚本明确宣布迁移后
成为 Agent 默认。回滚通过发布版本或内部脚本清单完成，不让 Agent 追加协议选择
参数，也不在同一脚本中同时暴露两套机器信封。

## 决策

“全部支持”是目标，但实现单位是脚本而不是一次性全量改造。先统一运行时和高风险
写脚本，证明 dry-run/部分失败/流向契约，再逐批覆盖其余脚本；在此之前，Skill
必须准确声明每个脚本当前实际支持的参数。
