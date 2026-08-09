#!/usr/bin/env python3
"""Agent-only review of devapp shortcut pagination projection.

This is intentionally a release-review aid rather than a CI or policy gate.
It runs the focused in-memory Go test, inspects the four shortcut call sites,
and writes Markdown evidence only. It stores no response fixtures or JSON
catalogs in the repository.
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
SOURCE = ROOT / "internal/shortcut/devapp/devapp.go"
TEST_PATTERN = r"TestDevAppProjectedLists(PreservePaginationEvidence|RejectInvalidPaginationEvidence)"
EXPECTED_CALLERS = (
    "ListApp",
    "PermissionList",
    "EventList",
    "VersionList",
)


def run() -> int:
    parser = argparse.ArgumentParser(description="Agent review of devapp pagination projection")
    parser.add_argument("--output", required=True, type=Path, help="Markdown evidence path")
    args = parser.parse_args()

    source = SOURCE.read_text(encoding="utf-8")
    missing = []
    for shortcut in EXPECTED_CALLERS:
        block = re.search(
            rf"var {shortcut} = shortcut\.Shortcut\{{(?:(?!^var ).)*?\n\}}",
            source,
            flags=re.MULTILINE | re.DOTALL,
        )
        if block is None or "projectDevAppPage(data" not in block.group(0):
            missing.append(shortcut)

    environment = os.environ.copy()
    environment.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-review")
    command = [
        "go",
        "test",
        "-count=1",
        "./internal/shortcut/devapp",
        "-run",
        TEST_PATTERN,
        "-v",
    ]
    result = subprocess.run(command, cwd=ROOT, env=environment, text=True, capture_output=True)

    passed = result.returncode == 0 and not missing
    status = "PASS" if passed else "REVIEW"
    checked_callers = len(EXPECTED_CALLERS) - len(missing)
    timestamp = dt.datetime.now().astimezone().isoformat(timespec="seconds")
    test_excerpt = (result.stdout + result.stderr).strip()
    if len(test_excerpt) > 3000:
        test_excerpt = test_excerpt[-3000:]

    report = f"""# devapp shortcut pagination projection — Agent review

扫描时间：{timestamp}

> 这是 Agent 的语义审阅取证，不是 CI / policy gate。扫描只调用内存中的 Go 测试，并输出 Markdown；不会保存任何上游响应或 JSON fixture。

## Result: {status}

- 已审阅分页 shortcut：**{checked_callers}/{len(EXPECTED_CALLERS)}**
- 覆盖入口：`devapp +list`、`+permission-list`、`+event-list`、`+version-list`
- 焦点测试：`{TEST_PATTERN}`
- 测试退出码：`{result.returncode}`

## Required behavior

1. 保留顶层及嵌套 `result/data/content/pageInfo/pagination` 中的有效 `hasMore` / `nextCursor`。
2. `hasMore` 非布尔值必须返回 `validation/pagination_invalid`，不能投影成空分页。
3. `nextCursor` 非字符串、`hasMore=true` 无游标必须返回 `validation/pagination_incomplete`。
4. 多层 `hasMore` 或非空 `nextCursor` 互相冲突、末页仍带游标必须返回 `validation/pagination_conflict`。
5. 上述失败均为 `response_projection`、不可安全重试；不得输出成功列表。

## Source coverage

"""
    if missing:
        report += "- REVIEW：下列 Shortcut 未在 Execute 中调用 `projectDevAppPage`：" + ", ".join(f"`{name}`" for name in missing) + "\n"
    else:
        report += "- PASS：四个列表 Shortcut 均先验证分页证据，再交给统一结果映射。\n"
    report += f"""

## Focused test transcript

```text
{test_excerpt or "(no output)"}
```

## Boundary

这证明本地投影不会再静默吞掉异常或矛盾的分页字段；真实 devapp 账号的续翻、服务端末页语义及 `dev ...` / `devapp +...` 的端到端对拍仍需单独 Agent 实测。
"""

    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(report, encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    sys.exit(run())
