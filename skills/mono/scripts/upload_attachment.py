#!/usr/bin/env python3
"""
上传附件到钉钉 AI 表格 attachment 字段

完整流程（内部自动执行 3 步）:
  1. dws aitable attachment upload → 获取 uploadUrl + fileToken
  2. HTTP PUT 上传文件到 OSS
  3. 返回 fileToken，可直接用于 record create/update

用法:
    python upload_attachment.py <baseId> <filePath>

输出 (JSON):
    { "fileToken": "ft_xxx", "fileName": "report.pdf", "size": 204800 }

然后在 record create/update 中使用:
    dws aitable record create --base-id <BASE_ID> --table-id <TABLE_ID> \
      --records '[{"cells":{"fldAttachId":[{"fileToken":"ft_xxx"}]}}]' --format json
"""

import sys
import json
import argparse
import os
import mimetypes
import re
from dataclasses import dataclass
from pathlib import Path
from typing import Optional, Dict, Any, Mapping
from urllib.request import Request, urlopen
from urllib.error import HTTPError, URLError

from _runtime import ChildDWSResult, add_contract_flags, emit, failure, run_child_dws, run_main

RESOURCE_ID_PATTERN = re.compile(r'^[A-Za-z0-9_-]{8,128}$')
MAX_FILE_SIZE = 100 * 1024 * 1024  # 100MB


def validate_resource_id(resource_id: str) -> bool:
    return bool(resource_id and RESOURCE_ID_PATTERN.match(resource_id.strip()))


def detect_mime_type(file_path: Path) -> str:
    """根据文件扩展名推断 MIME type。"""
    mime_type, _ = mimetypes.guess_type(str(file_path))
    return mime_type or 'application/octet-stream'


def run_dws(args: list[str]) -> ChildDWSResult:
    """Call the prepare operation without collapsing certainty into a truthy dict."""
    return run_child_dws(args, timeout=60)


@dataclass(frozen=True)
class PutResult:
    state: str
    error: Optional[Dict[str, Any]] = None


@dataclass(frozen=True)
class AttachmentResult:
    state: str
    phase: str
    data: Dict[str, Any]
    error: Optional[Dict[str, Any]] = None
    meta: Optional[Dict[str, Any]] = None


def upload_to_oss(upload_url: str, file_path: Path, mime_type: str) -> PutResult:
    """通过 HTTP PUT 上传文件到 OSS。"""
    file_data = file_path.read_bytes()
    req = Request(upload_url, data=file_data, method='PUT')
    req.add_header('Content-Type', mime_type)

    try:
        with urlopen(req, timeout=120) as resp:
            if resp.status == 200:
                return PutResult('success')
            print(f"错误：OSS 上传失败，HTTP {resp.status}", file=sys.stderr)
            return PutResult('failed', {'type': 'api', 'message': f'OSS 上传返回 HTTP {resp.status}'})
    except HTTPError as e:
        print(f"错误：OSS 上传 HTTP 错误 {e.code}: {e.reason}", file=sys.stderr)
        return PutResult('failed', {'type': 'api', 'message': f'OSS 上传 HTTP {e.code}: {e.reason}'})
    except URLError as e:
        print(f"错误：OSS 上传网络错误: {e.reason}", file=sys.stderr)
        return PutResult('unknown', {'type': 'network', 'message': f'OSS 上传未收到终态响应：{e.reason}'})
    except TimeoutError:
        print('错误：OSS 上传超时', file=sys.stderr)
        return PutResult('unknown', {'type': 'network', 'message': 'OSS 上传超时；文件是否已上传未知。'})


def child_data(result: ChildDWSResult) -> Optional[Dict[str, Any]]:
    if not isinstance(result.payload, Mapping):
        return None
    data = result.payload.get('data')
    return dict(data) if isinstance(data, Mapping) else None


def child_status(result: ChildDWSResult, data: Mapping[str, Any]) -> Optional[str]:
    if isinstance(result.payload, Mapping) and isinstance(result.payload.get('status'), str):
        return result.payload['status']
    status = data.get('status')
    return status if isinstance(status, str) else None


def upload_attachment(base_id: str, file_path_str: str, *, dry_run: bool = False) -> AttachmentResult:
    """
    执行完整的附件上传流程:
      1. prepare_attachment_upload → uploadUrl + fileToken
      2. PUT 文件到 OSS
      3. 返回 fileToken 信息
    """
    # 验证文件
    file_path = Path(file_path_str).resolve()
    if not file_path.exists():
        print(f"错误：文件不存在: {file_path}", file=sys.stderr)
        return AttachmentResult('failed', 'validate_file', {}, {'type': 'validation', 'message': '文件不存在'})
    if not file_path.is_file():
        print(f"错误：不是文件: {file_path}", file=sys.stderr)
        return AttachmentResult('failed', 'validate_file', {}, {'type': 'validation', 'message': '目标不是文件'})

    file_size = file_path.stat().st_size
    if file_size <= 0:
        print("错误：文件为空", file=sys.stderr)
        return AttachmentResult('failed', 'validate_file', {}, {'type': 'validation', 'message': '文件为空'})
    if file_size > MAX_FILE_SIZE:
        print(f"错误：文件过大 ({file_size:,} 字节，限制 {MAX_FILE_SIZE:,} 字节)", file=sys.stderr)
        return AttachmentResult('failed', 'validate_file', {}, {'type': 'validation', 'message': '文件超过大小限制'})

    file_name = file_path.name
    mime_type = detect_mime_type(file_path)

    # 步骤 1: prepare_attachment_upload
    print(f"步骤 1/3: 准备上传 {file_name} ({file_size:,} 字节, {mime_type})...", file=sys.stderr)
    dws_args = [
        'aitable', 'attachment', 'upload',
        '--base-id', base_id,
        '--file-name', file_name,
        '--size', str(file_size),
        '--mime-type', mime_type,
        '--format', 'json',
    ]
    if dry_run:
        return AttachmentResult('success', 'dry_run', {
            "baseId": base_id,
            "fileName": file_name,
            "size": file_size,
            "mimeType": mime_type,
            "steps": ["prepare_attachment_upload", "PUT uploadUrl", "return fileToken"],
            "request": dws_args,
        })
    result = run_dws(dws_args)
    base_data = {
        'baseId': base_id,
        'fileName': file_name,
        'size': file_size,
        'mimeType': mime_type,
    }
    if result.state != 'success':
        return AttachmentResult(
            result.state, 'prepare_attachment_upload', base_data,
            result.error, result.meta,
        )
    data = child_data(result)
    if data is None:
        return AttachmentResult(
            'unknown', 'prepare_attachment_upload', base_data,
            {'type': 'api', 'message': '准备上传返回结构无法验证'}, result.meta,
        )
    status = child_status(result, data)
    # Current attachment prepare replies may be a bare {uploadUrl,fileToken}
    # object.  Presence of both fields is the operation-specific success fact;
    # only an explicit non-success status is a negative result.
    if status is not None and status != 'success':
        return AttachmentResult(
            'unknown', 'prepare_attachment_upload', base_data,
            {'type': 'api', 'message': '准备上传未报告成功'}, result.meta,
        )
    upload_url = data.get('uploadUrl', '')
    file_token = data.get('fileToken', '')

    if not upload_url or not file_token:
        return AttachmentResult(
            'unknown', 'prepare_attachment_upload', base_data,
            {'type': 'api', 'message': '准备上传缺少 uploadUrl 或 fileToken'}, result.meta,
        )

    attachment_data = {**base_data, 'fileToken': file_token}

    # 步骤 2: PUT 文件到 OSS
    print(f"步骤 2/3: 上传文件到 OSS...", file=sys.stderr)
    put = upload_to_oss(upload_url, file_path, mime_type)
    if put.state != 'success':
        return AttachmentResult(put.state, 'upload_file', attachment_data, put.error, result.meta)

    # 步骤 3: 返回 fileToken
    print(f"步骤 3/3: 上传完成！", file=sys.stderr)
    return AttachmentResult('success', 'complete', attachment_data, meta=result.meta)


def main() -> int:
    parser = argparse.ArgumentParser(description="上传附件到钉钉 AI 表格 attachment 字段")
    parser.add_argument("base_id", help="目标 AI 表格 baseId")
    parser.add_argument("file_path", help="待上传文件路径")
    add_contract_flags(parser)
    args = parser.parse_args()

    base_id = args.base_id
    file_path = args.file_path

    if not validate_resource_id(base_id):
        return failure(args.format, '无效的 baseId 格式')

    result = upload_attachment(base_id, file_path, dry_run=args.dry_run)
    if result.state != 'success':
        data = {**result.data, 'phase': result.phase,
                'execution_state': 'unknown' if result.state == 'unknown' else 'not_executed'}
        return emit(
            fmt=args.format,
            outcome='failure',
            data=data,
            error=result.error or {'type': 'api', 'message': '附件上传未确认完成'},
            meta=result.meta,
            text='附件上传未确认完成',
        )

    return emit(
        fmt=args.format,
        outcome='success',
        data=result.data,
        dry_run=args.dry_run,
        meta=result.meta,
        text=json.dumps(result.data, ensure_ascii=False, indent=2),
    )


if __name__ == '__main__':
    sys.exit(run_main(main))
