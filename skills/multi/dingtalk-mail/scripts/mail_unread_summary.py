#!/usr/bin/env python3
"""查询今日未读邮件；邮箱发现与消息搜索使用同一机器结果边界。"""

from __future__ import annotations

import argparse
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any, Optional

_SHARED_RUNTIME = Path(__file__).resolve().parents[2] / 'dingtalk-shared' / 'scripts'
sys.path.insert(0, str(_SHARED_RUNTIME))

from _runtime import (
    ChildDWSResult,
    add_contract_flags,
    emit,
    failure,
    run_child_dws,
    run_main,
)


TZ = timezone(timedelta(hours=8))


def run_dws(args: list[str], dry_run: bool = False) -> ChildDWSResult:
    return run_child_dws(args, dry_run=dry_run)


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


def project_email(payload: Any) -> tuple[str, Optional[dict[str, Any]]]:
    value = unwrap_unified(payload)
    if isinstance(value, dict) and 'result' in value:
        value = value['result']
    if isinstance(value, dict) and 'content' in value and isinstance(value['content'], (dict, list)):
        value = value['content']

    if isinstance(value, dict) and isinstance(value.get('emailAccounts'), list):
        accounts = value['emailAccounts']
    elif isinstance(value, list):
        accounts = value
    elif isinstance(value, dict) and ('email' in value or 'address' in value):
        accounts = [value]
    else:
        return '', projection_error('邮箱列表响应缺少 emailAccounts[] 或邮箱对象。')

    projected: list[tuple[str, str]] = []
    for index, account in enumerate(accounts):
        if not isinstance(account, dict):
            return '', projection_error(f'邮箱列表第 {index + 1} 项不是对象。')
        email = account.get('email') or account.get('address')
        account_type = account.get('type') or ''
        if not isinstance(email, str) or not email.strip():
            return '', projection_error(f'邮箱列表第 {index + 1} 项缺少邮箱地址。')
        if not isinstance(account_type, str):
            return '', projection_error(f'邮箱列表第 {index + 1} 项 type 不是字符串。')
        projected.append((account_type, email.strip()))
    if not projected:
        return '', {
            'type': 'api',
            'subtype': 'mailbox_not_found',
            'message': '当前账号没有可用邮箱。',
        }
    for account_type, email in projected:
        if account_type == 'ORG':
            return email, None
    return projected[0][1], None


def project_messages(payload: Any) -> tuple[list[dict[str, str]], Optional[dict[str, Any]]]:
    value = unwrap_unified(payload)
    if isinstance(value, dict) and 'result' in value:
        value = value['result']
    if isinstance(value, list):
        messages = value
    elif isinstance(value, dict) and isinstance(value.get('items'), list):
        messages = value['items']
    elif isinstance(value, dict) and isinstance(value.get('messages'), list):
        messages = value['messages']
    else:
        return [], projection_error('邮件搜索响应缺少 items[]/messages[]。')

    projected: list[dict[str, str]] = []
    for index, message in enumerate(messages):
        if not isinstance(message, dict):
            return [], projection_error(f'邮件搜索第 {index + 1} 项不是对象。')
        subject = message.get('subject', '(无主题)')
        sender = message.get('from', {})
        if not isinstance(subject, str):
            return [], projection_error(f'邮件搜索第 {index + 1} 项 subject 不是字符串。')
        if isinstance(sender, dict):
            sender_name = sender.get('name') or sender.get('email') or '未知'
        elif isinstance(sender, str):
            sender_name = sender
        else:
            return [], projection_error(f'邮件搜索第 {index + 1} 项 from 类型不可识别。')
        if not isinstance(sender_name, str):
            return [], projection_error(f'邮件搜索第 {index + 1} 项发件人不是字符串。')
        projected.append({'subject': subject, 'sender': sender_name})
    return projected, None


def child_meta(identifier: str, result: ChildDWSResult) -> Optional[dict[str, Any]]:
    return {'id': identifier, 'meta': result.meta} if result.meta else None


def main() -> int:
    parser = argparse.ArgumentParser(description='查询今天未读邮件')
    parser.add_argument('--limit', type=int, default=20, help='返回数量')
    parser.add_argument(
        '--size', dest='limit', type=int, default=argparse.SUPPRESS,
        help=argparse.SUPPRESS,
    )
    add_contract_flags(parser)
    args = parser.parse_args()
    if args.limit <= 0:
        return failure(args.format, '--limit 必须大于 0')

    today = datetime.now(TZ).strftime('%Y-%m-%dT00:00:00Z')
    query = f'isRead:false AND date>{today}'
    if args.dry_run:
        run_dws(['mail', 'mailbox', 'list', '--format', 'json'], dry_run=True)
        run_dws([
            'mail', 'message', 'search', '--email', '<MY_EMAIL>', '--query', query,
            '--limit', str(args.limit), '--format', 'json',
        ], dry_run=True)
        return emit(
            fmt=args.format,
            outcome='success',
            data={'query': query, 'limit': args.limit},
            dry_run=True,
            text='[dry-run] 将发现邮箱并查询今日未读邮件',
        )

    meta_children: list[dict[str, Any]] = []
    print('📬 获取邮箱地址...', file=sys.stderr)
    mailbox_result = run_dws(['mail', 'mailbox', 'list', '--format', 'json'])
    entry = child_meta('mailbox:list', mailbox_result)
    if entry:
        meta_children.append(entry)
    if mailbox_result.state != 'success':
        return emit(
            fmt=args.format,
            outcome='failure',
            error=mailbox_result.error or projection_error('邮箱列表查询失败。'),
            meta={'children': meta_children} if meta_children else None,
            text='邮箱列表查询失败',
        )
    email, error = project_email(mailbox_result.payload)
    if error:
        return emit(
            fmt=args.format,
            outcome='failure',
            error=error,
            meta={'children': meta_children} if meta_children else None,
            text='无法可靠确定邮箱地址',
        )

    print('🔍 搜索未读邮件...', file=sys.stderr)
    search_result = run_dws([
        'mail', 'message', 'search', '--email', email, '--query', query,
        '--limit', str(args.limit), '--format', 'json',
    ])
    entry = child_meta('message:search', search_result)
    if entry:
        meta_children.append(entry)
    if search_result.state != 'success':
        return emit(
            fmt=args.format,
            outcome='failure',
            data={'email': email, 'query': query},
            error=search_result.error or projection_error('邮件搜索失败。'),
            meta={'children': meta_children} if meta_children else None,
            text='邮件搜索失败',
        )
    messages, error = project_messages(search_result.payload)
    if error:
        return emit(
            fmt=args.format,
            outcome='failure',
            data={'email': email, 'query': query},
            error=error,
            meta={'children': meta_children} if meta_children else None,
            text='邮件搜索响应无法可靠解析',
        )

    data = {'email': email, 'query': query, 'count': len(messages), 'messages': messages}
    meta = {'children': meta_children} if meta_children else None
    if args.format != 'text':
        return emit(fmt=args.format, outcome='success', data=data, meta=meta)

    print('📧 今日未读邮件')
    print('=' * 50)
    if not messages:
        print('  ✅ 收件箱清空，没有未读邮件！')
        return 0
    for item in messages:
        print(f"  📩 {item['subject']}")
        print(f"     发件人: {item['sender']}")
    print(f"\n合计: {len(messages)} 封未读邮件")
    return 0


if __name__ == '__main__':
    raise SystemExit(run_main(main))
