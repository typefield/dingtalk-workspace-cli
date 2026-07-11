---
name: dingtalk-skill
description: 悟空/DWS 技能市场管理（搜索、安装、发布、企业库上传）。Use when 用户说搜索技能/找技能/安装技能/发布技能/上传技能到企业库/技能市场/企业技能库。注意：这是元 skill，只管理 dws 平台上的技能资源。Distinct from dws-shared(钉钉产品路由入口)、其他 dingtalk-* 产品 skill(执行具体业务能力)、本地 Codex skill 开发。命令前缀：dws skill。
cli_version: ">=0.2.14"
metadata:
  category: product
  requires:
    bins:
      - dws
---

# 悟空技能管理 Skill

## 前置条件 — 执行操作前必读

> **`use_skill(dws-shared)`** — 认证、全局参数（`--format json` / `--yes`）、错误码、URL 模板、跨产品消歧、安全规则与 capability 边界。**执行任何 `dws` 命令前先读；** 单产品的清晰命令可直接用本 skill。

<!-- SAFETY_PREAMBLE_INJECT -->

> 命令参考：[skill.md](references/skill.md)。

## 意图表

| 用户说 | 命令 |
|--------|------|
| "搜索技能 / 找技能" | `dws skill search --query "<关键词>" [--source DingtalkMarket\|OrgInternal]` |
| "安装技能" | `dws skill install --skill-id <id> [--force]` |
| "发布技能 / 上传技能到企业库" | `dws skill publish <path> --name <skillName> --version <semver> [--changelog "..."]` |

## 安全检测

- `securityStatus=failed` 的技能默认拒绝安装；只有明确 `--force` 才能强装
- 发布后进入安全检测流程

## 环境

- `DWS_SKILL_API_HOST` 覆盖技能 API 地址（默认 `https://aihub.dingtalk.com`）

## 兼容提示

- `dws skill find` → 用 `dws skill search --query <关键词>`
- `dws skill add` → 用 `dws skill install --skill-id <id>`
- `dws skill upload` → 用 `dws skill publish <path>`
