# 前置条件 — 执行操作前必读

## 前置条件 — 执行操作前必读

**CRITICAL — 执行对应操作前，MUST 先用 Read 工具读取以下子文件：**

1. **解析 URL / 定位文档**（几乎所有命令都需要先拿 nodeId）
   → 必读 [`doc/doc-info.md`](./doc-info.md)（URL/dentryKey 提取规则、ID 边界、contentType 路由、**获取 nodeId 三种方式 A/B/C**）

2. **创建或编辑文档内容**（`doc create` / `doc update` / `doc block insert|update`）
   → 必读 [`doc/style/doc-update-workflow.md`](./style/doc-update-workflow.md)（**形态优先级硬规则：JSONML > element JSON > markdown**；markdown overwrite 会丢富结构）
   - 从零创建时加读 [`doc/style/doc-create-workflow.md`](./style/doc-create-workflow.md)
   - **任何 `doc create` 都必须先读 [`doc/style/doc-style-guideline.md`](./style/doc-style-guideline.md) §2.0 类型决策表 + §1 硬规则**（决定骨架 + 全局约束，不读就不知道用哪种骨架）
   - 涉及 callout / 分栏 / 富 block 精修时再加读 style-guideline §4-§7 + [`doc/format/doc-jsonml-cookbook.md`](./format/doc-jsonml-cookbook.md)

**未读以上文件就改写已有文档会导致富结构丢失、参数错误或样式不达标。其他命令（阅读 / 评论 / 权限 / 附件 / 下载导出 / 文件操作）按需查下方 §命令索引表跳转对应子文件加载，不必提前加载。**
