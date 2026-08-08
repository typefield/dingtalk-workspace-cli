#!/usr/bin/env python3
"""Send an email with optional CC without overstating delivery success.

The script owns sender resolution and delivery-status interpretation.  The
shared runtime owns output, exit codes, child-call classification and the last
exception boundary.
"""

from __future__ import annotations

import argparse
import sys
from collections.abc import Mapping
from pathlib import Path
import re
from typing import Any, Optional


_SHARED_RUNTIME = Path(__file__).resolve().parents[2] / "dingtalk-shared" / "scripts"
sys.path.insert(0, str(_SHARED_RUNTIME))

from _runtime import ChildDWSResult, add_contract_flags, emit, failure, run_child_dws, run_main  # noqa: E402


EMAIL_PATTERN = re.compile(r"^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$")


def _unwrap(value: Any) -> Any:
    """Unwrap known transport layers without treating arbitrary strings as data."""
    while isinstance(value, Mapping):
        for key in ("data", "result", "content"):
            nested = value.get(key)
            if isinstance(nested, (Mapping, list)):
                value = nested
                break
        else:
            return value
    return value


def _emails(raw: str, label: str) -> tuple[list[str], Optional[str]]:
    values = [item.strip() for item in raw.split(",") if item.strip()]
    if not values:
        return [], f"{label}不能为空。"
    invalid = [item for item in values if not EMAIL_PATTERN.fullmatch(item)]
    if invalid:
        return [], f"{label}包含无效邮箱地址：{', '.join(invalid)}"
    return list(dict.fromkeys(values)), None


def _first_string(value: Any, *keys: str) -> Optional[str]:
    if isinstance(value, Mapping):
        for key in keys:
            candidate = value.get(key)
            if isinstance(candidate, str) and candidate.strip():
                return candidate.strip()
        for key in ("data", "result", "content", "message"):
            candidate = value.get(key)
            found = _first_string(candidate, *keys)
            if found:
                return found
    return None


def _mailboxes(payload: Any) -> list[Mapping[str, Any]]:
    value = _unwrap(payload)
    if isinstance(value, Mapping):
        for key in ("emailAccounts", "mailboxes", "items", "list"):
            nested = value.get(key)
            if isinstance(nested, list):
                return [item for item in nested if isinstance(item, Mapping)]
        if _first_string(value, "email", "address"):
            return [value]
    if isinstance(value, list):
        return [item for item in value if isinstance(item, Mapping)]
    return []


def _select_sender(payload: Any, requested: str) -> tuple[Optional[str], Optional[str]]:
    """Choose one address deterministically; never silently take a first of many."""
    accounts = _mailboxes(payload)
    available: list[tuple[str, bool]] = []
    for account in accounts:
        email = _first_string(account, "email", "address")
        if email and EMAIL_PATTERN.fullmatch(email):
            account_type = str(account.get("type") or account.get("accountType") or "").upper()
            available.append((email, account_type == "ORG"))
    unique = list(dict.fromkeys(available))
    if requested:
        if not EMAIL_PATTERN.fullmatch(requested):
            return None, "--from 必须是有效邮箱地址。"
        if requested not in {email for email, _ in unique}:
            return None, "指定的 --from 不在当前可用邮箱中。"
        return requested, None
    org = [email for email, is_org in unique if is_org]
    if len(org) == 1:
        return org[0], None
    if len(org) > 1:
        return None, "检测到多个企业邮箱；请显式传入 --from 选择发件箱。"
    all_emails = [email for email, _ in unique]
    if len(all_emails) == 1:
        return all_emails[0], None
    if not all_emails:
        return None, "未读取到可用发件邮箱。"
    return None, "检测到多个可用邮箱；请显式传入 --from 选择发件箱。"


def _child_failure(
    *,
    fmt: str,
    stage: str,
    child: ChildDWSResult,
    execution_state: str,
    hint: str,
    data: Optional[dict[str, Any]] = None,
) -> int:
    error = dict(child.error or {"type": "api", "message": f"{stage}未获得可信结果。"})
    error.setdefault("hint", hint)
    result = dict(data or {})
    result.update({"stage": stage, "execution_state": execution_state})
    if child.meta:
        result["child_meta"] = child.meta
    return emit(
        fmt=fmt,
        outcome="failure",
        data=result,
        error=error,
        text=f"错误：{error['message']}",
    )


def _send_business_error(payload: Any) -> Optional[dict[str, Any]]:
    """Interpret only explicit boolean business failure, never string truthiness."""
    value = _unwrap(payload)
    if not isinstance(value, Mapping):
        return None
    if value.get("success") is False or value.get("sent") is False:
        return {
            "type": "api",
            "subtype": "mail_send_rejected",
            "message": str(value.get("message") or value.get("errorMessage") or "邮件服务明确拒绝了发送请求。"),
            "hint": "修正业务条件后再发起新的发送请求；不要把本次请求当作已发送。",
        }
    if "success" in value and not isinstance(value.get("success"), bool):
        return {
            "type": "api",
            "subtype": "mail_send_status_ambiguous",
            "message": "邮件服务返回了非布尔 success 字段，发送结果无法可靠判断。",
            "hint": "先用发送状态查询或已发送邮件核查；不要直接重发。",
        }
    return None


def _verify_status(payload: Any) -> Optional[str]:
    status = _first_string(_unwrap(payload), "sendStatus", "send_status", "status")
    return status.lower() if status else None


def main() -> int:
    parser = argparse.ArgumentParser(description="发送带抄送的邮件，并读取发送状态确认结果。")
    parser.add_argument("--to", required=True, help="收件人，多个邮箱用逗号分隔")
    parser.add_argument("--cc", default="", help="抄送人，多个邮箱用逗号分隔")
    parser.add_argument("--from", dest="sender", default="", help="发件邮箱；多邮箱时必填")
    parser.add_argument("--subject", required=True, help="标题")
    parser.add_argument("--body", required=True, help="正文")
    parser.add_argument("--yes", action="store_true", help="确认发送邮件")
    add_contract_flags(parser, default="json")
    args = parser.parse_args()

    recipients, error = _emails(args.to, "--to")
    if error:
        return failure(args.format, error)
    cc, error = _emails(args.cc, "--cc") if args.cc else ([], None)
    if error:
        return failure(args.format, error)
    if not args.subject.strip() or not args.body.strip():
        return failure(args.format, "--subject 和 --body 不能为空。")
    if args.sender and not EMAIL_PATTERN.fullmatch(args.sender):
        return failure(args.format, "--from 必须是有效邮箱地址。")

    plan = {
        "operation": "mail_send_with_cc",
        "to": recipients,
        "cc": cc,
        "sender": args.sender or "<按邮箱列表解析>",
        "verification": {"state": "not_applicable" if args.dry_run else "pending"},
    }
    if args.dry_run:
        return emit(
            fmt=args.format,
            outcome="success",
            data=plan,
            dry_run=True,
            text="预览：不会读取邮箱或发送邮件。",
        )
    if not args.yes:
        return emit(
            fmt=args.format,
            outcome="failure",
            data={"operation": "mail_send_with_cc", "execution_state": "not_executed"},
            error={
                "type": "policy",
                "subtype": "confirmation_required",
                "message": "发送邮件前需要显式确认。",
                "hint": "确认收件人、抄送人和正文后，使用 --yes 重新执行。",
            },
            text="错误：发送邮件前需要显式确认。",
        )

    mailbox = run_child_dws(["mail", "mailbox", "list", "--format", "json"])
    if mailbox.state != "success":
        return _child_failure(
            fmt=args.format,
            stage="resolve_sender",
            child=mailbox,
            execution_state="unknown" if mailbox.state == "unknown" else "not_executed",
            hint="先核查当前邮箱列表；尚未发送邮件。",
        )
    sender, error = _select_sender(mailbox.payload, args.sender)
    if error:
        return emit(
            fmt=args.format,
            outcome="failure",
            data={"stage": "resolve_sender", "execution_state": "not_executed"},
            error={"type": "precondition", "message": error, "hint": "选择一个可用发件邮箱后重新执行。"},
            text=f"错误：{error}",
        )

    send_args = [
        "mail", "message", "send", "--from", sender, "--to", ",".join(recipients),
        "--subject", args.subject, "--content", args.body, "--format", "json",
    ]
    if cc:
        send_args.extend(["--cc", ",".join(cc)])
    sent = run_child_dws(send_args)
    context = {"sender": sender, "to": recipients, "cc": cc}
    if sent.state != "success":
        return _child_failure(
            fmt=args.format,
            stage="send",
            child=sent,
            execution_state="unknown" if sent.state == "unknown" else "not_executed",
            hint="若请求可能已送达，请先检查已发送邮件或使用邮件状态查询；不要盲目重发。",
            data=context,
        )
    business_error = _send_business_error(sent.payload)
    if business_error:
        return emit(
            fmt=args.format,
            outcome="failure",
            data={**context, "stage": "send", "execution_state": "not_completed"},
            error=business_error,
            text=f"错误：{business_error['message']}",
        )
    internet_message_id = _first_string(sent.payload, "internetMessageId", "internet_message_id")
    if not internet_message_id:
        return emit(
            fmt=args.format,
            outcome="failure",
            data={**context, "stage": "send", "execution_state": "unknown"},
            error={
                "type": "api",
                "subtype": "mail_send_missing_identifier",
                "message": "发送接口未返回 internetMessageId，无法核验投递状态。",
                "hint": "先检查已发送邮件或发送状态；不要直接重发。",
            },
            text="错误：发送接口未返回可核验的邮件标识。",
        )

    verification = run_child_dws([
        "mail", "message", "verify", "--email", sender,
        "--internet-message-id", internet_message_id, "--format", "json",
    ])
    result = {**context, "internet_message_id": internet_message_id}
    if verification.state != "success":
        return _child_failure(
            fmt=args.format,
            stage="verify",
            child=verification,
            execution_state="unknown",
            hint="发送请求可能已受理；请使用 internetMessageId 查询或检查已发送邮件，勿直接重发。",
            data=result,
        )
    status = _verify_status(verification.payload)
    result["verification"] = {"state": "verified" if status == "success" else "not_verified", "send_status": status or "unknown"}
    if status == "success":
        return emit(
            fmt=args.format,
            outcome="success",
            data=result,
            meta={"send": sent.meta, "verify": verification.meta},
            text="邮件发送状态已验证为 success。",
        )
    if status == "partial_success":
        return emit(
            fmt=args.format,
            outcome="failure",
            data={**result, "execution_state": "partial_unknown"},
            error={"type": "api", "subtype": "mail_delivery_partial", "message": "邮件服务报告部分投递成功。", "hint": "根据 sendStatus 和收件人状态核查后再补发失败对象。"},
            text="邮件服务报告部分投递成功。",
        )
    if status in {"posting", "unknown", None}:
        message = "邮件仍在投递中。" if status == "posting" else "无法确认邮件的终态投递状态。"
        return emit(
            fmt=args.format,
            outcome="failure",
            data={**result, "execution_state": "unknown"},
            error={"type": "api", "subtype": "mail_delivery_not_terminal", "message": message, "hint": "使用 internetMessageId 再次查询；不要立即重发。"},
            text=f"错误：{message}",
        )
    return emit(
        fmt=args.format,
        outcome="failure",
        data={**result, "execution_state": "completed"},
        error={"type": "api", "subtype": "mail_delivery_failed", "message": f"邮件服务报告发送状态：{status}。", "hint": "修正投递条件后重新发起新的发送请求。"},
        text=f"错误：邮件服务报告发送状态：{status}。",
    )


if __name__ == "__main__":
    raise SystemExit(run_main(main, default_format="json"))
