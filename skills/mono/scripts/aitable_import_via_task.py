#!/usr/bin/env python3
"""
通过 MCP 文件导入任务（prepare_import_upload -> PUT -> import_data）导入 AI 表格。

与 import_records.py 的区别：
- 本脚本：走“文件导入任务”链路，通常会新建导入数据表。
- import_records.py：走 create_records，写入已有 table。

用法:
    python scripts/aitable_import_via_task.py <baseId> <filePath>
    python scripts/aitable_import_via_task.py <baseId> <filePath> --timeout 30
    python scripts/aitable_import_via_task.py <baseId> <filePath> --dws /tmp/dws
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Dict, Mapping, Optional, Tuple
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

from _runtime import ChildDWSResult, add_contract_flags, emit, failure, run_child_dws, run_main

RESOURCE_ID_PATTERN = re.compile(r"^[A-Za-z0-9_-]{8,128}$")
ALLOWED_EXTENSIONS = {".csv", ".xlsx", ".xls"}


def validate_resource_id(resource_id: str) -> bool:
    return bool(resource_id and RESOURCE_ID_PATTERN.match(resource_id.strip()))


def run_dws(dws_bin: str, args: list[str], timeout_sec: int = 120) -> ChildDWSResult:
    """Invoke a child DWS operation without treating transport completion as truth."""
    return run_child_dws(args, timeout=timeout_sec, executable=dws_bin)


@dataclass(frozen=True)
class PutResult:
    """Terminal certainty of the upload-byte phase, separate from the import task."""

    state: str
    error: Optional[Dict[str, Any]] = None


def put_file(upload_url: str, file_path: Path) -> PutResult:
    payload = file_path.read_bytes()
    req = Request(upload_url, data=payload, method="PUT")
    # 关键：清空 Content-Type，避免 SignatureDoesNotMatch。
    req.add_header("Content-Type", "")
    try:
        with urlopen(req, timeout=180) as resp:
            if resp.status == 200:
                return PutResult("success")
            return PutResult("failed", {"type": "api", "message": f"上传服务返回 HTTP {resp.status}"})
    except HTTPError as e:
        body = e.read().decode("utf-8", "ignore")
        return PutResult("failed", {"type": "api", "message": f"HTTP {e.code}: {body[:300]}"})
    except URLError as e:
        return PutResult(
            "unknown",
            {"type": "network", "message": f"PUT 上传未收到终态响应：{e.reason}"},
        )
    except TimeoutError:
        return PutResult(
            "unknown",
            {"type": "network", "message": "PUT 上传超时；文件是否已上传未知。"},
        )


def result_data(result: ChildDWSResult) -> Optional[Dict[str, Any]]:
    if not isinstance(result.payload, Mapping):
        return None
    data = result.payload.get("data")
    return dict(data) if isinstance(data, Mapping) else None


def business_status(result: ChildDWSResult, data: Mapping[str, Any]) -> Optional[str]:
    """Read an explicit task status without assuming where old/new payloads place it."""
    if isinstance(result.payload, Mapping) and isinstance(result.payload.get("status"), str):
        return result.payload["status"]
    status = data.get("status")
    return status if isinstance(status, str) else None


def child_metas(*entries: tuple[str, ChildDWSResult]) -> Optional[dict[str, Any]]:
    """Preserve per-step child metadata without merging unrelated facts.

    ``prepare_import_upload`` and ``import_data`` are separate requests.  A
    later response must not overwrite pagination, task, or transport metadata
    emitted by the earlier request.  Keep each fact under its stable step ID
    so an Agent can associate it with the matching operation.
    """
    children = [
        {"id": entry_id, "meta": dict(result.meta)}
        for entry_id, result in entries
        if result.meta
    ]
    return {"children": children} if children else None


def child_failure(
    *,
    fmt: str,
    message: str,
    phase: str,
    execution_state: str,
    plan: Dict[str, Any],
    error: Optional[Mapping[str, Any]] = None,
    meta: Optional[Mapping[str, Any]] = None,
) -> int:
    data = {**plan, "phase": phase, "execution_state": execution_state}
    return emit(
        fmt=fmt,
        outcome="failure",
        data=data,
        error=dict(error or {"type": "api", "message": message}),
        meta=meta,
        text=message,
    )


def main() -> int:
    parser = argparse.ArgumentParser(description="通过文件导入任务导入 AI 表格")
    parser.add_argument("base_id", help="目标 AI 表格 baseId")
    parser.add_argument("file_path", help="待导入文件路径（.csv/.xlsx/.xls）")
    parser.add_argument("--timeout-sec", type=int, default=300, help="CLI 内置轮询整体超时（秒），默认 300（5 分钟）")
    parser.add_argument("--dws", default="dws", help="dws 可执行文件路径，默认 dws")
    add_contract_flags(parser)
    args = parser.parse_args()

    base_id = args.base_id.strip()
    file_path = Path(args.file_path).expanduser().resolve()

    if not validate_resource_id(base_id):
        return failure(args.format, "无效的 baseId 格式")
    if not file_path.exists() or not file_path.is_file():
        return failure(args.format, f"文件不存在或不可读: {file_path}")
    if file_path.suffix.lower() not in ALLOWED_EXTENSIONS:
        return failure(args.format, f"仅支持 {sorted(ALLOWED_EXTENSIONS)}，当前文件: {file_path.name}")

    file_size = file_path.stat().st_size
    if file_size <= 0:
        return failure(args.format, "文件为空")

    plan = {
        "baseId": base_id,
        "fileName": file_path.name,
        "fileSize": file_size,
        "steps": ["prepare_import_upload", "PUT uploadUrl", "import_data"],
    }
    if args.dry_run:
        return emit(
            fmt=args.format,
            outcome="success",
            data=plan,
            dry_run=True,
            text="\n".join(f"[dry-run] {step}" for step in plan["steps"]),
        )

    print(f"[1/3] prepare import upload: {file_path.name} ({file_size} bytes)", file=sys.stderr)
    prepare = run_dws(
        args.dws,
        [
            "aitable",
            "import",
            "upload",
            "--base-id",
            base_id,
            "--file-name",
            file_path.name,
            "--file-size",
            str(file_size),
            "--format",
            "json",
        ],
    )
    prepare_metas = child_metas(("prepare_import_upload", prepare))
    if prepare.state != "success":
        return child_failure(
            fmt=args.format,
            message="prepare_import_upload 未返回可确认终态",
            phase="prepare_import_upload",
            execution_state="unknown" if prepare.state == "unknown" else "not_executed",
            plan=plan,
            error=prepare.error,
            meta=prepare_metas,
        )
    pdata = result_data(prepare)
    if pdata is None:
        return child_failure(
            fmt=args.format,
            message="prepare_import_upload 返回结构无法验证；请先核查导入任务。",
            phase="prepare_import_upload",
            execution_state="unknown",
            plan=plan,
            meta=prepare_metas,
        )
    upload_url = pdata.get("uploadUrl")
    import_id = pdata.get("importId")
    if not upload_url or not import_id:
        return child_failure(
            fmt=args.format,
            message="prepare_import_upload 缺少 uploadUrl/importId；请先核查导入任务。",
            phase="prepare_import_upload",
            execution_state="unknown",
            plan=plan,
            meta=prepare_metas,
        )

    plan["importId"] = import_id

    print("[2/3] upload file bytes via PUT", file=sys.stderr)
    upload = put_file(upload_url, file_path)
    if upload.state != "success":
        return child_failure(
            fmt=args.format,
            message="PUT 上传未确认完成；请先核查上传与导入任务。",
            phase="upload_file",
            execution_state="unknown" if upload.state == "unknown" else "not_executed",
            plan=plan,
            error=upload.error,
            meta=prepare_metas,
        )

    print("[3/3] trigger import_data", file=sys.stderr)
    trigger = run_dws(
        args.dws,
        [
            "aitable",
            "import",
            "data",
            "--import-id",
            import_id,
            # import data 的 --timeout 单位是秒、最大 30；脚本的 --timeout-sec
            # 是整体子进程预算，不能直接透传，这里用 CLI 允许的最大值。
            "--timeout",
            "30",
            "--format",
            "json",
        ],
        timeout_sec=max(120, args.timeout_sec + 30),
    )
    operation_metas = child_metas(
        ("prepare_import_upload", prepare),
        ("import_data", trigger),
    )
    if trigger.state != "success":
        return child_failure(
            fmt=args.format,
            message="import_data 未返回可确认终态；请先核查导入任务，避免重复触发。",
            phase="import_data",
            execution_state="unknown" if trigger.state == "unknown" else "not_executed",
            plan=plan,
            error=trigger.error,
            meta=operation_metas,
        )
    import_data = result_data(trigger)
    if import_data is None:
        return child_failure(
            fmt=args.format,
            message="import_data 返回结构无法验证；请先核查导入任务。",
            phase="import_data",
            execution_state="unknown",
            plan=plan,
            meta=operation_metas,
        )

    status = business_status(trigger, import_data)
    result = {
        "baseId": base_id,
        "fileName": file_path.name,
        "fileSize": file_size,
        "importId": import_id,
        "status": status,
        "summary": (
            trigger.payload.get("summary")
            if isinstance(trigger.payload, Mapping)
            else import_data.get("summary")
        ),
        "data": import_data,
    }
    if status != "success":
        return child_failure(
            fmt=args.format,
            message="import_data 未报告成功；请先核查导入任务，避免重复触发。",
            phase="import_data",
            execution_state="unknown",
            plan=result,
            error={"type": "api", "message": "import_data 未报告成功"},
            meta=operation_metas,
        )
    return emit(
        fmt=args.format,
        outcome="success",
        data=result,
        meta=operation_metas,
        text=json.dumps(result, ensure_ascii=False, indent=2),
    )


if __name__ == "__main__":
    sys.exit(run_main(main))
