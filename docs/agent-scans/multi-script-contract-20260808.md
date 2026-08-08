# Multi Skill 脚本契约扫描（Agent 实测）

扫描时间：2026-08-08。扫描入口：`scripts/agent/scan_multi_script_contract.py`。

## 口径

- 文件数是 `skills/multi/*/scripts/*.py` 的文件数，不能当作 Agent 命令数。
- Agent 入口只统计包含 `if __name__ == "__main__"` 的脚本；内部 helper 不要求有 CLI 参数。
- `--help` 只验证接口是否可观测，不证明 dry-run 没有远端副作用，也不证明 JSON 数据语义正确。
- `--dry-run`/`--format` 不是所有脚本的强制要求：固定输出的检查器、导入/上传工具可以有专用输出语义；只有 Skill 明确宣称脚本级契约时才应作为失败项。

## 当前结果

| 指标 | 实测 |
|---|---:|
| Python 文件 | 52 |
| Agent 入口 | 42 |
| `--help` 非零 | 0 |
| Help 文本提及 `--dry-run` | 30/42 |
| Help 文本提及 `--format` | 1/42 |

此前有 9 个入口因把 `--help` 当业务参数而非帮助请求返回非零；已增加无副作用的帮助分支。本轮又为 4 个明确会产生 AITable 远端写入/上传的入口补齐脚本级 `--dry-run`，并用本地 fixture 验证不会调用 dws 或 OSS。这里的 flag 数是 Help 文本提及数，不等同于 argparse 声明；多数 Multi 脚本使用手工 `sys.argv`，因此还需逐项确认参数是否真的被接受。剩余没有两个 flag 的入口不自动判错，须与对应 Skill 的调用说明逐项对账。

## 需要继续由 Agent 审核的重点

1. Skill 若宣称某脚本支持脚本级 `--dry-run` 或 `--format`，必须用该脚本自己的 `--help` 和一次受控 dry-run 取证；不能从脚本内部调用的 `dws ... --format json` 推断脚本自身支持。
2. 计划型脚本应优先在顶层 dry-run 早退；深层把 `dry_run` 传入子调用的脚本需由受控 runner 证明没有远端写入。
3. 无 format flag 的脚本应明确是固定 JSON、固定文本还是仅供内部调用，避免在 Skill 总则中笼统宣称“所有脚本统一支持”。
4. 每次新增/修改脚本重新运行本扫描，并把结果追加到评测台账；扫描产物保持 Markdown，不保存 JSON fixture。

本次复核还发现并修正了 AITable Skill 的两个脚本调用偏移：Skill 原先把
`bulk_add_fields.py`、`upload_attachment.py` 写成 `--base-id/--file` 选项形式，
但脚本契约是位置参数；现已统一为 `<baseId> <tableId> <fields.json>` 和
`<baseId> <filePath>`，并在写入口上标出可选 `--dry-run`。

随后扫描又发现 Minutes 深层 recipe 把脚本参数 `--max` 写成了 `--limit`；
`--limit` 只属于脚本内部调用的 `dws minutes list`，已改回脚本自身的 `--max`。
复扫结果：Documented Python-script flag mismatches = 0。

## 修复边界

本轮只修复了 `--help` 可观测性，不把 42 个入口强行改造成同一 CLI。强行添加两个 flag 会改变脚本参数和输出契约，反而扩大兼容风险；后续应按 Agent 实际使用路径逐个迁移并配套 dry-run 副作用证据。
