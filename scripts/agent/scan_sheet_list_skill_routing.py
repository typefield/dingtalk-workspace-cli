#!/usr/bin/env python3
"""Agent-only audit that Skill sheet-list recipes use the unified shortcut.

This deliberately inspects the Agent instruction corpus rather than becoming a
CI policy. It writes a Markdown review and no response fixture or JSON index.
"""

from __future__ import annotations

import argparse
import datetime as dt
import os
from pathlib import Path
import re
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[2]
SKILL_ROOTS = (ROOT / "skills/mono", ROOT / "skills/multi")
COMMAND_BOUNDARY = r"(?=$|[\s`)\]|])"
LEGACY = re.compile(r"\bdws\s+sheet\s+list" + COMMAND_BOUNDARY)
UNIFIED = re.compile(r"\bdws\s+sheet\s+\+list-sheets" + COMMAND_BOUNDARY)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path, help="Markdown evidence output path")
    args = parser.parse_args()

    legacy_hits: list[str] = []
    unified_hits = 0
    touched_files: set[Path] = set()
    for root in SKILL_ROOTS:
        for path in root.rglob("*"):
            if path.suffix not in {".md", ".py", ".sh"}:
                continue
            text = path.read_text(encoding="utf-8")
            for line_no, line in enumerate(text.splitlines(), start=1):
                if LEGACY.search(line):
                    legacy_hits.append(f"{path.relative_to(ROOT)}:{line_no}")
                if UNIFIED.search(line):
                    unified_hits += 1
                    touched_files.add(path)

    environment = os.environ.copy()
    environment.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-review")
    result = subprocess.run(
        ["go", "test", "-count=1", "./internal/shortcut/sheet"],
        cwd=ROOT,
        env=environment,
        text=True,
        capture_output=True,
    )
    passed = result.returncode == 0 and not legacy_hits and unified_hits > 0
    transcript = (result.stdout + result.stderr).strip()
    if len(transcript) > 3000:
        transcript = transcript[-3000:]
    legacy_text = "无" if not legacy_hits else "<br>".join(f"`{hit}`" for hit in legacy_hits[:30])
    report = f"""# Sheet 工作表列表 Skill 路由 — Agent review

扫描时间：{dt.datetime.now().astimezone().isoformat(timespec="seconds")}

> 本扫描由 Agent 在当前工作树执行，审阅实际 Skill 指令和本地 Sheet 回归；它只输出 Markdown，不是 CI / policy gate，也不保存 JSON fixture。

## Result: {"PASS" if passed else "REVIEW"}

- 正向 `dws sheet +list-sheets` 示例：**{unified_hits}** 处，分布在 **{len(touched_files)}** 个文件
- 仍教学 legacy `dws sheet list` 的示例：**{len(legacy_hits)}** 处
- Sheet 焦点测试退出码：`{result.returncode}`

## Routing rule

当 Agent 需要取得真实 `sheetId` 供后续读写时，使用：

```sh
dws sheet +list-sheets --node <NODE_ID_OR_URL> --format json
```

旧 `sheet list` 保持 CLI 兼容，但 Skill 不再把它作为正向 Agent 路径。新路径是已审阅的统一结果入口：只接受明确 sheet ID、未知响应 fail-closed，且不伪造分页终态。

## Legacy hits

{legacy_text}

## Focused test transcript

```text
{transcript or "(no output)"}
```

## Boundary

这验证的是 Skill 指令与本地统一输出入口的一致性；不证明真实表格权限、服务端响应形状或写后效果。
"""
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(report, encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    sys.exit(main())
