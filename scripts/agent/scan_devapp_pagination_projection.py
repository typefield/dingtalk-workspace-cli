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
TEST_PATTERN = r"TestDevApp(ProjectedLists(PreservePaginationEvidence|RejectInvalidPaginationEvidence)|ListProjectionSeparatesKnownEmptyFromUnknown|PaginatedShortcutsEmitUnifiedResumableResults)"
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
    inactive = []
    for shortcut in EXPECTED_CALLERS:
        block = re.search(
            rf"var {shortcut} = shortcut\.Shortcut\{{(?:(?!^var ).)*?\n\}}",
            source,
            flags=re.MULTILINE | re.DOTALL,
        )
        if block is None or "projectDevAppPage(data" not in block.group(0):
            missing.append(shortcut)
        # Output rollout is release-owned. An Agent must not select it through
        # argv; this review verifies that the terminal is active on its ordinary
        # --format json path.
        if f"frameworkUnified({shortcut})" not in source:
            inactive.append(shortcut)

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

    passed = result.returncode == 0 and not missing and not inactive
    status = "PASS" if passed else "REVIEW"
    checked_callers = len(EXPECTED_CALLERS) - len(missing) - len(inactive)
    timestamp = dt.datetime.now().astimezone().isoformat(timespec="seconds")
    test_excerpt = (result.stdout + result.stderr).strip()
    if len(test_excerpt) > 3000:
        test_excerpt = test_excerpt[:1450] + "\n... (middle transcript elided) ...\n" + test_excerpt[-1450:]

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
6. 只有显式的空数组才可投影为成功空列表；缺容器、非数组、非法行或仅展示字段的行必须为 `api/projection_unknown`。
7. 只有经本项 Agent 审阅的四条 terminal command 才在原路径直接输出统一结果；调用者只传 `--format json`。
8. 非末页必须在 `meta.pagination` 中保留 `endpoint_exhausted:false` 与 `next_token`，且不输出任何协议版本标记。

## Source coverage

"""
    if missing:
        report += "- REVIEW：下列 Shortcut 未在 Execute 中调用 `projectDevAppPage`：" + ", ".join(f"`{name}`" for name in missing) + "\n"
    else:
        report += "- PASS：四个列表 Shortcut 均先验证分页证据，再交给统一结果映射。\n"
    if inactive:
        report += "- REVIEW：下列 Shortcut 尚未进入 `unified_active`，外部仍为 legacy：" + ", ".join(f"`{name}`" for name in inactive) + "\n"
    else:
        report += "- PASS：四条已审阅的列表 Shortcut 均在原命令路径直接输出统一结果；没有公开版本/协议选择参数。\n"
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
