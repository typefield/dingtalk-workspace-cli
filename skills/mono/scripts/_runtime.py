#!/usr/bin/env python3
"""Shared runtime contract for executable mono Skill scripts.

This module is intentionally small and has no network or filesystem side
effects. Individual scripts own their business orchestration; this module only
normalizes flags, stdout/stderr, result envelopes, and exit codes.
"""

from __future__ import annotations

import argparse
import contextlib
import io
import json
import sys
from collections.abc import Callable, Sequence
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
            help="生成预览；不得执行远端/本地写入。是否包含远端只读探测以对应 Skill 的 dry-run 说明为准。",
        )


def _envelope(
    *,
    ok: bool,
    outcome: str,
    data: Any = None,
    error: Optional[Mapping[str, Any]] = None,
    meta: Optional[Mapping[str, Any]] = None,
    dry_run: bool = False,
) -> dict[str, Any]:
    result: dict[str, Any] = {"ok": ok, "outcome": outcome}
    if data is not None:
        result["data"] = data
    if error is not None:
        result["error"] = dict(error)
    if meta is not None:
        result["meta"] = dict(meta)
    if dry_run:
        result["dry_run"] = True
    return result


def emit(
    *,
    fmt: str,
    outcome: str,
    data: Any = None,
    error: Optional[Mapping[str, Any]] = None,
    meta: Optional[Mapping[str, Any]] = None,
    dry_run: bool = False,
    text: Optional[str] = None,
    items: Optional[Iterable[Mapping[str, Any]]] = None,
    stdout: Optional[TextIO] = None,
) -> int:
    """Emit one script result and return the matching process exit code.

    `ndjson` emits one result object per supplied item and otherwise falls back
    to one line. Diagnostics are deliberately not printed here; callers should
    send them to stderr before invoking this function.
    """
    output = sys.stdout if stdout is None else stdout
    ok = outcome in ("success", "pending")
    code = 0 if ok else PARTIAL_EXIT if outcome == "partial_failure" else FAILURE_EXIT
    envelope = _envelope(
        ok=ok,
        outcome=outcome,
        data=data,
        error=error,
        meta=meta,
        dry_run=dry_run,
    )

    if fmt == "text":
        if text is not None:
            print(text, file=output)
        else:
            print(json.dumps(envelope, ensure_ascii=False, indent=2), file=output)
        return code

    if fmt == "ndjson" and items is not None:
        for item in items:
            print(json.dumps(dict(item), ensure_ascii=False, separators=(",", ":")), file=output)
        return code

    print(json.dumps(envelope, ensure_ascii=False, separators=(",", ":")), file=output)
    return code


def failure(
    fmt: str,
    message: str,
    *,
    details: Any = None,
    meta: Optional[Mapping[str, Any]] = None,
    stdout: Optional[TextIO] = None,
) -> int:
    error: dict[str, Any] = {"type": "validation", "message": message}
    if details is not None:
        error["details"] = details
    return emit(
        fmt=fmt,
        outcome="failure",
        error=error,
        meta=meta,
        text=f"错误：{message}",
        stdout=stdout,
    )


def _requested_format(argv: Optional[Sequence[str]] = None) -> str:
    """Read a valid requested machine format without depending on argparse state."""
    args = list(sys.argv[1:] if argv is None else argv)
    for index, arg in enumerate(args):
        if arg.startswith("--format="):
            candidate = arg.partition("=")[2]
        elif arg == "--format" and index + 1 < len(args):
            candidate = args[index + 1]
        else:
            continue
        if candidate in {"text", "json", "ndjson"}:
            return candidate
    return "text"


def _exit_status(value: object) -> int:
    """Mirror sys.exit's integer outcome while keeping the runtime total."""
    if value is None:
        return 0
    if isinstance(value, int):
        return value
    return FAILURE_EXIT


def run_main(
    main_fn: Callable[[], Optional[int]],
    *,
    argv: Optional[Sequence[str]] = None,
    stdout: Optional[TextIO] = None,
    stderr: Optional[TextIO] = None,
) -> int:
    """Run an executable Skill entrypoint without allowing a traceback to break JSON.

    The wrapper is intentionally an entrypoint concern: individual scripts still
    own validation and business errors.  It only converts otherwise-unhandled
    failures into one stable machine result.  Normal ``--help`` exits retain
    argparse's native behavior, and text-mode failures preserve the existing
    command-line experience rather than inventing a second text protocol.
    """
    output = sys.stdout if stdout is None else stdout
    diagnostics = sys.stderr if stderr is None else stderr
    fmt = _requested_format(argv)
    buffered_machine_stdout = io.StringIO()
    try:
        if fmt == "text":
            return _exit_status(main_fn())
        with contextlib.redirect_stdout(buffered_machine_stdout):
            status = _exit_status(main_fn())
        output.write(buffered_machine_stdout.getvalue())
        return status
    except KeyboardInterrupt:
        raise
    except SystemExit as exc:
        status = _exit_status(exc.code)
        if status == 0:
            if fmt != "text":
                output.write(buffered_machine_stdout.getvalue())
            return status
        if fmt == "text":
            return status
        return emit(
            fmt=fmt,
            outcome="failure",
            error={
                "type": "validation",
                "message": "脚本参数或前置校验未通过；请检查 stderr 和 --help。",
                "details": {"exit_code": status},
            },
            stdout=output,
        )
    except Exception as exc:  # noqa: BLE001 - this is the intentional protocol boundary.
        print(
            f"✗ 脚本发生未预期错误（{type(exc).__name__}）；已输出可解析结果。",
            file=diagnostics,
        )
        return emit(
            fmt=fmt,
            outcome="failure",
            error={
                "type": "internal",
                "message": "脚本执行时发生未预期错误；请检查输入类型和格式后重试。",
                "details": {"exception_type": type(exc).__name__},
            },
            text="错误：脚本执行时发生未预期错误；请检查输入类型和格式后重试。",
            stdout=output,
        )
