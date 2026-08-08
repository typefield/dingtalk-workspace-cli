# skill — DWS 技能管理

> 这是元能力：只管理 dws 平台上的技能资源。Distinct from `dingtalk-shared`（钉钉产品路由入口）、其他 `dingtalk-*` 产品 skill（执行具体业务能力）、本地 Codex skill 开发。命令前缀：`dws skill`。

## 意图表

| 用户说 | 命令 |
|---|---|
| "搜索技能 / 找技能" | `dws skill search --query "<关键词>" [--source DingtalkMarket\|OrgInternal]` |
| "下载技能包" | `dws skill get --skill-id <skillId> --format json` |
| "安装市场技能" | 先 `dws skill install <skillId> <target> --dry-run --format json`；用户确认后加 `--yes` |
| "安装 DWS mono/multi skills" | 先 `dws skill setup --mode <mono\|multi> --target <target> --dry-run --format json`；用户确认删除/覆盖计划后加 `--yes` |

## 约束

- `skillId` 必须来自 `skill search` 返回，不能用名称代替。
- `skill install` 的 `skillId` 与 `target` 是位置参数，不是 `--skill-id` flag。
- `skill setup --mode multi` 可用 `--skill/-s` 只装指定产品，或用 `--exclude/-x` 排除产品，两者不能同时使用。
- `skill setup` 会覆盖目标中的同名 Skill，并删除相反模式的残留（mono 会清理 `dingtalk-*`，multi 会清理 `dws/`），属于高风险本地写操作；必须先看 `--dry-run` 返回的 `removals[]/installs[]`。
- 搜索结果中的 `securityStatus`（服务端返回时）需要如实展示；缺失或状态异常时不要把安装描述为已通过安全检测。
- `skill install` 会把第三方内容写入 Agent 的技能目录，必须先执行无网络、无写入的 `--dry-run`，再取得用户确认并追加 `--yes`。
- 开源 CLI 不提供技能发布/上传命令；发布需求应转到对应的技能市场发布流程。

## 兼容提示

- `dws skill find` → `dws skill search --query <关键词>`
- `dws skill add` → `dws skill install <skillId> <target>`

---

## 命令参考

### 搜索技能

```
Usage:
  dws skill search [flags]
Example:
  dws skill search --query "周报" --format json
  dws skill search --query "日报" --source OrgInternal --format json
Flags:
      --query string    搜索关键词 (必填)
      --source string   查询范围：DingtalkMarket / OrgInternal；空格分隔
```

从返回中提取真实 `skillId`、名称、版本、来源与 `securityStatus`。兼容入口 `skill find` 只会提示改用 `search`。

### 下载技能包

```
Usage:
  dws skill get --skill-id <skillId> --format json
Flags:
      --skill-id string   技能 ID (必填)
```

成功后返回本地临时目录路径，供检查或后续安装使用。

### 安装市场技能

```
Usage:
  dws skill install <skillId> <target>
Example:
  dws skill install skill-123 claude --dry-run --format json
  dws skill install skill-123 claude --yes --format json
```

`skillId` 来自搜索结果；`target` 使用 `skill install --help` 列出的 Agent 名称，或用 `.` 安装到当前目录。两个值均为位置参数。`--dry-run` 只解析目标路径，不访问技能市场、不下载、不写文件。

### 部署 DWS 内置技能

```
Usage:
  dws skill setup [flags]
Example:
  dws skill setup --mode mono --target codex --dry-run --format json
  dws skill setup --mode mono --target codex --yes --format json
  dws skill setup --mode multi --target qoder --dry-run --format json
  dws skill setup --mode multi -s aitable -s calendar --target qoder --yes --format json
  dws skill setup --mode multi -x live -x devdoc --target qoder --yes --format json
Flags:
      --mode string       mono | multi
      --target string     目标 Agent，默认 all
      --source string     显式 skill 源目录
  -s, --skill strings     multi 模式只安装指定子 skill
  -x, --exclude strings   multi 模式排除指定子 skill
      --yes               跳过确认
```

`--skill` 与 `--exclude` 互斥。未指定 `--source` 时使用当前二进制内置的 skill 版本。
`--dry-run` 直接读取内嵌或显式本地源，不展开临时目录、不删除、不创建、不复制。
正式执行的 `outcome` 可能是 `success`、`partial_failure` 或 `failure`：

- `partial_failure.data.succeeded[]` 是已完成的删除/安装，禁止重做整批；
- `failed[]` 是明确未执行成功的路径；
- `unknown[]` 表示复制或删除中断后终态不确定，必须先检查对应路径再重试。
- `skill setup --target .` 不受支持；当前目录的单 Skill 安装只使用 `skill install <skillId> .`。

## 上下文传递

| 操作 | 从返回中提取 | 用于 |
|---|---|---|
| `skill search` | `skillId`、版本、来源、安全状态 | 下载或安装 |
| `skill get` | 临时目录 | 本地检查 |
| `skill install` | 安装目标与结果 | 确认指定 Agent 已安装 |
| `skill setup` | dry-run 的 `removals[]/installs[]`；执行结果的 `operations[]` 或 partial 三通道 | 先确认计划；部分失败时只补偿未成功/不确定路径 |
