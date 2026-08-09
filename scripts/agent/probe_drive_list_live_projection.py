#!/usr/bin/env python3
"""Collect redacted live evidence for the active Drive directory result.

The probe performs read-only list calls, follows at most one continuation, and
writes Markdown only. File names, IDs, cursors, raw envelopes and JSON fixtures
are never persisted.
"""

from __future__ import annotations

import argparse
from datetime import datetime
import json
from pathlib import Path
import subprocess
from typing import Any


def call(dws: Path, *args: str) -> tuple[subprocess.CompletedProcess[str], dict[str, Any] | None]:
    proc = subprocess.run(
        [str(dws), "drive", "+list", *args, "--format", "json"],
        capture_output=True,
        text=True,
        timeout=30,
        check=False,
    )
    try:
        payload = json.loads(proc.stdout)
    except json.JSONDecodeError:
        payload = None
    return proc, payload if isinstance(payload, dict) else None


def inspect(payload: dict[str, Any] | None) -> tuple[bool, dict[str, Any]]:
    if payload is None:
        return False, {"reason": "stdout is not one JSON object"}
    data = payload.get("data")
    meta = payload.get("meta")
    if not isinstance(data, dict) or not isinstance(meta, dict):
        return False, {"reason": "missing data/meta object"}
    files = data.get("files")
    if not isinstance(files, list):
        return False, {"reason": "data.files is not an array"}
    stable = all(
        isinstance(item, dict)
        and isinstance(item.get("dentryId"), str)
        and bool(item["dentryId"].strip())
        for item in files
    )
    count = data.get("count")
    meta_count = meta.get("count")
    page = meta.get("pagination")
    token = page.get("next_token") if isinstance(page, dict) else None
    summary = {
        "count": count,
        "meta_count": meta_count,
        "stable_ids": stable,
        "scope": data.get("inventory_scope"),
        "pagination_known": data.get("pagination_known"),
        "pagination_present": isinstance(page, dict),
        "endpoint_exhausted": page.get("endpoint_exhausted") if isinstance(page, dict) else None,
        "continuation": isinstance(token, str) and bool(token.strip()),
        "token": token,
    }
    ok = (
        payload.get("ok") is True
        and payload.get("outcome") == "success"
        and "contract_version" not in payload
        and isinstance(count, int)
        and count == len(files)
        and meta_count == count
        and stable
        and data.get("inventory_scope") == "requested_location"
        and not any(key in data for key in ("hasMore", "nextCursor", "nextToken", "complete"))
    )
    return ok, summary


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dws", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()

    first_proc, first = call(args.dws, "--limit", "1")
    first_ok, first_summary = inspect(first)
    token = first_summary.pop("token", None)
    continuation_ok = (
        first_ok
        and first_proc.returncode == 0
        and first_proc.stderr == ""
        and first_summary["pagination_known"] is True
        and first_summary["pagination_present"] is True
        and first_summary["endpoint_exhausted"] is False
        and first_summary["continuation"] is True
        and isinstance(token, str)
    )

    second_summary: dict[str, Any] = {"reason": "first page had no continuation"}
    second_ok = False
    if continuation_ok:
        second_proc, second = call(args.dws, "--limit", "1", "--cursor", token)
        second_ok, second_summary = inspect(second)
        second_summary.pop("token", None)
        second_ok = second_ok and second_proc.returncode == 0 and second_proc.stderr == ""

    wide_proc, wide = call(args.dws, "--limit", "50")
    wide_ok, wide_summary = inspect(wide)
    wide_summary.pop("token", None)
    wide_ok = wide_ok and wide_proc.returncode == 0 and wide_proc.stderr == ""
    # A token proves continuation. Without a token/hasMore fact, the active
    # result must remain pagination_unknown rather than inventing exhaustion.
    terminal_truthful = wide_ok and (
        wide_summary["continuation"] is True
        or (
            wide_summary["pagination_present"] is False
            and wide_summary["pagination_known"] is False
        )
    )

    checks = [
        ("首屏统一信封与 scoped inventory", first_ok and first_proc.returncode == 0 and first_proc.stderr == ""),
        ("token-only continuation 可恢复", continuation_ok),
        ("续页仍保持统一投影", second_ok),
        ("无终态证据不伪造 endpoint exhausted", terminal_truthful),
        ("结果不含协议版本或 legacy 分页键", first_ok and wide_ok),
    ]
    passed = all(value for _, value in checks)
    rows = "\n".join(f"| {name} | {'PASS' if value else 'REVIEW'} |" for name, value in checks)
    report = f"""# Drive `+list` live projection — Agent evidence

扫描时间：{datetime.now().astimezone().isoformat(timespec="seconds")}

> 当前源码构建二进制执行只读目录列表。报告只保存计数、类型和分页状态；不保存文件名、dentry ID、cursor、原始 JSON 或账号信息。

## Result: {"PASS" if passed else "REVIEW"}

| 检查项 | 结果 |
|---|---|
{rows}

## Redacted observations

- 首屏：count={first_summary.get('count')}, stable_ids={first_summary.get('stable_ids')}, scope={first_summary.get('scope')}, continuation={first_summary.get('continuation')}。
- 续页：count={second_summary.get('count')}, stable_ids={second_summary.get('stable_ids')}, scope={second_summary.get('scope')}。
- 宽页：count={wide_summary.get('count')}, pagination_known={wide_summary.get('pagination_known')}, pagination_meta_present={wide_summary.get('pagination_present')}。

## Boundary

- 非空 token 只证明当前 endpoint 可续页；它不证明租户 Drive 全量目录、索引健康或权限覆盖。
- token 缺失且没有显式终态布尔时保持 `pagination_known:false`，不把当前页包装成 endpoint exhausted。
- 本证据不注入服务端异常，未知容器、非法条目、矛盾 `hasMore/token` 仍由本地 fixture 回归证明 fail-closed。
"""
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(report, encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
