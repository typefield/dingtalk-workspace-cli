#!/usr/bin/env python3
"""Produce Agent-review evidence for DWS's typed-error surface.

This is deliberately not a CI policy. It inventories source-level subtype
declarations and their recovery metadata so an Agent reviewer can decide which
values deserve a stable public contract. It distinguishes legacy free-form
`WithReason` calls from registered `WithSubtype` calls. The only output is
human-readable Markdown; no runtime JSON fixture is saved.
"""

from __future__ import annotations

import argparse
from collections import defaultdict
from dataclasses import dataclass, field
from datetime import date
from pathlib import Path
import re


REASON = re.compile(r"\b(?:(?:apperrors|errors)\.)?WithReason\(\s*\"([^\"]+)\"\s*\)")
SUBTYPE = re.compile(r"\b(?:(?:apperrors|errors)\.)?WithSubtype\(\s*(?:(?:apperrors|errors)\.)?(Subtype[A-Za-z0-9_]+)\s*\)")
STABLE_SUBTYPE_BRIDGE = re.compile(
    r"\b(?:(?:apperrors|errors)\.)?WithStableSubtypeAndLegacyReason\(\s*(?:(?:apperrors|errors)\.)?(Subtype[A-Za-z0-9_]+)\s*,\s*(?:\"[^\"]+\"|[a-z][A-Za-z0-9_]*)\s*,?\s*\)"
)
# A stable mapping may be returned by a finite helper, or selected from a
# finite local enum before it reaches WithSubtype.  Record both forms for
# Agent review: neither is a free-form public subtype, but neither should be
# silently omitted from the evidence count either.
INDIRECT_SUBTYPE_CALL = re.compile(r"\b(?:(?:apperrors|errors)\.)?WithSubtype\(\s*([a-z][A-Za-z0-9_]*)\(")
INDIRECT_SUBTYPE_VARIABLE = re.compile(r"\b(?:(?:apperrors|errors)\.)?WithSubtype\(\s*([a-z][A-Za-z0-9_]*)\s*\)")
SUBTYPE_CONST = re.compile(r"\b(Subtype[A-Za-z0-9_]+)\s+Subtype\s*=\s*\"([^\"]+)\"")
DESCRIPTOR_CATEGORY = re.compile(r"\b(Subtype[A-Za-z0-9_]+):\s*\{\s*Subtype:\s*\1,\s*Category:\s*Category([A-Za-z0-9_]+)", re.MULTILINE)
DESCRIPTOR_DEFAULT_HINT = re.compile(
    r"\b(Subtype[A-Za-z0-9_]+):\s*\{(?:(?!\n\t\},).)*\bDefaultHint:\s*\"([^\"]+)\"",
    re.DOTALL,
)
# Deliberately line-bounded: a call spread across lines needs manual Agent
# review, but a broad `[^)]` expression accidentally sees function/test names
# containing the word WithReason as runtime construction sites.
NON_LITERAL_REASON = re.compile(r"\b(?:(?:apperrors|errors)\.)?WithReason\(\s*(?!\")([^\n)]*)\)")
STRUCT_SUBTYPE = re.compile(r"\bSubtype\s*:\s*\"([^\"]+)\"")
# Unified output uses ErrorInfo.Subtype (a string field), so the type-safe
# spelling is `string(apperrors.SubtypeFoo)`. It is still a direct machine
# subtype assignment and must stay visible to Agent review rather than being
# hidden merely because it is not a quoted literal.
STRUCT_SUBTYPE_CONST = re.compile(
    r"\bSubtype\s*:\s*string\(\s*(?:(?:apperrors|errors)\.)?(Subtype[A-Za-z0-9_]+)\s*\)"
)
CONSTRUCTOR = re.compile(r"\b(?:apperrors\.)?New(API|Auth|Validation|Discovery|Internal)\(")
HINT = re.compile(r"(?:\b(?:apperrors|errors)\.)?WithHint\(")
ACTIONS = re.compile(r"(?:\b(?:apperrors|errors)\.)?WithActions\(")
RETRYABLE = re.compile(r"(?:\b(?:apperrors|errors)\.)?WithRetryable\(")
RETRYABLE_VALUE = re.compile(r"(?:\b(?:apperrors|errors)\.)?WithRetryable\(\s*(true|false)\s*\)")
RETRY_AFTER = re.compile(r"(?:\b(?:apperrors|errors)\.)?WithRetryAfterSeconds\(")
EXECUTION = re.compile(r"(?:\b(?:apperrors|errors)\.)?WithExecutionStarted\(")


@dataclass
class Occurrence:
    path: str
    line: int
    category: str
    hint: bool
    actions: bool
    retryable: str
    retry_after: bool
    execution_started: bool
    registered: bool


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


def nearby_retryable_value(context: str) -> str:
    """Describe a nearby explicit retryability value without guessing it.

    A mere `WithRetryable(...)` occurrence is not equivalent to
    `retryable:true`: high-risk write paths deliberately use
    `WithRetryable(false)`.  This scan is source-local rather than a control
    flow proof, so a variable expression stays `unknown` and competing nearby
    constructors stay `mixed`.
    """
    values = {match.group(1) for match in RETRYABLE_VALUE.finditer(context)}
    if len(values) == 1:
        return values.pop()
    if len(values) > 1:
        return "mixed"
    if RETRYABLE.search(context):
        return "unknown"
    return "not_declared"


def nearby_category(lines: list[str], index: int) -> str:
    before = "\n".join(lines[max(0, index - 16): index + 1])
    matches = list(CONSTRUCTOR.finditer(before))
    return matches[-1].group(1).lower() if matches else "unresolved"


def scan(root: Path) -> tuple[dict[str, ReasonFacts], list[str], list[str], dict[str, list[str]], dict[str, str]]:
    facts: dict[str, ReasonFacts] = defaultdict(ReasonFacts)
    dynamic: list[str] = []
    indirect_subtypes: list[str] = []
    struct_subtypes: dict[str, list[str]] = defaultdict(list)
    excluded = {".git", "vendor", "node_modules", ".worktrees"}

    paths = [path for path in sorted(root.rglob("*.go")) if not path.name.endswith("_test.go") and not any(part in excluded for part in path.parts)]
    subtype_constants: dict[str, str] = {}
    descriptor_categories: dict[str, str] = {}
    # A descriptor-level default is an effective recovery hint when a command
    # does not need to provide a more-specific one. This remains an Agent scan
    # fact, not a CI policy or a claim that the recovery was executed live.
    descriptor_default_hints: dict[str, str] = {}
    for path in paths:
        text = path.read_text(encoding="utf-8")
        subtype_constants.update({match.group(1): match.group(2) for match in SUBTYPE_CONST.finditer(text)})
        descriptor_categories.update({match.group(1): match.group(2).lower() for match in DESCRIPTOR_CATEGORY.finditer(text)})
        descriptor_default_hints.update({match.group(1): match.group(2) for match in DESCRIPTOR_DEFAULT_HINT.finditer(text)})

    for path in paths:
        text = path.read_text(encoding="utf-8")
        lines = text.splitlines()
        relative = path.relative_to(root).as_posix()
        literal_spans: list[tuple[int, int]] = []
        for match in REASON.finditer(text):
            line = line_at(text, match.start())
            if lines[line - 1].lstrip().startswith("//"):
                continue
            literal_spans.append(match.span())
            reason = match.group(1)
            context = local_context(lines, line - 1)
            occurrence = Occurrence(
                path=relative,
                line=line,
                category=nearby_category(lines, line - 1),
                hint=bool(HINT.search(context)),
                actions=bool(ACTIONS.search(context)),
                retryable=nearby_retryable_value(context),
                retry_after=bool(RETRY_AFTER.search(context)),
                execution_started=bool(EXECUTION.search(context)),
                registered=False,
            )
            facts[reason].occurrences.append(occurrence)
            facts[reason].sources.add(relative)
        for pattern in (SUBTYPE, STABLE_SUBTYPE_BRIDGE):
            for match in pattern.finditer(text):
                constant = match.group(1)
                line = line_at(text, match.start())
                if lines[line - 1].lstrip().startswith("//"):
                    continue
                subtype = subtype_constants.get(constant)
                if subtype is None:
                    dynamic.append(f"{relative}:{line}: `{match.group(0).strip()}` (unknown subtype constant)")
                    continue
                context = local_context(lines, line - 1)
                occurrence = Occurrence(
                    path=relative,
                    line=line,
                    category=descriptor_categories.get(constant, nearby_category(lines, line - 1)),
                    hint=bool(HINT.search(context)) or bool(descriptor_default_hints.get(constant)),
                    actions=bool(ACTIONS.search(context)),
                    retryable=nearby_retryable_value(context),
                    retry_after=bool(RETRY_AFTER.search(context)),
                    execution_started=bool(EXECUTION.search(context)),
                    registered=True,
                )
                facts[subtype].occurrences.append(occurrence)
                facts[subtype].sources.add(relative)
        for pattern in (INDIRECT_SUBTYPE_CALL, INDIRECT_SUBTYPE_VARIABLE):
            for match in pattern.finditer(text):
                line = line_at(text, match.start())
                if lines[line - 1].lstrip().startswith("//"):
                    continue
                indirect_subtypes.append(f"{relative}:{line}: `{match.group(0).strip()}`")
        for match in NON_LITERAL_REASON.finditer(text):
            if any(match.start() >= start and match.end() <= end for start, end in literal_spans):
                continue
            line = line_at(text, match.start())
            source_line = lines[line - 1].lstrip()
            if source_line.startswith("func ") or source_line.startswith("//") or "WithReason(string(subtype))" in source_line:
                continue
            dynamic.append(f"{relative}:{line}: `{match.group(0).strip()}`")
        for match in STRUCT_SUBTYPE.finditer(text):
            line = line_at(text, match.start())
            if not lines[line - 1].lstrip().startswith("//"):
                struct_subtypes[match.group(1)].append(f"{relative}:{line}")
        for match in STRUCT_SUBTYPE_CONST.finditer(text):
            line = line_at(text, match.start())
            if lines[line - 1].lstrip().startswith("//"):
                continue
            constant = match.group(1)
            subtype = subtype_constants.get(constant)
            if subtype is None:
                dynamic.append(f"{relative}:{line}: `{match.group(0).strip()}` (unknown ErrorInfo subtype constant)")
                continue
            struct_subtypes[subtype].append(f"{relative}:{line}")

    return facts, sorted(dynamic), sorted(indirect_subtypes), dict(sorted(struct_subtypes.items())), dict(sorted(subtype_constants.items()))


def observed(values: list[Occurrence], attr: str) -> str:
    return "yes" if any(getattr(value, attr) for value in values) else "no"


def retryability(values: list[Occurrence]) -> str:
    declared = {value.retryable for value in values if value.retryable != "not_declared"}
    if not declared:
        return "none"
    if len(declared) == 1:
        return next(iter(declared))
    return "mixed"


def categories(values: list[Occurrence]) -> str:
    return ", ".join(sorted({value.category for value in values}))


def examples(values: list[Occurrence]) -> str:
    locations = [f"{value.path}:{value.line}" for value in values[:3]]
    suffix = " …" if len(values) > 3 else ""
    return "<br>".join(f"`{location}`" for location in locations) + suffix


def status(values: list[Occurrence]) -> str:
    registered = sum(1 for value in values if value.registered)
    free = len(values) - registered
    if registered and free:
        return f"registered {registered} / free {free}"
    if registered:
        return f"registered {registered}"
    return f"free {free}"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True, help="Markdown evidence path")
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[2]
    facts, dynamic, indirect_subtypes, struct_subtypes, subtype_constants = scan(root)
    all_occurrences = sum(len(item.occurrences) for item in facts.values())
    registered_occurrences = sum(sum(1 for value in item.occurrences if value.registered) for item in facts.values())
    free_occurrences = all_occurrences - registered_occurrences
    no_hint = sum(1 for item in facts.values() if not any(value.hint for value in item.occurrences))
    unresolved = sum(1 for item in facts.values() if "unresolved" in {value.category for value in item.occurrences})
    registered_subtype_values = set(subtype_constants.values())
    registered_struct_subtypes = sorted(set(struct_subtypes) & registered_subtype_values)
    unregistered_struct_subtypes = sorted(set(struct_subtypes) - registered_subtype_values)

    if free_occurrences:
        registry_status = "已出现受治理的 subtype registry，但未注册的 `WithReason(string)` 仍是自由字符串。这份扫描的用途是展示迁移进度，**不**把“出现过”误写成“已经 wire-stable”。"
    elif dynamic:
        registry_status = "所有字面 `WithReason(\"…\")` 已映射到受治理 registry；仍保留的动态 reason 必须继续由 Agent 逐项审阅，不能被计数清零误写成已经 wire-stable。"
    else:
        registry_status = "所有字面和变量 `WithReason` 调用均已迁入受治理 registry 或兼容桥；间接 subtype 仍须由 Agent 审阅其有限映射与真实服务端终态，不能据此宣称写入已经验证。"
    lines = [
        "# DWS 错误契约 Agent 扫描",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 本报告由 Agent 读取当前 Go 源码生成，不是 CI 门禁；只保存 Markdown 证据，不生成或保存运行时 JSON fixture。",
        "",
        "## 当前事实",
        "",
        f"- 已注册 descriptor：**{len(subtype_constants)}** 个；直接 `WithSubtype(...)` / 兼容桥 `WithStableSubtypeAndLegacyReason(...)` 调用点：**{registered_occurrences}** 个；间接映射调用点：**{len(indirect_subtypes)}** 个。",
        f"- `WithReason(\"…\")` 的自由字面调用点：**{free_occurrences}** 个；与已注册调用合计覆盖 **{len(facts)}** 个 subtype、**{all_occurrences}** 个调用点。",
        f"- 直接构造 `ErrorInfo.Subtype`：**{len(struct_subtypes)}** 个不同值，其中已登记 **{len(registered_struct_subtypes)}** 个、未登记 **{len(unregistered_struct_subtypes)}** 个。",
        f"- 动态 `WithReason(variable)` 调用：**{len(dynamic)}** 个。",
        f"- 至少一个调用点既没有邻近 `WithHint`、也没有 registry `DefaultHint` 的 subtype：**{no_hint}** 个。",
        f"- 无法从同一局部构造窗口解析 Category 的 subtype：**{unresolved}** 个。",
        "",
        registry_status,
        "",
        "## 源码 subtype 清单",
        "",
        "| subtype | 治理状态 | 调用点 | 推断 Category | 有效 hint | actions | retryable 值 | retry-after | execution-started | 例子 |",
        "|---|---|---:|---|:---:|:---:|:---:|:---:|:---:|---|",
    ]
    for reason, item in sorted(facts.items()):
        values = item.occurrences
        lines.append(
            f"| `{reason}` | {status(values)} | {len(values)} | `{categories(values)}` | {observed(values, 'hint')} | "
            f"{observed(values, 'actions')} | {retryability(values)} | "
            f"{observed(values, 'retry_after')} | {observed(values, 'execution_started')} | {examples(values)} |"
        )

    lines += [
        "",
        "## 直接 ErrorInfo subtype",
        "",
        "这类值绕过 `WithReason`；迁移到注册表时必须一并纳入，不能只扫描错误构造函数。",
        "",
        "| subtype | registry | 位置 |",
        "|---|---|---|",
    ]
    if struct_subtypes:
        for subtype, locations in struct_subtypes.items():
            registry = "registered" if subtype in registered_subtype_values else "unregistered"
            lines.append(f"| `{subtype}` | {registry} | {', '.join(f'`{location}`' for location in locations[:5])}{' …' if len(locations) > 5 else ''} |")
    else:
        lines.append("| — | — | — |")

    lines += [
        "",
        "## 动态 subtype 构造",
        "",
        "动态 `WithReason` 在注册表启用前必须人工审阅：要么映射到声明的稳定 subtype，要么按当前 Category 归入 `upstream_unclassified` / `discovery_upstream_unclassified` 并保留上游码/trace，不能把上游任意文本直接变成 Agent 分支键。",
        "",
    ]
    if dynamic:
        lines.extend(f"- {item}" for item in dynamic)
    else:
        lines.append("- 无")

    lines += [
        "",
        "## 间接稳定 subtype 映射",
        "",
        "这类调用使用有限映射函数而非字面量；Agent 必须阅读映射函数和对应测试，确认它没有把上游文本或任意数值重新拼进 subtype。",
        "",
    ]
    if indirect_subtypes:
        lines.extend(f"- {item}" for item in indirect_subtypes)
    else:
        lines.append("- 无")

    lines += [
        "",
        "## Agent 审阅结论",
        "",
        "1. 当前 `Category`/退出码、`hint/actions/retryable/retry_after_seconds` 已是可复用底座；表中的 retryable 值区分显式 `true` 与 `false`，不把“声明过字段”误读为“允许重试”。",
    ]
    if free_occurrences or dynamic:
        lines += [
            "2. registry 已覆盖本地校验、目标预检、下载完整性与 transport/服务端响应等高价值路径；剩余自由或动态 `WithReason` 必须继续迁入有限 subtype，不能把上游文本变成 Agent 分支键。",
            "3. 下一步应逐命令处理这些自由调用或有限映射；不应一次性重命名现有 wire 字段或类别。",
        ]
    else:
        lines += [
            "2. 当前源码的字面与变量 `WithReason` 均已归入 registry 或兼容桥；下一步是审阅有限间接映射与实际恢复行为，而不是重复开展 reason 清零迁移。",
            "3. 对写请求，`retryable:false` 或省略仍不等于“绝无副作用”；应以 execution state、幂等键和真实账号验证决定恢复流程。",
        ]
    lines += [
        "4. 扫描只证明源码出现与邻近选项，不能证明服务端终态或 recovery action 在真实账号上可执行。",
    ]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
