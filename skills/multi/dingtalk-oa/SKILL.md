---
name: dingtalk-oa
description: 钉钉 OA 审批。Use when 用户说 OA/审批/待处理审批/同意审批/拒绝审批/撤销审批/已发起审批/审批记录/批量审批。不做待办任务（走 dingtalk-todo）、日志（走 dingtalk-report）。命令前缀：dws oa。
cli_version: ">=0.2.14"
metadata:
  category: product
  requires:
    bins:
      - dws
---

# 钉钉 OA 审批 Skill

## 前置条件 — 执行操作前必读

> **`use_skill(dws-shared)`** — 认证、全局参数（`--format json` / `--yes`）、错误码、URL 模板、跨产品消歧、安全规则与 capability 边界。**执行任何 `dws` 命令前先读；** 单产品的清晰命令可直接用本 skill。

<!-- SAFETY_PREAMBLE_INJECT -->

> 命令参考：[oa.md](references/oa.md)。

## 意图表

| 用户说 | 命令 |
|--------|------|
| "待我处理的审批 / 7 天内待审" | `python scripts/oa_pending_review.py --days 7` |
| "查审批详情" | `dws oa approval detail --instance-id <processInstanceId> --format json` |
| "同意 / 拒绝审批" | 先 `dws oa approval tasks --instance-id <id> --format json` 取 `taskId`，再 `dws oa approval approve --instance-id <id> --task-id <taskId> --format json` / `reject --instance-id <id> --task-id <taskId> --format json`（需用户确认） |
| "批量同意 / 批量拒绝" | `python scripts/oa_batch_approve.py --action approve --days 7` |
| "撤销审批" | `dws oa approval revoke --instance-id <id> --format json` |
| "我已发起的审批" | `dws oa approval list-submitted --format json` |

## 危险操作

`approval approve / reject` 不可撤回，必须先向用户展示摘要并获得明确同意，再执行审批命令。

## 跨产品协作

- 催别人审批 → 在群里 @对方（`dingtalk-chat`），不要走 #1 消息剧本里的 escalate-ding
- 审批通过后建待办 → 切到 `dingtalk-todo`
