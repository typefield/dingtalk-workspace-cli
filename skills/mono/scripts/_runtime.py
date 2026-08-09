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
import subprocess
import sys
from collections.abc import Callable, Sequence
from dataclasses import dataclass
from typing import Any, Iterable, Mapping, Optional, TextIO


PARTIAL_EXIT = 7
FAILURE_EXIT = 1


@dataclass(frozen=True)
class ChildDWSResult:
    """The only three truthful states a script can infer from a child dws call.

    ``failed`` means the child supplied a pre-execution failure (for example a
    local validation or confirmation gate).  ``unknown`` is deliberately the
    conservative default for a request that may have reached DingTalk but did
    not produce a terminal success response.  A batch caller must preserve
    that distinction instead of converting both cases into a boolean.
    """

    state: str
    payload: Any = None
    error: Optional[dict[str, Any]] = None
    meta: Optional[dict[str, Any]] = None
    command: tuple[str, ...] = ()


_DEFINITELY_NOT_EXECUTED = {
    "validation",
    "policy",
    "confirmation_required",
    "precondition",
    "auth",
    "authorization",
}


def _child_error(payload: Any, fallback: str, *, exit_code: Optional[int] = None) -> dict[str, Any]:
    """Project a child error without inventing a terminal business result."""
    candidate: Any = payload.get("error") if isinstance(payload, Mapping) else None
    if not isinstance(candidate, Mapping):
        candidate = {}
    error_type = candidate.get("type") or candidate.get("category") or "api"
    message = candidate.get("message") or fallback
    error: dict[str, Any] = {
        "type": str(error_type),
        "message": str(message),
    }
    for key in ("subtype", "hint", "retryable", "retry_after_seconds"):
        if key in candidate:
            error[key] = candidate[key]
    if exit_code is not None:
        error["exit_code"] = exit_code
    return error


def _meta_from_child(payload: Any) -> Optional[dict[str, Any]]:
    if isinstance(payload, Mapping) and isinstance(payload.get("meta"), Mapping):
        return dict(payload["meta"])
    return None


def _child_explicitly_failed(payload: Any) -> bool:
    """Recognize only typed, unambiguous failure declarations from a child.

    Older DWS payloads may use ``success`` while the unified envelope uses
    ``ok``.  Both are accepted only when they are actual booleans: the string
    ``"false"`` is data-quality drift, not a safe execution fact.  Treating
    that string as a Python truthy/falsy shortcut would recreate the exact
    cross-language ambiguity this runtime is meant to prevent.
    """
    if not isinstance(payload, Mapping):
        return False
    outcome = payload.get("outcome")
    return (
        payload.get("ok") is False
        or payload.get("success") is False
        or (isinstance(outcome, str) and outcome in {"failure", "partial_failure"})
    )


def _child_status_is_ambiguous(payload: Any) -> bool:
    """Reject malformed unified status without breaking legacy bare payloads."""
    if not isinstance(payload, Mapping):
        return False
    if any(key in payload and not isinstance(payload[key], bool) for key in ("ok", "success")):
        return True
    # Older commands can still return bare business JSON. Once a response
    # declares the unified outcome, however, it must carry the matching `ok`
    # boolean. Otherwise a child could claim success with outcome=failure.
    if "outcome" not in payload:
        return False
    outcome = payload["outcome"]
    if not isinstance(outcome, str) or outcome not in {"success", "pending", "partial_failure", "failure"}:
        return True
    expected_ok = outcome in {"success", "pending"}
    return payload.get("ok") is not expected_ok


def run_child_dws(
    args: Sequence[str],
    *,
    dry_run: bool = False,
    timeout: float = 60,
    executable: str = "dws",
) -> ChildDWSResult:
    """Run one JSON-mode child dws command without erasing execution certainty.

    Callers must include ``--format json`` in ``args``.  A non-zero child exit,
    timeout, malformed JSON, or untyped upstream failure is *not* evidence
    that a write did not happen, so it becomes ``unknown``.  Only stable
    pre-execution error classes become ``failed``.  No child stderr is copied
    into the result envelope because it is diagnostic text, not wire data.
    """
    command = (str(executable), *[str(arg) for arg in args])
    if dry_run:
        return ChildDWSResult(
            state="success",
            payload={"command": list(command), "dry_run": True},
            command=command,
        )
    try:
        completed = subprocess.run(
            list(command), capture_output=True, text=True, timeout=timeout,
        )
    except FileNotFoundError:
        return ChildDWSResult(
            state="failed",
            error={"type": "internal", "message": "未找到 dws 可执行文件。"},
            command=command,
        )
    except subprocess.TimeoutExpired:
        return ChildDWSResult(
            state="unknown",
            error={
                "type": "network",
                "message": "dws 调用超时；请求是否已执行未知，请先核查目标状态。",
            },
            command=command,
        )
    except OSError as exc:
        return ChildDWSResult(
            state="failed",
            error={"type": "internal", "message": f"无法启动 dws：{type(exc).__name__}"},
            command=command,
        )

    payload: Any = None
    decoded = False
    if completed.stdout.strip():
        try:
            payload = json.loads(completed.stdout)
            decoded = True
        except json.JSONDecodeError:
            pass
    meta = _meta_from_child(payload)
    if completed.returncode == 0 and decoded:
        if not _child_explicitly_failed(payload) and not _child_status_is_ambiguous(payload):
            return ChildDWSResult("success", payload=payload, meta=meta, command=command)

    if decoded and isinstance(payload, Mapping):
        if _child_status_is_ambiguous(payload):
            error = {
                "type": "api",
                "subtype": "untyped_status",
                "message": "dws 返回了不一致或无法识别的 ok/outcome 状态，执行结果无法可靠判断。",
            }
        else:
            error = _child_error(
                payload,
                f"dws 未返回终态成功（exit {completed.returncode}）。",
                exit_code=completed.returncode or None,
            )
        state = "failed" if error["type"] in _DEFINITELY_NOT_EXECUTED else "unknown"
        return ChildDWSResult(state, payload=payload, error=error, meta=meta, command=command)

    return ChildDWSResult(
        state="unknown",
        error={
            "type": "api",
            "message": "dws 未返回可解析的终态结果；请求是否已执行未知，请先核查目标状态。",
            "exit_code": completed.returncode,
        },
        command=command,
    )


def batch_data(
    *,
    succeeded: Iterable[Mapping[str, Any]] = (),
    failed: Iterable[Mapping[str, Any]] = (),
    unknown: Iterable[Mapping[str, Any]] = (),
    total: Optional[int] = None,
    **extra: Any,
) -> dict[str, Any]:
    """Build the portable three-channel batch shape used by partial results."""
    channels = {
        "succeeded": [dict(item) for item in succeeded],
        "failed": [dict(item) for item in failed],
        "unknown": [dict(item) for item in unknown],
    }
    for channel, entries in channels.items():
        for entry in entries:
            if not isinstance(entry.get("id"), str) or not entry["id"]:
                raise ValueError(f"{channel} entry requires a non-empty id")
            if channel == "failed":
                error = entry.get("error")
                if not isinstance(error, Mapping) or not isinstance(error.get("type"), str) or not error["type"]:
                    raise ValueError("failed entry requires a typed error")
            if channel == "unknown" and (not isinstance(entry.get("reason"), str) or not entry["reason"]):
                raise ValueError("unknown entry requires a reason")
    computed_total = sum(len(entries) for entries in channels.values())
    if total is None:
        total = computed_total
    if total != computed_total:
        raise ValueError(f"batch total {total} does not equal channel count {computed_total}")
    return {"total": total, **channels, **extra}


def batch_outcome(data: Mapping[str, Any]) -> str:
    """Derive the terminal outcome from a ``batch_data`` result."""
    succeeded = data.get("succeeded") or []
    failed = data.get("failed") or []
    unknown = data.get("unknown") or []
    if not failed and not unknown:
        return "success"
    return "partial_failure" if succeeded else "failure"


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
            help="生成预览；不得执行远端/本地写入。远端只读探测边界由对应 Skill 的 Agent 扫描台账说明。",
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


def _machine_stdout_is_contract(fmt: str, captured: str, status: int) -> bool:
    """Return whether buffered machine stdout can be safely passed through.

    ``run_main`` is the last common boundary before a script writes its result
    to an Agent.  A stray ``print()`` from any business helper would otherwise
    turn a nominal JSON success into an unparsable multi-line stream.  Do not
    try to recover or reinterpret that text: a clean typed failure is more
    truthful than a corrupt success result.

    JSON mode has one terminal envelope and its ``ok/outcome`` must agree with
    the process exit code. NDJSON is allowed to have no lines for an empty
    stream, but every non-empty line must still be an object. The detailed
    per-item schema remains the responsibility of the script's business
    contract.
    """
    lines = [line for line in captured.splitlines() if line.strip()]
    if fmt == "json":
        if len(lines) != 1:
            return False
        try:
            payload = json.loads(lines[0])
        except json.JSONDecodeError:
            return False
        if (
            not isinstance(payload, Mapping)
            or not isinstance(payload.get("ok"), bool)
            or not isinstance(payload.get("outcome"), str)
        ):
            return False
        outcome = payload["outcome"]
        if outcome in {"success", "pending"}:
            return payload["ok"] is True and status == 0
        if outcome == "partial_failure":
            return payload["ok"] is False and status == PARTIAL_EXIT
        if outcome == "failure":
            return payload["ok"] is False and status == FAILURE_EXIT
        return False
    if fmt == "ndjson":
        for line in lines:
            try:
                if not isinstance(json.loads(line), Mapping):
                    return False
            except json.JSONDecodeError:
                return False
        return True
    return True


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
        captured = buffered_machine_stdout.getvalue()
        if not _machine_stdout_is_contract(fmt, captured, status):
            # Never forward the captured text: it may contain partially
            # rendered data or sensitive input. Keep a concise diagnostic on
            # stderr and replace the whole result with one typed envelope.
            print(
                "✗ 脚本输出不符合机器结果契约；已拒绝污染或退出码不一致的 stdout。",
                file=diagnostics,
            )
            return emit(
                fmt=fmt,
                outcome="failure",
                error={
                    "type": "internal",
                    "message": "脚本产生了非契约机器输出；请修复脚本后重试。",
                    "details": {"violation": "machine_stdout_contract"},
                },
                text="错误：脚本产生了非契约机器输出；请修复脚本后重试。",
                stdout=output,
            )
        output.write(captured)
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
