#!/usr/bin/env python3
"""Agent-only review of the active Drive directory-list shortcut.

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
                # Tables may inventory a command, while executable prose or
                # scripts prove that the active route is actually taught.
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
    passed = bool(workflow_routes or catalog_mentions) and tests.returncode == 0 and help_ok

    transcript = (tests.stdout + tests.stderr).strip()
    if len(transcript) > 3000:
        transcript = transcript[-3000:]
    hits = lambda values: "无" if not values else "<br>".join(f"`{value}`" for value in values[:30])
    report = f"""# Drive `+list` unified-active — Agent review

扫描时间：{dt.datetime.now().astimezone().isoformat(timespec="seconds")}

> 本审阅读实际 Skill、当前 CLI Help 和 Drive 回归，只保存 Markdown；它不是 CI / policy gate，也不保存运行时 JSON、文件名或 dentry ID。

## Result: {"PASS" if passed else "REVIEW"}

- 正向教学 shortcut 路由 `dws drive +list`：**{len(workflow_routes)}** 处
- 路由表教学：**{len(catalog_mentions)}** 处
- Drive 焦点测试退出码：`{tests.returncode}`
- `+list --help` 声明 `--folder/--cursor/--limit`：`{str(help_ok).lower()}`

## 当前路由与边界

`drive +list` 已进入 **unified_active**：普通 `--format json` 直接返回 `ok/outcome/data/meta`。Skill 可以把它作为指定位置目录浏览的默认 Agent 路由；不公开协议选择参数，也不输出版本标记。

结果边界是：它只列出**请求的 space/folder 的一页**。一致的 `hasMore + nextCursor` 或非空 token-only continuation 都可安全续页；token 缺失而没有显式终态布尔时必须标记为未知。无论哪种情况，都不能把结果扩大为“全部可访问钉盘文件”。跨目录按名称定位文件应使用 `dws drive +search --query "<关键词>" --format json`。

## Active shortcut route hits

{hits(workflow_routes)}

## Route-table mentions

{hits(catalog_mentions)}

## Focused test transcript

```text
{transcript or "(no output)"}
```

## Boundary

本审证明的是本地 CLI projection、Help 与 Skill 路由一致性；真实租户中的 token-only 续页形状由独立脱敏 live probe 记录。两者都不证明目录召回率、死条目治理、权限可见性或租户级目录完整。
"""
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(report, encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
