#!/usr/bin/env python3
"""Agent-only review of the Drive directory-list shortcut during dual validation.

The review inspects the Skill corpus, current CLI Help, and focused projection
tests. It writes Markdown evidence only; it is not a CI or policy gate and
never persists runtime JSON fixtures or Drive item names.
"""

from __future__ import annotations

import argparse
import datetime as dt
import os
from pathlib import Path
import re
import subprocess


ROOT = Path(__file__).resolve().parents[2]
SKILL_ROOTS = (ROOT / "skills/mono", ROOT / "skills/multi")
BOUNDARY = r"(?=$|[\s`)\]|])"
SHORTCUT_ROUTE = re.compile(r"\bdws\s+drive\s+\+list" + BOUNDARY)


def run(command: list[str], environment: dict[str, str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run(command, cwd=ROOT, env=environment, text=True, capture_output=True, check=False)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path, help="Markdown evidence output")
    args = parser.parse_args()

    workflow_routes: list[str] = []
    catalog_mentions: list[str] = []
    for root in SKILL_ROOTS:
        for path in root.rglob("*"):
            if path.suffix not in {".md", ".py", ".sh"}:
                continue
            text = path.read_text(encoding="utf-8")
            for line_no, line in enumerate(text.splitlines(), start=1):
                if not SHORTCUT_ROUTE.search(line):
                    continue
                location = f"{path.relative_to(ROOT)}:{line_no}"
                # Tables may inventory a command, but only executable prose or
                # scripts would teach an Agent to use this still-dual shortcut.
                (catalog_mentions if line.lstrip().startswith("|") else workflow_routes).append(location)

    environment = os.environ.copy()
    environment.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-review")
    tests = run(["go", "test", "-count=1", "./internal/shortcut/drive"], environment)
    help_result = run(["go", "run", "./cmd", "drive", "+list", "--help"], environment)
    help_ok = (
        help_result.returncode == 0
        and "--folder string" in help_result.stdout
        and "--cursor string" in help_result.stdout
        and "--limit int" in help_result.stdout
    )
    passed = not workflow_routes and tests.returncode == 0 and help_ok

    transcript = (tests.stdout + tests.stderr).strip()
    if len(transcript) > 3000:
        transcript = transcript[-3000:]
    hits = lambda values: "无" if not values else "<br>".join(f"`{value}`" for value in values[:30])
    report = f"""# Drive `+list` dual-validate — Agent review

扫描时间：{dt.datetime.now().astimezone().isoformat(timespec="seconds")}

> 本审阅读实际 Skill、当前 CLI Help 和 Drive 回归，只保存 Markdown；它不是 CI / policy gate，也不保存运行时 JSON、文件名或 dentry ID。

## Result: {"PASS" if passed else "REVIEW"}

- 过早教学 shortcut 路由 `dws drive +list`：**{len(workflow_routes)}** 处
- catalog-only 提及：**{len(catalog_mentions)}** 处
- Drive 焦点测试退出码：`{tests.returncode}`
- `+list --help` 声明 `--folder/--cursor/--limit`：`{str(help_ok).lower()}`

## 当前路由与边界

本轮 `drive +list` 仅进入 **dual_validate**：外部仍是既有 legacy payload，框架在内部严格验证下一版候选结果。Skill 不应将该 shortcut 教成新的默认 Agent 路由。

候选结果的边界是：它只列出**请求的 space/folder 的一页**。当服务端给出一致的 `hasMore + nextCursor` 时，可继续翻页；没有分页证据时必须标记为未知。无论哪种情况，都不能把结果扩大为“全部可访问钉盘文件”。跨目录按名称定位文件应使用公开的 `dws drive search --query "<关键词>" --format json`。

## Premature shortcut route hits

{hits(workflow_routes)}

## Catalog-only mentions

{hits(catalog_mentions)}

## Focused test transcript

```text
{transcript or "(no output)"}
```

## Boundary

本审证明的是本地 CLI projection、Help 与 Skill 路由一致性；不证明真实租户中的目录召回率、死条目治理、权限可见性或服务端分页正确性。后者需要独立的脱敏 live evidence 才可晋级 active。
"""
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(report, encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
