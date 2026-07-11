---
name: dingtalk-live
description: 钉钉直播列表与直播记录查询。Use when 用户说直播/我的直播/直播列表/查直播/直播回放/直播记录。不做视频会议/会议控制（走 dingtalk-conference）、AI 听记/转写摘要（走 dingtalk-minutes）、群消息（走 dingtalk-chat）。命令前缀：dws live。
cli_version: ">=0.2.14"
metadata:
  category: product
  requires:
    bins:
      - dws
---

# 钉钉直播 Skill

## 前置条件 — 执行操作前必读

> **`use_skill(dws-shared)`** — 认证、全局参数（`--format json` / `--yes`）、错误码、URL 模板、跨产品消歧、安全规则与 capability 边界。**执行任何 `dws` 命令前先读；** 单产品的清晰命令可直接用本 skill。

<!-- SAFETY_PREAMBLE_INJECT -->

> 命令参考：[live.md](references/live.md)。

## 意图表

| 用户说 | 命令 |
|--------|------|
| "我的直播 / 直播列表" | `dws live stream list` |
