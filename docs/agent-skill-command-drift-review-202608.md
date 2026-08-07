# 2026-08 Agent Skill 指令偏移审阅与整改

## 目的

本轮不是用 CI 的“命令路径存在”代替 Agent 审阅，而是以当前构建出的
`dws --help`、Schema、Skill 正文和脚本实际 argv 为四份证据逐项对拍。

仓库现有 `skill-command-check` 会解析 Skill 中的命令路径，但在解析引用后
只按 path 去重并验证路由；它不能证明 flag 是公开 canonical、必填参数齐全，
也不能证明脚本传给子进程的参数与 Help 一致。因此该检查通过仍可能存在
“命令能找到，但 Agent 复制即失败”或“长期学习隐藏兼容别名”的问题。

## 本轮发现与处置

| 偏移 | 当前 Help/Schema 事实 | 处置 |
|---|---|---|
| 部门成员查询大量使用 `--ids` | `contact dept list-members` 的公开必填参数是 `--depts`；`--ids` 只是隐藏兼容别名 | mono/multi Skill、attendance recipe、共享 conventions 和 Python 子进程 argv 全部改用 `--depts`；兼容别名仍保留给旧调用方 |
| AITable view update 示例遗漏目标身份 | 根命令及 12 个属性更新叶都需要 `--base-id`、`--table-id`、`--view-id` | 同步修正 Cobra Help、ContractFinal Agent examples 和 AITable Skill；增加单测，逐叶检查三项必填身份参数 |
| 考勤明细脚本使用 `--from/--to` | `attendance check result/record` 公开参数是 `--start/--end` | mono/multi 脚本改用公开参数 |
| 班次脚本使用 `--page-index/--page-size` | `attendance class search` 公开参数是 `--page/--limit` | 导入/导出脚本改用公开参数 |
| 听记脚本对子命令传 `--max` | `minutes list mine` 公开参数是 `--limit` | 保留脚本自身 `--max` 用户选项，但对子进程统一传 canonical `--limit` |
| 会议室脚本传隐藏 `--available` | `calendar room search` 在传入 `--start/--end` 后即执行时段可用性查询，公开 Help 无该 flag | 删除对子进程的隐藏 flag |
| mono devdoc 把 `--page/--size` 写成 int | 两个公开入口的 Help 均声明 string，默认值为 `"1"`/`"10"` | 只修类型说明；`devdoc article search` 仍是有效兼容入口，不删除、不弱化路由 |
| Mail Skill 把隐藏兼容 flag 写成公开别名 | 公开 flag 必须按具体命令的 Help/Schema：消息列表使用 `--folder-id`，文件夹列表使用 `--folder`，其他相关命令使用 `--limit`/`--content`/`--from`；`--size`/`--page-size`/`--body`/`--sender` 以及命令各自的反向 folder 别名只属于执行兼容层 | mono/multi 参考文档只教各命令公开 flag；脚本对 DWS 子进程改传 `--limit`/`--content`，脚本自身仅保留隐藏 `--size` 兼容 |

## 验收口径

1. Agent-facing 文档和脚本不再教授上述隐藏兼容参数。
2. `dws contact dept list-members --help` 只以 `--depts` 为 canonical。
3. AITable 12 个 view update 子命令的 Help 示例和最终 Agent example 都包含
   `--base-id`、`--table-id`、`--view-id`。
4. 修改后的 Python 文件通过 AST 解析，且子进程 argv 与当前 Help 对齐。
5. Skill 路径完整性、Agent example contract、Help/Schema 同源检查继续通过。

## 边界

- 本轮不删除兼容别名，避免破坏旧脚本；只停止在新 Agent 材料中继续传播。
- `--max` 等参数如果是 Python 脚本自己的用户接口，可以保留；整改对象是脚本
  调用 `dws` 时使用的隐藏参数。
- Agent 语义审阅仍然是发布前活动，不把 CI path checker 描述成能够发现全部
  参数、结果语义或安全偏移。
