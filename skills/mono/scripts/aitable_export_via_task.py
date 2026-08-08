#!/usr/bin/env python3
"""
通过 MCP 导出任务（export_data）导出 AI 表格，并可自动下载文件。

与普通命令的区别：
- 自动处理 taskId 轮询（直到拿到 downloadUrl 或达到轮询上限）。
- 自动保存导出文件到本地（可选 --output）。

用法:
    python scripts/aitable_export_via_task.py <baseId> --scope all
    python scripts/aitable_export_via_task.py <baseId> --scope table --table-id <tableId>
    python scripts/aitable_export_via_task.py <baseId> --scope view --table-id <tableId> --view-id <viewId>
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
import time
from pathlib import Path
from typing import Any, Dict, Optional, Tuple
from urllib.error import HTTPError, URLError
from urllib.parse import urlparse
from urllib.request import Request, urlopen

from _runtime import add_contract_flags, emit, failure, run_main

RESOURCE_ID_PATTERN = re.compile(r"^[A-Za-z0-9_-]{8,128}$")
ALLOWED_FORMATS = {"excel", "attachment", "excel_and_attachment", "excel_with_inline_images"}


def validate_resource_id(resource_id: str) -> bool:
    return bool(resource_id and RESOURCE_ID_PATTERN.match(resource_id.strip()))


def run_dws(dws_bin: str, args: list[str], timeout_sec: int = 120) -> Tuple[int, str, str]:
    cmd = [dws_bin] + args
    try:
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout_sec)
        return result.returncode, result.stdout.strip(), result.stderr.strip()
    except subprocess.TimeoutExpired:
        return 124, "", f"dws command timeout after {timeout_sec}s"
    except FileNotFoundError:
        return 127, "", f"dws binary not found: {dws_bin}"


def parse_json_output(raw: str) -> Optional[Dict[str, Any]]:
    try:
        obj = json.loads(raw)
        return obj if isinstance(obj, dict) else None
    except json.JSONDecodeError:
        return None


def normalize_download_url(url: str) -> str:
    if url.startswith("http://") or url.startswith("https://"):
        return url
    return f"https://{url}"


def download_file(url: str, output_path: Path) -> Tuple[bool, str]:
    req = Request(url, method="GET")
    try:
        with urlopen(req, timeout=180) as resp:
            if resp.status != 200:
                return False, f"download http status: {resp.status}"
            output_path.write_bytes(resp.read())
            return True, ""
    except HTTPError as e:
        body = e.read().decode("utf-8", "ignore")
        return False, f"HTTP {e.code}: {body[:300]}"
    except URLError as e:
        return False, f"URL error: {e.reason}"


def fail(msg: str, fmt: str) -> int:
    return failure(fmt, msg, text=f"错误：{msg}")


def build_start_args(args: argparse.Namespace) -> list[str]:
    cmd = [
        "aitable",
        "export",
        "data",
        "--base-id",
        args.base_id,
        "--scope",
        args.scope,
        "--format",
        args.export_format,
        # CLI 的 --timeout-ms 是单次等待上限（毫秒，最大 30000）；脚本自身的
        # --timeout-sec 用于整体轮询/子进程超时，二者语义不同，不能混用。
        "--timeout-ms",
        "30000",
    ]
    if args.table_id:
        cmd.extend(["--table-id", args.table_id])
    if args.view_id:
        cmd.extend(["--view-id", args.view_id])
    return cmd


def main() -> int:
    parser = argparse.ArgumentParser(description="通过 MCP 导出任务导出 AI 表格")
    parser.add_argument("base_id", help="目标 AI 表格 baseId")
    parser.add_argument("--scope", choices=["all", "table", "view"], required=True, help="导出范围")
    parser.add_argument("--table-id", help="scope=table/view 时必填")
    parser.add_argument("--view-id", help="scope=view 时必填")
    parser.add_argument("--export-format", default="excel", choices=sorted(ALLOWED_FORMATS), help="导出格式")
    parser.add_argument("--timeout-sec", type=int, default=300, help="CLI 内置轮询整体超时（秒），默认 300（5 分钟）")
    parser.add_argument("--max-polls", type=int, default=10, help="（兼容旧参数，开源 CLI 内置轮询不再使用）")
    parser.add_argument("--output", help="本地保存路径（不传则按 fileName 保存到当前目录）")
    parser.add_argument("--dws", default="dws", help="dws 可执行文件路径，默认 dws")
    parser.add_argument("--no-download", action="store_true", help="仅返回 downloadUrl，不下载文件")
    add_contract_flags(parser)
    args = parser.parse_args()

    if not validate_resource_id(args.base_id):
        return fail("无效的 baseId 格式", args.format)
    if args.scope in ("table", "view") and not args.table_id:
        return fail("scope=table/view 时必须传 --table-id", args.format)
    if args.scope == "view" and not args.view_id:
        return fail("scope=view 时必须传 --view-id", args.format)

    if args.dry_run:
        return emit(
            fmt=args.format,
            outcome="success",
            data={"plan": {
                "baseId": args.base_id,
                "scope": args.scope,
                "exportFormat": args.export_format,
                "download": not args.no_download,
            }},
            dry_run=True,
            text="[dry-run] 将启动导出任务、轮询 taskId 并下载结果文件。",
        )

    print("[1/2] start export task", file=sys.stderr)
    rc, out, err = run_dws(args.dws, build_start_args(args), timeout_sec=120)
    if rc != 0:
        return fail(f"export_data 启动失败: {err or out}", args.format)
    obj = parse_json_output(out)
    if not obj:
        return fail(f"export_data 返回非 JSON: {out[:300]}", args.format)

    data = obj.get("data", {}) or {}
    status = obj.get("status")
    if status == "error":
        return fail(f"export_data 返回失败: {json.dumps(obj, ensure_ascii=False)}", args.format)

    download_url = data.get("downloadUrl")
    task_id = data.get("taskId")
    file_name = data.get("fileName") or "export_result.bin"

    polls = 0
    while not download_url and task_id and polls < args.max_polls:
        polls += 1
        print(f"[2/2] polling task ({polls}/{args.max_polls})", file=sys.stderr)
        rc2, out2, err2 = run_dws(
            args.dws,
            [
                "aitable",
                "export",
                "data",
                "--base-id",
                args.base_id,
                "--task-id",
                task_id,
                "--timeout-ms",
                "30000",
            ],
            timeout_sec=max(120, args.timeout_sec + 60),
        )
        if rc2 != 0:
            return fail(f"export_data 轮询失败: {err2 or out2}", args.format)
        obj2 = parse_json_output(out2)
        if not obj2:
            return fail(f"export_data 轮询返回非 JSON: {out2[:300]}", args.format)
        if obj2.get("status") == "error":
            return fail(f"export_data 轮询返回失败: {json.dumps(obj2, ensure_ascii=False)}", args.format)
        d2 = obj2.get("data", {}) or {}
        download_url = d2.get("downloadUrl") or download_url
        file_name = d2.get("fileName") or file_name
        task_id = d2.get("taskId") or task_id
        if not download_url:
            time.sleep(0.2)

    result: Dict[str, Any] = {
        "baseId": args.base_id,
        "scope": args.scope,
        "exportFormat": args.export_format,
        "taskId": task_id,
        "fileName": file_name,
        "downloadUrl": download_url,
        "polledTimes": polls,
    }

    if not download_url:
        result["status"] = "pending"
        result["summary"] = "导出任务仍在处理中，请继续用 taskId 轮询。"
        return emit(fmt=args.format, outcome="pending", data=result,
                    text=json.dumps(result, ensure_ascii=False, indent=2))

    if args.no_download:
        result["status"] = "success"
        result["summary"] = "导出完成（未下载文件）。"
        return emit(fmt=args.format, outcome="success", data=result,
                    text=json.dumps(result, ensure_ascii=False, indent=2))

    norm_url = normalize_download_url(download_url)
    output_path = Path(args.output).expanduser().resolve() if args.output else Path.cwd() / file_name
    ok, dl_err = download_file(norm_url, output_path)
    if not ok:
        return fail(f"downloadUrl 下载失败: {dl_err}", args.format)

    result["status"] = "success"
    result["summary"] = "导出完成并已下载。"
    result["savedPath"] = str(output_path)
    return emit(fmt=args.format, outcome="success", data=result,
                text=json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    sys.exit(run_main(main))
