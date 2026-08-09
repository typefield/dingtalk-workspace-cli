#!/usr/bin/env python3
"""Export an AITable through its asynchronous task without hiding state.

The remote export task and the local download are separate operations.  This
script therefore never turns a task that is still processing into a terminal
success, and never describes a local file as remotely verified when the
service did not supply a checksum.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import sys
import tempfile
import time
from collections.abc import Mapping
from pathlib import Path
from typing import Any, Optional
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

from _runtime import ChildDWSResult, add_contract_flags, emit, failure, run_child_dws, run_main


RESOURCE_ID_PATTERN = re.compile(r"^[A-Za-z0-9_-]{8,128}$")
ALLOWED_FORMATS = {"excel", "attachment", "excel_and_attachment", "excel_with_inline_images"}
TERMINAL_FAILURES = {"error", "failed", "failure", "cancelled", "canceled"}


def _valid_resource_id(value: str) -> bool:
    return bool(value and RESOURCE_ID_PATTERN.match(value.strip()))


def _unwrap(value: Any) -> Any:
    """Unwrap known DWS envelopes without treating arbitrary maps as success."""
    while isinstance(value, Mapping):
        for key in ("data", "result", "content"):
            nested = value.get(key)
            if isinstance(nested, (Mapping, list)):
                value = nested
                break
        else:
            return value
    return value


def _task_data(payload: Any) -> dict[str, Any]:
    value = _unwrap(payload)
    return dict(value) if isinstance(value, Mapping) else {}


def _task_status(payload: Any, data: Mapping[str, Any]) -> str:
    """Read terminal task state from old and unified response layouts."""
    candidates: list[Mapping[str, Any]] = [data]
    if isinstance(payload, Mapping):
        candidates.append(payload)
        for key in ("result", "content"):
            nested = payload.get(key)
            if isinstance(nested, Mapping):
                candidates.append(nested)
    for candidate in candidates:
        for key in ("status", "state", "taskStatus"):
            value = candidate.get(key)
            if isinstance(value, str) and value.strip():
                return value.strip().lower()
    return ""


def _child_error(child: ChildDWSResult, fallback: str) -> dict[str, Any]:
    error = dict(child.error or {"type": "api", "message": fallback})
    error.setdefault("hint", "请保留 taskId 并先查询导出任务状态；不要重复创建导出任务。")
    return error


def _child_meta(identifier: str, child: ChildDWSResult) -> Optional[dict[str, Any]]:
    return {"id": identifier, "meta": dict(child.meta)} if child.meta else None


def _children_meta(children: list[dict[str, Any]]) -> Optional[dict[str, Any]]:
    return {"children": children} if children else None


def _query_command(base_id: str, task_id: str, timeout_ms: int) -> str:
    return (
        "dws aitable export data"
        f" --base-id {base_id} --task-id {task_id} --timeout-ms {timeout_ms}"
    )


def _pending(
    *,
    fmt: str,
    base_id: str,
    scope: str,
    export_format: str,
    task_id: str,
    file_name: str,
    polls: int,
    timeout_ms: int,
    children: list[dict[str, Any]],
    last_poll: Optional[dict[str, Any]] = None,
) -> int:
    data: dict[str, Any] = {
        "baseId": base_id,
        "scope": scope,
        "exportFormat": export_format,
        "taskId": task_id,
        "fileName": file_name,
        "polledTimes": polls,
        "execution_state": "pending",
        "next_command": _query_command(base_id, task_id, timeout_ms),
        "verification": {"state": "not_applicable", "reason": "导出任务尚未提供下载地址。"},
    }
    if last_poll is not None:
        data["last_poll"] = last_poll
    return emit(
        fmt=fmt,
        outcome="pending",
        data=data,
        meta=_children_meta(children),
        text="导出任务仍在运行；请使用 next_command 继续查询，勿重复创建导出任务。",
    )


def _start_args(args: argparse.Namespace) -> list[str]:
    command = [
        "aitable", "export", "data", "--base-id", args.base_id,
        "--scope", args.scope, "--format", args.export_format,
        "--timeout-ms", str(args.timeout_ms),
    ]
    if args.table_id:
        command.extend(["--table-id", args.table_id])
    if args.view_id:
        command.extend(["--view-id", args.view_id])
    return command


def _poll_args(base_id: str, task_id: str, timeout_ms: int) -> list[str]:
    return [
        "aitable", "export", "data", "--base-id", base_id,
        "--task-id", task_id, "--timeout-ms", str(timeout_ms),
    ]


def _safe_output_path(requested: str, file_name: str) -> Path:
    if requested:
        return Path(requested).expanduser().resolve()
    name = Path(file_name).name
    if name in {"", ".", ".."}:
        name = "export_result.bin"
    return (Path.cwd() / name).resolve()


def _download_file(url: str, output_path: Path, *, overwrite: bool) -> tuple[bool, dict[str, Any], dict[str, Any]]:
    """Download into the destination directory and publish only atomically."""
    if output_path.exists() and not overwrite:
        return False, {}, {
            "type": "validation",
            "subtype": "output_exists",
            "message": f"输出文件已存在：{output_path}",
            "hint": "请改用新的 --output 路径，或在确认覆盖后传 --overwrite。",
        }
    if not output_path.parent.is_dir():
        return False, {}, {
            "type": "validation",
            "subtype": "output_parent_missing",
            "message": f"输出目录不存在：{output_path.parent}",
        }

    temporary_path: Optional[Path] = None
    try:
        fd, temporary_name = tempfile.mkstemp(
            prefix=f".{output_path.name}.", suffix=".part", dir=output_path.parent,
        )
        temporary_path = Path(temporary_name)
        digest = hashlib.sha256()
        size = 0
        request = Request(url, method="GET")
        with urlopen(request, timeout=180) as response, os.fdopen(fd, "wb") as destination:
            status = getattr(response, "status", 200)
            if status != 200:
                raise HTTPError(url, status, f"download http status: {status}", response.headers, None)
            while chunk := response.read(64 * 1024):
                destination.write(chunk)
                digest.update(chunk)
                size += len(chunk)
            destination.flush()
            os.fsync(destination.fileno())
        os.replace(temporary_path, output_path)
        temporary_path = None
        return True, {
            "savedPath": str(output_path),
            "bytes": size,
            "sha256": digest.hexdigest(),
            "verification": {
                "state": "local_written",
                "method": "atomic_download_sha256",
                "source_integrity": "unverified_no_remote_checksum",
            },
        }, {}
    except HTTPError as exc:
        return False, {}, {
            "type": "network",
            "subtype": "download_http_error",
            "message": f"下载导出文件失败：HTTP {exc.code}",
            "details": {"http_status": exc.code},
        }
    except URLError as exc:
        return False, {}, {
            "type": "network",
            "subtype": "download_network_error",
            "message": f"下载导出文件失败：{exc.reason}",
        }
    except OSError as exc:
        return False, {}, {
            "type": "internal",
            "subtype": "download_local_write_failed",
            "message": f"保存导出文件失败：{type(exc).__name__}",
        }
    finally:
        if temporary_path is not None:
            temporary_path.unlink(missing_ok=True)


def main() -> int:
    parser = argparse.ArgumentParser(description="通过异步 MCP 导出任务导出 AI 表格，并诚实报告任务与本地文件状态。")
    parser.add_argument("base_id", help="目标 AI 表格 baseId")
    parser.add_argument("--scope", choices=("all", "table", "view"), required=True, help="导出范围")
    parser.add_argument("--table-id", help="scope=table/view 时必填")
    parser.add_argument("--view-id", help="scope=view 时必填")
    parser.add_argument("--export-format", default="excel", choices=sorted(ALLOWED_FORMATS), help="导出文件格式")
    parser.add_argument("--timeout-ms", type=int, default=30000, help="单次创建任务等待毫秒数")
    parser.add_argument("--poll-timeout-ms", type=int, default=30000, help="单次任务查询等待毫秒数")
    parser.add_argument("--timeout-sec", type=int, default=300, help="单个 dws 子进程的最大等待秒数")
    parser.add_argument("--max-polls", type=int, default=10, help="当前进程的最大轮询次数")
    parser.add_argument("--output", help="本地保存路径；默认使用服务端 fileName 于当前目录")
    parser.add_argument("--overwrite", action="store_true", help="明确允许覆盖已有本地输出文件")
    parser.add_argument("--no-download", action="store_true", help="仅返回下载地址，不写本地文件")
    parser.add_argument("--dws", default="dws", help="dws 可执行文件路径，默认 dws")
    add_contract_flags(parser)
    args = parser.parse_args()

    if not _valid_resource_id(args.base_id):
        return failure(args.format, "无效的 baseId 格式。")
    if args.scope in {"table", "view"} and not _valid_resource_id(args.table_id or ""):
        return failure(args.format, "scope=table/view 时必须传有效的 --table-id。")
    if args.scope == "view" and not _valid_resource_id(args.view_id or ""):
        return failure(args.format, "scope=view 时必须传有效的 --view-id。")
    if args.timeout_ms <= 0 or args.poll_timeout_ms <= 0 or args.timeout_sec <= 0 or args.max_polls < 0:
        return failure(args.format, "--timeout-ms、--poll-timeout-ms、--timeout-sec 必须为正数，--max-polls 不能为负数。")
    if args.no_download and args.output:
        return failure(args.format, "--no-download 不能与 --output 同时使用。")
    if args.output and _safe_output_path(args.output, "export_result.bin").exists() and not args.overwrite:
        return emit(
            fmt=args.format,
            outcome="failure",
            error={
                "type": "validation",
                "subtype": "output_exists",
                "message": "指定 --output 已存在。",
                "hint": "请改用新路径，或在确认覆盖后显式传 --overwrite。",
            },
            text="错误：指定 --output 已存在。",
        )

    plan = {
        "operation": "aitable_export_via_task",
        "baseId": args.base_id,
        "scope": args.scope,
        "tableId": args.table_id,
        "viewId": args.view_id,
        "exportFormat": args.export_format,
        "steps": ["start export task", "poll task until downloadUrl"] + ([] if args.no_download else ["atomically download file"]),
        "verification": {"state": "not_applicable"},
    }
    if args.dry_run:
        return emit(
            fmt=args.format,
            outcome="success",
            data=plan,
            dry_run=True,
            text="预览：不会创建导出任务或写本地文件。",
        )

    children: list[dict[str, Any]] = []
    print("[1/2] 创建导出任务", file=sys.stderr)
    started = run_child_dws(_start_args(args), executable=args.dws, timeout=args.timeout_sec)
    if (entry := _child_meta("export:start", started)):
        children.append(entry)
    if started.state != "success":
        error = _child_error(started, "导出任务创建未返回可信结果。")
        execution_state = "not_executed" if started.state == "failed" else "unknown"
        return emit(
            fmt=args.format,
            outcome="failure",
            data={**plan, "execution_state": execution_state},
            error=error,
            meta=_children_meta(children),
            text="导出任务创建未得到可信结果；请先核查，不要重复创建。",
        )

    task = _task_data(started.payload)
    task_id = str(task.get("taskId") or task.get("task_id") or "")
    download_url = str(task.get("downloadUrl") or task.get("download_url") or "")
    file_name = str(task.get("fileName") or task.get("file_name") or "export_result.bin")
    start_status = _task_status(started.payload, task)
    if start_status in TERMINAL_FAILURES:
        return emit(
            fmt=args.format,
            outcome="failure",
            data={**plan, "execution_state": "failed", "taskId": task_id or None, "taskStatus": start_status},
            error={"type": "api", "subtype": "export_task_failed", "message": "导出任务已明确失败。"},
            meta=_children_meta(children),
            text="导出任务已明确失败。",
        )
    if not download_url and not task_id:
        return emit(
            fmt=args.format,
            outcome="failure",
            data={**plan, "execution_state": "unknown"},
            error={
                "type": "api",
                "subtype": "export_task_missing_id",
                "message": "导出请求成功但未返回 taskId 或 downloadUrl。",
                "hint": "请先在 AI 表格中核查是否已生成导出任务；不要立即重复创建。",
            },
            meta=_children_meta(children),
            text="导出响应缺少 taskId；请先核查。",
        )

    polls = 0
    while not download_url and task_id and polls < args.max_polls:
        polls += 1
        print(f"[2/2] 查询导出任务（{polls}/{args.max_polls}）", file=sys.stderr)
        polled = run_child_dws(
            _poll_args(args.base_id, task_id, args.poll_timeout_ms),
            executable=args.dws,
            timeout=args.timeout_sec,
        )
        if (entry := _child_meta(f"export:poll:{polls}", polled)):
            children.append(entry)
        if polled.state != "success":
            return _pending(
                fmt=args.format,
                base_id=args.base_id,
                scope=args.scope,
                export_format=args.export_format,
                task_id=task_id,
                file_name=file_name,
                polls=polls,
                timeout_ms=args.poll_timeout_ms,
                children=children,
                last_poll={"state": polled.state, "error": _child_error(polled, "导出任务查询未返回可信结果。")},
            )
        task = _task_data(polled.payload)
        status = _task_status(polled.payload, task)
        file_name = str(task.get("fileName") or task.get("file_name") or file_name)
        task_id = str(task.get("taskId") or task.get("task_id") or task_id)
        download_url = str(task.get("downloadUrl") or task.get("download_url") or download_url)
        if status in TERMINAL_FAILURES:
            return emit(
                fmt=args.format,
                outcome="failure",
                data={"baseId": args.base_id, "scope": args.scope, "taskId": task_id, "taskStatus": status, "polledTimes": polls, "execution_state": "failed"},
                error={"type": "api", "subtype": "export_task_failed", "message": "导出任务已明确失败。"},
                meta=_children_meta(children),
                text="导出任务已明确失败。",
            )
        if not download_url and polls < args.max_polls:
            time.sleep(0.2)

    if not download_url:
        return _pending(
            fmt=args.format,
            base_id=args.base_id,
            scope=args.scope,
            export_format=args.export_format,
            task_id=task_id,
            file_name=file_name,
            polls=polls,
            timeout_ms=args.poll_timeout_ms,
            children=children,
        )

    result: dict[str, Any] = {
        "baseId": args.base_id,
        "scope": args.scope,
        "exportFormat": args.export_format,
        "taskId": task_id,
        "fileName": file_name,
        "downloadUrl": download_url,
        "polledTimes": polls,
        "execution_state": "completed",
    }
    if args.no_download:
        result["verification"] = {"state": "not_requested", "reason": "调用方选择不落盘。"}
        return emit(
            fmt=args.format,
            outcome="success",
            data=result,
            meta=_children_meta(children),
            text="导出任务已完成；未下载本地文件。",
        )

    output_path = _safe_output_path(args.output or "", file_name)
    downloaded, download_data, download_error = _download_file(download_url, output_path, overwrite=args.overwrite)
    if not downloaded:
        return emit(
            fmt=args.format,
            outcome="failure",
            data={**result, "local_output": {"state": "failed", "path": str(output_path)}},
            error=download_error,
            meta=_children_meta(children),
            text="导出任务已完成，但本地文件未能安全写入。",
        )
    result.update(download_data)
    return emit(
        fmt=args.format,
        outcome="success",
        data=result,
        meta=_children_meta(children),
        text="导出任务完成；文件已原子写入本地，但服务端未提供校验和。",
    )


if __name__ == "__main__":
    sys.exit(run_main(main))
