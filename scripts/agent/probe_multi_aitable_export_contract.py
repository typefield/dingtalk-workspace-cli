#!/usr/bin/env python3
"""Agent-side semantic probe for the AITable asynchronous export script.

It keeps every fake response and downloaded byte in a temporary directory and
only writes a concise Markdown evidence report.  This is an Agent review, not
a CI gate and not proof of a real tenant's export terminal state.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "skills" / "multi" / "dingtalk-aitable" / "scripts" / "aitable_export_via_task.py"
BASE_ID = "base_12345678"


def _single_json(process: subprocess.CompletedProcess[str]) -> tuple[bool, dict[str, Any] | None, str]:
    lines = [line for line in process.stdout.splitlines() if line.strip()]
    if len(lines) != 1:
        return False, None, f"stdout lines={len(lines)}"
    try:
        payload = json.loads(lines[0])
    except json.JSONDecodeError as exc:
        return False, None, f"invalid JSON: {exc}"
    return isinstance(payload, dict), payload if isinstance(payload, dict) else None, "ok"


def _fake_dws(path: Path) -> None:
    path.write_text(
        """#!/usr/bin/env python3
import json, os, sys
from pathlib import Path

log = Path(os.environ['EXPORT_PROBE_CALLS'])
log.write_text(log.read_text() + ' '.join(sys.argv[1:]) + '\\n' if log.exists() else ' '.join(sys.argv[1:]) + '\\n')
behavior = os.environ['EXPORT_PROBE_BEHAVIOR']
if '--task-id' not in sys.argv:
    if behavior == 'start_unknown':
        print('gateway unavailable', file=sys.stderr)
        raise SystemExit(1)
    print(json.dumps({'ok': True, 'data': {'taskId': 'task-1', 'fileName': 'server-report.xlsx'}}))
    raise SystemExit(0)
if behavior == 'poll_unknown':
    print(json.dumps({'ok': False, 'error': {'type': 'network', 'message': 'poll interrupted'}}))
    raise SystemExit(1)
if behavior == 'terminal_failed':
    print(json.dumps({'ok': True, 'data': {'taskId': 'task-1', 'status': 'failed'}}))
    raise SystemExit(0)
if behavior == 'pending':
    print(json.dumps({'ok': True, 'data': {'taskId': 'task-1', 'status': 'processing'}}))
    raise SystemExit(0)
print(json.dumps({'ok': True, 'data': {'taskId': 'task-1', 'status': 'succeeded', 'fileName': 'server-report.xlsx', 'downloadUrl': os.environ['EXPORT_PROBE_URL']}}))
""",
        encoding="utf-8",
    )
    path.chmod(0o755)


class _DownloadHandler(BaseHTTPRequestHandler):
    body = b"agent export probe bytes\n"

    def do_GET(self) -> None:  # noqa: N802 - HTTP hook name.
        if self.path != "/report":
            self.send_response(404)
            self.end_headers()
            return
        self.send_response(200)
        self.send_header("Content-Length", str(len(self.body)))
        self.end_headers()
        self.wfile.write(self.body)

    def log_message(self, _format: str, *_args: Any) -> None:
        return


def _run(temp_dir: Path, behavior: str, extra: list[str], download_url: str) -> subprocess.CompletedProcess[str]:
    calls = temp_dir / "calls.log"
    environment = os.environ.copy()
    environment.update({
        "EXPORT_PROBE_CALLS": str(calls),
        "EXPORT_PROBE_BEHAVIOR": behavior,
        "EXPORT_PROBE_URL": download_url,
    })
    return subprocess.run(
        [sys.executable, str(SCRIPT), BASE_ID, "--scope", "all", "--dws", str(temp_dir / "dws"), "--format", "json", *extra],
        cwd=temp_dir,
        capture_output=True,
        text=True,
        timeout=30,
        env=environment,
    )


def _record(results: list[tuple[str, str, str]], name: str, passed: bool, detail: str) -> None:
    results.append((name, "PASS" if passed else "FAIL", detail.replace("|", "\\|")))


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, help="write Markdown report; default stdout")
    args = parser.parse_args()
    results: list[tuple[str, str, str]] = []

    with tempfile.TemporaryDirectory(prefix="dws-aitable-export-probe-") as temp_name:
        temp_dir = Path(temp_name)
        _fake_dws(temp_dir / "dws")
        server = ThreadingHTTPServer(("127.0.0.1", 0), _DownloadHandler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        download_url = f"http://127.0.0.1:{server.server_port}/report"
        try:
            help_process = subprocess.run([sys.executable, str(SCRIPT), "--help"], cwd=temp_dir, capture_output=True, text=True, timeout=30)
            _record(results, "脚本 Help 可发现 output contract", help_process.returncode == 0 and "--format" in help_process.stdout and "--dry-run" in help_process.stdout and "--overwrite" in help_process.stdout, f"rc={help_process.returncode}")

            dry = _run(temp_dir, "pending", ["--dry-run"], download_url)
            valid, payload, detail = _single_json(dry)
            calls = (temp_dir / "calls.log").read_text() if (temp_dir / "calls.log").exists() else ""
            _record(results, "dry-run 不创建任务或落盘", valid and dry.returncode == 0 and payload is not None and payload.get("dry_run") is True and not calls, f"rc={dry.returncode}; {detail}; calls={bool(calls)}")

            invalid = subprocess.run([sys.executable, str(SCRIPT), BASE_ID, "--scope", "table", "--dws", str(temp_dir / "dws"), "--format", "json"], cwd=temp_dir, capture_output=True, text=True, timeout=30)
            valid, payload, detail = _single_json(invalid)
            _record(results, "scope 缺 tableId 在调用前拒绝", valid and invalid.returncode == 1 and payload is not None and payload.get("error", {}).get("type") == "validation", f"rc={invalid.returncode}; {detail}")

            pending = _run(temp_dir, "pending", ["--max-polls", "1"], download_url)
            valid, payload, detail = _single_json(pending)
            _record(results, "任务未完成返回可续 pending", valid and pending.returncode == 0 and payload is not None and payload.get("outcome") == "pending" and payload.get("data", {}).get("taskId") == "task-1" and str(payload.get("data", {}).get("next_command", "")).startswith("dws aitable export data"), f"rc={pending.returncode}; {detail}")

            unknown = _run(temp_dir, "poll_unknown", ["--max-polls", "1"], download_url)
            valid, payload, detail = _single_json(unknown)
            _record(results, "查询不可信不伪造任务失败", valid and unknown.returncode == 0 and payload is not None and payload.get("outcome") == "pending" and payload.get("data", {}).get("last_poll", {}).get("state") == "unknown", f"rc={unknown.returncode}; {detail}")

            failed = _run(temp_dir, "terminal_failed", ["--max-polls", "1"], download_url)
            valid, payload, detail = _single_json(failed)
            _record(results, "任务明确失败返回 typed failure", valid and failed.returncode == 1 and payload is not None and payload.get("error", {}).get("subtype") == "export_task_failed", f"rc={failed.returncode}; {detail}")

            no_download = _run(temp_dir, "ready", ["--no-download"], download_url)
            valid, payload, detail = _single_json(no_download)
            _record(results, "完成任务可只返回下载地址", valid and no_download.returncode == 0 and payload is not None and payload.get("outcome") == "success" and payload.get("data", {}).get("verification", {}).get("state") == "not_requested", f"rc={no_download.returncode}; {detail}")

            output = temp_dir / "completed.xlsx"
            downloaded = _run(temp_dir, "ready", ["--output", str(output)], download_url)
            valid, payload, detail = _single_json(downloaded)
            expected_hash = hashlib.sha256(_DownloadHandler.body).hexdigest()
            _record(results, "下载原子落盘且不夸大远端完整性", valid and downloaded.returncode == 0 and payload is not None and output.read_bytes() == _DownloadHandler.body and payload.get("data", {}).get("sha256") == expected_hash and payload.get("data", {}).get("verification", {}).get("source_integrity") == "unverified_no_remote_checksum" and not list(temp_dir.glob(".completed.xlsx.*.part")), f"rc={downloaded.returncode}; {detail}")

            existing = temp_dir / "existing.xlsx"
            existing.write_bytes(b"keep")
            calls_before = (temp_dir / "calls.log").read_text() if (temp_dir / "calls.log").exists() else ""
            blocked = _run(temp_dir, "ready", ["--output", str(existing)], download_url)
            valid, payload, detail = _single_json(blocked)
            calls_after = (temp_dir / "calls.log").read_text() if (temp_dir / "calls.log").exists() else ""
            _record(results, "已有输出须显式允许覆盖", valid and blocked.returncode == 1 and payload is not None and payload.get("error", {}).get("subtype") == "output_exists" and existing.read_bytes() == b"keep" and calls_after == calls_before, f"rc={blocked.returncode}; {detail}")
        finally:
            server.shutdown()
            server.server_close()

    passed = sum(status == "PASS" for _, status, _ in results)
    lines = [
        "# Multi AITable 异步导出 Agent 语义探针", "",
        "临时 child runner 与本地 HTTP server 只验证脚本编排、任务状态表达和原子落盘；不保存 JSON fixture，也不证明真实租户导出终态。", "",
        "| 检查 | 结果 | 证据 |", "|---|---|---|",
    ]
    lines.extend(f"| {name} | {status} | {detail} |" for name, status, detail in results)
    lines.extend(["", f"结论：**{passed}/{len(results)} PASS**。", "", "范围：验证 Help、预览、参数门禁、异步 pending、任务终态、下载原子性和覆盖保护；真实导出内容、下载 URL 权限与远端校验和仍须隔离账号复验。", ""])
    report = "\n".join(lines)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(report, encoding="utf-8")
    else:
        print(report)
    return 0 if passed == len(results) else 1


if __name__ == "__main__":
    raise SystemExit(main())
