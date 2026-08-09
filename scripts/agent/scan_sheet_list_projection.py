#!/usr/bin/env python3
"""Agent-only review of the sheet list projection and output rollout.

It writes Markdown evidence after inspecting the terminal declaration and
running in-memory Go cases. This is not a CI/policy gate and stores no service
response or JSON fixture in the repository.
"""

from __future__ import annotations

import argparse
import datetime as dt
import os
from pathlib import Path
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[2]
SOURCE = ROOT / "internal/shortcut/sheet/sheet.go"
TEST_PATTERN = r"TestListSheets(Project|UsesUnifiedOutput)"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path, help="Markdown evidence output path")
    args = parser.parse_args()

    source = SOURCE.read_text(encoding="utf-8")
    active = "OutputRollout: output.RolloutUnifiedActive" in source
    selector = source[source.index("func listSheetsProject"):source.index("func listSheetsResolveList")]
    generic_id_rejected = '"sheetId", "sheet_id", "id"' not in selector
    environment = os.environ.copy()
    environment.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-review")
    result = subprocess.run(
        ["go", "test", "-count=1", "./internal/shortcut/sheet", "-run", TEST_PATTERN, "-v"],
        cwd=ROOT,
        env=environment,
        text=True,
        capture_output=True,
    )
    passed = result.returncode == 0 and active and generic_id_rejected
    transcript = (result.stdout + result.stderr).strip()
    if len(transcript) > 3000:
        transcript = transcript[-3000:]
    report = f"""# Sheet 工作表列表投影 — Agent review

扫描时间：{dt.datetime.now().astimezone().isoformat(timespec="seconds")}

> 本扫描由 Agent 在当前工作树执行。它只检查源码声明并运行内存测试，产出 Markdown；不是 CI / policy gate，且不保存服务端响应或 JSON fixture。

## Result: {"PASS" if passed else "REVIEW"}

- 普通 `dws sheet +list-sheets --format json` 已直接走统一结果：**{"yes" if active else "no"}**
- 仅接受明确 `sheetId` / `sheet_id`：**{"yes" if generic_id_rejected else "no"}**
- 焦点测试：`{TEST_PATTERN}`
- 测试退出码：`{result.returncode}`

## Required behavior

1. 明确空数组才表达成功空列表；未知容器、非法行、展示字段或仅有通用 `id` 都必须 fail-closed 为不可重试的 `api/projection_unknown`。
2. 成功 JSON 只有统一 `ok/outcome/data/meta` 语义，`data.count` 与 `meta.count` 对齐；不输出版本标记。
3. 上游未提供分页事实时不得伪造 endpoint 完整性。
4. 该变更只将经过 Agent 审阅的单条幂等读命令从 dual validation 迁入 active；调用者不选择协议版本。

## Focused test transcript

```text
{transcript or "(no output)"}
```

## Boundary

本地证据不能证明真实工作表的权限、空表、服务端嵌套形状或潜在分页行为；这些仍须用隔离文档由 Agent 单独取证。
"""
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(report, encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    sys.exit(main())
