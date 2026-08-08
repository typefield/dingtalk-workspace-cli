#!/usr/bin/env python3
"""Import an AITable file through prepare-upload → PUT → import-data.

This is a Multi-Skill pilot for the shared script result boundary.  It keeps
the established positional arguments and ``--timeout`` flag; only result
truthfulness and ``--format`` become common with the Mono counterpart.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Mapping, Optional
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

from _runtime import ChildDWSResult, add_contract_flags, emit, failure, run_child_dws, run_main


RESOURCE_ID_PATTERN = re.compile(r"^[A-Za-z0-9_-]{8,128}$")
ALLOWED_EXTENSIONS = {".csv", ".xlsx", ".xls"}


def validate_resource_id(resource_id: str) -> bool:
    return bool(resource_id and RESOURCE_ID_PATTERN.match(resource_id.strip()))


def run_dws(dws_bin: str, args: list[str], timeout_sec: int = 120) -> ChildDWSResult:
    return run_child_dws(args, timeout=timeout_sec, executable=dws_bin)


@dataclass(frozen=True)
class PutResult:
    state: str
    error: Optional[dict[str, Any]] = None


def put_file(upload_url: str, file_path: Path) -> PutResult:
    request = Request(upload_url, data=file_path.read_bytes(), method="PUT")
    request.add_header("Content-Type", "")
    try:
        with urlopen(request, timeout=180) as response:
            if response.status == 200:
                return PutResult("success")
            return PutResult("failed", {"type": "api", "message": f"上传服务返回 HTTP {response.status}"})
    except HTTPError as exc:
        body = exc.read().decode("utf-8", "ignore")
        return PutResult("failed", {"type": "api", "message": f"HTTP {exc.code}: {body[:300]}"})
    except (URLError, TimeoutError) as exc:
        return PutResult("unknown", {"type": "network", "message": f"PUT 上传未收到终态响应：{exc}"})


def result_data(result: ChildDWSResult) -> Optional[dict[str, Any]]:
    if not isinstance(result.payload, Mapping):
        return None
    data = result.payload.get("data")
    return dict(data) if isinstance(data, Mapping) else None


def business_status(result: ChildDWSResult, data: Mapping[str, Any]) -> Optional[str]:
    if isinstance(result.payload, Mapping) and isinstance(result.payload.get("status"), str):
        return result.payload["status"]
    status = data.get("status")
    return status if isinstance(status, str) else None


def child_failure(
    *, fmt: str, message: str, phase: str, execution_state: str, plan: Mapping[str, Any],
    error: Optional[Mapping[str, Any]] = None, meta: Optional[Mapping[str, Any]] = None,
) -> int:
    return emit(
        fmt=fmt,
        outcome="failure",
        data={**plan, "phase": phase, "execution_state": execution_state},
        error=dict(error or {"type": "api", "message": message}),
        meta=meta,
        text=message,
    )


def main() -> int:
    parser = argparse.ArgumentParser(description="通过文件导入任务导入 AI 表格")
    parser.add_argument("base_id", help="目标 AI 表格 baseId")
    parser.add_argument("file_path", help="待导入文件路径（.csv/.xlsx/.xls）")
    parser.add_argument("--timeout", type=int, default=30, help="import_data 等待秒数，默认 30")
    parser.add_argument("--dws", default="dws", help="dws 可执行文件路径，默认 dws")
    # This script historically emitted JSON by default. Preserve that public
    # default while adding an explicit human-readable mode for local use.
    add_contract_flags(parser, default="json")
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
    if args.timeout <= 0:
        return failure(args.format, "--timeout 必须为正整数")

    plan: dict[str, Any] = {
        "baseId": base_id,
        "fileName": file_path.name,
        "fileSize": file_size,
        "steps": ["prepare_import_upload", "PUT uploadUrl", "import_data"],
    }
    if args.dry_run:
        return emit(
            fmt=args.format, outcome="success", data=plan, dry_run=True,
            text="\n".join(f"[dry-run] {step}" for step in plan["steps"]),
        )

    print(f"[1/3] prepare import upload: {file_path.name} ({file_size} bytes)", file=sys.stderr)
    prepare = run_dws(args.dws, [
        "aitable", "import", "upload", "--base-id", base_id, "--file-name", file_path.name,
        "--file-size", str(file_size), "--format", "json",
    ])
    if prepare.state != "success":
        return child_failure(
            fmt=args.format, message="prepare_import_upload 未返回可确认终态", phase="prepare_import_upload",
            execution_state="unknown" if prepare.state == "unknown" else "not_executed", plan=plan,
            error=prepare.error, meta=prepare.meta,
        )
    prepared = result_data(prepare)
    if prepared is None:
        return child_failure(fmt=args.format, message="prepare_import_upload 返回结构无法验证；请先核查导入任务。", phase="prepare_import_upload", execution_state="unknown", plan=plan, meta=prepare.meta)
    upload_url, import_id = prepared.get("uploadUrl"), prepared.get("importId")
    if not isinstance(upload_url, str) or not isinstance(import_id, str) or not upload_url or not import_id:
        return child_failure(fmt=args.format, message="prepare_import_upload 缺少 uploadUrl/importId；请先核查导入任务。", phase="prepare_import_upload", execution_state="unknown", plan=plan, meta=prepare.meta)
    plan["importId"] = import_id

    print("[2/3] upload file bytes via PUT", file=sys.stderr)
    uploaded = put_file(upload_url, file_path)
    if uploaded.state != "success":
        return child_failure(
            fmt=args.format, message="PUT 上传未确认完成；请先核查上传与导入任务。", phase="upload_file",
            execution_state="unknown" if uploaded.state == "unknown" else "not_executed", plan=plan,
            error=uploaded.error, meta=prepare.meta,
        )

    print("[3/3] trigger import_data", file=sys.stderr)
    trigger = run_dws(args.dws, [
        "aitable", "import", "data", "--import-id", import_id, "--timeout", str(args.timeout), "--format", "json",
    ], timeout_sec=max(120, args.timeout + 30))
    if trigger.state != "success":
        return child_failure(
            fmt=args.format, message="import_data 未返回可确认终态；请先核查导入任务，避免重复触发。", phase="import_data",
            execution_state="unknown" if trigger.state == "unknown" else "not_executed", plan=plan,
            error=trigger.error, meta=trigger.meta,
        )
    imported = result_data(trigger)
    if imported is None:
        return child_failure(fmt=args.format, message="import_data 返回结构无法验证；请先核查导入任务。", phase="import_data", execution_state="unknown", plan=plan, meta=trigger.meta)
    status = business_status(trigger, imported)
    result = {
        **plan,
        "status": status,
        "summary": trigger.payload.get("summary") if isinstance(trigger.payload, Mapping) else imported.get("summary"),
        "data": imported,
    }
    if status != "success":
        return child_failure(
            fmt=args.format, message="import_data 未报告成功；请先核查导入任务，避免重复触发。", phase="import_data",
            execution_state="unknown", plan=result, error={"type": "api", "message": "import_data 未报告成功"}, meta=trigger.meta,
        )
    return emit(fmt=args.format, outcome="success", data=result, meta=trigger.meta, text=json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    sys.exit(run_main(main, default_format="json"))
