---
name: recipe-generate-standup-report
description: "查询指定日期的日程和未完成待办，生成站会或每日工作摘要。"
metadata:
  category: "recipe"
  domain: "productivity"
  requires:
    bins:
      - dws
    skills:
      - dws-shared
      - dingtalk-calendar
      - dingtalk-todo
---

# 生成站会摘要

查询指定日期的日程和未完成待办，按时间顺序生成站会或每日工作摘要。

> **PREREQUISITE:** 先读取 `dws-shared`，再读取 `dingtalk-calendar` 和 `dingtalk-todo`。

## Steps

1. 查询目标日期的日程：`dws calendar +agenda --start "<YYYY-MM-DDT00:00:00+08:00>" --end "<YYYY-MM-DDT23:59:59+08:00>" --limit 100 --format json`
2. 查询未完成待办：`dws todo +get-my-tasks --status false --size 20 --format json`
