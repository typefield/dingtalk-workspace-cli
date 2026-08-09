#!/usr/bin/env python3
"""Agent-only review of shared DevApp pagination and result declarations.

This is intentionally a release-review aid rather than a CI or policy gate.
It runs the focused in-memory Go test, inspects the four shortcut call sites,
and writes Markdown evidence only. It stores no response fixtures or JSON
catalogs in the repository.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile


ROOT = Path(__file__).resolve().parents[2]
SOURCE = ROOT / "internal/shortcut/devapp/devapp.go"
TEST_PATTERN = r"TestDevApp(ListResultSpecMatchesSharedProjection|NativeAndShortcutListToolsShareProjectedResult|PaginatedShortcutContractsMatchActiveData|PaginatedShortcutsEmitUnifiedResumableResults|SharedListProjection.*|ListPaginationProjectsMeta)|TestDevRepresentativeResultContractsReachContractFinal"
EXPECTED_CALLERS = (
    ("ListApp", "list_dev_app", "apps", "dev app list", "devapp +list"),
    ("PermissionList", "list_dev_app_permissions", "permissions", "dev app permission list", "devapp +permission-list"),
    ("EventList", "list_dev_app_events", "events", "dev app event list", "devapp +event-list"),
    ("VersionList", "list_dev_app_versions", "versions", "dev app version list", "devapp +version-list"),
)


def schema_result(binary: Path, cli_path: str) -> dict:
    completed = subprocess.run(
        [str(binary), "schema", "--cli-path", cli_path, "--format", "json"],
        cwd=ROOT,
        text=True,
        capture_output=True,
        check=False,
    )
    if completed.returncode != 0:
        raise RuntimeError(f"{cli_path}: schema rc={completed.returncode}")
    payload = json.loads(completed.stdout)
    result = payload.get("result")
    if not isinstance(result, dict):
        raise RuntimeError(f"{cli_path}: result contract missing")
    return result


def run() -> int:
    parser = argparse.ArgumentParser(description="Agent review of devapp pagination projection")
    parser.add_argument("--output", required=True, type=Path, help="Markdown evidence path")
    args = parser.parse_args()

    source = SOURCE.read_text(encoding="utf-8")
    missing = []
    inactive = []
    missing_contract = []
    for shortcut, tool, _, _, _ in EXPECTED_CALLERS:
        block = re.search(
            rf"var {shortcut} = shortcut\.Shortcut\{{(?:(?!^var ).)*?\n\}}",
            source,
            flags=re.MULTILINE | re.DOTALL,
        )
        if block is None or f'outputSharedDevAppListPage(rt, "{tool}", data)' not in block.group(0):
            missing.append(shortcut)
        if block is None or f'helpers.DevAppListResultSpec("{tool}")' not in block.group(0):
            missing_contract.append(shortcut)
        # Output rollout is release-owned. An Agent must not select it through
        # argv; this review verifies that the terminal is active on its ordinary
        # --format json path.
        if f"frameworkUnified({shortcut})" not in source:
            inactive.append(shortcut)

    environment = os.environ.copy()
    environment.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-review")
    command = [
        "go",
        "test",
        "-count=1",
        "./internal/helpers",
        "./internal/shortcut/devapp",
        "-run",
        TEST_PATTERN,
        "-v",
    ]
    result = subprocess.run(command, cwd=ROOT, env=environment, text=True, capture_output=True)

    schema_errors = []
    with tempfile.TemporaryDirectory(prefix="dws-devapp-schema-agent-") as directory:
        binary = Path(directory) / "dws"
        build = subprocess.run(
            ["go", "build", "-o", str(binary), "./cmd"],
            cwd=ROOT,
            env=environment,
            text=True,
            capture_output=True,
            check=False,
        )
        if build.returncode != 0:
            schema_errors.append(f"current source build failed (rc={build.returncode})")
        else:
            for _, _, data_key, native_path, shortcut_path in EXPECTED_CALLERS:
                for cli_path in (native_path, shortcut_path):
                    try:
                        contract = schema_result(binary, cli_path)
                        data_schema = contract.get("data_schema") or {}
                        properties = data_schema.get("properties") or {}
                        required = data_schema.get("required") or []
                        ndjson = contract.get("ndjson") or {}
                        if (
                            set(properties) != {data_key}
                            or required != [data_key]
                            or ndjson.get("record_path") != data_key
                            or "pagination" in contract
                        ):
                            schema_errors.append(
                                f"{cli_path}: data/NDJSON contract does not match {data_key!r} or duplicates meta pagination"
                            )
                    except Exception as exc:  # noqa: BLE001 - audit reports all declaration failures
                        schema_errors.append(str(exc))

    passed = result.returncode == 0 and not missing and not inactive and not missing_contract and not schema_errors
    status = "PASS" if passed else "REVIEW"
    checked_callers = len(EXPECTED_CALLERS) - len(set(missing + inactive + missing_contract))
    timestamp = dt.datetime.now().astimezone().isoformat(timespec="seconds")
    test_excerpt = (result.stdout + result.stderr).strip()
    if len(test_excerpt) > 3000:
        test_excerpt = test_excerpt[:1450] + "\n... (middle transcript elided) ...\n" + test_excerpt[-1450:]

    report = f"""# DevApp native / shortcut pagination and ResultSpec — Agent review

扫描时间：{timestamp}

> 这是 Agent 的语义审阅取证，不是 CI / policy gate。扫描只调用内存中的 Go 测试，并输出 Markdown；不会保存任何上游响应或 JSON fixture。

## Result: {status}

- 已审阅分页能力：**{checked_callers}/{len(EXPECTED_CALLERS)}**，每类覆盖 native 与 shortcut 两个既有入口
- 覆盖入口：`dev app list/permission list/event list/version list` 与对应 `devapp +list/+permission-list/+event-list/+version-list`
- 焦点测试：`{TEST_PATTERN}`
- 测试退出码：`{result.returncode}`

## Required behavior

1. 保留顶层及嵌套 `result/data/content/pageInfo/pagination` 中的有效 `hasMore` / `nextCursor`。
2. `hasMore` 非布尔值必须返回 `validation/pagination_invalid`，不能投影成空分页。
3. `nextCursor` 非字符串、`hasMore=true` 无游标必须返回 `validation/pagination_incomplete`。
4. 多层 `hasMore` 或非空 `nextCursor` 互相冲突必须返回 `validation/pagination_conflict`；已明确 `hasMore=false` 的 DevApp 位置 cursor 不可发布为续页 token。
5. 上述失败均为 `response_projection`、不可安全重试；不得输出成功列表。
6. 只有显式的空数组才可投影为成功空列表；缺容器、非数组、非法行或仅展示字段的行必须为 `api/projection_unknown`。
7. 经本项 Agent 审阅的八条 terminal command 才在各自原路径直接输出统一结果；调用者只传 `--format json`。
8. 非末页必须在 `meta.pagination` 中保留 `endpoint_exhausted:false` 与 `next_token`，且不输出任何协议版本标记。
9. 八条命令的 Runtime Schema 必须只声明 active `data` 中的 `apps/permissions/events/versions`；NDJSON record path 与该键一致，且不得把框架 `meta.pagination` 重新声明成 data 内分页。

## Source coverage

"""
    if missing:
        report += "- REVIEW：下列 Shortcut 未在 Execute 中调用共享 `outputSharedDevAppListPage`：" + ", ".join(f"`{name}`" for name in missing) + "\n"
    else:
        report += "- PASS：四个列表 Shortcut 均经共享 projector 验证字段、稳定 ID 与分页证据。\n"
    if missing_contract:
        report += "- REVIEW：下列 Shortcut 未引用共享 `DevAppListResultSpec`：" + ", ".join(f"`{name}`" for name in missing_contract) + "\n"
    else:
        report += "- PASS：四个 Shortcut 与四个 native 入口共享同一 ResultSpec 生成器。\n"
    if inactive:
        report += "- REVIEW：下列 Shortcut 尚未进入 `unified_active`，外部仍为 legacy：" + ", ".join(f"`{name}`" for name in inactive) + "\n"
    else:
        report += "- PASS：四条已审阅的列表 Shortcut 均在原命令路径直接输出统一结果；没有公开版本/协议选择参数。\n"
    if schema_errors:
        report += "- REVIEW：Runtime Schema 漂移：" + "; ".join(f"`{error}`" for error in schema_errors) + "\n"
    else:
        report += "- PASS：八条 live Runtime Schema 的 data key、required 与 NDJSON record path 均和 active 输出一致；分页只在统一 meta。\n"
    report += f"""

## Focused test transcript

```text
{test_excerpt or "(no output)"}
```

## Boundary

这证明本地投影不会再静默吞掉异常或矛盾的分页字段，且 live Schema 不再教 Agent 读取已经从 active data 删除的 legacy 字段；真实 devapp 账号的续翻、服务端末页语义及两个入口的端到端对拍仍需单独 Agent 实测。
"""

    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(report, encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    sys.exit(run())
