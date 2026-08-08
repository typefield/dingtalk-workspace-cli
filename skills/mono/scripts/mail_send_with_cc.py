#!/usr/bin/env python3
"""
发送带抄送的邮件（自动获取发件地址、校验参数）

用法:
    python mail_send_with_cc.py \
        --to colleague@company.com \
        --cc boss@company.com,team@company.com \
        --subject "周报" \
        --body "本周完成任务A和任务B"

    python mail_send_with_cc.py --dry-run \
        --to a@b.com --subject "test" --body "hello"
"""

import sys
import json
import subprocess
import re
import argparse
from typing import List, Any, Optional

from _runtime import add_contract_flags, emit, failure

EMAIL_PATTERN = re.compile(
    r'^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$'
)


def run_dws(
    args: List[str], dry_run: bool = False,
) -> Optional[Any]:
    cmd = ['dws'] + args
    if dry_run:
        print(f"[dry-run] {' '.join(cmd)}", file=sys.stderr)
        return {'dry_run': True}
    try:
        result = subprocess.run(
            cmd, capture_output=True, text=True, timeout=60
        )
        if result.returncode != 0:
            print(f"错误：{result.stderr.strip()}", file=sys.stderr)
            return None
        return json.loads(result.stdout)
    except (subprocess.TimeoutExpired, json.JSONDecodeError,
            FileNotFoundError) as e:
        print(f"错误：{e}", file=sys.stderr)
        return None


def validate_emails(emails_str: str) -> bool:
    for email in emails_str.split(','):
        email = email.strip()
        if not EMAIL_PATTERN.match(email):
            print(f"错误：无效邮箱地址 '{email}'", file=sys.stderr)
            return False
    return True


def unwrap_result(data: Any) -> Any:
    while isinstance(data, dict):
        for key in ('result', 'content', 'data'):
            nested = data.get(key)
            if isinstance(nested, (dict, list)):
                data = nested
                break
        else:
            return data
    return data


def get_my_email(dry_run: bool = False) -> Optional[str]:
    data = run_dws([
        'mail', 'mailbox', 'list', '--format', 'json',
    ], dry_run=dry_run)
    if dry_run:
        return '<MY_EMAIL>'
    if not data:
        return None
    data = unwrap_result(data)
    if isinstance(data, dict) and isinstance(data.get('emailAccounts'), list):
        accounts = data['emailAccounts']
        # 优先企业邮箱(type=ORG)，否则取第一个
        for acc in accounts:
            if isinstance(acc, dict) and acc.get('type') == 'ORG' and acc.get('email'):
                return acc['email']
        if accounts and isinstance(accounts[0], dict):
            return accounts[0].get('email')
        return None
    if isinstance(data, list) and data:
        item = data[0]
        return (item.get('email') or item.get('address')
                if isinstance(item, dict) else str(item))
    if isinstance(data, dict):
        return data.get('email') or data.get('address')
    return None


def main() -> int:
    parser = argparse.ArgumentParser(
        description='发送带抄送的邮件'
    )
    parser.add_argument('--to', required=True, help='收件人')
    parser.add_argument('--cc', default='', help='抄送人')
    parser.add_argument('--subject', required=True, help='标题')
    parser.add_argument('--body', required=True, help='正文')
    add_contract_flags(parser)
    args = parser.parse_args()

    if not validate_emails(args.to):
        return failure(args.format, '收件人邮箱地址无效')
    if args.cc and not validate_emails(args.cc):
        return failure(args.format, '抄送邮箱地址无效')

    print('📬 获取发件邮箱...', file=sys.stderr)
    from_email = get_my_email(dry_run=args.dry_run)
    if not from_email and not args.dry_run:
        return failure(args.format, '无法获取发件邮箱')

    cmd_args = [
        'mail', 'message', 'send',
        '--from', from_email or '<MY_EMAIL>',
        '--to', args.to,
        '--subject', args.subject,
        '--content', args.body,
        '--format', 'json',
    ]
    if args.cc:
        cmd_args.extend(['--cc', args.cc])

    if args.dry_run:
        return emit(fmt=args.format, outcome='success', data={
            'from': from_email, 'to': args.to,
            'cc': [x.strip() for x in args.cc.split(',') if x.strip()],
            'subject': args.subject,
        }, dry_run=True, text='[dry-run] 将发送邮件')

    print('📤 发送邮件...', file=sys.stderr)
    result = run_dws(cmd_args, dry_run=args.dry_run)
    if result:
        print(f"  ✓ 邮件已发送", file=sys.stderr)
        print(f"    收件人: {args.to}", file=sys.stderr)
        if args.cc:
            print(f"    抄送: {args.cc}", file=sys.stderr)
        print(f"    主题: {args.subject}", file=sys.stderr)
        return emit(fmt=args.format, outcome='success', data={
            'from': from_email, 'to': args.to,
            'cc': [x.strip() for x in args.cc.split(',') if x.strip()],
            'subject': args.subject,
        })
    else:
        return failure(args.format, '发送失败')


if __name__ == '__main__':
    sys.exit(main())
