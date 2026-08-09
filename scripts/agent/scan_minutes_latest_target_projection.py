#!/usr/bin/env python3
"""Agent-only review of the latest Minutes target selection boundary.

The evidence intentionally combines source inspection with an in-memory Go
test and writes Markdown only. It is not a CI or policy gate and stores no
service response fixture.
"""

from __future__ import annotations

import argparse
import datetime as dt
import os
from pathlib import Path
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[2]
SOURCE = ROOT / "internal/shortcut/smart/latest_minutes.go"
TEST_PATTERN = r"TestLatestMinutesTaskUUID"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path, help="Markdown evidence output path")
    args = parser.parse_args()

    source = SOURCE.read_text(encoding="utf-8")
    selector = source[source.index("func latestMinutesUUID"):source.index("func latestMinutesCreateTime")]
    ambiguous_id_rejected = '"id"' not in selector
    partial_time_rejected = "timedCount != len(raw)" in source
    invalid_time_rejected = "不是有效正整数时间戳" in source
    environment = os.environ.copy()
    environment.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-review")
    result = subprocess.run(
        ["go", "test", "-count=1", "./internal/shortcut/smart", "-run", TEST_PATTERN, "-v"],
        cwd=ROOT,
        env=environment,
        text=True,
        capture_output=True,
    )
    passed = result.returncode == 0 and ambiguous_id_rejected and partial_time_rejected and invalid_time_rejected
    transcript = (result.stdout + result.stderr).strip()
    if len(transcript) > 3000:
        transcript = transcript[-3000:]
    report = f"""# Minutes 最近听记目标选择 — Agent review

扫描时间：{dt.datetime.now().astimezone().isoformat(timespec="seconds")}

> 本扫描由 Agent 在当前工作树执行，只运行内存测试并输出 Markdown；不是 CI / policy gate，不保存任何服务端响应或 JSON fixture。

## Result: {"PASS" if passed else "REVIEW"}

- `latestMinutesUUID` 拒绝通用 `id` 回退：**{"yes" if ambiguous_id_rejected else "no"}**
- 部分条目缺少时间时拒绝猜测最新项：**{"yes" if partial_time_rejected else "no"}**
- 非法时间值 fail-closed：**{"yes" if invalid_time_rejected else "no"}**
- 焦点测试：`{TEST_PATTERN}`
- 测试退出码：`{result.returncode}`

## Required behavior

1. 最近听记复合读取只可使用 `taskUuid`、`taskUUID` 或 `uuid` 作为下游详情接口的 task UUID。
2. 仅有通用 `id` 的条目必须成为不可重试的 `api/projection_unknown`，不能被选择后继续读取另一个资源。
3. 明确空列表仍表示“暂无妙记”；未知容器、非法行或无真实 task UUID 均 fail-closed。
4. 只在所有条目都有可比较时间时按时间取最大值；全部无时间才采用服务端顺序，部分时间或非法时间必须 fail-closed。
5. `+latest-minutes`、`+action-items` 与 `+transcript` 继续保持隐藏；本修复不扩大 Agent 的公开命令面。

## Focused test transcript

```text
{transcript or "(no output)"}
```

## Boundary

这证明本地选择器不会将 Minutes 文档 ID 误作 task UUID。真实服务端不同响应形状、时间排序和后续详情读取仍须在隔离账号中由 Agent 单独取证。
"""
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(report, encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    sys.exit(main())
