# doc 文件管理迁移说明

> 本页只用于识别历史入口和迁移目标，不是 `doc` 文件管理命令的正向操作手册。

执行任何文档操作前，必须先阅读 [`../doc.md`](../doc.md) 的路由和安全说明，并对目标
命令运行 leaf `--help`。文件管理入口已经逐步从 `doc` 迁移到 `drive` / `wiki`；
Agent 不应根据本页重新生成隐藏或 deprecated 的 `doc upload/download/copy/move/rename/delete`
命令。

## 当前路由

| 用户意图 | 当前优先入口 | 说明 |
|---|---|---|
| 上传/下载/复制/移动/重命名/删除钉盘文件 | `dws drive ...` | 具体 flags 以 `dws drive <verb> --help` 为准 |
| 在知识库中创建目录或节点 | `dws wiki node ...` | 需要 workspace 时使用 Schema/Help 声明的参数 |
| 导出在线文档为文件 | `dws doc export ...` | “导出”不是“下载已有文件” |
| 在文档正文插入附件 | `dws doc media ...` | 与独立文件上传不同 |
| 读取/搜索文档节点 | `dws doc read/search` 或 Schema 指定入口 | 先确认节点类型和权限 |

## 迁移规则

- `doc upload`、`doc download`、`doc copy`、`doc move`、`doc rename`、`doc delete`：
  仅作为历史兼容入口保留；新调用统一改用 `drive` 对应 verb。
- `doc folder create`：个人空间/钉盘使用 `drive mkdir`；知识库目录使用
  `wiki node create --type folder`，具体参数必须以 leaf Help 为准。
- `doc export` 仍是在线文档格式转换；不能用 `drive download` 替代。
- 删除、移动、重命名等写操作仍必须遵循当前 Schema 的 risk/confirmation 语义，不能
  因为旧 `doc` 入口存在而绕过确认。

## 兼容入口处理

如果历史自动化仍传入 `doc` 文件管理命令：

1. 先运行原命令的 `--help`，确认当前版本是否仍注册；
2. 读取弃用提示中的目标入口，并用目标入口的 Help 重组参数；
3. 不要把旧命令写入 Skill recipe、Agent 示例或新的自动化脚本；
4. 对删除/移动等不可逆操作，迁移后仍需重新执行确认流程。

## 验收要求

本页不保存命令输出或 JSON fixture。发布前由 Agent 扫描：

- 深层 reference 不再把 deprecated `doc` 文件管理命令作为推荐 recipe；
- 推荐入口在当前 Schema/Help 中可发现；
- 所有迁移后的写操作保留 confirmation、dry-run 和输出契约；
- 旧入口只作为兼容事实记录，不作为 Agent canonical identity。
