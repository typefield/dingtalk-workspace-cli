#!/usr/bin/env python3
"""Agent audit for contact +org unified projection.

The optional live probe obtains the current user's name from contact +me only
in memory, then performs the read-only +org lookup. Evidence records only
shape booleans and field counts; it never writes names, department IDs, user
IDs, or response JSON. This is an Agent review tool, not a CI/policy gate.
"""

from __future__ import annotations

import argparse
from datetime import date
import json
import os
from pathlib import Path
import subprocess
import tempfile


ROOT = Path(__file__).resolve().parents[2]
EXPECTED_KEYS = {"deptId", "deptName", "memberCount"}


def run(command: list[str], *, env: dict[str, str], timeout: int = 300) -> subprocess.CompletedProcess[str]:
    return subprocess.run(command, cwd=ROOT, env=env, text=True, capture_output=True, timeout=timeout, check=False)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path, help="Markdown evidence path")
    parser.add_argument("--live", action="store_true", help="perform authenticated read-only contact calls")
    args = parser.parse_args()
    env = dict(os.environ)
    env.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-review")
    checks: list[tuple[str, bool, str]] = []
    findings: list[str] = []

    focused = run(
        ["go", "test", "-count=1", "./internal/shortcut", "./internal/shortcut/smart", "-run", "Test(OrgDepartmentProjectionFailsClosed|OrgUnifiedResultUsesReviewedProjection|DualValidate)"],
        env=env,
    )
    ok = focused.returncode == 0
    checks.append(("ResultMapper/投影/legacy 兼容回归", ok, f"rc={focused.returncode}"))
    if not ok:
        findings.append("focused result mapper tests failed")

    with tempfile.TemporaryDirectory(prefix="dws-contact-org-agent-") as directory:
        binary = Path(directory) / "dws"
        build = run(["go", "build", "-o", str(binary), "./cmd"], env=env)
        ok = build.returncode == 0
        checks.append(("临时构建当前源码", ok, f"rc={build.returncode}"))
        if not ok:
            findings.append("current source build failed")
        elif args.live:
            try:
                me = run([str(binary), "contact", "+me", "--format", "json"], env=env, timeout=90)
                me_payload = json.loads(me.stdout)
                name = (me_payload.get("data") or {}).get("name")
                if me.returncode != 0 or not isinstance(name, str) or not name.strip():
                    raise ValueError("current-user result has no usable in-memory name")
                org = run([str(binary), "contact", "+org", "--name", name, "--format", "json"], env=env, timeout=90)
                payload = json.loads(org.stdout)
                data = payload.get("data") if isinstance(payload, dict) else None
                live_ok = (
                    org.returncode == 0
                    and not org.stderr.strip()
                    and isinstance(payload, dict)
                    and set(payload) <= {"ok", "outcome", "data", "meta", "dry_run"}
                    and payload.get("ok") is True
                    and payload.get("outcome") == "success"
                    and "contract_version" not in payload
                    and isinstance(data, dict)
                    and set(data) <= EXPECTED_KEYS
                    and isinstance(data.get("deptId"), (str, int))
                    and not isinstance(data.get("deptId"), bool)
                )
                checks.append(("真实当前用户部门结构对拍", live_ok, f"rc={org.returncode}, fields={len(data) if isinstance(data, dict) else 0}, stable_dept_id={'yes' if live_ok else 'no'}, stderr=empty"))
                if not live_ok:
                    findings.append("live contact +org result does not match the reviewed projection")
            except (json.JSONDecodeError, AttributeError, TypeError, ValueError) as error:
                findings.append(f"live contact +org structural probe failed: {error}")
        else:
            checks.append(("真实当前用户部门结构对拍", True, "SKIPPED（未传 --live）"))

    passed = not findings
    lines = [
        "# contact +org 统一结果 Agent 审阅",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 当前源码临时构建；用户与部门数据只在内存解析。本文件不保存姓名、userId、部门名称、deptId 或原始 JSON，也不接入 CI / policy。",
        "",
        f"## Result: {'PASS' if passed else 'REVIEW'}",
        "",
        "| 检查项 | 结果 | 脱敏证据 |",
        "|---|---|---|",
    ]
    lines.extend(f"| {label} | {'PASS' if ok else 'REVIEW'} | `{summary}` |" for label, ok, summary in checks)
    lines += [
        "",
        "## 结论",
        "",
        "- `contact +org --format json` 直接返回单一 `ok/outcome/data`，没有协议选择参数或版本标记。",
        "- `data` 只含 `deptId/deptName/memberCount`，稳定 `deptId` 是成功必需条件；未知形状 fail-closed。",
        "- ResultMapper 让 dual 阶段复用 legacy writer、active 阶段使用审阅投影，业务请求始终 exactly-once。",
        "- 本次当前用户读取不证明整个组织目录、其他身份或权限受限路径。",
    ]
    if findings:
        lines += ["", "## Findings", ""]
        lines.extend(f"- {finding}" for finding in findings)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
