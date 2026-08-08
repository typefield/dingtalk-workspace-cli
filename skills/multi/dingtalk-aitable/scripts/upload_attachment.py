#!/usr/bin/env python3
"""Upload a file for an AITable attachment field.

The script performs prepare-upload → OSS PUT → fileToken projection.  It is a
second, independent Multi-Skill pilot of the common result boundary: fileToken
is usable only after the PUT has a confirmed terminal response.
"""

from __future__ import annotations

import argparse
import json
import mimetypes
import re
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Mapping, Optional
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

from _runtime import ChildDWSResult, add_contract_flags, emit, failure, run_child_dws, run_main


RESOURCE_ID_PATTERN = re.compile(r"^[A-Za-z0-9_-]{8,128}$")
MAX_FILE_SIZE = 100 * 1024 * 1024


def validate_resource_id(resource_id: str) -> bool:
    return bool(resource_id and RESOURCE_ID_PATTERN.match(resource_id.strip()))


def detect_mime_type(file_path: Path) -> str:
    mime_type, _ = mimetypes.guess_type(str(file_path))
    return mime_type or "application/octet-stream"


def run_dws(args: list[str]) -> ChildDWSResult:
    return run_child_dws(args, timeout=60)


@dataclass(frozen=True)
class PutResult:
    state: str
    error: Optional[dict[str, Any]] = None


@dataclass(frozen=True)
class AttachmentResult:
    state: str
    phase: str
    data: dict[str, Any]
    error: Optional[dict[str, Any]] = None
    meta: Optional[dict[str, Any]] = None


def upload_to_oss(upload_url: str, file_path: Path, mime_type: str) -> PutResult:
    request = Request(upload_url, data=file_path.read_bytes(), method="PUT")
    request.add_header("Content-Type", mime_type)
    try:
        with urlopen(request, timeout=120) as response:
            if response.status == 200:
                return PutResult("success")
            return PutResult("failed", {"type": "api", "message": f"OSS 上传返回 HTTP {response.status}"})
    except HTTPError as exc:
        return PutResult("failed", {"type": "api", "message": f"OSS 上传 HTTP {exc.code}: {exc.reason}"})
    except URLError as exc:
        return PutResult("unknown", {"type": "network", "message": f"OSS 上传未收到终态响应：{exc.reason}"})
    except TimeoutError:
        return PutResult("unknown", {"type": "network", "message": "OSS 上传超时；文件是否已上传未知。"})


def child_data(result: ChildDWSResult) -> Optional[dict[str, Any]]:
    if not isinstance(result.payload, Mapping):
        return None
    data = result.payload.get("data")
    return dict(data) if isinstance(data, Mapping) else None


def child_status(result: ChildDWSResult, data: Mapping[str, Any]) -> Optional[str]:
    if isinstance(result.payload, Mapping) and isinstance(result.payload.get("status"), str):
        return result.payload["status"]
    status = data.get("status")
    return status if isinstance(status, str) else None


def upload_attachment(base_id: str, file_path_str: str, *, dry_run: bool = False) -> AttachmentResult:
    file_path = Path(file_path_str).resolve()
    if not file_path.exists():
        return AttachmentResult("failed", "validate_file", {}, {"type": "validation", "message": "文件不存在"})
    if not file_path.is_file():
        return AttachmentResult("failed", "validate_file", {}, {"type": "validation", "message": "目标不是文件"})
    file_size = file_path.stat().st_size
    if file_size <= 0:
        return AttachmentResult("failed", "validate_file", {}, {"type": "validation", "message": "文件为空"})
    if file_size > MAX_FILE_SIZE:
        return AttachmentResult("failed", "validate_file", {}, {"type": "validation", "message": "文件超过大小限制"})

    file_name = file_path.name
    mime_type = detect_mime_type(file_path)
    dws_args = [
        "aitable", "attachment", "upload", "--base-id", base_id, "--file-name", file_name,
        "--size", str(file_size), "--mime-type", mime_type, "--format", "json",
    ]
    if dry_run:
        return AttachmentResult("success", "dry_run", {
            "baseId": base_id,
            "fileName": file_name,
            "size": file_size,
            "mimeType": mime_type,
            "steps": ["prepare_attachment_upload", "PUT uploadUrl", "return fileToken"],
            "request": dws_args,
        })

    print(f"步骤 1/3: 准备上传 {file_name} ({file_size:,} 字节, {mime_type})…", file=sys.stderr)
    prepared = run_dws(dws_args)
    base_data = {"baseId": base_id, "fileName": file_name, "size": file_size, "mimeType": mime_type}
    if prepared.state != "success":
        return AttachmentResult(prepared.state, "prepare_attachment_upload", base_data, prepared.error, prepared.meta)
    data = child_data(prepared)
    if data is None:
        return AttachmentResult("unknown", "prepare_attachment_upload", base_data, {"type": "api", "message": "准备上传返回结构无法验证"}, prepared.meta)
    status = child_status(prepared, data)
    # Some current replies are a bare {uploadUrl,fileToken}; both values are
    # therefore the operation-specific success fact.  An explicit negative
    # status remains nonterminal, never a safe 'not executed' conclusion.
    if status is not None and status != "success":
        return AttachmentResult("unknown", "prepare_attachment_upload", base_data, {"type": "api", "message": "准备上传未报告成功"}, prepared.meta)
    upload_url, file_token = data.get("uploadUrl"), data.get("fileToken")
    if not isinstance(upload_url, str) or not isinstance(file_token, str) or not upload_url or not file_token:
        return AttachmentResult("unknown", "prepare_attachment_upload", base_data, {"type": "api", "message": "准备上传缺少 uploadUrl 或 fileToken"}, prepared.meta)

    attachment_data = {**base_data, "fileToken": file_token}
    print("步骤 2/3: 上传文件到 OSS…", file=sys.stderr)
    uploaded = upload_to_oss(upload_url, file_path, mime_type)
    if uploaded.state != "success":
        return AttachmentResult(uploaded.state, "upload_file", attachment_data, uploaded.error, prepared.meta)
    print("步骤 3/3: 上传完成。", file=sys.stderr)
    return AttachmentResult("success", "complete", attachment_data, meta=prepared.meta)


def main() -> int:
    parser = argparse.ArgumentParser(description="上传附件到钉钉 AI 表格 attachment 字段")
    parser.add_argument("base_id", help="目标 AI 表格 baseId")
    parser.add_argument("file_path", help="待上传文件路径")
    # This script historically emitted JSON; preserve its default callers.
    add_contract_flags(parser, default="json")
    args = parser.parse_args()
    if not validate_resource_id(args.base_id):
        return failure(args.format, "无效的 baseId 格式")

    result = upload_attachment(args.base_id, args.file_path, dry_run=args.dry_run)
    if result.state != "success":
        data = {
            **result.data,
            "phase": result.phase,
            "execution_state": "unknown" if result.state == "unknown" else "not_executed",
        }
        return emit(
            fmt=args.format, outcome="failure", data=data,
            error=result.error or {"type": "api", "message": "附件上传未确认完成"},
            meta=result.meta, text="附件上传未确认完成",
        )
    return emit(
        fmt=args.format, outcome="success", data=result.data, dry_run=args.dry_run,
        meta=result.meta, text=json.dumps(result.data, ensure_ascii=False, indent=2),
    )


if __name__ == "__main__":
    sys.exit(run_main(main, default_format="json"))
