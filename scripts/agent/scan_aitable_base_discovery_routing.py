#!/usr/bin/env python3
"""Agent-only review of active AITable Base discovery routes.

The review reads the actual Skill corpus and current CLI Help/test surface. It
writes Markdown evidence only; it is not a CI or policy gate and never saves a
runtime JSON fixture.
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
BOUNDARY = r"(?=$|[\s`)\]|])"
NATIVE = re.compile(r"\bdws\s+aitable\s+base\s+(?:list|search)" + BOUNDARY)
# A shortcut catalog can name a command without teaching an Agent to invoke it.
# Only a complete `dws ... +base-*` invocation is an external route.
SHORTCUT_ROUTE = re.compile(r"\bdws\s+aitable\s+\+base-(?:list|search)" + BOUNDARY)


def run(command: list[str], environment: dict[str, str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run(command, cwd=ROOT, env=environment, text=True, capture_output=True, check=False)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path, help="Markdown evidence output")
    args = parser.parse_args()

    native_hits: list[str] = []
    shortcut_routes: list[str] = []
    shortcut_catalog: list[str] = []
    for root in SKILL_ROOTS:
        for path in root.rglob("*"):
            if path.suffix not in {".md", ".py", ".sh"}:
                continue
            text = path.read_text(encoding="utf-8")
            for line_no, line in enumerate(text.splitlines(), start=1):
                location = f"{path.relative_to(ROOT)}:{line_no}"
                if NATIVE.search(line):
                    native_hits.append(location)
                if SHORTCUT_ROUTE.search(line):
                    # Generated shortcut overviews are descriptive inventory,
                    # not a workflow instruction. A non-table occurrence is a
                    # positive Agent route and would be premature in dual.
                    (shortcut_catalog if line.lstrip().startswith("|") else shortcut_routes).append(location)

    environment = os.environ.copy()
    environment.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-review")
    tests = run(["go", "test", "-count=1", "./internal/shortcut/aitable"], environment)
    search_help = run(["go", "run", "./cmd", "aitable", "+base-search", "--help"], environment)
    list_help = run(["go", "run", "./cmd", "aitable", "+base-list", "--help"], environment)
    search_help_ok = search_help.returncode == 0 and "--query string" in search_help.stdout and "必填" in search_help.stdout
    list_help_ok = list_help.returncode == 0 and "--limit int" in list_help.stdout and "--cursor string" in list_help.stdout
    # Active discovery routes must teach the unified shortcuts. The historical
    # native commands remain executable compatibility, but must not be the
    # positive Agent route after rollout.
    passed = not native_hits and bool(shortcut_routes) and tests.returncode == 0 and search_help_ok and list_help_ok

    transcript = (tests.stdout + tests.stderr).strip()
    if len(transcript) > 3000:
        transcript = transcript[-3000:]
    hits = lambda values: "无" if not values else "<br>".join(f"`{value}`" for value in values[:30])
    report = f"""# AITable Base 发现 unified-active — Agent review

扫描时间：{dt.datetime.now().astimezone().isoformat(timespec="seconds")}

> 本审阅读取实际 Skill 语料、当前 CLI Help 和 AITable 回归，只保存 Markdown，不是 CI / policy gate，也不保存 JSON fixture。

## Result: {"PASS" if passed else "REVIEW"}

- native Agent 路由 `dws aitable base list/search`：**{len(native_hits)}** 处
- shortcut catalog 提及（非工作流）：**{len(shortcut_catalog)}** 处
- active shortcut Agent 路由 `dws aitable +base-list/+base-search`：**{len(shortcut_routes)}** 处
- AITable 焦点测试退出码：`{tests.returncode}`
- `+base-search --help` 声明必填 `--query`：`{str(search_help_ok).lower()}`
- `+base-list --help` 声明 `--limit/--cursor`：`{str(list_help_ok).lower()}`

## 当前路由规则

有名称关键词时：

```sh
dws aitable +base-search --query "<名称>" --format json
```

只浏览最近访问时：

```sh
dws aitable +base-list --format json
```

二者都不是全量 Base 目录；`+base-search` 的零候选也不证明业务上不存在。没有可信候选时请求 URL 或 baseId，不得臆造标识符。Agent 只按顶层 `ok/outcome`、`data` 和 `meta.pagination` 分支；不再解析 historical 裸 payload。

## Native route hits

{hits(native_hits)}

## Active shortcut route hits

{hits(shortcut_routes)}

## Catalog-only mentions

{hits(shortcut_catalog)}

## Focused test transcript

```text
{transcript or "(no output)"}
```

## Boundary

本审阅验证本地路由、Help 和投影契约；不证明最近访问列表的召回率、搜索索引健康、死条目治理或真实租户权限。
"""
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(report, encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
