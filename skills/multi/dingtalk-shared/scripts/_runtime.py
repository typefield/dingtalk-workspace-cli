#!/usr/bin/env python3
"""Shared result boundary for Multi scripts that declare dingtalk-shared.

The boundary has no product side effects.  It provides one machine writer,
strict child-result classification, and the final exception handler.  Product
scripts still own their workflow and business-result verification.
"""

from __future__ import annotations

import argparse
import contextlib
import io
import json
import subprocess
import sys
from collections.abc import Callable, Iterable, Mapping, Sequence
from dataclasses import dataclass
from typing import Any, Optional


FAILURE_EXIT = 1
PARTIAL_EXIT = 7
_NOT_EXECUTED_TYPES = {"validation", "policy", "confirmation_required", "precondition", "auth", "authorization"}


@dataclass(frozen=True)
class ChildDWSResult:
    state: str
    payload: Any = None
    error: Optional[dict[str, Any]] = None
    meta: Optional[dict[str, Any]] = None
    command: tuple[str, ...] = ()


def _error(payload: Any, fallback: str, *, exit_code: Optional[int] = None) -> dict[str, Any]:
    raw = payload.get("error") if isinstance(payload, Mapping) else None
    raw = raw if isinstance(raw, Mapping) else {}
    result: dict[str, Any] = {"type": str(raw.get("type") or raw.get("category") or "api"), "message": str(raw.get("message") or fallback)}
    for key in ("subtype", "hint", "retryable", "retry_after_seconds"):
        if key in raw:
            result[key] = raw[key]
    if exit_code is not None:
        result["exit_code"] = exit_code
    return result


def run_child_dws(args: Sequence[str], *, dry_run: bool = False, timeout: float = 60, executable: str = "dws") -> ChildDWSResult:
    command = (str(executable), *[str(arg) for arg in args])
    if dry_run:
        return ChildDWSResult("success", payload={"command": list(command), "dry_run": True}, command=command)
    try:
        completed = subprocess.run(list(command), capture_output=True, text=True, timeout=timeout)
    except FileNotFoundError:
        return ChildDWSResult("failed", error={"type": "internal", "message": "未找到 dws 可执行文件。"}, command=command)
    except subprocess.TimeoutExpired:
        return ChildDWSResult("unknown", error={"type": "network", "message": "dws 调用超时；请求是否已执行未知，请先核查目标状态。"}, command=command)
    except OSError as exc:
        return ChildDWSResult("failed", error={"type": "internal", "message": f"无法启动 dws：{type(exc).__name__}"}, command=command)
    try:
        payload = json.loads(completed.stdout) if completed.stdout.strip() else None
    except json.JSONDecodeError:
        payload = None
    meta = dict(payload["meta"]) if isinstance(payload, Mapping) and isinstance(payload.get("meta"), Mapping) else None
    child_outcome = payload.get("outcome") if isinstance(payload, Mapping) else None
    explicitly_failed = isinstance(payload, Mapping) and (
        payload.get("ok") is False or payload.get("success") is False or (isinstance(child_outcome, str) and child_outcome in {"failure", "partial_failure"})
    )
    ambiguous_status = isinstance(payload, Mapping) and (
        any(key in payload and not isinstance(payload[key], bool) for key in ("ok", "success"))
        or (
            "outcome" in payload
            and (
                not isinstance(payload["outcome"], str)
                or payload["outcome"] not in {"success", "pending", "partial_failure", "failure"}
                or payload.get("ok") is not (payload["outcome"] in {"success", "pending"})
            )
        )
    )
    pending = isinstance(payload, Mapping) and payload.get("ok") is True and payload.get("outcome") == "pending"
    if completed.returncode == 0 and payload is not None and not explicitly_failed and not ambiguous_status and not pending:
        return ChildDWSResult("success", payload=payload, meta=meta, command=command)
    if isinstance(payload, Mapping):
        error = (
            {"type": "api", "subtype": "operation_pending", "message": "dws 操作尚未完成；请保留任务信息并按恢复指令继续核查。"}
            if pending
            else {"type": "api", "subtype": "untyped_status", "message": "dws 返回了不一致或无法识别的 ok/outcome 状态，执行结果无法可靠判断。"}
            if ambiguous_status
            else _error(payload, f"dws 未返回终态成功（exit {completed.returncode}）。", exit_code=completed.returncode or None)
        )
        return ChildDWSResult("failed" if error["type"] in _NOT_EXECUTED_TYPES else "unknown", payload=payload, error=error, meta=meta, command=command)
    return ChildDWSResult("unknown", error={"type": "api", "message": "dws 未返回可解析的终态结果；请求是否已执行未知，请先核查目标状态。", "exit_code": completed.returncode}, command=command)


def batch_data(
    *,
    succeeded: Iterable[Mapping[str, Any]] = (),
    failed: Iterable[Mapping[str, Any]] = (),
    unknown: Iterable[Mapping[str, Any]] = (),
    total: Optional[int] = None,
    **extra: Any,
) -> dict[str, Any]:
    """Create the stable three-channel shape for a multi-step write.

    A ``partial_failure`` result is only truthful when it retains the concrete
    entries that did succeed and the entries that failed or became uncertain.
    Callers cannot use this helper to silently drop the latter two channels.
    """
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
    computed = sum(len(entries) for entries in channels.values())
    if total is None:
        total = computed
    if total != computed:
        raise ValueError(f"batch total {total} does not equal channel count {computed}")
    return {"total": total, **channels, **extra}


def batch_outcome(data: Mapping[str, Any]) -> str:
    """Derive the outcome without hiding incomplete or uncertain entries."""
    if not data.get("failed") and not data.get("unknown"):
        return "success"
    return "partial_failure" if data.get("succeeded") else "failure"


def add_contract_flags(parser: argparse.ArgumentParser, *, default: str = "text") -> None:
    parser.add_argument("--format", choices=("text", "json", "ndjson"), default=default, help=f"输出格式：text|json|ndjson（默认 {default}）")
    parser.add_argument("--dry-run", action="store_true", help="生成预览；不得执行远端/本地写入。远端只读探测边界由对应 Skill 的 Agent 扫描台账说明。")


def emit(*, fmt: str, outcome: str, data: Any = None, error: Optional[Mapping[str, Any]] = None, meta: Optional[Mapping[str, Any]] = None, dry_run: bool = False, text: Optional[str] = None) -> int:
    ok = outcome in {"success", "pending"}
    result: dict[str, Any] = {"ok": ok, "outcome": outcome}
    if data is not None:
        result["data"] = data
    if error is not None:
        result["error"] = dict(error)
    if meta is not None:
        result["meta"] = dict(meta)
    if dry_run:
        result["dry_run"] = True
    if fmt == "text":
        print(text if text is not None else json.dumps(result, ensure_ascii=False, indent=2))
    else:
        print(json.dumps(result, ensure_ascii=False, separators=(",", ":")))
    if ok:
        return 0
    return PARTIAL_EXIT if outcome == "partial_failure" else FAILURE_EXIT


def failure(fmt: str, message: str, *, details: Any = None) -> int:
    error: dict[str, Any] = {"type": "validation", "message": message}
    if details is not None:
        error["details"] = details
    return emit(fmt=fmt, outcome="failure", error=error, text=f"错误：{message}")


def _format_from_argv(default: str) -> str:
    args = sys.argv[1:]
    for index, arg in enumerate(args):
        candidate = arg.partition("=")[2] if arg.startswith("--format=") else args[index + 1] if arg == "--format" and index + 1 < len(args) else None
        if candidate in {"text", "json", "ndjson"}:
            return candidate
    return default


def _valid_machine_stdout(value: str, status: int) -> bool:
    lines = [line for line in value.splitlines() if line.strip()]
    if len(lines) != 1:
        return False
    try:
        result = json.loads(lines[0])
    except json.JSONDecodeError:
        return False
    if not isinstance(result, Mapping) or not isinstance(result.get("ok"), bool) or not isinstance(result.get("outcome"), str):
        return False
    if result["outcome"] in {"success", "pending"}:
        return result["ok"] is True and status == 0
    if result["outcome"] == "partial_failure":
        return result["ok"] is False and status == PARTIAL_EXIT
    return result["ok"] is False and result["outcome"] == "failure" and status == FAILURE_EXIT


def run_main(main_fn: Callable[[], Optional[int]], *, default_format: str = "text") -> int:
    fmt = _format_from_argv(default_format)
    captured = io.StringIO()
    try:
        if fmt == "text":
            result = main_fn()
            return 0 if result is None else int(result)
        with contextlib.redirect_stdout(captured):
            result = main_fn()
        status = 0 if result is None else int(result)
        if not _valid_machine_stdout(captured.getvalue(), status):
            print("✗ 脚本输出不符合机器结果契约；已拒绝污染 stdout。", file=sys.stderr)
            return emit(fmt=fmt, outcome="failure", error={"type": "internal", "message": "脚本产生了非契约机器输出；请修复脚本后重试。"})
        sys.stdout.write(captured.getvalue())
        return status
    except KeyboardInterrupt:
        raise
    except SystemExit as exc:
        status = exc.code if isinstance(exc.code, int) else FAILURE_EXIT
        if fmt == "text":
            return status
        if status == 0:
            sys.stdout.write(captured.getvalue())
            return status
        return emit(fmt=fmt, outcome="failure", error={"type": "validation", "message": "脚本参数或前置校验未通过；请检查 --help。", "details": {"exit_code": status}})
    except Exception as exc:  # noqa: BLE001 - intentional executable boundary.
        print(f"✗ 脚本发生未预期错误（{type(exc).__name__}）；已输出可解析结果。", file=sys.stderr)
        return emit(fmt=fmt, outcome="failure", error={"type": "internal", "message": "脚本执行时发生未预期错误；请检查输入类型和格式后重试。", "details": {"exception_type": type(exc).__name__}})
