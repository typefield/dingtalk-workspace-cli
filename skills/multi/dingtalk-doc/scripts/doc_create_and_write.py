#!/usr/bin/env python3
"""Create a document, write Markdown chunks, and expose every write outcome.

The operation is deliberately non-retrying: ``doc update`` is not proven
idempotent, so a timeout or malformed response cannot safely be replayed.
"""

from __future__ import annotations

import argparse
import sys
from collections.abc import Mapping
from pathlib import Path
from typing import Any, Optional


_SHARED_RUNTIME = Path(__file__).resolve().parents[2] / "dingtalk-shared" / "scripts"
sys.path.insert(0, str(_SHARED_RUNTIME))

from _runtime import (  # noqa: E402
    ChildDWSResult,
    add_contract_flags,
    batch_data,
    batch_outcome,
    emit,
    failure,
    run_child_dws,
    run_main,
)


CHUNK_SIZE = 30_000


def _unwrap(value: Any) -> Any:
    while isinstance(value, Mapping):
        for key in ("data", "result", "content"):
            nested = value.get(key)
            if isinstance(nested, (Mapping, list)):
                value = nested
                break
        else:
            return value
    return value


def _document_id(payload: Any) -> str:
    value = _unwrap(payload)
    if not isinstance(value, Mapping):
        return ""
    for key in ("nodeId", "node_id", "dentryUuid", "id"):
        candidate = value.get(key)
        if isinstance(candidate, str) and candidate:
            return candidate
    return ""


def _readback_text(value: Any) -> Optional[str]:
    """Extract real body text only when the read API exposes a stable field."""
    if isinstance(value, Mapping):
        for key in ("markdown", "content", "text", "body"):
            candidate = value.get(key)
            if isinstance(candidate, str):
                return candidate
        for key in ("data", "result", "content", "document"):
            found = _readback_text(value.get(key))
            if found is not None:
                return found
    return None


def _chunks(content: str) -> list[str]:
    result: list[str] = []
    start = 0
    while start < len(content):
        end = min(start + CHUNK_SIZE, len(content))
        if end < len(content):
            newline = content.rfind("\n", start, end)
            if newline > start:
                end = newline + 1
        result.append(content[start:end])
        start = end
    return result


def _child_meta(entry_id: str, child: ChildDWSResult) -> Optional[dict[str, Any]]:
    return {"id": entry_id, "meta": child.meta} if child.meta else None


def _failure_from_child(
    *,
    fmt: str,
    stage: str,
    child: ChildDWSResult,
    execution_state: str,
    data: dict[str, Any],
    meta: list[dict[str, Any]],
) -> int:
    error = dict(child.error or {"type": "api", "message": f"{stage}未获得可信结果。"})
    error.setdefault("hint", "先核查文档是否已创建或写入；不要盲目重试。")
    return emit(
        fmt=fmt,
        outcome="failure",
        data={**data, "stage": stage, "execution_state": execution_state},
        error=error,
        meta={"children": meta} if meta else None,
        text=f"错误：{error['message']}",
    )


def main() -> int:
    parser = argparse.ArgumentParser(description="创建文档并按块写入 Markdown；发送前需显式确认。")
    parser.add_argument("--name", required=True, help="文档名称")
    parser.add_argument("--content", default="", help="Markdown 内容")
    parser.add_argument("--content-file", default="", help="UTF-8 Markdown 文件路径")
    parser.add_argument("--folder", default="", help="目标文件夹 ID 或 URL")
    parser.add_argument("--workspace", default="", help="目标知识库 ID")
    parser.add_argument("--mode", default="append", choices=("overwrite", "append"), help="首块写入模式（默认 append）")
    parser.add_argument("--max-retries", type=int, default=1, help="兼容参数；非幂等写入固定不自动重试，必须为 1")
    parser.add_argument("--yes", action="store_true", help="确认创建并写入文档")
    add_contract_flags(parser, default="json")
    args = parser.parse_args()

    if not args.name.strip():
        return failure(args.format, "--name 不能为空。")
    if args.content and args.content_file:
        return failure(args.format, "--content 与 --content-file 只能指定一个。")
    if args.max_retries != 1:
        return failure(args.format, "为避免重复写入，--max-retries 只能为 1。")
    content = args.content
    if args.content_file:
        source = Path(args.content_file)
        if not source.is_file():
            return failure(args.format, f"内容文件不存在：{source}")
        content = source.read_text(encoding="utf-8")
    if not content:
        return failure(args.format, "需要 --content 或 --content-file。")
    chunks = _chunks(content)
    plan = {
        "operation": "doc_create_and_write",
        "name": args.name,
        "folder": args.folder or None,
        "workspace": args.workspace or None,
        "mode": args.mode,
        "characters": len(content),
        "chunks": len(chunks),
        "verification": {"state": "not_applicable" if args.dry_run else "pending"},
    }
    if args.dry_run:
        return emit(
            fmt=args.format,
            outcome="success",
            data=plan,
            dry_run=True,
            text="预览：不会创建文档、写入内容或读取远端文档。",
        )
    if not args.yes:
        return emit(
            fmt=args.format,
            outcome="failure",
            data={"operation": "doc_create_and_write", "execution_state": "not_executed"},
            error={
                "type": "policy",
                "subtype": "confirmation_required",
                "message": "创建并写入文档前需要显式确认。",
                "hint": "确认名称、位置、内容和覆盖模式后，使用 --yes 重新执行。",
            },
            text="错误：创建并写入文档前需要显式确认。",
        )

    create_args = ["doc", "create", "--name", args.name, "--format", "json"]
    if args.folder:
        create_args.extend(["--folder", args.folder])
    if args.workspace:
        create_args.extend(["--workspace", args.workspace])
    print(f"📝 创建文档：{args.name}", file=sys.stderr)
    create = run_child_dws(create_args)
    child_meta: list[dict[str, Any]] = []
    if (entry := _child_meta("document:create", create)):
        child_meta.append(entry)
    if create.state != "success":
        error = create.error or {"type": "api", "message": "创建文档未获得可信结果。"}
        channels: dict[str, list[dict[str, Any]]] = {"succeeded": [], "failed": [], "unknown": []}
        if create.state == "failed":
            channels["failed"].append({"id": "document:create", "error": error})
            state = "not_executed"
        else:
            channels["unknown"].append({"id": "document:create", "reason": "创建请求未返回终态；文档可能已经创建。", "error": error})
            state = "unknown"
        return _failure_from_child(
            fmt=args.format,
            stage="create",
            child=create,
            execution_state=state,
            data=batch_data(total=1, **channels, name=args.name),
            meta=child_meta,
        )
    node_id = _document_id(create.payload)
    if not node_id:
        error = {"type": "api", "subtype": "doc_create_missing_node_id", "message": "创建响应未返回 nodeId。", "hint": "先搜索或读取文档确认是否已创建；不要直接重试。"}
        return emit(
            fmt=args.format,
            outcome="failure",
            data=batch_data(total=1, unknown=[{"id": "document:create", "reason": "创建调用成功但没有可用于回读的 nodeId。", "error": error}], name=args.name),
            error=error,
            meta={"children": child_meta} if child_meta else None,
            text="错误：创建响应未返回 nodeId。",
        )

    succeeded = [{"id": "document:create", "nodeId": node_id}]
    failed: list[dict[str, Any]] = []
    unknown: list[dict[str, Any]] = []
    for index, chunk in enumerate(chunks):
        chunk_id = f"{node_id}:chunk:{index + 1}"
        mode = args.mode if index == 0 else "append"
        command = ["doc", "update", "--node", node_id, "--content", chunk, "--mode", mode, "--format", "json"]
        if mode == "overwrite":
            command.append("--yes")
        print(f"✍️ 写入第 {index + 1}/{len(chunks)} 块", file=sys.stderr)
        write = run_child_dws(command)
        if (entry := _child_meta(chunk_id, write)):
            child_meta.append(entry)
        if write.state == "success":
            succeeded.append({"id": chunk_id, "chunk": index + 1, "characters": len(chunk), "mode": mode})
            continue
        error = write.error or {"type": "api", "message": "文档写入未获得可信结果。"}
        if write.state == "failed":
            failed.append({"id": chunk_id, "error": error})
        else:
            unknown.append({"id": chunk_id, "reason": "写入请求未返回终态；该块可能已经写入。", "error": error})
        for remaining in range(index + 1, len(chunks)):
            failed.append({
                "id": f"{node_id}:chunk:{remaining + 1}",
                "error": {"type": "precondition", "message": "前一块未得到终态结果，后续块没有执行。"},
            })
        break

    data = batch_data(
        succeeded=succeeded,
        failed=failed,
        unknown=unknown,
        total=1 + len(chunks),
        nodeId=node_id,
        characters=len(content),
        chunks=len(chunks),
    )
    outcome = batch_outcome(data)
    if outcome != "success":
        top_error = failed[0]["error"] if failed else unknown[0]["error"]
        return emit(
            fmt=args.format,
            outcome=outcome,
            data=data,
            error=top_error,
            meta={"children": child_meta} if child_meta else None,
            text="文档已部分写入或写入结果未知；请先回读核查，禁止盲目重试。",
        )

    readback = run_child_dws(["doc", "read", "--node", node_id, "--format", "json"])
    if (entry := _child_meta("document:readback", readback)):
        child_meta.append(entry)
    body = _readback_text(readback.payload) if readback.state == "success" else None
    all_chunks_visible = body is not None and all(chunk in body for chunk in chunks)
    data["verification"] = {
        "state": "verified" if all_chunks_visible else "not_verified",
        "method": "doc_read" if readback.state == "success" else "readback_unavailable",
    }
    if readback.state != "success":
        data["verification"]["reason"] = "写请求均返回成功，但读回失败或结果不确定。"
    elif not all_chunks_visible:
        data["verification"]["reason"] = "读回成功，但当前响应没有可逐块对拍的正文。"
    return emit(
        fmt=args.format,
        outcome="success",
        data=data,
        meta={"children": child_meta} if child_meta else None,
        text="文档创建和写入请求均已完成；请以 verification 状态判断是否已回读验证。",
    )


if __name__ == "__main__":
    raise SystemExit(run_main(main, default_format="json"))
