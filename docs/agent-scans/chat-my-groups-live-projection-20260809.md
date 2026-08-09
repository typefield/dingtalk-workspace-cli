# Chat my-groups 投影 Agent 实测

扫描日期：2026-08-09

> 本探针只读取当前用户已加入群的一页；群名、会话 ID 与原始 JSON 都不落盘。它是 Agent 审阅证据，不接入 CI。

## 结果

**PASS**

## 可观测事实

- 当前命令仍为 legacy JSON（无 `ok/outcome` 信封）；这次扫描不把它误报为 unified active。
- 投影群数与 `count` 一致：1。
- 所有投影群均有稳定 `conversationId`：true。
- 单页续页事实可判定：`hasMore` 为 bool，continuation 自洽：true。
- stderr 为空：true。

## 边界

本次仅验证一个真实正常单页；未验证空列表、第二页、游标冲突或网关异常形状。未知容器、非法行或缺 stable conversationId 的 fail-closed 行为由专项单元回归覆盖。
