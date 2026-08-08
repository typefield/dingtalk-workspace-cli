#!/usr/bin/env python3
"""Produce Agent-review evidence for DWS's typed-error surface.

This is deliberately not a CI policy.  It inventories source-level subtype
(`WithReason`) declarations and the recovery metadata accompanying them so an
Agent reviewer can decide which values deserve a stable public contract.  The
only output is human-readable Markdown; no runtime JSON fixture is saved.
"""

from __future__ import annotations

import argparse
from collections import defaultdict
from dataclasses import dataclass, field
from datetime import date
from pathlib import Path
import re


REASON = re.compile(r"\b(?:(?:apperrors|errors)\.)?WithReason\(\s*\"([^\"]+)\"\s*\)")
# Deliberately line-bounded: a call spread across lines needs manual Agent
# review, but a broad `[^)]` expression accidentally sees function/test names
# containing the word WithReason as runtime construction sites.
NON_LITERAL_REASON = re.compile(r"\b(?:(?:apperrors|errors)\.)?WithReason\(\s*(?!\")([^\n)]*)\)")
STRUCT_SUBTYPE = re.compile(r"\bSubtype\s*:\s*\"([^\"]+)\"")
CONSTRUCTOR = re.compile(r"\b(?:apperrors\.)?New(API|Auth|Validation|Discovery|Internal)\(")
HINT = re.compile(r"(?:\b(?:apperrors|errors)\.)?WithHint\(")
ACTIONS = re.compile(r"(?:\b(?:apperrors|errors)\.)?WithActions\(")
RETRYABLE = re.compile(r"(?:\b(?:apperrors|errors)\.)?WithRetryable\(")
RETRY_AFTER = re.compile(r"(?:\b(?:apperrors|errors)\.)?WithRetryAfterSeconds\(")
EXECUTION = re.compile(r"(?:\b(?:apperrors|errors)\.)?WithExecutionStarted\(")


@dataclass
class Occurrence:
    path: str
    line: int
    category: str
    hint: bool
    actions: bool
    retryable: bool
    retry_after: bool
    execution_started: bool


@dataclass
class ReasonFacts:
    occurrences: list[Occurrence] = field(default_factory=list)
    sources: set[str] = field(default_factory=set)


def line_at(text: str, pos: int) -> int:
    return text.count("\n", 0, pos) + 1


def local_context(lines: list[str], index: int) -> str:
    """Return one source-local construction window, not a semantic proof.

    Error option functions commonly share the `New*` call across a small group
    of lines.  The window intentionally reports *observed nearby metadata*;
    reviewers must still inspect the source for control-flow-specific recovery.
    """

    start = max(0, index - 8)
    end = min(len(lines), index + 13)
    return "\n".join(lines[start:end])


def nearby_category(lines: list[str], index: int) -> str:
    before = "\n".join(lines[max(0, index - 16): index + 1])
    matches = list(CONSTRUCTOR.finditer(before))
    return matches[-1].group(1).lower() if matches else "unresolved"


def scan(root: Path) -> tuple[dict[str, ReasonFacts], list[str], dict[str, list[str]]]:
    facts: dict[str, ReasonFacts] = defaultdict(ReasonFacts)
    dynamic: list[str] = []
    struct_subtypes: dict[str, list[str]] = defaultdict(list)
    excluded = {".git", "vendor", "node_modules", ".worktrees"}

    for path in sorted(root.rglob("*.go")):
        if path.name.endswith("_test.go") or any(part in excluded for part in path.parts):
            continue
        text = path.read_text(encoding="utf-8")
        lines = text.splitlines()
        relative = path.relative_to(root).as_posix()
        literal_spans: list[tuple[int, int]] = []
        for match in REASON.finditer(text):
            literal_spans.append(match.span())
            reason = match.group(1)
            line = line_at(text, match.start())
            context = local_context(lines, line - 1)
            occurrence = Occurrence(
                path=relative,
                line=line,
                category=nearby_category(lines, line - 1),
                hint=bool(HINT.search(context)),
                actions=bool(ACTIONS.search(context)),
                retryable=bool(RETRYABLE.search(context)),
                retry_after=bool(RETRY_AFTER.search(context)),
                execution_started=bool(EXECUTION.search(context)),
            )
            facts[reason].occurrences.append(occurrence)
            facts[reason].sources.add(relative)
        for match in NON_LITERAL_REASON.finditer(text):
            if any(match.start() >= start and match.end() <= end for start, end in literal_spans):
                continue
            line = line_at(text, match.start())
            if lines[line - 1].lstrip().startswith("func "):
                continue
            dynamic.append(f"{relative}:{line}: `{match.group(0).strip()}`")
        for match in STRUCT_SUBTYPE.finditer(text):
            struct_subtypes[match.group(1)].append(f"{relative}:{line_at(text, match.start())}")

    return facts, sorted(dynamic), dict(sorted(struct_subtypes.items()))


def observed(values: list[Occurrence], attr: str) -> str:
    return "yes" if any(getattr(value, attr) for value in values) else "no"


def categories(values: list[Occurrence]) -> str:
    return ", ".join(sorted({value.category for value in values}))


def examples(values: list[Occurrence]) -> str:
    locations = [f"{value.path}:{value.line}" for value in values[:3]]
    suffix = " …" if len(values) > 3 else ""
    return "<br>".join(f"`{location}`" for location in locations) + suffix


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True, help="Markdown evidence path")
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[2]
    facts, dynamic, struct_subtypes = scan(root)
    all_occurrences = sum(len(item.occurrences) for item in facts.values())
    no_hint = sum(1 for item in facts.values() if not any(value.hint for value in item.occurrences))
    unresolved = sum(1 for item in facts.values() if "unresolved" in {value.category for value in item.occurrences})

    lines = [
        "# DWS 错误契约 Agent 扫描",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 本报告由 Agent 读取当前 Go 源码生成，不是 CI 门禁；只保存 Markdown 证据，不生成或保存运行时 JSON fixture。",
        "",
        "## 当前事实",
        "",
        f"- `WithReason(\"…\")` 的字面 subtype：**{len(facts)}** 个；调用点：**{all_occurrences}** 个。",
        f"- 直接构造 `ErrorInfo.Subtype`：**{len(struct_subtypes)}** 个不同值。",
        f"- 动态 `WithReason(variable)` 调用：**{len(dynamic)}** 个。",
        f"- 至少一个调用点缺少邻近 `WithHint` 的 subtype：**{no_hint}** 个。",
        f"- 无法从同一局部构造窗口解析 Category 的 subtype：**{unresolved}** 个。",
        "",
        "当前 DWS 没有 subtype 注册表；以上值均是自由字符串。这份扫描的用途是建立迁移基线，**不**把“出现过”误写成“已经 wire-stable”。",
        "",
        "## 字面 subtype 清单",
        "",
        "| subtype | 调用点 | 推断 Category | hint | actions | retryable | retry-after | execution-started | 例子 |",
        "|---|---:|---|:---:|:---:|:---:|:---:|:---:|---|",
    ]
    for reason, item in sorted(facts.items()):
        values = item.occurrences
        lines.append(
            f"| `{reason}` | {len(values)} | `{categories(values)}` | {observed(values, 'hint')} | "
            f"{observed(values, 'actions')} | {observed(values, 'retryable')} | "
            f"{observed(values, 'retry_after')} | {observed(values, 'execution_started')} | {examples(values)} |"
        )

    lines += [
        "",
        "## 直接 ErrorInfo subtype",
        "",
        "这类值绕过 `WithReason`；迁移到注册表时必须一并纳入，不能只扫描错误构造函数。",
        "",
        "| subtype | 位置 |",
        "|---|---|",
    ]
    if struct_subtypes:
        for subtype, locations in struct_subtypes.items():
            lines.append(f"| `{subtype}` | {', '.join(f'`{location}`' for location in locations[:5])}{' …' if len(locations) > 5 else ''} |")
    else:
        lines.append("| — | — |")

    lines += [
        "",
        "## 动态 subtype 构造",
        "",
        "动态构造在注册表启用前必须人工审阅：要么映射到声明的稳定 subtype，要么统一归入 `api/upstream_unclassified` 并保留上游码/trace，不能把上游任意文本直接变成 Agent 分支键。",
        "",
    ]
    if dynamic:
        lines.extend(f"- {item}" for item in dynamic)
    else:
        lines.append("- 无")

    lines += [
        "",
        "## Agent 审阅结论",
        "",
        "1. 当前 `Category`/退出码、`hint/actions/retryable/retry_after_seconds` 已是可复用底座。",
        "2. 不能直接宣布 subtype 已稳定：`WithReason(string)` 没有闭集，也没有逐 subtype 的恢复字段声明。",
        "3. 下一步应先从本清单审定少量高频、跨产品 subtype（如 `missing_required_flags`、`unknown_flag`、`confirmation_required`、`rate_limit`、`pagination_inconsistent`、`projection_unknown`），建立注册表；不应一次性重命名现有 wire 字段或类别。",
        "4. 扫描只证明源码出现与邻近选项，不能证明服务端终态或 recovery action 在真实账号上可执行。",
    ]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
