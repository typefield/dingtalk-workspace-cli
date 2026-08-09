# Todo 聚合统一结果真实只读 Agent 复验

扫描时间：2026-08-09T19:45:01+08:00

> 本扫描使用当前源码临时构建二进制，只执行 Todo 只读命令。原始响应只在内存解析，仓库仅保存脱敏 Markdown；不保存 JSON fixture。

| 命令 | 结果 | rc | 投影条数 | 证据 |
|---|---|---:|---:|---|
| `dws todo +due-today --format json` | PASS | 0 | 0 | 统一 success；count 对齐；稳定 taskId；pagination_known=false，且未伪造 endpoint 耗尽 |
| `dws todo +related-tasks --format json` | PASS | 0 | 62 | 统一 success；count 对齐；稳定 taskId；pagination_known=false，且未伪造 endpoint 耗尽 |

结果：**2/2 PASS**。

## 边界

- 本证据证明当前账号下两个读取入口能投影真实响应并直接产生统一结果；不证明 Todo 服务端短页等于权威终页。
- 因上游没有可信 continuation，结果必须保留 `pagination_known:false`，且不得输出 `meta.pagination.endpoint_exhausted:true`。
- 本扫描不创建、修改、完成或删除待办，也不证明其他账号、组织权限或后端覆盖率。
