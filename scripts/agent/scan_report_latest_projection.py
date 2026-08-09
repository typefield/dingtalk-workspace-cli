#!/usr/bin/env python3
"""Agent-only review of report +report-latest projection semantics."""

from __future__ import annotations

import argparse
import datetime as dt
import os
from pathlib import Path
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[2]
SOURCE = ROOT / "internal/shortcut/smart/report_latest.go"
TEST = ROOT / "internal/shortcut/smart/report_latest_projection_test.go"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()

    source = SOURCE.read_text(encoding="utf-8")
    test_source = TEST.read_text(encoding="utf-8")
    checks = [
        ("未知列表容器返回 projection_unknown", "reportLatestProjectionUnknown" in source and "缺少可识别的列表容器" in source),
        ("显式空数组与未知响应分离", "if len(raw) == 0" in source and "if !known" in source),
        ("非对象条目 fail-closed", "日志列表包含无法识别的条目" in source),
        ("缺 reportId 不再输出原始行成功", "缺少稳定 reportId" in source and "return rt.Output(row)" not in source),
        ("部分时间证据不猜最新项", "timedCount != len(items)" in source),
        ("回归覆盖空/未知/非法/稳定选择", all(token in test_source for token in ("known empty", "unknown container", "missing stable id", "partial time coverage", "SelectsNewestStableEntry"))),
    ]

    env = os.environ.copy()
    env.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-review")
    result = subprocess.run(
        ["go", "test", "-count=1", "./internal/shortcut/smart", "-run", "TestReportLatestProjection", "-v"],
        cwd=ROOT,
        env=env,
        text=True,
        capture_output=True,
    )
    checks.append(("焦点 Go 回归", result.returncode == 0))
    passed = all(ok for _, ok in checks)
    rows = "\n".join(f"| {label} | {'PASS' if ok else 'REVIEW'} |" for label, ok in checks)
    transcript = (result.stdout + result.stderr).strip()

    report = f"""# report +report-latest 投影 — Agent review

扫描时间：{dt.datetime.now().astimezone().isoformat(timespec="seconds")}

> 本扫描只验证本地投影和失败分类，生成 Markdown 证据；不是 CI gate，不保存服务端响应或 JSON fixture。

| 检查 | 结果 |
|---|---|
{rows}

结论：**{'PASS' if passed else 'REVIEW'}**。

`report +report-latest` 仍保留在 Agent exclusion：它与规范的 outbox list + detail 路径重叠，且没有真实账号的稳定详情样本。本轮只关闭“未知响应伪装暂无日志”和“缺 reportId 原始行伪装成功”两项本地投影错误，不将其扩大为公共能力或真实终态证明。

```text
{transcript or '(no output)'}
```
"""
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(report, encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    sys.exit(main())
