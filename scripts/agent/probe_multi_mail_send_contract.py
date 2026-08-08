#!/usr/bin/env python3
"""Agent semantic probe for the Multi Mail send-with-CC script.

The script uses temporary input and a fake ``dws`` only.  It records Markdown
evidence, never a JSON fixture, and does not prove real mail-server delivery.
"""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "skills" / "multi" / "dingtalk-mail" / "scripts" / "mail_send_with_cc.py"


def execute(args: list[str], env: dict[str, str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run([sys.executable, str(SCRIPT), *args], cwd=ROOT, env=env, capture_output=True, text=True, timeout=30)


def envelope(proc: subprocess.CompletedProcess[str]) -> tuple[dict[str, Any] | None, str]:
    lines = [line for line in proc.stdout.splitlines() if line.strip()]
    if len(lines) != 1:
        return None, f"stdout lines={len(lines)}"
    try:
        value = json.loads(lines[0])
    except json.JSONDecodeError as exc:
        return None, f"invalid JSON: {exc}"
    return (value, "ok") if isinstance(value, dict) else (None, "result is not an object")


def write_dws(path: Path, *, send: str, verify: str = "success") -> None:
    path.write_text(
        "#!/usr/bin/env python3\n"
        "import json, sys\n"
        "argv = sys.argv[1:]\n"
        "if argv[:3] == ['mail', 'mailbox', 'list']:\n"
        "  print(json.dumps({'ok': True, 'data': {'emailAccounts': [{'type': 'ORG', 'email': 'sender@example.test'}]}}))\n"
        "elif argv[:3] == ['mail', 'message', 'send']:\n"
        f"  print(json.dumps({send}))\n"
        "elif argv[:3] == ['mail', 'message', 'verify']:\n"
        f"  print(json.dumps({verify}))\n"
        "else:\n"
        "  print(json.dumps({'ok': False, 'error': {'type': 'internal', 'message': 'unexpected command'}}))\n",
        encoding="utf-8",
    )
    path.chmod(0o755)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, help="write Markdown report; default is stdout")
    args = parser.parse_args()
    outcomes: list[tuple[str, str, str]] = []
    with tempfile.TemporaryDirectory(prefix="dws-multi-mail-probe-") as directory:
        tmp = Path(directory)
        no_call_bin = tmp / "no-call-bin"
        no_call_bin.mkdir()
        sentinel = tmp / "unexpected-dws-call"
        no_call = no_call_bin / "dws"
        no_call.write_text(f"#!/bin/sh\ntouch '{sentinel}'\nexit 99\n", encoding="utf-8")
        no_call.chmod(0o755)
        base_env = {**os.environ, "PATH": f"{no_call_bin}{os.pathsep}{os.environ.get('PATH', '')}"}
        common = ["--to", "recipient@example.test", "--subject", "subject", "--body", "body", "--format", "json"]

        help_proc = execute(["--help"], base_env)
        help_ok = help_proc.returncode == 0 and "--format {text,json,ndjson}" in help_proc.stdout and "--dry-run" in help_proc.stdout and "--yes" in help_proc.stdout
        outcomes.append(("脚本 Help 可发现 format/dry-run/yes", "PASS" if help_ok else "FAIL", f"rc={help_proc.returncode}"))

        dry_run = execute([*common, "--dry-run"], base_env)
        payload, detail = envelope(dry_run)
        dry_ok = dry_run.returncode == 0 and payload is not None and payload.get("ok") is True and payload.get("dry_run") is True and not sentinel.exists()
        outcomes.append(("dry-run 零 child 调用且返回单信封", "PASS" if dry_ok else "FAIL", f"rc={dry_run.returncode}; {detail}; sentinel={sentinel.exists()}"))

        no_yes = execute(common, base_env)
        payload, detail = envelope(no_yes)
        confirmation_ok = no_yes.returncode == 1 and payload is not None and payload.get("error", {}).get("subtype") == "confirmation_required" and not sentinel.exists()
        outcomes.append(("未确认发送 fail-closed", "PASS" if confirmation_ok else "FAIL", f"rc={no_yes.returncode}; {detail}; sentinel={sentinel.exists()}"))

        invalid = execute(["--to", "not-an-email", "--subject", "subject", "--body", "body", "--format", "json"], base_env)
        payload, detail = envelope(invalid)
        invalid_ok = invalid.returncode == 1 and payload is not None and payload.get("error", {}).get("type") == "validation" and "Traceback" not in invalid.stderr
        outcomes.append(("输入校验错误不泄漏 traceback", "PASS" if invalid_ok else "FAIL", f"rc={invalid.returncode}; {detail}"))

        reject_bin = tmp / "reject-bin"
        reject_bin.mkdir()
        write_dws(reject_bin / "dws", send=repr({"ok": True, "data": {"success": False, "message": "auto send disabled"}}))
        rejected = execute([*common, "--yes"], {**base_env, "PATH": f"{reject_bin}{os.pathsep}{base_env['PATH']}"})
        payload, detail = envelope(rejected)
        reject_ok = rejected.returncode == 1 and payload is not None and payload.get("outcome") == "failure" and payload.get("error", {}).get("subtype") == "mail_send_rejected"
        outcomes.append(("旧业务 success:false 不误报已发送", "PASS" if reject_ok else "FAIL", f"rc={rejected.returncode}; {detail}"))

        verified_bin = tmp / "verified-bin"
        verified_bin.mkdir()
        write_dws(
            verified_bin / "dws",
            send=repr({"ok": True, "data": {"internetMessageId": "internet_1"}, "meta": {"request": "accepted"}}),
            verify=repr({"ok": True, "data": {"sendStatus": "success"}, "meta": {"readback": True}}),
        )
        verified = execute([*common, "--yes"], {**base_env, "PATH": f"{verified_bin}{os.pathsep}{base_env['PATH']}"})
        payload, detail = envelope(verified)
        verified_ok = verified.returncode == 0 and payload is not None and payload.get("outcome") == "success" and payload.get("data", {}).get("verification", {}).get("state") == "verified"
        outcomes.append(("发送后仅在 verify=success 时报告已验证", "PASS" if verified_ok else "FAIL", f"rc={verified.returncode}; {detail}"))

        pending_bin = tmp / "pending-bin"
        pending_bin.mkdir()
        write_dws(
            pending_bin / "dws",
            send=repr({"ok": True, "data": {"internetMessageId": "internet_2"}}),
            verify=repr({"ok": True, "data": {"sendStatus": "posting"}}),
        )
        pending = execute([*common, "--yes"], {**base_env, "PATH": f"{pending_bin}{os.pathsep}{base_env['PATH']}"})
        payload, detail = envelope(pending)
        pending_ok = pending.returncode == 1 and payload is not None and payload.get("data", {}).get("execution_state") == "unknown" and payload.get("error", {}).get("subtype") == "mail_delivery_not_terminal"
        outcomes.append(("投递中不伪装终态成功", "PASS" if pending_ok else "FAIL", f"rc={pending.returncode}; {detail}"))

        partial_bin = tmp / "partial-bin"
        partial_bin.mkdir()
        write_dws(
            partial_bin / "dws",
            send=repr({"ok": True, "data": {"internetMessageId": "internet_3"}}),
            verify=repr({"ok": True, "data": {"sendStatus": "partial_success"}}),
        )
        partial = execute([*common, "--yes"], {**base_env, "PATH": f"{partial_bin}{os.pathsep}{base_env['PATH']}"})
        payload, detail = envelope(partial)
        partial_ok = partial.returncode == 1 and payload is not None and payload.get("outcome") == "failure" and payload.get("data", {}).get("execution_state") == "partial_unknown" and payload.get("error", {}).get("subtype") == "mail_delivery_partial"
        outcomes.append(("无逐收件人明细的部分投递不伪造 partial 三通道", "PASS" if partial_ok else "FAIL", f"rc={partial.returncode}; {detail}"))

    passed = sum(1 for _, status, _ in outcomes if status == "PASS")
    lines = [
        "# Multi Mail 带抄送发送 Agent 语义探针", "",
        "临时 child runner 仅验证编排与结果表达；本报告不保存 JSON fixture，也不向真实邮箱发送邮件。", "",
        "| 检查 | 结果 | 证据 |", "|---|---|---|",
    ]
    for name, status, detail in outcomes:
        lines.append("| {} | {} | {} |".format(name, status, detail.replace("|", "\\|")))
    lines.extend(["", f"结论：**{passed}/{len(outcomes)} PASS**。", "", "范围：验证 Help、确认门禁、零写预览、旧业务失败、发送后 readback 与未终态表达；真实投递仍须隔离账号复验。", ""])
    report = "\n".join(lines)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(report, encoding="utf-8")
    else:
        print(report)
    return 0 if passed == len(outcomes) else 1


if __name__ == "__main__":
    raise SystemExit(main())
