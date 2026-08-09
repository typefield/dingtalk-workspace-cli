# AITable Base discovery Agent 实测

扫描日期：2026-08-09

> 本探针只读取当前用户最近访问 Base 的一页；不保存 Base 名称、ID 或原始 JSON，不接入 CI。

## 结果

**PASS**

## 可观测事实

- 当前命令为 legacy JSON；本扫描不把它误报为 unified active。
- 投影 Base 数与 `count` 一致：1。
- 所有投影 Base 均有稳定 `baseId`：true。
- 非权威目录边界：`authoritativeInventory:false`、`inventoryCoverageKnown:false`。
- 分页事实已知性为 False，已知时的 hasMore/continuation 自洽：true。
- stderr 为空：true。

## 边界

本次只验证一个真实正常单页，不证明最近访问列表等于所有可访问 Base，也不验证死条目、检索召回或服务端索引健康。分页矛盾/缺 continuation 的 fail-closed 行为由专项单元回归覆盖。
