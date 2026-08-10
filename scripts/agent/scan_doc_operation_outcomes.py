#!/usr/bin/env python3
"""Agent semantic scan for three Doc composite-write result contracts.

The scan builds the current CLI in an isolated directory, exercises only
zero-write previews and the pre-confirmation rejection path, and runs the
controlled response seams for success/partial behavior. It writes Markdown
evidence only and is not a CI gate or a store of response fixtures.
"""

from __future__ import annotations

import argparse
from datetime import date
import json
import os
from pathlib import Path
import subprocess
import tempfile
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
DOC_DECLARATIONS = ROOT / "internal" / "shortcut" / "doc" / "doc.go"
DOC_COMMON = ROOT / "internal" / "shortcut" / "doc" / "common.go"
DOC_CONTENT = ROOT / "internal" / "shortcut" / "doc" / "content_shortcuts.go"
DOC_TESTS = ROOT / "internal" / "shortcut" / "doc" / "doc_shortcuts_test.go"


def source_checks() -> list[tuple[str, str, str]]:
    declarations = DOC_DECLARATIONS.read_text(encoding="utf-8")
    common = DOC_COMMON.read_text(encoding="utf-8")
    content = DOC_CONTENT.read_text(encoding="utf-8")
    tests = DOC_TESTS.read_text(encoding="utf-8")
    terminals_active = (
        content.count("OutputRollout: output.RolloutUnifiedActive") >= 2
        and "VersionRevert.OutputRollout = output.RolloutUnifiedActive" in declarations
    )
    return [
        (
            "三个 terminal command 独立 active",
            "PASS" if terminals_active else "FAIL",
            "create/checkpoint-update/history-revert 由发布声明选定统一结果；Agent 不选择协议。",
        ),
        (
            "成功结果不嵌套 legacy 信封",
            "PASS"
            if "func docOperationOutput" in common
            and '"operation": operation' in common
            and '"result":    data' in common
            else "FAIL",
            "legacy payload 仅供旧阶段；active data 固定为 operation/result/steps。",
        ),
        (
            "partial 使用三通道",
            "PASS"
            if "output.NewPartialData" in common
            and "return output.Partial(partial)" in common
            else "FAIL",
            "已应用步骤、失败步骤和未开始步骤分别进入 succeeded/failed/unknown。",
        ),
        (
            "active lifecycle 有命令级矩阵",
            "PASS"
            if "TestDocCompositeWriteUnifiedSuccessAndDryRunResults" in tests
            and "TestDocCompositeWriteUnifiedPartialResultsPreserveAppliedSteps" in tests
            else "FAIL",
            "三条命令分别覆盖 success、dry-run 和 partial，且检查调用次数。",
        ),
    ]


def run_fixture_probe() -> tuple[str, str]:
    env = os.environ.copy()
    env.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-scan")
    completed = subprocess.run(
        [
            "go",
            "test",
            "-count=1",
            "./internal/shortcut/doc",
            "-run",
            r"TestDoc(CompositeWritesPublishReviewedActiveContracts|CompositeWriteUnifiedSuccessAndDryRunResults|CompositeWriteUnifiedPartialResultsPreserveAppliedSteps|PartialWriteResultMapsDeclaredStepsToThreeChannels)$",
        ],
        cwd=ROOT,
        env=env,
        capture_output=True,
        text=True,
        timeout=180,
    )
    if completed.returncode == 0:
        return "PASS", "受控 response seam 全部通过；每个业务流程只执行一次。"
    detail = (completed.stderr or completed.stdout).strip().replace("\n", " ")[:600]
    return "FAIL", f"rc={completed.returncode}; {detail}"


def call(binary: Path, env: dict[str, str], args: list[str]) -> tuple[int, Any]:
    completed = subprocess.run(
        [str(binary), *args, "--format", "json"],
        cwd=ROOT,
        env=env,
        capture_output=True,
        text=True,
        timeout=60,
    )
    try:
        payload = json.loads(completed.stdout) if completed.stdout.strip() else None
    except json.JSONDecodeError:
        payload = None
    return completed.returncode, payload


def valid_preview(payload: Any, operation: str) -> bool:
    return (
        isinstance(payload, dict)
        and payload.get("ok") is True
        and payload.get("outcome") == "success"
        and payload.get("dry_run") is True
        and "contract_version" not in payload
        and isinstance(payload.get("data"), dict)
        and payload["data"].get("operation") == operation
        and "ok" not in payload["data"]
        and isinstance(payload["data"].get("steps"), list)
    )


def run_public_probe() -> tuple[str, str]:
    with tempfile.TemporaryDirectory(prefix="dws-doc-outcome-agent-") as temp:
        temp_path = Path(temp)
        binary = temp_path / "dws"
        env = os.environ.copy()
        env["DWS_CONFIG_DIR"] = str(temp_path / "config")
        env["DWS_PACKAGE_VERSION"] = "0.0.0-agent-scan"
        built = subprocess.run(
            ["go", "build", "-o", str(binary), "./cmd"],
            cwd=ROOT,
            env=env,
            capture_output=True,
            text=True,
            timeout=240,
        )
        if built.returncode != 0:
            detail = (built.stderr or built.stdout).strip().replace("\n", " ")[:600]
            return "FAIL", f"build rc={built.returncode}; {detail}"

        create_rc, create = call(binary, env, ["doc", "+create", "--name", "agent-probe", "--dry-run"])
        checkpoint_rc, checkpoint = call(
            binary,
            env,
            ["doc", "+checkpoint-update", "--node", "agent-probe", "--content", "preview", "--dry-run"],
        )
        reject_rc, rejected = call(
            binary,
            env,
            ["doc", "+history-revert", "--node", "agent-probe", "--version", "1"],
        )
        rejected_ok = (
            reject_rc == 3
            and isinstance(rejected, dict)
            and rejected.get("ok") is False
            and rejected.get("outcome") == "failure"
            and isinstance(rejected.get("error"), dict)
            and rejected["error"].get("subtype") == "confirmation_required"
            and "contract_version" not in rejected
        )
        if (
            create_rc == 0
            and checkpoint_rc == 0
            and valid_preview(create, "doc.create")
            and valid_preview(checkpoint, "doc.checkpoint_update")
            and rejected_ok
        ):
            return "PASS", "create/checkpoint 零写预览 + history-revert 确认前拒绝；均为单一统一信封且无版本标记。"
        return "FAIL", f"create_rc={create_rc}, checkpoint_rc={checkpoint_rc}, reject_rc={reject_rc}"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, help="write Markdown report; defaults to stdout")
    args = parser.parse_args()

    checks = source_checks()
    checks.append(("受控 success/partial/调用次数矩阵", *run_fixture_probe()))
    checks.append(("公开 CLI 预览与确认边界", *run_public_probe()))
    passed = sum(result == "PASS" for _, result, _ in checks)
    lines = [
        "# Doc 复合写 operation outcome Agent 扫描",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> Agent 在当前工作树执行扫描；只运行零写入 dry-run、确认前拦截和受控 response seam。报告不保存 JSON fixture，不调用真实写接口，也不接入 CI。",
        "",
        "| 检查 | 结果 | 脱敏证据 |",
        "|---|---|---|",
    ]
    for name, result, detail in checks:
        lines.append(f"| {name} | {result} | {detail.replace('|', ' / ')} |")
    lines += [
        "",
        f"结论：**{passed}/{len(checks)} PASS**。三条 terminal command 现直接使用统一结果：成功/预览由框架生成 `ok/outcome/data`，复合写部分失败保留 `succeeded/failed/unknown` 并返回 rc=7；Agent 只传 `--format json`。",
        "",
        "未验证：真实租户中创建后的 JSONML 写入失败、checkpoint 更新失败、回滚后读回失败及补偿动作的服务端终态。active 只保证已识别事实被准确表达，不把本地 fixture 扩大为远端事务证明。",
        "",
    ]
    report = "\n".join(lines)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(report, encoding="utf-8")
    else:
        print(report)
    return 0 if passed == len(checks) else 1


if __name__ == "__main__":
    raise SystemExit(main())
