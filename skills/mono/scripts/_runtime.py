#!/usr/bin/env python3
"""Shared runtime contract for executable mono Skill scripts.

This module is intentionally small and has no network or filesystem side
effects. Individual scripts own their business orchestration; this module only
normalizes flags, stdout/stderr, result envelopes, and exit codes.
"""

from __future__ import annotations

import argparse
import json
import sys
from typing import Any, Iterable, Mapping, Optional, TextIO


PARTIAL_EXIT = 7
FAILURE_EXIT = 1


def add_contract_flags(parser: argparse.ArgumentParser, *, dry_run: bool = True) -> None:
    """Add the stable script-level output flags exactly once."""
    parser.add_argument(
        "--format",
        choices=("text", "json", "ndjson"),
        default="text",
        help="输出格式：text|json|ndjson（默认 text）",
    )
    if dry_run:
        parser.add_argument(
            "--dry-run",
            action="store_true",
            help="生成计划；不得执行远端/本地写入（脚本可按 Help 说明执行只读探测）",
        )


def _envelope(
    *,
    ok: bool,
    outcome: str,
    data: Any = None,
    error: Optional[Mapping[str, Any]] = None,
    dry_run: bool = False,
) -> dict[str, Any]:
    result: dict[str, Any] = {"ok": ok, "outcome": outcome}
    if data is not None:
        result["data"] = data
    if error is not None:
        result["error"] = dict(error)
    if dry_run:
        result["dry_run"] = True
    return result


def emit(
    *,
    fmt: str,
    outcome: str,
    data: Any = None,
    error: Optional[Mapping[str, Any]] = None,
    dry_run: bool = False,
    text: Optional[str] = None,
    items: Optional[Iterable[Mapping[str, Any]]] = None,
    stdout: TextIO = sys.stdout,
) -> int:
    """Emit one script result and return the matching process exit code.

    `ndjson` emits one result object per supplied item and otherwise falls back
    to one line. Diagnostics are deliberately not printed here; callers should
    send them to stderr before invoking this function.
    """
    ok = outcome in ("success", "pending")
    code = 0 if ok else PARTIAL_EXIT if outcome == "partial_failure" else FAILURE_EXIT
    envelope = _envelope(ok=ok, outcome=outcome, data=data, error=error, dry_run=dry_run)

    if fmt == "text":
        if text is not None:
            print(text, file=stdout)
        else:
            print(json.dumps(envelope, ensure_ascii=False, indent=2), file=stdout)
        return code

    if fmt == "ndjson" and items is not None:
        for item in items:
            print(json.dumps(dict(item), ensure_ascii=False, separators=(",", ":")), file=stdout)
        return code

    print(json.dumps(envelope, ensure_ascii=False, separators=(",", ":")), file=stdout)
    return code


def failure(fmt: str, message: str, *, details: Any = None, stdout: TextIO = sys.stdout) -> int:
    error: dict[str, Any] = {"type": "validation", "message": message}
    if details is not None:
        error["details"] = details
    return emit(fmt=fmt, outcome="failure", error=error, text=f"错误：{message}", stdout=stdout)
