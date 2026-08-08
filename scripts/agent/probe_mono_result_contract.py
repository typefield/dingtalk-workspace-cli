#!/usr/bin/env python3
"""Agent-side result-contract probe for Mono Skill scripts.

This is deliberately an evidence collector, not a CI gate.  It executes the
shared runtime's error boundary and one representative script validation path,
then writes only human-readable Markdown.  Temporary JSON input exists only
inside the probe and is never checked in as a fixture.
"""

from __future__ import annotations

import argparse
import ast
from datetime import date
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile


ROOT = Path(__file__).resolve().parents[2]
SCRIPT_DIR = ROOT / "skills" / "mono" / "scripts"


def is_agent_entry(path: Path) -> bool:
    tree = ast.parse(path.read_text(encoding="utf-8"))
    return any(
        isinstance(node, ast.If)
        and isinstance(node.test, ast.Compare)
        and isinstance(node.test.left, ast.Name)
        and node.test.left.id == "__name__"
        and any(
            isinstance(value, ast.Constant) and value.value == "__main__"
            for value in node.test.comparators
        )
        for node in ast.walk(tree)
    )


def parse_single_result(proc: subprocess.CompletedProcess[str]) -> tuple[bool, dict[str, object] | None, str]:
    lines = [line for line in proc.stdout.splitlines() if line.strip()]
    if len(lines) != 1:
        return False, None, f"stdout lines={len(lines)}"
    try:
        payload = json.loads(lines[0])
    except json.JSONDecodeError as exc:
        return False, None, f"invalid JSON: {exc}"
    if not isinstance(payload, dict):
        return False, None, "result is not an object"
    return True, payload, "ok"


def runtime_probe(source: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [sys.executable, "-c", source],
        cwd=ROOT,
        capture_output=True,
        text=True,
        timeout=30,
    )


def result(status: str, detail: str) -> tuple[str, str]:
    return status, detail.replace("|", "\\|").replace("\n", " ")


def run_with_fake_dws(
    command: list[str],
    fake_source: str,
    *,
    temp_dir: Path,
) -> subprocess.CompletedProcess[str]:
    """Execute one script against a temporary child runner, never a real tenant."""
    fake_dws = temp_dir / "dws"
    fake_dws.write_text("#!/usr/bin/env python3\n" + fake_source, encoding="utf-8")
    fake_dws.chmod(0o755)
    environment = os.environ.copy()
    environment["PATH"] = f"{temp_dir}{os.pathsep}{environment.get('PATH', '')}"
    return subprocess.run(
        command,
        cwd=ROOT,
        capture_output=True,
        text=True,
        timeout=30,
        env=environment,
    )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, help="write Markdown report; default is stdout")
    args = parser.parse_args()
    outcomes: list[tuple[str, str, str]] = []

    entries = [path for path in SCRIPT_DIR.glob("*.py") if is_agent_entry(path)]
    missing_wrapper = [path.name for path in entries if "run_main(main)" not in path.read_text(encoding="utf-8")]
    outcomes.append((
        "32 个入口统一异常边界",
        *result("PASS" if not missing_wrapper else "FAIL", ", ".join(missing_wrapper) or f"{len(entries)}/{len(entries)} 使用 run_main"),
    ))

    internal = runtime_probe(
        "import sys; sys.path.insert(0, 'skills/mono/scripts'); import _runtime; "
        "raise SystemExit(_runtime.run_main(lambda: (print('pre-failure-leak'), (_ for _ in ()).throw(AttributeError('secret input')))[1], argv=['--format','json']))"
    )
    valid, payload, detail = parse_single_result(internal)
    internal_ok = (
        valid
        and internal.returncode == 1
        and payload is not None
        and payload.get("ok") is False
        and payload.get("outcome") == "failure"
        and isinstance(payload.get("error"), dict)
        and payload["error"].get("type") == "internal"
        and "Traceback" not in internal.stderr
        and "secret input" not in internal.stderr
        and "pre-failure-leak" not in internal.stdout
    )
    outcomes.append(("未捕获异常 JSON 兜底", *result("PASS" if internal_ok else "FAIL", detail)))

    system_exit = runtime_probe(
        "import sys; sys.path.insert(0, 'skills/mono/scripts'); import _runtime; "
        "raise SystemExit(_runtime.run_main(lambda: (_ for _ in ()).throw(SystemExit(2)), argv=['--format','json']))"
    )
    valid, payload, detail = parse_single_result(system_exit)
    system_exit_ok = (
        valid
        and system_exit.returncode == 1
        and payload is not None
        and payload.get("outcome") == "failure"
        and isinstance(payload.get("error"), dict)
        and payload["error"].get("type") == "validation"
    )
    outcomes.append(("非零 SystemExit JSON 兜底", *result("PASS" if system_exit_ok else "FAIL", detail)))

    partial = runtime_probe(
        "import sys; sys.path.insert(0, 'skills/mono/scripts'); import _runtime; "
        "raise SystemExit(_runtime.emit(fmt='json', outcome='partial_failure', data="
        "_runtime.batch_data(succeeded=[{'id':'a'}],failed=[{'id':'b','error':{'type':'validation','message':'bad'}}],unknown=[])))"
    )
    valid, payload, detail = parse_single_result(partial)
    partial_ok = (
        valid
        and partial.returncode == 7
        and payload is not None
        and payload.get("ok") is False
        and payload.get("outcome") == "partial_failure"
    )
    outcomes.append(("部分成功结果与退出码", *result("PASS" if partial_ok else "FAIL", detail)))

    meta = runtime_probe(
        "import sys; sys.path.insert(0, 'skills/mono/scripts'); import _runtime; "
        "raise SystemExit(_runtime.emit(fmt='json', outcome='success', data={}, meta={'pagination':{'endpoint_exhausted':False}}))"
    )
    valid, payload, detail = parse_single_result(meta)
    meta_ok = (
        valid
        and meta.returncode == 0
        and payload is not None
        and payload.get("meta") == {"pagination": {"endpoint_exhausted": False}}
    )
    outcomes.append(("可选 meta 承载", *result("PASS" if meta_ok else "FAIL", detail)))

    with tempfile.TemporaryDirectory(prefix="dws-mono-result-contract-") as temp_dir:
        payload_path = Path(temp_dir) / "todos.json"
        payload_path.write_text(
            json.dumps([{"title": "probe", "executors": ["not-a-string"]}]),
            encoding="utf-8",
        )
        todo = subprocess.run(
            [
                sys.executable,
                str(SCRIPT_DIR / "todo_batch_create.py"),
                str(payload_path),
                "--format",
                "json",
            ],
            cwd=ROOT,
            capture_output=True,
            text=True,
            timeout=30,
        )
    valid, payload, detail = parse_single_result(todo)
    todo_ok = (
        valid
        and todo.returncode == 1
        and payload is not None
        and payload.get("outcome") == "failure"
        and isinstance(payload.get("error"), dict)
        and payload["error"].get("type") == "validation"
        and "Traceback" not in todo.stderr
    )
    outcomes.append(("待办错误类型输入", *result("PASS" if todo_ok else "FAIL", detail)))

    with tempfile.TemporaryDirectory(prefix="dws-mono-child-outcome-") as temp_dir_name:
        temp_dir = Path(temp_dir_name)
        todo_path = temp_dir / "todos.json"
        todo_path.write_text(
            json.dumps([
                {"title": "confirmed", "executors": "u1"},
                {"title": "ambiguous", "executors": "u2"},
            ]),
            encoding="utf-8",
        )
        todo_child = run_with_fake_dws(
            [sys.executable, str(SCRIPT_DIR / "todo_batch_create.py"), str(todo_path), "--format", "json"],
            """import json, sys
title = sys.argv[sys.argv.index('--title') + 1]
if title == 'confirmed':
    print(json.dumps({'ok': True, 'data': {'taskId': 'task-1'}, 'meta': {'request': 'one'}}))
else:
    print(json.dumps({'ok': False, 'error': {'type': 'api', 'message': 'connection dropped'}, 'meta': {'request': 'two'}}))
    raise SystemExit(1)
""",
            temp_dir=temp_dir,
        )
        valid, payload, detail = parse_single_result(todo_child)
        todo_child_ok = (
            valid and todo_child.returncode == 7 and payload is not None
            and payload.get("outcome") == "partial_failure"
            and isinstance(payload.get("data"), dict)
            and [item.get("id") for item in payload["data"].get("succeeded", [])] == ["1"]
            and [item.get("id") for item in payload["data"].get("unknown", [])] == ["2"]
            and payload.get("meta", {}).get("children", [])[0].get("id") == "1"
        )
        outcomes.append(("待办保留成功与未知写入", *result("PASS" if todo_child_ok else "FAIL", detail)))

        oa_log = temp_dir / "oa-calls.log"
        oa_child = run_with_fake_dws(
            [
                sys.executable, str(SCRIPT_DIR / "oa_batch_approve.py"),
                "--action", "approve", "--instance-ids", "a,b", "--yes", "--format", "json",
            ],
            f"""import json, pathlib, sys
args = sys.argv[1:]
pathlib.Path({str(oa_log)!r}).open('a', encoding='utf-8').write(' '.join(args) + '\\n')
instance = args[args.index('--instance-id') + 1] if '--instance-id' in args else ''
if args[:3] == ['oa', 'approval', 'tasks']:
    if instance == 'a':
        print(json.dumps({{'ok': True, 'data': {{'tasks': [{{'taskId': 'task-a'}}]}}}}))
    else:
        print(json.dumps({{'ok': False, 'error': {{'type': 'validation', 'message': 'invalid instance'}}}}))
        raise SystemExit(3)
elif args[:3] == ['oa', 'approval', 'approve']:
    print(json.dumps({{'ok': True, 'data': {{'instanceId': instance}}}}))
else:
    raise SystemExit(9)
""",
            temp_dir=temp_dir,
        )
        valid, payload, detail = parse_single_result(oa_child)
        oa_calls = oa_log.read_text(encoding="utf-8") if oa_log.exists() else ""
        oa_child_ok = (
            valid and oa_child.returncode == 7 and payload is not None
            and payload.get("outcome") == "partial_failure"
            and isinstance(payload.get("data"), dict)
            and [item.get("id") for item in payload["data"].get("succeeded", [])] == ["a"]
            and [item.get("id") for item in payload["data"].get("failed", [])] == ["b"]
            and "approve --instance-id b" not in oa_calls
        )
        outcomes.append(("审批任务解析失败不发送占位写入", *result("PASS" if oa_child_ok else "FAIL", detail)))

        doc_log = temp_dir / "doc-calls.log"
        doc_child = run_with_fake_dws(
            [
                sys.executable, str(SCRIPT_DIR / "doc_create_and_write.py"),
                "--name", "probe", "--content", "body", "--max-retries", "3", "--format", "json",
            ],
            f"""import json, pathlib, sys
args = sys.argv[1:]
pathlib.Path({str(doc_log)!r}).open('a', encoding='utf-8').write(' '.join(args) + '\\n')
if args[:2] == ['doc', 'create']:
    print(json.dumps({{'ok': True, 'data': {{'nodeId': 'node-1'}}}}))
elif args[:2] == ['doc', 'update']:
    print(json.dumps({{'ok': False, 'error': {{'type': 'api', 'message': 'connection dropped'}}}}))
    raise SystemExit(1)
else:
    raise SystemExit(9)
""",
            temp_dir=temp_dir,
        )
        valid, payload, detail = parse_single_result(doc_child)
        doc_calls = doc_log.read_text(encoding="utf-8") if doc_log.exists() else ""
        doc_child_ok = (
            valid and doc_child.returncode == 7 and payload is not None
            and payload.get("outcome") == "partial_failure"
            and isinstance(payload.get("data"), dict)
            and [item.get("id") for item in payload["data"].get("succeeded", [])] == ["document:create"]
            and [item.get("id") for item in payload["data"].get("unknown", [])] == ["node-1:chunk:1"]
            and sum(1 for line in doc_calls.splitlines() if line.startswith("doc update")) == 1
        )
        outcomes.append(("文档写入失败不自动重放且标记未知", *result("PASS" if doc_child_ok else "FAIL", detail)))

    passed = sum(status == "PASS" for _, status, _ in outcomes)
    lines = [
        "# Mono 脚本结果契约 Agent 探针",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 本报告由 Agent 在当前工作树执行。它验证共享异常边界和代表性输入错误；不保存运行 JSON fixture，也不替代真实服务端副作用验证。",
        "",
        "| 检查项 | 结果 | 说明 |",
        "|---|---|---|",
    ]
    lines.extend(f"| {name} | {status} | {detail} |" for name, status, detail in outcomes)
    lines.extend([
        "",
        f"结果：{passed}/{len(outcomes)} 通过",
        "",
        "## 边界",
        "",
        "- 本探针证明入口都接入共享异常边界，并证明该边界在机器格式下不会以 traceback 取代结果信封。",
        "- 子 dws 探针覆盖待办、审批和文档的代表性混合结果：成功、明确未执行和可能已执行不得压成布尔值；它不替代其他脚本和真实服务端终态验证。",
        "- dry-run 零写、真实服务端终态和批量每项语义，仍按独立受控探针或真实环境证据标记。",
        "",
    ])
    report = "\n".join(lines)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(report, encoding="utf-8")
    else:
        sys.stdout.write(report)
    return 0 if passed == len(outcomes) else 1


if __name__ == "__main__":
    raise SystemExit(main())
