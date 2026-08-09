# Skill 已迁移 Doc 文件管理路由 Agent 审阅

扫描日期：2026-08-09

> 本扫描检查 Agent 文档是否把旧 `dws doc` 文件管理入口当作正向路径。它是 Markdown 评测证据，不接入 CI，也不保存 JSON fixture。

## 结论

**PASS**：Mono/Multi Skill 均未发现未标注为迁移/兼容的 `doc upload/download/copy/move/rename/delete/list/search` 正向路由。

默认路由：文件发现、传输和节点管理使用 `dws drive`；文档正文读取、创建、块编辑、导出和媒体嵌入保留 `dws doc`。
