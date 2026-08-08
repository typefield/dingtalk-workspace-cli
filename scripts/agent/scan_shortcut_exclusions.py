#!/usr/bin/env python3
"""Agent audit of executable shortcuts outside the public Agent surface.

The runtime catalog is queried in mock mode and held in memory.  The audit does
not publish hidden commands and does not persist the JSON response; it produces
only a Markdown review queue so every exclusion has an explicit owner decision.
"""

from __future__ import annotations

import argparse
import json
from datetime import date
from pathlib import Path
import subprocess

# Decisions for exclusions that have already been reviewed.  Keeping the
# rationale beside the scanner prevents a later catalog regeneration from
# turning a previous decision into an unexplained yes/no bit.
REVIEWED_HIDDEN_REASONS = {
    ("chat", "+conversation-mute-at-all"): "保留隐藏：写操作会影响群成员提醒状态，当前 Agent 选择面暂不发布；运行时继续保留 user_required 门禁。",
    ("chat", "+conversation-mute-red-envelope"): "保留隐藏：写操作会影响群成员红包提醒状态，当前 Agent 选择面暂不发布；运行时继续保留 user_required 门禁。",
    ("doc", "+comment-create-inline"): "保留隐藏：文档内联评论写入需要文档位置和内容的额外语义审阅，暂不作为通用 Agent 入口。",
    ("doc", "+template-apply"): "保留隐藏：模板套用会对目标文档产生写入，待 dry-run/回滚和真实投影证据齐全后再决定公开。",
}


def load_runtime(root: Path) -> list[dict]:
    result = subprocess.run(
        ["go", "run", "./cmd", "shortcut", "list", "--all", "--mock", "--format", "json"],
        cwd=root, capture_output=True, text=True, check=False, timeout=900,
    )
    if result.returncode != 0:
        raise RuntimeError(f"shortcut list failed (rc={result.returncode}): {result.stderr.strip()}")
    payload = json.loads(result.stdout)
    return list(payload.get("shortcuts", []))


def render(root: Path, output: Path) -> int:
    try:
        rows = load_runtime(root)
    except Exception as exc:  # noqa: BLE001 - evidence must record probe failure
        output.write_text(
            "# Shortcut exclusion Agent scan\n\n"
            f"- generated_at: `{date.today().isoformat()}`\n- result: **FAIL**\n- error: `{exc}`\n",
            encoding="utf-8",
        )
        return 1

    hidden = [row for row in rows if row.get("public") is not True]
    unreviewed = [row for row in hidden if row.get("reviewed") is not True]
    reviewed_hidden = [row for row in hidden if row.get("reviewed") is True]
    missing_reasons = [
        (str(row.get("service", "")), str(row.get("command", "")))
        for row in reviewed_hidden
        if (str(row.get("service", "")), str(row.get("command", "")))
        not in REVIEWED_HIDDEN_REASONS
    ]
    lines = [
        "# Shortcut exclusion Agent scan",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 本报告由 Agent 从当前 Runtime Schema 生成；不修改公开目录，不保存运行时 JSON。`public=false` 只表示未进入 Agent 选择面，不代表命令不存在。",
        "",
        "## 汇总",
        "",
        "| 指标 | 数量 |",
        "|---|---:|",
        f"| 运行时 shortcut 总数 | {len(rows)} |",
        f"| public=true | {len(rows) - len(hidden)} |",
        f"| exclusion（public=false） | {len(hidden)} |",
        f"| 已 review 的 exclusion | {len(reviewed_hidden)} |",
        f"| 未 review 的 exclusion | {len(unreviewed)} |",
        "",
        "## 逐条队列",
        "",
        "| service | command | risk | confirmation | reviewed | decision / reason |",
        "|---|---|---|---|:---:|---|",
    ]
    for row in sorted(hidden, key=lambda item: (str(item.get("service")), str(item.get("command")))):
        reviewed = row.get("reviewed") is True
        key = (str(row.get("service", "")), str(row.get("command", "")))
        decision = (
            REVIEWED_HIDDEN_REASONS.get(key, "已审阅但缺少固定原因：不得继续保持该状态")
            if reviewed else "待 Agent 审阅：公开 / 删除 / 保留并写原因"
        )
        lines.append(
            f"| `{row.get('service', '')}` | `{row.get('command', '')}` | "
            f"`{row.get('risk', '')}` | `{row.get('confirmation', '')}` | "
            f"{'yes' if reviewed else 'no'} | {decision} |"
        )
    lines += [
        "",
        "## 规则",
        "",
        "- 只有完成 Contract、Safety、Help/Schema、dry-run 或只读证明，并写入审阅理由后，才允许从 exclusion 移入 public。",
        "- 写/高风险 shortcut 不因“可执行”自动公开；无稳定结果投影或真实后端证据时应继续隐藏。",
        "- 该扫描是 Agent review queue，不是 CI 门禁；每次评测或发布前重新运行。",
    ]
    if missing_reasons:
        lines += [
            "",
            "## 审计错误",
            "",
            "以下已 review exclusion 没有固定理由，不能继续以成功状态通过：",
            *[f"- `{service} {command}`" for service, command in missing_reasons],
        ]
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return 1 if missing_reasons else 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    return render(Path(__file__).resolve().parents[2], args.output)


if __name__ == "__main__":
    raise SystemExit(main())
