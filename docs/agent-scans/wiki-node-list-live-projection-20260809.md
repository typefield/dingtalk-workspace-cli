# Wiki node-list 统一结果 Agent 实测

扫描日期：2026-08-09

> 本探针先只读发现一个可见知识库，再只读其根目录；只记录计数和结果契约，不保存 workspaceId、nodeId、标题或运行 JSON，也不接入 CI。

## 结果

**PASS**

## 可观测事实

- 返回统一 `ok:true` / `outcome:success`，且 `data`/`meta` 均为对象。
- 根目录节点数与 `meta.count` 一致：46；所有返回节点均有非空 string nodeId。
- 分页事实自洽：`hasMore:false`，`endpoint_exhausted:true`。

## 边界

本次仅验证一个可见知识库的正常根目录响应。仍需真实复验空目录、续页、嵌套分页和服务端异常形状；不能把这一页扩大为所有知识库目录均已验证。
