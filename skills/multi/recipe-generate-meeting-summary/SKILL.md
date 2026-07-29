---
name: recipe-generate-meeting-summary
description: "查询时间范围内的听记，汇总会议摘要与待办，并按需生成钉钉文档。"
metadata:
  category: "recipe"
  domain: "meetings"
  requires:
    bins:
      - dws
    skills:
      - dws-shared
      - dingtalk-minutes
      - dingtalk-doc
---

# 生成会议纪要汇总

查询时间范围内的听记，汇总每场会议的摘要与待办，并在用户明确要求时生成钉钉文档。

> **PREREQUISITE:** 先读取 `dws-shared`，再读取 `dingtalk-minutes`；仅在用户要求生成文档时读取 `dingtalk-doc`。

## Steps

1. 查询时间范围内可访问的听记：`dws minutes list all --start "<YYYY-MM-DDT00:00:00+08:00>" --end "<YYYY-MM-DDT23:59:59+08:00>" --limit 30 --format json`
2. 对每个 `taskUuid` 获取基础信息、摘要和待办：`dws minutes +detail --id "<TASK_UUID>" --artifacts basic,summary,todos --format json`
3. 仅在用户明确要求生成文档时写入汇总结果：`dws doc create --name "会议纪要汇总 <START_DATE> - <END_DATE>" --content "<MARKDOWN_CONTENT>" --format json`
