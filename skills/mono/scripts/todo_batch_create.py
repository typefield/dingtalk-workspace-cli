#!/usr/bin/env python3
"""
从 JSON 文件批量创建待办（含优先级、截止时间、执行者）

用法:
    python todo_batch_create.py todos.json
    python todo_batch_create.py todos.json --dry-run

todos.json 格式:
    [
        {"title": "修复线上Bug", "executors": "userId1,userId2", "priority": 40},
        {"title": "写周报", "executors": "userId1", "due": "2026-03-15"},
        {"title": "代码评审", "executors": "userId1"},
        {"title": "每日站会", "executors": "userId1", "due": "2026-03-20",
         "recurrence": "DTSTART:20260320T020000Z\\nRRULE:FREQ=DAILY;INTERVAL=1"}
    ]

字段说明:
- title:     待办标题 (必填)
- executors: 执行者 userId，多人逗号分隔 (必填)
- priority:  优先级 10=低/20=普通/30=较高/40=紧急 (可选)
- due:       截止日期 YYYY-MM-DD 或毫秒时间戳 (可选)
- recurrence: 循环待办规则 (可选，需同时有 due)；字符串内需含换行，与 dws --recurrence 一致
"""

import sys
import json
import subprocess
import re
import argparse
from datetime import datetime
from pathlib import Path
from typing import List, Dict, Any, Optional

from _runtime import add_contract_flags, emit, failure

ALLOWED_PRIORITIES = {10, 20, 30, 40}
DATE_PATTERN = re.compile(r'^\d{4}-\d{2}-\d{2}$')
MAX_FILE_SIZE = 10 * 1024 * 1024


def run_dws(
    args: List[str], dry_run: bool = False,
) -> tuple[bool, Optional[Dict[str, Any]], Optional[str]]:
    cmd = ['dws'] + args
    if dry_run:
        return True, {'command': cmd, 'dry_run': True}, None
    try:
        result = subprocess.run(
            cmd, capture_output=True, text=True, timeout=60
        )
        if result.returncode != 0:
            return False, None, result.stderr.strip() or f"dws exited {result.returncode}"
        payload = json.loads(result.stdout)
        if isinstance(payload, dict) and payload.get('ok') is False:
            error = payload.get('error') or {'type': 'api', 'message': 'dws returned failure'}
            return False, payload, str(error)
        return True, payload, None
    except subprocess.TimeoutExpired:
        return False, None, '命令执行超时'
    except (json.JSONDecodeError, FileNotFoundError) as e:
        return False, None, str(e)


def parse_due(due_value) -> Optional[str]:
    if not due_value:
        return None
    due_str = str(due_value)
    if due_str.isdigit() and len(due_str) >= 10:
        return due_str
    if DATE_PATTERN.match(due_str):
        dt = datetime.strptime(due_str, '%Y-%m-%d')
        dt = dt.replace(hour=23, minute=59, second=59)
        return str(int(dt.timestamp() * 1000))
    print(f"  ⚠ 无法解析截止时间：{due_value}，跳过", file=sys.stderr)
    return None


def validate_todo(item: Dict[str, Any], idx: int) -> bool:
    if not isinstance(item, dict):
        print(f"  ✗ #{idx+1} 不是有效对象", file=sys.stderr)
        return False
    if not item.get('title', '').strip():
        print(f"  ✗ #{idx+1} 缺少 title", file=sys.stderr)
        return False
    if not item.get('executors', '').strip():
        print(f"  ✗ #{idx+1} 缺少 executors", file=sys.stderr)
        return False
    priority = item.get('priority')
    if priority is not None and int(priority) not in ALLOWED_PRIORITIES:
        print(f"  ✗ #{idx+1} 无效优先级：{priority}", file=sys.stderr)
        return False
    recurrence = item.get('recurrence')
    if recurrence and not str(recurrence).strip():
        print(f"  ✗ #{idx+1} recurrence 不能为空字符串", file=sys.stderr)
        return False
    if recurrence and not item.get('due'):
        print(f"  ✗ #{idx+1} 设置 recurrence 时必须提供 due", file=sys.stderr)
        return False
    return True


def main() -> int:
    parser = argparse.ArgumentParser(description='从 JSON 文件批量创建待办')
    parser.add_argument('input', help='待办 JSON 文件')
    add_contract_flags(parser)
    args = parser.parse_args()

    file_path = Path(args.input)
    if not file_path.exists():
        return failure(args.format, f"文件不存在：{file_path}")
    if file_path.stat().st_size > MAX_FILE_SIZE:
        return failure(args.format, f"文件过大 (限制 {MAX_FILE_SIZE // 1024}KB)")

    try:
        with open(file_path, 'r', encoding='utf-8') as f:
            todos = json.load(f)
    except (OSError, json.JSONDecodeError) as exc:
        return failure(args.format, f"无法读取待办 JSON：{exc}")
    if not isinstance(todos, list) or not todos:
        return failure(args.format, 'JSON 文件必须是非空数组')

    for i, item in enumerate(todos):
        if not validate_todo(item, i):
            return failure(args.format, f'第 {i + 1} 条待办参数无效')

    if args.format == 'text':
        print(f"📋 准备创建 {len(todos)} 条待办\n")
    records: list[dict[str, Any]] = []
    success, fail = 0, 0
    for i, item in enumerate(todos):
        title = item['title'].strip()
        cmd_args = [
            'todo', 'task', 'create',
            '--title', title,
            '--executors', item['executors'].strip(),
            '--format', 'json',
        ]
        priority = item.get('priority')
        if priority is not None:
            cmd_args.extend(['--priority', str(int(priority))])
        due = parse_due(item.get('due'))
        if due:
            cmd_args.extend(['--due', due])
        recurrence = item.get('recurrence')
        if recurrence:
            rr = str(recurrence).replace('\\n', '\n')
            cmd_args.extend(['--recurrence', rr])

        succeeded, result, error = run_dws(cmd_args, dry_run=args.dry_run)
        record: dict[str, Any] = {'index': i + 1, 'title': title, 'command': cmd_args}
        if succeeded:
            record.update({'ok': True, 'outcome': 'success', 'result': result})
            if args.format == 'text':
                print(f"  ✓ [{i+1}/{len(todos)}] {title}")
            success += 1
        else:
            record.update({'ok': False, 'outcome': 'failure', 'error': {'type': 'api', 'message': error or 'unknown error'}})
            if args.format == 'text':
                print(f"  ✗ [{i+1}/{len(todos)}] {title}", file=sys.stderr)
            fail += 1
        records.append(record)

    if args.format == 'text':
        print(f"\n完成: 成功 {success}, 失败 {fail}")
    if fail == 0:
        outcome = 'success'
    elif success > 0:
        outcome = 'partial_failure'
    else:
        outcome = 'failure'
    data = {'total': len(records), 'success_count': success, 'failure_count': fail, 'items': records}
    if args.format == 'text':
        return 0 if outcome == 'success' else 7 if outcome == 'partial_failure' else 1
    return emit(
        fmt=args.format,
        outcome=outcome,
        data=data,
        dry_run=args.dry_run,
        items=records,
    )


if __name__ == '__main__':
    sys.exit(main())
