#!/usr/bin/env python3
"""Portable result boundary for Multi AITable executable scripts.

This module intentionally owns only the process boundary: common flags,
machine envelopes, child-DWS result classification, and the last exception
handler before an Agent consumes stdout.  It has no business side effects;
individual scripts remain responsible for their product workflow.
"""

from __future__ import annotations

import argparse
import contextlib
import io
import json
import subprocess
import sys
from collections.abc import Callable, Iterable, Sequence
from dataclasses import dataclass
from typing import Any, Mapping, Optional, TextIO


PARTIAL_EXIT = 7
FAILURE_EXIT = 1


@dataclass(frozen=True)
class ChildDWSResult:
    """A child call's known terminal state, without inventing write safety."""

    state: str
    payload: Any = None
    error: Optional[dict[str, Any]] = None
    meta: Optional[dict[str, Any]] = None
    command: tuple[str, ...] = ()


_DEFINITELY_NOT_EXECUTED = {
    "validation", "policy", "confirmation_required", "precondition", "auth", "authorization",
}


def _child_error(payload: Any, fallback: str, *, exit_code: Optional[int] = None) -> dict[str, Any]:
    candidate = payload.get("error") if isinstance(payload, Mapping) else None
    if not isinstance(candidate, Mapping):
        candidate = {}
    error: dict[str, Any] = {
        "type": str(candidate.get("type") or candidate.get("category") or "api"),
        "message": str(candidate.get("message") or fallback),
    }
    for key in ("subtype", "hint", "retryable", "retry_after_seconds"):
        if key in candidate:
            error[key] = candidate[key]
    if exit_code is not None:
        error["exit_code"] = exit_code
    return error


def _child_explicitly_failed(payload: Any) -> bool:
    """Only boolean failure flags are stable execution facts.

    In particular, legacy ``success: \"false\"`` must not become a Python
    truthiness decision: that would recreate the string-boolean drift this
    boundary is meant to contain.
    """
    outcome = payload.get("outcome") if isinstance(payload, Mapping) else None
    return (
        isinstance(payload, Mapping)
        and (
            payload.get("ok") is False
            or payload.get("success") is False
            or (isinstance(outcome, str) and outcome in {"failure", "partial_failure"})
        )
    )


def _child_status_is_ambiguous(payload: Any) -> bool:
    """Reject malformed unified status while allowing legacy bare payloads."""
    if not isinstance(payload, Mapping):
        return False
    if any(key in payload and not isinstance(payload[key], bool) for key in ("ok", "success")):
        return True
    if "outcome" not in payload:
        return False
    outcome = payload["outcome"]
    if not isinstance(outcome, str) or outcome not in {"success", "pending", "partial_failure", "failure"}:
        return True
    return payload.get("ok") is not (outcome in {"success", "pending"})


def _child_is_pending(payload: Any) -> bool:
    return (
        isinstance(payload, Mapping)
        and payload.get("ok") is True
        and payload.get("outcome") == "pending"
    )


def _meta_from_child(payload: Any) -> Optional[dict[str, Any]]:
    if isinstance(payload, Mapping) and isinstance(payload.get("meta"), Mapping):
        return dict(payload["meta"])
    return None


def run_child_dws(
    args: Sequence[str], *, dry_run: bool = False, timeout: float = 60, executable: str = "dws",
) -> ChildDWSResult:
    """Run a JSON-mode child command while preserving execution uncertainty.

    A timeout, malformed response, or nonterminal server failure can occur
    after a write reached DingTalk.  Those cases are therefore ``unknown``;
    only stable pre-execution failure classes are ``failed``.
    """
    command = (str(executable), *[str(arg) for arg in args])
    if dry_run:
        return ChildDWSResult(
            "success", payload={"command": list(command), "dry_run": True}, command=command,
        )
    try:
        completed = subprocess.run(list(command), capture_output=True, text=True, timeout=timeout)
    except FileNotFoundError:
        return ChildDWSResult("failed", error={"type": "internal", "message": "未找到 dws 可执行文件。"}, command=command)
    except subprocess.TimeoutExpired:
        return ChildDWSResult(
            "unknown",
            error={"type": "network", "message": "dws 调用超时；请求是否已执行未知，请先核查目标状态。"},
            command=command,
        )
    except OSError as exc:
        return ChildDWSResult(
            "failed",
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
    if completed.returncode == 0 and decoded and not _child_explicitly_failed(payload) and not _child_status_is_ambiguous(payload):
        if _child_is_pending(payload):
            return ChildDWSResult(
                "unknown", payload=payload,
                error={"type": "api", "subtype": "operation_pending", "message": "dws 操作尚未完成；请保留任务信息并按恢复指令继续核查。"},
                meta=meta, command=command,
            )
        return ChildDWSResult("success", payload=payload, meta=meta, command=command)
    if decoded and isinstance(payload, Mapping):
        error = (
            {"type": "api", "subtype": "operation_pending", "message": "dws 操作尚未完成；请保留任务信息并按恢复指令继续核查。"}
            if _child_is_pending(payload)
            else {"type": "api", "subtype": "untyped_status", "message": "dws 返回了不一致或无法识别的 ok/outcome 状态，执行结果无法可靠判断。"}
            if _child_status_is_ambiguous(payload)
            else _child_error(payload, f"dws 未返回终态成功（exit {completed.returncode}）。", exit_code=completed.returncode or None)
        )
        state = "failed" if error["type"] in _DEFINITELY_NOT_EXECUTED else "unknown"
        return ChildDWSResult(state, payload=payload, error=error, meta=meta, command=command)
    return ChildDWSResult(
        "unknown",
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
    """Build the strict three-channel form for a non-atomic batch write."""
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
    """Derive the only truthful terminal outcome for a batch result."""
    if not data.get("failed") and not data.get("unknown"):
        return "success"
    return "partial_failure" if data.get("succeeded") else "failure"


def add_contract_flags(parser: argparse.ArgumentParser, *, default: str = "text") -> None:
    """Install the one stable script output surface."""
    if default not in {"text", "json", "ndjson"}:
        raise ValueError(f"unsupported contract default: {default}")
    parser.add_argument(
        "--format",
        choices=("text", "json", "ndjson"),
        default=default,
        help=f"输出格式：text|json|ndjson（默认 {default}）",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="生成预览；不得执行远端/本地写入。远端只读探测边界由对应 Skill 的 Agent 扫描台账说明。",
    )


def _envelope(
    *, ok: bool, outcome: str, data: Any = None, error: Optional[Mapping[str, Any]] = None,
    meta: Optional[Mapping[str, Any]] = None, dry_run: bool = False,
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
    *, fmt: str, outcome: str, data: Any = None, error: Optional[Mapping[str, Any]] = None,
    meta: Optional[Mapping[str, Any]] = None, dry_run: bool = False, text: Optional[str] = None,
    stdout: Optional[TextIO] = None,
) -> int:
    """Emit exactly one terminal envelope in machine modes."""
    output = sys.stdout if stdout is None else stdout
    code = 0 if outcome in {"success", "pending"} else PARTIAL_EXIT if outcome == "partial_failure" else FAILURE_EXIT
    envelope = _envelope(ok=code == 0, outcome=outcome, data=data, error=error, meta=meta, dry_run=dry_run)
    if fmt == "text" and text is not None:
        print(text, file=output)
    else:
        print(json.dumps(envelope, ensure_ascii=False, separators=(",", ":")) if fmt != "text" else json.dumps(envelope, ensure_ascii=False, indent=2), file=output)
    return code


def failure(
    fmt: str, message: str, *, details: Any = None, meta: Optional[Mapping[str, Any]] = None,
    stdout: Optional[TextIO] = None,
) -> int:
    error: dict[str, Any] = {"type": "validation", "message": message}
    if details is not None:
        error["details"] = details
    return emit(fmt=fmt, outcome="failure", error=error, meta=meta, text=f"错误：{message}", stdout=stdout)


def _requested_format(argv: Optional[Sequence[str]] = None, *, default: str = "text") -> str:
    if default not in {"text", "json", "ndjson"}:
        raise ValueError(f"unsupported contract default: {default}")
    args = list(sys.argv[1:] if argv is None else argv)
    for index, arg in enumerate(args):
        candidate = arg.partition("=")[2] if arg.startswith("--format=") else args[index + 1] if arg == "--format" and index + 1 < len(args) else None
        if candidate in {"text", "json", "ndjson"}:
            return candidate
    return default


def _exit_status(value: object) -> int:
    return 0 if value is None else value if isinstance(value, int) else FAILURE_EXIT


def _machine_stdout_is_contract(fmt: str, captured: str, status: int) -> bool:
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
            or ("meta" in payload and not isinstance(payload["meta"], Mapping))
            or ("dry_run" in payload and not isinstance(payload["dry_run"], bool))
        ):
            return False
        if payload["outcome"] == "failure":
            error = payload.get("error")
            if not isinstance(error, Mapping) or not isinstance(error.get("type"), str) or not error["type"]:
                return False
        return (
            (payload["outcome"] in {"success", "pending"} and payload["ok"] is True and status == 0)
            or (payload["outcome"] == "partial_failure" and payload["ok"] is False and status == PARTIAL_EXIT)
            or (payload["outcome"] == "failure" and payload["ok"] is False and status == FAILURE_EXIT)
        )
    if fmt == "ndjson":
        try:
            return all(isinstance(json.loads(line), Mapping) for line in lines)
        except json.JSONDecodeError:
            return False
    return True


def run_main(
    main_fn: Callable[[], Optional[int]], *, argv: Optional[Sequence[str]] = None,
    stdout: Optional[TextIO] = None, stderr: Optional[TextIO] = None, default_format: str = "text",
) -> int:
    """Contain unhandled exceptions so JSON-mode Agents never receive a traceback."""
    output = sys.stdout if stdout is None else stdout
    diagnostics = sys.stderr if stderr is None else stderr
    fmt = _requested_format(argv, default=default_format)
    captured = io.StringIO()
    try:
        if fmt == "text":
            return _exit_status(main_fn())
        with contextlib.redirect_stdout(captured):
            status = _exit_status(main_fn())
        machine_stdout = captured.getvalue()
        if not _machine_stdout_is_contract(fmt, machine_stdout, status):
            print("✗ 脚本输出不符合机器结果契约；已拒绝污染或退出码不一致的 stdout。", file=diagnostics)
            return emit(
                fmt=fmt, outcome="failure",
                error={"type": "internal", "message": "脚本产生了非契约机器输出；请修复脚本后重试。", "details": {"violation": "machine_stdout_contract"}},
                text="错误：脚本产生了非契约机器输出；请修复脚本后重试。", stdout=output,
            )
        output.write(machine_stdout)
        return status
    except KeyboardInterrupt:
        raise
    except SystemExit as exc:
        status = _exit_status(exc.code)
        if status == 0:
            if fmt != "text":
                output.write(captured.getvalue())
            return status
        if fmt == "text":
            return status
        return emit(
            fmt=fmt, outcome="failure",
            error={"type": "validation", "message": "脚本参数或前置校验未通过；请检查 stderr 和 --help。", "details": {"exit_code": status}},
            stdout=output,
        )
    except Exception as exc:  # noqa: BLE001 - intentional executable boundary.
        print(f"✗ 脚本发生未预期错误（{type(exc).__name__}）；已输出可解析结果。", file=diagnostics)
        return emit(
            fmt=fmt, outcome="failure",
            error={"type": "internal", "message": "脚本执行时发生未预期错误；请检查输入类型和格式后重试。", "details": {"exception_type": type(exc).__name__}},
            text="错误：脚本执行时发生未预期错误；请检查输入类型和格式后重试。", stdout=output,
        )
