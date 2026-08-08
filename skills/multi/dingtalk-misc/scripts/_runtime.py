#!/usr/bin/env python3
"""Portable machine-result boundary for Multi Misc executable scripts.

Each installable Skill keeps this small boundary locally.  It has no product
side effects: it only owns stable output flags, child-DWS classification,
batch result channels, and the last exception boundary before Agent stdout.
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
from typing import Any, Optional, TextIO


PARTIAL_EXIT = 7
FAILURE_EXIT = 1
_NOT_EXECUTED_TYPES = {"validation", "policy", "confirmation_required", "precondition", "auth", "authorization"}


@dataclass(frozen=True)
class ChildDWSResult:
    state: str
    payload: Any = None
    error: Optional[dict[str, Any]] = None
    meta: Optional[dict[str, Any]] = None
    command: tuple[str, ...] = ()


def _error(payload: Any, fallback: str, *, exit_code: Optional[int] = None) -> dict[str, Any]:
    candidate = payload.get("error") if isinstance(payload, Mapping) else None
    candidate = candidate if isinstance(candidate, Mapping) else {}
    result: dict[str, Any] = {"type": str(candidate.get("type") or candidate.get("category") or "api"), "message": str(candidate.get("message") or fallback)}
    for key in ("subtype", "hint", "retryable", "retry_after_seconds"):
        if key in candidate:
            result[key] = candidate[key]
    if exit_code is not None:
        result["exit_code"] = exit_code
    return result


def run_child_dws(
    args: Sequence[str], *, dry_run: bool = False, timeout: float = 60, executable: str = "dws",
) -> ChildDWSResult:
    """Classify only proven pre-execution failures as safe failures."""
    command = (str(executable), *[str(arg) for arg in args])
    if dry_run:
        return ChildDWSResult("success", payload={"command": list(command), "dry_run": True}, command=command)
    try:
        completed = subprocess.run(list(command), capture_output=True, text=True, timeout=timeout)
    except FileNotFoundError:
        return ChildDWSResult("failed", error={"type": "internal", "message": "未找到 dws 可执行文件。"}, command=command)
    except subprocess.TimeoutExpired:
        return ChildDWSResult("unknown", error={"type": "network", "message": "dws 调用超时；请求是否已执行未知，请先核查审批状态。"}, command=command)
    except OSError as exc:
        return ChildDWSResult("failed", error={"type": "internal", "message": f"无法启动 dws：{type(exc).__name__}"}, command=command)
    try:
        payload = json.loads(completed.stdout) if completed.stdout.strip() else None
    except json.JSONDecodeError:
        payload = None
    meta = dict(payload["meta"]) if isinstance(payload, Mapping) and isinstance(payload.get("meta"), Mapping) else None
    explicitly_failed = isinstance(payload, Mapping) and (payload.get("ok") is False or payload.get("success") is False)
    if completed.returncode == 0 and payload is not None and not explicitly_failed:
        return ChildDWSResult("success", payload=payload, meta=meta, command=command)
    if isinstance(payload, Mapping):
        error = _error(payload, f"dws 未返回终态成功（exit {completed.returncode}）。", exit_code=completed.returncode or None)
        return ChildDWSResult("failed" if error["type"] in _NOT_EXECUTED_TYPES else "unknown", payload=payload, error=error, meta=meta, command=command)
    return ChildDWSResult("unknown", error={"type": "api", "message": "dws 未返回可解析的终态结果；请求是否已执行未知，请先核查审批状态。", "exit_code": completed.returncode}, command=command)


def batch_data(
    *, succeeded: Iterable[Mapping[str, Any]] = (), failed: Iterable[Mapping[str, Any]] = (),
    unknown: Iterable[Mapping[str, Any]] = (), total: Optional[int] = None, **extra: Any,
) -> dict[str, Any]:
    channels = {"succeeded": [dict(item) for item in succeeded], "failed": [dict(item) for item in failed], "unknown": [dict(item) for item in unknown]}
    for channel, entries in channels.items():
        for item in entries:
            if not isinstance(item.get("id"), str) or not item["id"]:
                raise ValueError(f"{channel} entry requires a non-empty id")
            if channel == "failed" and (not isinstance(item.get("error"), Mapping) or not isinstance(item["error"].get("type"), str) or not item["error"]["type"]):
                raise ValueError("failed entry requires a typed error")
            if channel == "unknown" and (not isinstance(item.get("reason"), str) or not item["reason"]):
                raise ValueError("unknown entry requires a reason")
    count = sum(len(value) for value in channels.values())
    if total is None:
        total = count
    if total != count:
        raise ValueError(f"batch total {total} does not equal channel count {count}")
    return {"total": total, **channels, **extra}


def batch_outcome(data: Mapping[str, Any]) -> str:
    if not data.get("failed") and not data.get("unknown"):
        return "success"
    return "partial_failure" if data.get("succeeded") else "failure"


def add_contract_flags(parser: argparse.ArgumentParser, *, default: str = "text") -> None:
    parser.add_argument("--format", choices=("text", "json", "ndjson"), default=default, help=f"输出格式：text|json|ndjson（默认 {default}）")
    parser.add_argument("--dry-run", action="store_true", help="生成预览；不得执行远端/本地写入。远端只读探测边界由对应 Skill 的 Agent 扫描台账说明。")


def emit(
    *, fmt: str, outcome: str, data: Any = None, error: Optional[Mapping[str, Any]] = None,
    meta: Optional[Mapping[str, Any]] = None, dry_run: bool = False, text: Optional[str] = None,
    items: Optional[Iterable[Mapping[str, Any]]] = None, stdout: Optional[TextIO] = None,
) -> int:
    output = sys.stdout if stdout is None else stdout
    ok = outcome in {"success", "pending"}
    code = 0 if ok else PARTIAL_EXIT if outcome == "partial_failure" else FAILURE_EXIT
    envelope: dict[str, Any] = {"ok": ok, "outcome": outcome}
    if data is not None:
        envelope["data"] = data
    if error is not None:
        envelope["error"] = dict(error)
    if meta is not None:
        envelope["meta"] = dict(meta)
    if dry_run:
        envelope["dry_run"] = True
    if fmt == "text":
        print(text if text is not None else json.dumps(envelope, ensure_ascii=False, indent=2), file=output)
    elif fmt == "ndjson" and items is not None:
        for item in items:
            print(json.dumps(dict(item), ensure_ascii=False, separators=(",", ":")), file=output)
    else:
        print(json.dumps(envelope, ensure_ascii=False, separators=(",", ":")), file=output)
    return code


def failure(fmt: str, message: str, *, details: Any = None) -> int:
    error: dict[str, Any] = {"type": "validation", "message": message}
    if details is not None:
        error["details"] = details
    return emit(fmt=fmt, outcome="failure", error=error, text=f"错误：{message}")


def _format_from_argv(argv: Optional[Sequence[str]] = None, *, default: str = "text") -> str:
    args = list(sys.argv[1:] if argv is None else argv)
    for index, arg in enumerate(args):
        candidate = arg.partition("=")[2] if arg.startswith("--format=") else args[index + 1] if arg == "--format" and index + 1 < len(args) else None
        if candidate in {"text", "json", "ndjson"}:
            return candidate
    return default


def _valid_machine_stdout(fmt: str, value: str, status: int) -> bool:
    lines = [line for line in value.splitlines() if line.strip()]
    if fmt == "json":
        if len(lines) != 1:
            return False
        try:
            payload = json.loads(lines[0])
        except json.JSONDecodeError:
            return False
        if not isinstance(payload, Mapping) or not isinstance(payload.get("ok"), bool) or not isinstance(payload.get("outcome"), str):
            return False
        return ((payload["outcome"] in {"success", "pending"} and payload["ok"] and status == 0) or (payload["outcome"] == "partial_failure" and not payload["ok"] and status == PARTIAL_EXIT) or (payload["outcome"] == "failure" and not payload["ok"] and status == FAILURE_EXIT))
    if fmt == "ndjson":
        try:
            return all(isinstance(json.loads(line), Mapping) for line in lines)
        except json.JSONDecodeError:
            return False
    return True


def run_main(main_fn: Callable[[], Optional[int]], *, default_format: str = "text") -> int:
    fmt = _format_from_argv(default=default_format)
    captured = io.StringIO()
    try:
        if fmt == "text":
            result = main_fn()
            return 0 if result is None else int(result)
        with contextlib.redirect_stdout(captured):
            result = main_fn()
        status = 0 if result is None else int(result)
        if not _valid_machine_stdout(fmt, captured.getvalue(), status):
            print("✗ 脚本输出不符合机器结果契约；已拒绝污染 stdout。", file=sys.stderr)
            return emit(fmt=fmt, outcome="failure", error={"type": "internal", "message": "脚本产生了非契约机器输出；请修复脚本后重试。"})
        sys.stdout.write(captured.getvalue())
        return status
    except KeyboardInterrupt:
        raise
    except SystemExit as exc:
        status = exc.code if isinstance(exc.code, int) else FAILURE_EXIT
        if fmt == "text" or status == 0:
            return status
        return emit(fmt=fmt, outcome="failure", error={"type": "validation", "message": "脚本参数或前置校验未通过；请检查 --help。", "details": {"exit_code": status}})
    except Exception as exc:  # noqa: BLE001 - executable boundary by design.
        print(f"✗ 脚本发生未预期错误（{type(exc).__name__}）；已输出可解析结果。", file=sys.stderr)
        return emit(fmt=fmt, outcome="failure", error={"type": "internal", "message": "脚本执行时发生未预期错误；请检查输入类型和格式后重试。", "details": {"exception_type": type(exc).__name__}})
