#!/usr/bin/env python3
"""Agent-only review of unified Minutes detail artifact failures.

The scan establishes that the public `minutes +detail` shortcut preserves
typed child failure guidance rather than flattening all failed artifacts into
a retryable API error.  It writes only Markdown evidence and is deliberately
not a CI or policy gate.
"""

from __future__ import annotations

import argparse
import datetime as dt
import os
from pathlib import Path
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[2]
SOURCE = ROOT / "internal/shortcut/smart/minutes_detail.go"
TEST_PATTERN = r"TestMinutesDetail(Result|PreservesTyped)"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path, help="Markdown evidence output path")
    args = parser.parse_args()

    source = SOURCE.read_text(encoding="utf-8")
    active = "OutputRollout: output.RolloutUnifiedActive" in source
    typed_projection = all(
        token in source
        for token in (
            "stderrors.As(err, &typed)",
            "typed.Category",
            "typed.StableSubtype",
            "typed.RetryableSet",
            "typed.RetryAfterSeconds",
            "typed.Actions",
        )
    )
    all_failed_details = '"failed_artifacts"' in source and '"error": entry.Error' in source

    environment = os.environ.copy()
    environment.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-review")
    result = subprocess.run(
        ["go", "test", "-count=1", "./internal/shortcut/smart", "-run", TEST_PATTERN, "-v"],
        cwd=ROOT,
        env=environment,
        text=True,
        capture_output=True,
    )
    passed = result.returncode == 0 and active and typed_projection and all_failed_details
    transcript = (result.stdout + result.stderr).strip()
    if len(transcript) > 3000:
        transcript = transcript[-3000:]

    report = f"""# Minutes 详情 artifact 失败语义 — Agent review

扫描时间：{dt.datetime.now().astimezone().isoformat(timespec="seconds")}

> 本扫描由 Agent 在当前工作树运行。它结合源码关系与内存 Go 测试生成 Markdown 证据；不是 CI / policy gate，也不保存服务端响应或 JSON fixture。

## Result: {"PASS" if passed else "REVIEW"}

- `minutes +detail` 当前为 unified active：**{"yes" if active else "no"}**
- 部分失败保留 child 的 category / subtype / hint / actions / retry guidance：**{"yes" if typed_projection else "no"}**
- 全部 artifact 失败时在 aggregate error.details 保留逐项 typed error：**{"yes" if all_failed_details else "no"}**
- 焦点测试：`{TEST_PATTERN}`
- 测试退出码：`{result.returncode}`

## Required behavior

1. 已成功 artifact 与失败 artifact 同时存在时，必须输出 `partial_failure` / rc=7；`failed[]` 中的每项保留稳定 ID 和 typed error。
2. 子错误已经提供 auth、validation、projection 或 retry 指引时，聚合层不得重写成笼统的 `api + retryable:true`。
3. 纯读取的、未分类的临时错误可以保留可重试建议；明确 `retryable:false` 必须保持为不鼓励重放。
4. 全部 artifact 失败必须是普通 `failure`，但 error.details 要保留每个 artifact 的错误事实，不能只丢一个名称列表。
5. 该本地证据不证明真实听记任务的 artifact 可读性、权限或服务端终态。

## Focused test transcript

```text
{transcript or "(no output)"}
```
"""
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(report, encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    sys.exit(main())
