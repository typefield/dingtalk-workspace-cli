#!/usr/bin/env python3
"""Agent-side evidence probe for Multi AITable write-script safety.

This is deliberately a semantic probe, not a CI gate.  It keeps all fake
commands, local HTTP endpoints, and input files in a temporary directory and
writes only human-readable Markdown when ``--output`` is supplied.
"""

from __future__ import annotations

import argparse
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
SCRIPT_DIR = ROOT / "skills" / "multi" / "dingtalk-aitable" / "scripts"
BASE = "ABCdef12"
TABLE = "ABCdef13"


def run(script: Path, args: list[str], env: dict[str, str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [sys.executable, str(script), *args], cwd=ROOT, env=env,
        capture_output=True, text=True, timeout=30,
    )


def one_result(proc: subprocess.CompletedProcess[str]) -> tuple[dict[str, Any] | None, str]:
    lines = [line for line in proc.stdout.splitlines() if line.strip()]
    if len(lines) != 1:
        return None, f"stdout lines={len(lines)}"
    try:
        payload = json.loads(lines[0])
    except json.JSONDecodeError as exc:
        return None, f"invalid JSON: {exc}"
    return (payload, "ok") if isinstance(payload, dict) else (None, "JSON result is not an object")


def check_plan(
    name: str, script: Path, args: list[str], env: dict[str, str], *, machine_contract: bool,
) -> tuple[str, str]:
    command = [*args, "--dry-run"]
    if machine_contract:
        command.extend(["--format", "json"])
    proc = run(script, command, env)
    if machine_contract:
        payload, detail = one_result(proc)
    else:
        try:
            payload = json.loads(proc.stdout)
            detail = "legacy JSON plan"
        except json.JSONDecodeError as exc:
            payload, detail = None, f"invalid JSON plan: {exc}"
    passed = proc.returncode == 0 and isinstance(payload, dict) and payload.get("ok") is True and payload.get("dry_run") is True
    if machine_contract:
        passed = passed and payload.get("outcome") == "success"
    return ("PASS" if passed else "FAIL", f"{name}: rc={proc.returncode}; {detail}")


def start_put_server() -> tuple[ThreadingHTTPServer, str, list[int]]:
    requests: list[int] = []

    class Handler(BaseHTTPRequestHandler):
        def do_PUT(self) -> None:  # noqa: N802 - HTTP handler spelling.
            length = int(self.headers.get("Content-Length", "0"))
            self.rfile.read(length)
            requests.append(length)
            self.send_response(200)
            self.end_headers()

        def log_message(self, _format: str, *_args: object) -> None:
            return

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    host, port = server.server_address[:2]
    return server, f"http://{host}:{port}/upload", requests


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, help="write Markdown report; default is stdout")
    args = parser.parse_args()
    outcomes: list[tuple[str, str, str]] = []

    with tempfile.TemporaryDirectory(prefix="dws-multi-aitable-probe-") as directory:
        tmp = Path(directory)
        bin_dir = tmp / "bin"
        bin_dir.mkdir()
        sentinel = tmp / "unexpected-dws-call"
        fake = bin_dir / "dws"
        fake.write_text(f"#!/bin/sh\ntouch '{sentinel}'\nexit 99\n", encoding="utf-8")
        fake.chmod(0o755)

        csv = tmp / "records.csv"
        csv.write_text("a,b\n1,2\n3,4\n", encoding="utf-8")
        fields = tmp / "fields.json"
        fields.write_text('[{"fieldName":"A","type":"text"}]', encoding="utf-8")
        attachment = tmp / "attachment.txt"
        attachment.write_text("x", encoding="utf-8")
        env = os.environ.copy()
        env["PATH"] = f"{bin_dir}{os.pathsep}{env.get('PATH', '')}"
        env["OPENCLAW_WORKSPACE"] = str(tmp)

        checks = [
            ("aitable_import_via_task", SCRIPT_DIR / "aitable_import_via_task.py", [BASE, str(csv), "--dws", str(fake)], True),
            ("bulk_add_fields", SCRIPT_DIR / "bulk_add_fields.py", [BASE, TABLE, str(fields)], True),
            ("import_records", SCRIPT_DIR / "import_records.py", [BASE, TABLE, str(csv)], True),
            ("upload_attachment", SCRIPT_DIR / "upload_attachment.py", [BASE, str(attachment)], True),
        ]
        for name, script, command_args, machine_contract in checks:
            status, detail = check_plan(name, script, command_args, env, machine_contract=machine_contract)
            suffix = "统一契约" if machine_contract else "legacy 预览"
            outcomes.append((name + " dry-run 零写入（" + suffix + "）", status, detail))
        outcomes.append((
            "dry-run 未调用 dws/OSS",
            "PASS" if not sentinel.exists() else "FAIL",
            "sentinel 未出现" if not sentinel.exists() else "sentinel 被触发",
        ))

        migrated_help = subprocess.run(
            [sys.executable, str(SCRIPT_DIR / "aitable_import_via_task.py"), "--help"],
            cwd=ROOT, capture_output=True, text=True, timeout=30,
        )
        required = {"--format {text,json,ndjson}", "--dry-run", "--timeout"}
        # Help spacing may change with argparse, so require the literal flags.
        help_ok = migrated_help.returncode == 0 and all(flag in migrated_help.stdout for flag in required)
        outcomes.append((
            "文件导入可发现契约",
            "PASS" if help_ok else "FAIL",
            f"rc={migrated_help.returncode}; flags={','.join(sorted(required))}",
        ))

        runtime_exception = subprocess.run(
            [
                sys.executable, "-c",
                "import sys; sys.path.insert(0, 'skills/multi/dingtalk-aitable/scripts'); "
                "import _runtime; raise SystemExit(_runtime.run_main(lambda: (_ for _ in ()).throw(AttributeError('probe secret')), default_format='json'))",
            ],
            cwd=ROOT, capture_output=True, text=True, timeout=30,
        )
        payload, detail = one_result(runtime_exception)
        exception_ok = (
            runtime_exception.returncode == 1 and payload is not None
            and payload.get("ok") is False and payload.get("outcome") == "failure"
            and isinstance(payload.get("error"), dict) and payload["error"].get("type") == "internal"
            and "Traceback" not in runtime_exception.stderr and "probe secret" not in runtime_exception.stderr
        )
        outcomes.append(("未捕获异常 JSON 兜底", "PASS" if exception_ok else "FAIL", detail))

        put_server, upload_url, put_requests = start_put_server()
        fake_result = tmp / "dws-result"
        fake_result.write_text(
            "#!/usr/bin/env python3\n"
            "import json, sys\n"
            "if sys.argv[1:4] == ['aitable','import','upload']:\n"
            f"  print(json.dumps({{'ok': True, 'data': {{'uploadUrl': {upload_url!r}, 'importId': 'import_probe'}}}}))\n"
            "else:\n"
            "  print(json.dumps({'success': False, 'error': {'type': 'api', 'message': 'ambiguous trigger'}}))\n",
            encoding="utf-8",
        )
        fake_result.chmod(0o755)
        try:
            fake_success = tmp / "dws-success"
            fake_success.write_text(
                "#!/usr/bin/env python3\n"
                "import json, sys\n"
                "if sys.argv[1:4] == ['aitable','import','upload']:\n"
                f"  print(json.dumps({{'ok': True, 'data': {{'uploadUrl': {upload_url!r}, 'importId': 'import_success'}}}}))\n"
                "else:\n"
                "  print(json.dumps({'ok': True, 'data': {'status': 'success', 'tableIds': ['table_probe']}, 'meta': {'task': 'done'}}))\n",
                encoding="utf-8",
            )
            fake_success.chmod(0o755)
            success = run(
                SCRIPT_DIR / "aitable_import_via_task.py",
                [BASE, str(csv), "--dws", str(fake_success), "--format", "json"], env,
            )
            ambiguous = run(
                SCRIPT_DIR / "aitable_import_via_task.py",
                [BASE, str(csv), "--dws", str(fake_result), "--format", "json"], env,
            )
            attachment_success_bin = tmp / "attachment-success-bin"
            attachment_success_bin.mkdir()
            fake_attachment = attachment_success_bin / "dws"
            fake_attachment.write_text(
                "#!/usr/bin/env python3\n"
                "import json\n"
                f"print(json.dumps({{'ok': True, 'data': {{'uploadUrl': {upload_url!r}, 'fileToken': 'file_success'}}, 'meta': {{'transfer': 'prepared'}}}}))\n",
                encoding="utf-8",
            )
            fake_attachment.chmod(0o755)
            attachment_success = run(
                SCRIPT_DIR / "upload_attachment.py",
                [BASE, str(attachment)],
                {**env, "PATH": f"{attachment_success_bin}{os.pathsep}{env['PATH']}"},
            )
            attachment_unknown_bin = tmp / "attachment-unknown-bin"
            attachment_unknown_bin.mkdir()
            fake_attachment_unknown = attachment_unknown_bin / "dws"
            fake_attachment_unknown.write_text(
                "#!/usr/bin/env python3\n"
                "import json\n"
                "print(json.dumps({'ok': True, 'data': {'uploadUrl': 'http://127.0.0.1:1/upload', 'fileToken': 'file_unknown'}}))\n",
                encoding="utf-8",
            )
            fake_attachment_unknown.chmod(0o755)
            attachment_unknown = run(
                SCRIPT_DIR / "upload_attachment.py",
                [BASE, str(attachment)],
                {**env, "PATH": f"{attachment_unknown_bin}{os.pathsep}{env['PATH']}"},
            )
        finally:
            put_server.shutdown()
            put_server.server_close()
        payload, detail = one_result(success)
        success_ok = (
            success.returncode == 0 and payload is not None
            and payload.get("ok") is True and payload.get("outcome") == "success"
            and payload.get("data", {}).get("importId") == "import_success"
            and payload.get("data", {}).get("status") == "success"
            and payload.get("meta") == {"task": "done"}
        )
        outcomes.append(("当前信封成功回包透传", "PASS" if success_ok else "FAIL", f"rc={success.returncode}; {detail}"))
        payload, detail = one_result(ambiguous)
        ambiguous_ok = (
            ambiguous.returncode == 1 and payload is not None
            and payload.get("ok") is False and payload.get("outcome") == "failure"
            and payload.get("data", {}).get("importId") == "import_probe"
            and payload.get("data", {}).get("phase") == "import_data"
            and payload.get("data", {}).get("execution_state") == "unknown"
            and len(put_requests) == 3
        )
        outcomes.append(("导入触发不确定不伪装成功", "PASS" if ambiguous_ok else "FAIL", f"rc={ambiguous.returncode}; {detail}"))

        payload, detail = one_result(attachment_success)
        attachment_success_ok = (
            attachment_success.returncode == 0 and payload is not None
            and payload.get("ok") is True and payload.get("outcome") == "success"
            and payload.get("data", {}).get("fileToken") == "file_success"
            and payload.get("meta") == {"transfer": "prepared"}
        )
        outcomes.append(("附件 PUT 确认后才返回 fileToken", "PASS" if attachment_success_ok else "FAIL", f"rc={attachment_success.returncode}; {detail}"))

        payload, detail = one_result(attachment_unknown)
        attachment_unknown_ok = (
            attachment_unknown.returncode == 1 and payload is not None
            and payload.get("ok") is False and payload.get("outcome") == "failure"
            and payload.get("data", {}).get("fileToken") == "file_unknown"
            and payload.get("data", {}).get("phase") == "upload_file"
            and payload.get("data", {}).get("execution_state") == "unknown"
        )
        outcomes.append(("附件 PUT 未知不误报可用", "PASS" if attachment_unknown_ok else "FAIL", f"rc={attachment_unknown.returncode}; {detail}"))

        record_bin = tmp / "record-batch-bin"
        record_bin.mkdir()
        record_counter = tmp / "record-batch-count"
        record_dws = record_bin / "dws"
        record_dws.write_text(
            "#!/usr/bin/env python3\n"
            "import json, os\n"
            "counter = os.environ['PROBE_RECORD_COUNTER']\n"
            "try:\n"
            "  count = int(open(counter, encoding='utf-8').read())\n"
            "except FileNotFoundError:\n"
            "  count = 0\n"
            "count += 1\n"
            "open(counter, 'w', encoding='utf-8').write(str(count))\n"
            "if count == 1:\n"
            "  print(json.dumps({'ok': True, 'data': {'newRecordIds': ['record_1']}}))\n"
            "else:\n"
            "  print(json.dumps({'success': False, 'error': {'type': 'api', 'message': 'ambiguous second batch'}}))\n",
            encoding="utf-8",
        )
        record_dws.chmod(0o755)
        record_partial = run(
            SCRIPT_DIR / "import_records.py",
            [BASE, TABLE, str(csv), "1", "--format", "json"],
            {**env, "PATH": f"{record_bin}{os.pathsep}{env['PATH']}", "PROBE_RECORD_COUNTER": str(record_counter)},
        )
        payload, detail = one_result(record_partial)
        record_partial_ok = (
            record_partial.returncode == 7 and payload is not None
            and payload.get("ok") is False and payload.get("outcome") == "partial_failure"
            and payload.get("data", {}).get("succeeded", [{}])[0].get("id") == "batch:1"
            and payload.get("data", {}).get("unknown", [{}])[0].get("id") == "batch:2"
        )
        outcomes.append(("记录导入保留成功与未知批次", "PASS" if record_partial_ok else "FAIL", f"rc={record_partial.returncode}; {detail}"))

        field_bin = tmp / "field-create-bin"
        field_bin.mkdir()
        field_dws = field_bin / "dws"
        field_dws.write_text(
            "#!/usr/bin/env python3\n"
            "import json\n"
            "print(json.dumps({'success': False, 'error': {'type': 'api', 'message': 'ambiguous field create'}}))\n",
            encoding="utf-8",
        )
        field_dws.chmod(0o755)
        field_unknown = run(
            SCRIPT_DIR / "bulk_add_fields.py",
            [BASE, TABLE, str(fields), "--format", "json"],
            {**env, "PATH": f"{field_bin}{os.pathsep}{env['PATH']}"},
        )
        payload, detail = one_result(field_unknown)
        field_unknown_ok = (
            field_unknown.returncode == 1 and payload is not None
            and payload.get("ok") is False and payload.get("outcome") == "failure"
            and payload.get("data", {}).get("execution_state") == "unknown"
            and payload.get("error", {}).get("type") == "api"
        )
        outcomes.append(("字段创建旧业务失败不误报成功", "PASS" if field_unknown_ok else "FAIL", f"rc={field_unknown.returncode}; {detail}"))

    passed = sum(1 for _, status, _ in outcomes if status == "PASS")
    report = ["# Multi AITable 脚本 Agent 语义探针", "", "临时 child runner、HTTP PUT 服务和输入文件仅在本次执行期间存在；本报告不保存 JSON fixture。", "", "| 检查 | 结果 | 证据 |", "|---|---|---|"]
    report.extend(
        "| {} | {} | {} |".format(name, status, detail.replace("|", "\\|"))
        for name, status, detail in outcomes
    )
    report.extend(["", f"结论：**{passed}/{len(outcomes)} PASS**。", "", "范围：文件导入、字段批量创建、附件上传与记录批量导入均已迁入 Multi 共享结果边界。受控 child runner 只能证明本地分类、stdout 与零写预览，不代替真实服务端终态验证。", ""])
    rendered = "\n".join(report)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(rendered, encoding="utf-8")
    else:
        print(rendered)
    return 0 if passed == len(outcomes) else 1


if __name__ == "__main__":
    raise SystemExit(main())
