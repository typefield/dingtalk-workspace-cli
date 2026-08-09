#!/usr/bin/env python3
"""查询待审列表并逐条读取详情；详情失败保留为 partial。"""

from __future__ import annotations

import argparse
import sys
from datetime import datetime, timedelta
from typing import Any, Optional

from _runtime import ChildDWSResult, add_contract_flags, batch_data, batch_outcome, emit, failure, run_child_dws, run_main


def run_dws(args: list[str], dry_run: bool = False) -> ChildDWSResult:
    return run_child_dws(args, dry_run=dry_run)


def to_iso(value: datetime) -> str:
    return value.strftime('%Y-%m-%dT%H:%M:%S+08:00')


def unwrap_unified(payload: Any) -> Any:
    if isinstance(payload, dict) and isinstance(payload.get('ok'), bool) and isinstance(payload.get('outcome'), str) and 'data' in payload:
        return payload['data']
    return payload


def projection_error(message: str) -> dict[str, Any]:
    return {'type': 'api', 'subtype': 'projection_unknown', 'message': message}


def project_instances(payload: Any) -> tuple[list[dict[str, str]], Optional[dict[str, Any]]]:
    value = unwrap_unified(payload)
    if isinstance(value, list):
        rows = value
    elif isinstance(value, dict) and isinstance(value.get('processInstanceList'), list):
        rows = value['processInstanceList']
    elif isinstance(value, dict) and isinstance(value.get('items'), list):
        rows = value['items']
    elif isinstance(value, dict) and isinstance(value.get('result'), list):
        rows = value['result']
    elif isinstance(value, dict) and isinstance(value.get('result'), dict):
        inner = value['result']
        rows = inner.get('processInstanceList') if isinstance(inner.get('processInstanceList'), list) else inner.get('items')
        if not isinstance(rows, list):
            return [], projection_error('待审列表 result 缺少 processInstanceList[]/items[]。')
    else:
        return [], projection_error('待审列表响应缺少已知实例数组。')
    records: list[dict[str, str]] = []
    for index, row in enumerate(rows):
        if not isinstance(row, dict):
            return [], projection_error(f'待审列表第 {index + 1} 项不是对象。')
        instance_id = row.get('processInstanceId')
        if not isinstance(instance_id, str) or not instance_id.strip():
            return [], projection_error(f'待审列表第 {index + 1} 项缺少稳定 processInstanceId。')
        title = row.get('title') or row.get('name') or '无标题'
        status = row.get('status') or row.get('result') or ''
        create_time = row.get('createTime') or ''
        if not isinstance(title, str) or not isinstance(status, str):
            return [], projection_error(f'待审列表第 {index + 1} 项标题或状态类型不可识别。')
        if isinstance(create_time, (int, float)) and not isinstance(create_time, bool):
            create_time = datetime.fromtimestamp(create_time / 1000).strftime('%Y-%m-%d %H:%M')
        elif not isinstance(create_time, str):
            return [], projection_error(f'待审列表第 {index + 1} 项创建时间类型不可识别。')
        records.append({'id': instance_id.strip(), 'title': title, 'status': status, 'createTime': create_time})
    return records, None


def project_detail(payload: Any) -> tuple[list[dict[str, str]], Optional[dict[str, Any]]]:
    value = unwrap_unified(payload)
    if isinstance(value, dict) and isinstance(value.get('result'), dict):
        value = value['result']
    if not isinstance(value, dict) or not isinstance(value.get('formComponentValues'), list):
        return [], projection_error('审批详情缺少 formComponentValues[]。')
    forms: list[dict[str, str]] = []
    for index, row in enumerate(value['formComponentValues']):
        if not isinstance(row, dict):
            return [], projection_error(f'审批详情表单第 {index + 1} 项不是对象。')
        name = row.get('name') or ''
        content = row.get('value') or ''
        if not isinstance(name, str) or not isinstance(content, str):
            return [], projection_error(f'审批详情表单第 {index + 1} 项名称或值不是字符串。')
        forms.append({'name': name, 'value': content})
    return forms, None


def child_meta(identifier: str, result: ChildDWSResult) -> Optional[dict[str, Any]]:
    return {'id': identifier, 'meta': result.meta} if result.meta else None


def main() -> int:
    parser = argparse.ArgumentParser(description='查看待我审批列表')
    parser.add_argument('--days', type=int, default=7, help='查询天数 (默认 7)')
    add_contract_flags(parser)
    args = parser.parse_args()
    if args.days <= 0:
        return failure(args.format, '--days 必须大于 0')
    now = datetime.now()
    start = now - timedelta(days=args.days)
    if args.dry_run:
        run_dws(['oa', 'approval', 'list-pending', '--start', to_iso(start), '--end', to_iso(now), '--format', 'json'], dry_run=True)
        run_dws(['oa', 'approval', 'detail', '--instance-id', '<INSTANCE_ID>', '--format', 'json'], dry_run=True)
        return emit(fmt=args.format, outcome='success', data={'days': args.days, 'start': to_iso(start), 'end': to_iso(now)}, dry_run=True, text='[dry-run] 将查询待审列表并逐条读取详情')

    meta_children: list[dict[str, Any]] = []
    result = run_dws(['oa', 'approval', 'list-pending', '--start', to_iso(start), '--end', to_iso(now), '--format', 'json'])
    entry = child_meta('pending:list', result)
    if entry:
        meta_children.append(entry)
    if result.state != 'success':
        return emit(fmt=args.format, outcome='failure', error=result.error or projection_error('待审列表查询失败。'), meta={'children': meta_children} if meta_children else None, text='待审列表查询失败')
    instances, error = project_instances(result.payload)
    if error:
        return emit(fmt=args.format, outcome='failure', error=error, meta={'children': meta_children} if meta_children else None, text='待审列表响应无法可靠解析')

    succeeded: list[dict[str, Any]] = []
    failed: list[dict[str, Any]] = []
    items: list[dict[str, Any]] = []
    for instance in instances:
        instance_id = instance['id']
        detail = run_dws(['oa', 'approval', 'detail', '--instance-id', instance_id, '--format', 'json'])
        entry = child_meta(f'detail:{instance_id}', detail)
        if entry:
            meta_children.append(entry)
        if detail.state != 'success':
            failed.append({'id': instance_id, 'error': detail.error or projection_error('审批详情查询失败。')})
            continue
        forms, error = project_detail(detail.payload)
        if error:
            failed.append({'id': instance_id, 'error': error})
            continue
        record = {**instance, 'formComponentValues': forms}
        items.append(record)
        succeeded.append(record)

    result_data = batch_data(succeeded=succeeded, failed=failed, count=len(items), items=items, coverage={'scope': 'server_list_response', 'complete': False})
    outcome = batch_outcome(result_data)
    meta = {'children': meta_children} if meta_children else None
    if outcome == 'failure':
        return emit(fmt=args.format, outcome='failure', data=result_data, error=failed[0]['error'], meta=meta, text='所有待审详情均读取失败')
    if args.format != 'text':
        data = {'count': len(items), 'items': items, 'coverage': {'scope': 'server_list_response', 'complete': False}} if outcome == 'success' else result_data
        return emit(fmt=args.format, outcome=outcome, data=data, meta=meta)

    if not instances:
        print('✅ 暂无待审批事项')
        return 0
    print(f"\n🔔 待审批列表 ({len(instances)} 条)")
    print('=' * 50)
    for index, item in enumerate(items, 1):
        print(f"\n  [{index}] {item['title']}")
        print(f"      状态: {item['status']}  创建: {item['createTime']}")
        print(f"      ID: {item['id']}")
        for form in item['formComponentValues'][:5]:
            if form['value']:
                print(f"      {form['name']}: {form['value'][:60]}")
    if outcome == 'partial_failure':
        return emit(fmt=args.format, outcome='partial_failure', data=result_data, meta=meta, text='警告：部分待审详情读取失败；已保留成功项。')
    return 0


if __name__ == '__main__':
    raise SystemExit(run_main(main))
