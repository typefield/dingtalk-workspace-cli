---
name: dingtalk-drive
description: 钉钉文件管理（存储层）。Use when 用户说 钉盘/上传文件/下载文件/文件夹/查文件/找文件/全局搜索文件/复制/移动/重命名/删除/回收站/还原删除文件/权限管理/普通文件下载。任何文件类型都适用；文档内容编辑走 dingtalk-doc，知识库空间和空间内节点管理走 dingtalk-wiki。命令前缀：dws drive。
cli_version: ">=0.2.14"
metadata:
  category: product
  requires:
    bins:
      - dws
---

# 钉盘 Skill

## 前置条件 — 执行操作前必读

> **`use_skill(dws-shared)`** — 认证、全局参数（`--format json` / `--yes`）、错误码、URL 模板、跨产品消歧、安全规则与 capability 边界。**执行任何 `dws` 命令前先读；** 单产品的清晰命令可直接用本 skill。

<!-- SAFETY_PREAMBLE_INJECT -->

> 命令参考：[drive.md](references/drive.md)。

## 意图表

| 用户说 | 命令 |
|--------|------|
| "看钉盘文件 / 文件夹列表" | `dws drive list [--folder <dentryUuid>]` |
| "钉盘目录树" | `python scripts/drive_tree_list.py --depth 2` |
| "查文件元数据" | `dws drive info --node <dentryUuid>` |
| "搜文件 / 找文件" | `dws drive search --query "<关键词>"` |
| "下载文件" | `dws drive download --node <dentryUuid> --output <path>` |
| "上传文件" | `dws drive upload --file <path> [--folder <id>]` |
| "建文件夹" | `dws drive mkdir --name "<名称>" [--folder <id>]` |
| "复制/移动/重命名/删除/权限管理" | `dws drive copy/move/rename/delete/permission ...` |
| "回收站 / 还原删除的文件" | `dws drive recycle list` / `dws drive recycle restore --id <recycleItemId>` |

## 标准 SOP（必遵流程）

> 命中以下意图**必须**按对应 SOP 顺序执行；**禁止**跳步、替换命令、编造 dentryUuid/nodeId。每条命令必须带 `--format json`。破坏性操作（删除/移动/覆盖/公开）**必须**先与用户确认。

### SOP-1 找文件（find-file）

**触发**：找文件/搜文件/我的文件/最近文件/某文档在哪。

1. **选源（必须）**：最近访问 → `dws drive recent --limit <n> --format json`（翻页用上次返回的 `nextCursor` 传 `--cursor`）；按内容/名称全局搜 → `dws drive search --query "<关键词>" --format json`；浏览某目录 → `dws drive list --folder <dentryUuid> --format json`。
2. **解析（必须）**：取真实 `dentryUuid`（= `id`/`nodeId`）；多候选让用户确认，**禁止**默认取第一个。
3. **下钻（必须）**：根目录没命中时，进入最相关文件夹继续 `drive list --folder`，必要时 `python scripts/drive_tree_list.py --depth 2` 递归，**禁止**只看根目录就放弃。
4. **回读元数据（必须）**：命中后 `dws drive info --node <dentryUuid> --format json` 确认类型（在线文档 vs 普通文件）。

**禁止**：编造 dentryUuid、只看根目录放弃、用 `drive list` 替代 `drive search` 做全局查找。

### SOP-2 上传 / 下载（upload-download）

**触发**：上传文件/下载文件/传到钉盘。

1. **上传（必须）**：`dws drive upload --file <本地路径> [--folder <dentryUuid>] --format json`；返回取 `dentryUuid`，`drive info --node` 回读确认。
2. **下载（必须）**：先 `drive info --node <dentryUuid>` 判断类型——在线文档（ALIDOC 等）切 `dingtalk-doc` 用 `doc export`；普通文件 `dws drive download --node <dentryUuid> --output <本地路径>`。

**禁止**：对在线文档用 `drive download`（会失败）、上传后不回读。

### SOP-3 文件夹 / 复制 / 移动 / 重命名（folder-ops）

**触发**：建文件夹/复制/移动/重命名。

1. **执行（必须）**：建文件夹 `dws drive mkdir --name "<名称>" [--folder <id>]`；复制 `drive copy --node <dentryUuid> --folder <目标>`；移动 `drive move --node <dentryUuid> --folder <目标>`；重命名 `drive rename --node <dentryUuid> --name "<新名>"`。全部 `--format json`。
2. **验证（必须）**：操作后 `drive info --node <新dentryUuid>` 或 `drive list --folder <目标>` 回读。

**禁止**：未确认就移动/覆盖他人文件、跳过回读。

### SOP-4 回收站（recycle）

**触发**：删文件/回收站/还原。

1. **删除（必须）**：`dws drive delete --node <dentryUuid> --format json`（**必须**先与用户确认）。
2. **还原（必须）**：`dws drive recycle list --format json` 取 `recycleItemId` → `dws drive recycle restore --id <recycleItemId> --format json`。

**禁止**：未确认就删除、把 `dentryUuid` 当 `recycleItemId` 传给 restore。

### SOP-5 互联网公开（publish）

**触发**：互联网公开/取消公开/查公开状态。

1. **执行（必须）**：查状态 `dws drive publish get --node <dentryUuid> --format json`；开启公开 `dws drive publish set --node <dentryUuid> --yes`（**[危险]** 必须用户确认）；关闭公开 `dws drive publish unset --node <dentryUuid> --yes`。
2. **边界（必须）**：对外公开前**必须**与用户确认边界与后果。

**禁止**：未确认就 `publish set`、跳过 `--yes`。

## 高频硬约束

- 查找文件不要只看根目录后放弃；根目录没命中时，进入最相关的目标文件夹继续 `drive list --folder <dentryUuid>`，必要时用目录树脚本递归到合理深度。
- `drive list` 默认 `--limit 20`，自动化场景里保守使用 `--limit 50` 以内并处理 `nextToken` 翻页；不要因为参数边界报错反复重试。
- 全局找文件优先 `drive search --query`；指定目录浏览用 `drive list`，命中后必须 `drive info --node <dentryUuid> --format json` 回读元数据。
- 删除、覆盖、移动等破坏性操作必须确认；上传、创建文件夹、下载后要读回或列目录验证。
- 所有 `dws drive` 命令加 `--format json`。

## 跨产品协作

- 文件内容编辑（钉钉文档）→ 切到 `dingtalk-doc`
- 知识库空间 → 切到 `dingtalk-wiki`
## 局部意图与 Recipe

- [局部意图消歧](references/intent-guide.md)；[Lite Recipe](references/lite-recipes.md)。
