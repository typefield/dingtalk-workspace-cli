#!/usr/bin/env python3
"""Audit wiki +resolve-space without storing space names, IDs, tokens, or JSON fixtures."""

from __future__ import annotations

import argparse
from datetime import date
import json
import os
from pathlib import Path
import subprocess
import tempfile


ROOT = Path(__file__).resolve().parents[2]
DATA_KEYS = {"resolved", "count", "candidates"}


def run(command: list[str], env: dict[str, str], timeout: int = 300) -> subprocess.CompletedProcess[str]:
    return subprocess.run(command, cwd=ROOT, env=env, text=True, capture_output=True, timeout=timeout, check=False)


def stable_candidates(rows: object) -> bool:
    return isinstance(rows, list) and bool(rows) and all(
        isinstance(row, dict)
        and set(row) == {"spaceId", "name"}
        and isinstance(row.get("spaceId"), str)
        and bool(row["spaceId"].strip())
        and isinstance(row.get("name"), str)
        and bool(row["name"].strip())
        for row in rows
    )


def read_complete_org_directory(binary: Path, env: dict[str, str]) -> tuple[list[dict[str, object]], int]:
    spaces: list[dict[str, object]] = []
    cursor = ""
    seen_tokens: set[str] = set()
    pages = 0
    while pages < 100:
        command = [str(binary), "wiki", "space", "list", "--type", "orgWikiSpace", "--limit", "50", "--format", "json"]
        if cursor:
            command.extend(["--cursor", cursor])
        result = run(command, env, 120)
        payload = json.loads(result.stdout)
        rows = payload.get("wikiSpaces")
        has_more = payload.get("hasMore")
        token = payload.get("nextPageToken", "")
        if (
            result.returncode != 0
            or result.stderr.strip()
            or payload.get("success") is not True
            or not isinstance(rows, list)
            or not isinstance(has_more, bool)
            or not isinstance(token, str)
            or any(
                not isinstance(row, dict)
                or not isinstance(row.get("workspaceId"), str)
                or not row["workspaceId"].strip()
                or not isinstance(row.get("name"), str)
                or not row["name"].strip()
                for row in rows
            )
        ):
            raise ValueError("organization Wiki directory does not match the reviewed shape")
        pages += 1
        spaces.extend(rows)
        token = token.strip()
        if not has_more:
            if token:
                raise ValueError("terminal Wiki directory page carries a token")
            return spaces, pages
        if not token or token == cursor or token in seen_tokens:
            raise ValueError("Wiki directory continuation token is missing or repeated")
        seen_tokens.add(token)
        cursor = token
    raise ValueError("Wiki directory exceeded the 100-page Agent audit budget")


def normalize_legacy(payload: object) -> list[dict[str, object]]:
    if not isinstance(payload, dict):
        return []
    if payload.get("resolved") is True:
        return [{"spaceId": payload.get("spaceId"), "name": payload.get("name")}]
    rows = payload.get("candidates")
    return rows if isinstance(rows, list) else []


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--expected", required=True, choices=("dual", "active"))
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--live", action="store_true")
    args = parser.parse_args()
    env = dict(os.environ)
    env.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-review")
    checks: list[tuple[str, bool, str]] = []
    findings: list[str] = []

    focused = run(["go", "test", "-count=1", "./internal/shortcut/smart", "-run", "TestResolveSpace"], env)
    focused_ok = focused.returncode == 0
    checks.append(("严格投影、分页与 rollout 回归", focused_ok, f"rc={focused.returncode}"))
    if not focused_ok:
        findings.append("focused resolve-space tests failed")

    with tempfile.TemporaryDirectory(prefix="dws-resolve-space-agent-") as directory:
        binary = Path(directory) / "dws"
        build = run(["go", "build", "-o", str(binary), "./cmd"], env)
        build_ok = build.returncode == 0
        checks.append(("临时构建当前源码", build_ok, f"rc={build.returncode}"))
        if not build_ok:
            findings.append("current source build failed")
        else:
            schema = run([str(binary), "schema", "--cli-path", "wiki +resolve-space", "--format", "json"], env)
            property_count = 0
            schema_ok = False
            try:
                result = (json.loads(schema.stdout).get("result") or {})
                properties = set(((result.get("data_schema") or {}).get("properties") or {}))
                property_count = len(properties)
                schema_ok = schema.returncode == 0 and properties == DATA_KEYS
            except (json.JSONDecodeError, AttributeError, TypeError):
                pass
            checks.append(("Runtime Schema 固定消歧结果", schema_ok, f"properties={property_count}, rc={schema.returncode}"))
            if not schema_ok:
                findings.append("resolve-space Runtime Schema is incomplete")

            if args.live:
                try:
                    spaces, source_pages = read_complete_org_directory(binary, env)
                    if not spaces:
                        raise ValueError("organization Wiki directory is empty")
                    query = str(spaces[0]["name"]).strip()
                    expected = [
                        {"spaceId": row["workspaceId"], "name": str(row["name"]).strip()}
                        for row in spaces
                        if query.casefold() in str(row["name"]).strip().casefold()
                    ]
                    if not expected:
                        raise ValueError("in-memory Wiki name did not match its source row")
                    resolved = run([str(binary), "wiki", "+resolve-space", "--name", query, "--format", "json"], env, 300)
                    payload = json.loads(resolved.stdout)
                    if args.expected == "dual":
                        rows = normalize_legacy(payload)
                        live_ok = (
                            resolved.returncode == 0
                            and not resolved.stderr.strip()
                            and isinstance(payload, dict)
                            and "ok" not in payload
                            and stable_candidates(rows)
                            and rows == expected
                        )
                        summary = f"rc={resolved.returncode}, legacy=yes, source_pages={source_pages}, candidates={len(rows)}, stable_ids={'yes' if live_ok else 'no'}"
                    else:
                        data = payload.get("data") if isinstance(payload, dict) else None
                        meta = payload.get("meta") if isinstance(payload, dict) else None
                        rows = data.get("candidates") if isinstance(data, dict) else None
                        count = data.get("count") if isinstance(data, dict) else None
                        pagination = meta.get("pagination") if isinstance(meta, dict) else None
                        live_ok = (
                            resolved.returncode == 0
                            and not resolved.stderr.strip()
                            and isinstance(payload, dict)
                            and payload.get("ok") is True
                            and payload.get("outcome") == "success"
                            and "contract_version" not in payload
                            and isinstance(data, dict)
                            and set(data) == DATA_KEYS
                            and isinstance(count, int)
                            and count == len(expected)
                            and rows == expected
                            and stable_candidates(rows)
                            and isinstance(meta, dict)
                            and meta.get("count") == count
                            and isinstance(pagination, dict)
                            and pagination.get("endpoint_exhausted") is True
                            and pagination.get("pages") == source_pages
                            and pagination.get("items") == count
                        )
                        summary = f"rc={resolved.returncode}, unified=yes, source_pages={source_pages}, candidates={len(rows) if isinstance(rows, list) else -1}, count_aligned={'yes' if live_ok else 'no'}"
                    checks.append(("真实组织知识库完整目录消歧对拍", live_ok, summary))
                    if not live_ok:
                        findings.append("live resolve-space result does not match the independently exhausted directory")
                except (json.JSONDecodeError, AttributeError, KeyError, TypeError, ValueError) as error:
                    findings.append(f"live resolve-space structural probe failed: {type(error).__name__}")
            else:
                checks.append(("真实组织知识库完整目录消歧对拍", True, "SKIPPED（未传 --live）"))

    passed = not findings
    lines = [
        f"# wiki +resolve-space {args.expected} Agent 审阅",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 当前源码临时构建；空间名称、workspaceId 和分页 token 只在内存中用于独立对拍。本文件不保存查询词、名称、ID、token 或原始 JSON，也不接入 CI / policy。",
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
        "- resolver 使用 `list_wikiSpaces` 逐页耗尽当前身份可访问的组织知识库目录，不再把无覆盖事实的搜索首屏扩大为唯一结果。",
        "- 候选稳定 ID 只接受真实服务字段 `workspaceId`；未知容器、非法条目、空/重复 ID 与未知字段均 fail-closed。",
        "- `hasMore/nextPageToken` 缺失、矛盾、重复或超过安全页数均返回稳定分页错误，不会发布 `resolved:true`。",
        "- `--format json` 是唯一 Agent 输出入口；没有协议选择参数或版本字段。",
    ]
    if findings:
        lines += ["", "## Findings", ""] + [f"- {finding}" for finding in findings]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
