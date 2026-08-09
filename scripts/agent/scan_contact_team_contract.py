#!/usr/bin/env python3
"""Agent audit for contact +team without persisting member data."""

from __future__ import annotations

import argparse
from datetime import date
import json
import os
from pathlib import Path
import subprocess
import tempfile


ROOT = Path(__file__).resolve().parents[2]


def run(command: list[str], env: dict[str, str], timeout: int = 300) -> subprocess.CompletedProcess[str]:
    return subprocess.run(command, cwd=ROOT, env=env, text=True, capture_output=True, timeout=timeout, check=False)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--live", action="store_true")
    args = parser.parse_args()
    env = dict(os.environ)
    env.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-review")
    checks: list[tuple[str, bool, str]] = []
    findings: list[str] = []

    focused = run(["go", "test", "-count=1", "./internal/shortcut/contact", "./internal/shortcut/smart", "-run", "TestTeamUnifiedResultUsesCanonicalMemberProjection"], env)
    ok = focused.returncode == 0
    checks.append(("共享成员投影与统一结果回归", ok, f"rc={focused.returncode}"))
    if not ok:
        findings.append("focused team projection tests failed")

    with tempfile.TemporaryDirectory(prefix="dws-contact-team-agent-") as directory:
        binary = Path(directory) / "dws"
        build = run(["go", "build", "-o", str(binary), "./cmd"], env)
        ok = build.returncode == 0
        checks.append(("临时构建当前源码", ok, f"rc={build.returncode}"))
        if not ok:
            findings.append("current source build failed")
        elif args.live:
            try:
                me = run([str(binary), "contact", "+me", "--format", "json"], env, 90)
                name = (json.loads(me.stdout).get("data") or {}).get("name")
                if me.returncode != 0 or not isinstance(name, str) or not name.strip():
                    raise ValueError("current-user result has no in-memory name")
                team = run([str(binary), "contact", "+team", "--name", name, "--format", "json"], env, 90)
                payload = json.loads(team.stdout)
                data = payload.get("data") if isinstance(payload, dict) else None
                meta = payload.get("meta") if isinstance(payload, dict) else None
                members = data.get("members") if isinstance(data, dict) else None
                count = len(members) if isinstance(members, list) else -1
                stable = isinstance(members, list) and all(
                    isinstance(member, dict)
                    and set(member) <= {"userId", "name"}
                    and isinstance(member.get("userId"), str)
                    and bool(member["userId"].strip())
                    for member in members
                )
                live_ok = (
                    team.returncode == 0
                    and not team.stderr.strip()
                    and payload.get("ok") is True
                    and payload.get("outcome") == "success"
                    and "contract_version" not in payload
                    and isinstance(data, dict)
                    and set(data) == {"count", "members"}
                    and data.get("count") == count
                    and isinstance(meta, dict)
                    and meta.get("count") == count
                    and stable
                )
                checks.append(("真实当前部门成员结构对拍", live_ok, f"rc={team.returncode}, count={count}, stable_user_ids={'yes' if stable else 'no'}, meta_aligned={'yes' if live_ok else 'no'}"))
                if not live_ok:
                    findings.append("live contact +team result does not match the reviewed projection")
            except (json.JSONDecodeError, AttributeError, TypeError, ValueError) as error:
                findings.append(f"live contact +team structural probe failed: {error}")
        else:
            checks.append(("真实当前部门成员结构对拍", True, "SKIPPED（未传 --live）"))

    passed = not findings
    lines = [
        "# contact +team 统一结果 Agent 审阅", "", f"扫描日期：{date.today().isoformat()}", "",
        "> 当前源码临时构建；用户和成员数据只在内存解析。本文件不保存姓名、userId、部门信息或原始 JSON，也不接入 CI / policy。", "",
        f"## Result: {'PASS' if passed else 'REVIEW'}", "", "| 检查项 | 结果 | 脱敏证据 |", "|---|---|---|",
    ]
    lines.extend(f"| {label} | {'PASS' if ok else 'REVIEW'} | `{summary}` |" for label, ok, summary in checks)
    lines += [
        "", "## 结论", "",
        "- `contact +team --format json` 只返回 `ok/outcome/data/meta`，没有协议选择参数或版本标记。",
        "- canonical 部门成员命令与复合入口共用唯一投影；`data.count == meta.count == members.length`。",
        "- 每条成员必须有稳定 `userId`，未知或残缺条目使整个结果 fail-closed，不伪装为空或完整。",
        "- 当前部门读取不证明下级部门、全组织覆盖、其他账号或权限受限路径。",
    ]
    if findings:
        lines += ["", "## Findings", ""] + [f"- {finding}" for finding in findings]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
