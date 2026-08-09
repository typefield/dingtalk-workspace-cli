# OA list-forms 统一分页投影 Agent 实测

扫描日期：2026-08-09

> 本探针只读调用 `oa +list-forms`；只记录结构、计数与分页事实，不保存模板、processCode 或运行 JSON，也不接入 CI。

## 结果

**PASS**

## 可观测事实

- 统一信封为 `ok:true`、`outcome:success`，且 `data`/`meta` 均为对象。
- 已投影表单数与 `meta.count` 一致：93。
- 上游未给可信总数或续页事实时，输出 `data.pagination_known:false`，且没有伪造 `meta.pagination`。

## 边界

本次只证明当前身份的一条正常只读响应。仍需真实复验空结果、续页、响应形状漂移和权限失败；不能把当前页或空结果扩大为完整审批目录。
