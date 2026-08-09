#!/usr/bin/env python3
"""按部门名查询成员；逐部门失败不会被静默丢弃。"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path
from typing import Any, Optional

_SHARED_RUNTIME = Path(__file__).resolve().parents[2] / 'dingtalk-shared' / 'scripts'
sys.path.insert(0, str(_SHARED_RUNTIME))

from _runtime import (
    ChildDWSResult,
    add_contract_flags,
    batch_data,
    batch_outcome,
    emit,
    failure,
    run_child_dws,
    run_main,
)


def run_dws(args: list[str], dry_run: bool = False) -> ChildDWSResult:
    return run_child_dws(args, dry_run=dry_run)


def strip_highlight(value: str) -> str:
    return re.sub(r'</?red>', '', value)


def unwrap_unified(payload: Any) -> Any:
    if (
        isinstance(payload, dict)
        and isinstance(payload.get('ok'), bool)
        and isinstance(payload.get('outcome'), str)
        and 'data' in payload
    ):
        return payload['data']
    return payload


def projection_error(message: str) -> dict[str, Any]:
    return {'type': 'api', 'subtype': 'projection_unknown', 'message': message}


def project_departments(payload: Any) -> tuple[list[dict[str, str]], Optional[dict[str, Any]]]:
    value = unwrap_unified(payload)
    if isinstance(value, list):
        rows = value
    elif isinstance(value, dict) and isinstance(value.get('deptList'), list):
        rows = value['deptList']
    elif isinstance(value, dict) and isinstance(value.get('items'), list):
        rows = value['items']
    elif isinstance(value, dict) and isinstance(value.get('result'), list):
        rows = value['result']
    elif isinstance(value, dict) and isinstance(value.get('result'), dict):
        inner = value['result']
        rows = inner.get('deptList') if isinstance(inner.get('deptList'), list) else inner.get('items')
        if not isinstance(rows, list):
            return [], projection_error('部门搜索 result 缺少 deptList[]/items[]。')
    else:
        return [], projection_error('部门搜索响应缺少已知部门数组。')

    departments: list[dict[str, str]] = []
    for index, row in enumerate(rows):
        if not isinstance(row, dict):
            return [], projection_error(f'部门搜索第 {index + 1} 项不是对象。')
        dept_id = row.get('deptId') if row.get('deptId') is not None else row.get('id')
        name = row.get('deptName') or row.get('name') or '未知'
        if isinstance(dept_id, bool) or not isinstance(dept_id, (str, int)) or not str(dept_id).strip():
            return [], projection_error(f'部门搜索第 {index + 1} 项缺少稳定 deptId。')
        if not isinstance(name, str):
            return [], projection_error(f'部门搜索第 {index + 1} 项名称不是字符串。')
        departments.append({'deptId': str(dept_id).strip(), 'name': strip_highlight(name)})
    return departments, None


def project_members(payload: Any) -> tuple[list[dict[str, str]], Optional[dict[str, Any]]]:
    value = unwrap_unified(payload)
    if isinstance(value, list):
        rows = value
    elif isinstance(value, dict) and isinstance(value.get('deptUserList'), list):
        rows = value['deptUserList']
    elif isinstance(value, dict) and isinstance(value.get('userlist'), list):
        rows = value['userlist']
    elif isinstance(value, dict) and isinstance(value.get('result'), list):
        rows = value['result']
    elif isinstance(value, dict) and isinstance(value.get('result'), dict):
        inner = value['result']
        rows = inner.get('deptUserList') if isinstance(inner.get('deptUserList'), list) else inner.get('userlist')
        if not isinstance(rows, list):
            return [], projection_error('部门成员 result 缺少 deptUserList[]/userlist[]。')
    else:
        return [], projection_error('部门成员响应缺少已知成员数组。')

    members: list[dict[str, str]] = []
    for index, row in enumerate(rows):
        if not isinstance(row, dict):
            return [], projection_error(f'部门成员第 {index + 1} 项不是对象。')
        info = row.get('userInfo', row)
        if not isinstance(info, dict):
            return [], projection_error(f'部门成员第 {index + 1} 项 userInfo 不是对象。')
        user_id = info.get('userId') or info.get('userid')
        name = info.get('name') or info.get('userName') or '未知'
        title = info.get('title') or info.get('position') or ''
        if not isinstance(user_id, str) or not user_id.strip():
            return [], projection_error(f'部门成员第 {index + 1} 项缺少稳定 userId。')
        if not isinstance(name, str) or not isinstance(title, str):
            return [], projection_error(f'部门成员第 {index + 1} 项姓名或职位类型不可识别。')
        members.append({'userId': user_id.strip(), 'name': name, 'title': title})
    return members, None


def child_meta(identifier: str, result: ChildDWSResult) -> Optional[dict[str, Any]]:
    return {'id': identifier, 'meta': result.meta} if result.meta else None


def main() -> int:
    parser = argparse.ArgumentParser(description='按部门名称搜索并列出所有成员')
    parser.add_argument('--query', required=True, help='部门名称关键词')
    add_contract_flags(parser)
    args = parser.parse_args()
    query = args.query.strip()
    if not query:
        return failure(args.format, '--query 不能为空')

    if args.dry_run:
        run_dws(['contact', 'dept', 'search', '--query', query, '--format', 'json'], dry_run=True)
        run_dws([
            'contact', 'dept', 'list-members', '--depts', '<DEPT_ID>', '--format', 'json',
        ], dry_run=True)
        return emit(
            fmt=args.format,
            outcome='success',
            data={'query': query, 'plan': 'search department then list members'},
            dry_run=True,
            text='[dry-run] 将搜索部门并逐部门列出成员',
        )

    meta_children: list[dict[str, Any]] = []
    print(f'🔍 搜索部门: {query}', file=sys.stderr)
    search_result = run_dws(['contact', 'dept', 'search', '--query', query, '--format', 'json'])
    entry = child_meta('dept:search', search_result)
    if entry:
        meta_children.append(entry)
    if search_result.state != 'success':
        return emit(
            fmt=args.format,
            outcome='failure',
            error=search_result.error or projection_error('部门搜索失败。'),
            meta={'children': meta_children} if meta_children else None,
            text='部门搜索失败',
        )
    departments, error = project_departments(search_result.payload)
    if error:
        return emit(
            fmt=args.format,
            outcome='failure',
            error=error,
            meta={'children': meta_children} if meta_children else None,
            text='部门搜索响应无法可靠解析',
        )

    succeeded: list[dict[str, Any]] = []
    failed: list[dict[str, Any]] = []
    output: list[dict[str, Any]] = []
    for department in departments:
        dept_id = department['deptId']
        result = run_dws([
            'contact', 'dept', 'list-members', '--depts', dept_id, '--format', 'json',
        ])
        entry = child_meta(f'members:{dept_id}', result)
        if entry:
            meta_children.append(entry)
        if result.state != 'success':
            failed.append({'id': dept_id, 'error': result.error or projection_error('部门成员查询失败。')})
            continue
        members, error = project_members(result.payload)
        if error:
            failed.append({'id': dept_id, 'error': error})
            continue
        record = {'department': department['name'], 'deptId': dept_id, 'members': members}
        output.append(record)
        succeeded.append({'id': dept_id, **record})

    result_data = batch_data(
        succeeded=succeeded,
        failed=failed,
        query=query,
        departments=output,
        coverage={'scope': 'server_search_response', 'complete': False},
    )
    outcome = batch_outcome(result_data)
    meta = {'children': meta_children} if meta_children else None
    if outcome == 'failure':
        return emit(
            fmt=args.format,
            outcome='failure',
            data=result_data,
            error=failed[0]['error'],
            meta=meta,
            text='所有匹配部门的成员查询均失败',
        )
    if args.format != 'text':
        if outcome == 'success':
            data = {
                'query': query,
                'departments': output,
                'coverage': {'scope': 'server_search_response', 'complete': False},
            }
        else:
            data = result_data
        return emit(fmt=args.format, outcome=outcome, data=data, meta=meta)

    if not departments:
        print('未找到匹配部门')
        return 0
    for record in output:
        print(f"\n📂 {record['department']} (ID: {record['deptId']})")
        print('-' * 40)
        if not record['members']:
            print('  (暂无成员)')
        for member in record['members']:
            line = f"  👤 {member['name']}"
            if member['title']:
                line += f" ({member['title']})"
            line += f"  [ID: {member['userId']}]"
            print(line)
        print(f"  共 {len(record['members'])} 人")
    if outcome == 'partial_failure':
        return emit(
            fmt=args.format,
            outcome='partial_failure',
            data=result_data,
            meta=meta,
            text='警告：部分部门成员查询失败；已保留成功部门。',
        )
    return 0


if __name__ == '__main__':
    raise SystemExit(run_main(main))
