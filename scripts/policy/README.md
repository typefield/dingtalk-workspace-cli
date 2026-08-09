# scripts/policy —— 统一返回的本地审计辅助工具（Phase I）

本 README 覆盖 Phase I 的三个本地检查器与共享临时样本库。它们只由
`scripts/agent/scan_unified_result_surface.py` 或人工 Agent 审阅调用，不能作为
CI / `make policy` 门禁或替代真实环境验证。

- `check-stdout-json.sh`（B161）—— json 模式 stdout 纯 JSON 检查（AC-11）
- `check-string-bool.sh`（B162）—— 字符串布尔违规扫描（AC-02）
- `check-envelope-keys.sh`（B163）—— 非标准信封键名扫描（信封顶层键集合固化，G1）
- `unified-result-lib.sh` —— 三脚本共享库（样本表、隔离 HOME、重试纪律、self-test 框架）；这是内部文件名，不是 CLI 协议或用户接口。
- `unified-result-lib.sh` 在临时目录按需生成 self-test 输入（合法统一返回 + 各类违规样例），仓库不保存派生 JSON fixture。`contract_version` 仅以“必须被拒绝”的临时负向样本存在，绝不属于 CLI 输出或仓库结果。

## 定位：Agent 审计辅助，不是门禁

这三个脚本检查代表性终端命令的运行时 stdout。它们只能提供某一轮 Agent
审阅的证据，不能证明全表面正确，更不能替代服务端终态或 dry-run 零写验证。

- **永不接入 `make policy` 或 CI**。测试可以覆盖代码分支，但“命令是否如实表达业务结果”必须由 Agent 按当前代码、当前二进制和必要的真实账号环境审阅。
- 脚本自带 `--self-test`，用运行时生成的临时输入验证扫描逻辑本身（合法样例必须 pass、违规样例必须 fail）。自检不是发布准入，也不产出仓库 JSON fixture。
- 返回锚点：AC-02（ok/success 类布尔恒为 JSON 布尔）、AC-11（json 模式 stdout 零日志字节、primary result 恰为一个可解析 JSON 文档）、顶层键集合固定（`ok/outcome/identity/dry_run/data/meta/error/_notice` snake_case；`contract_version/errcode/error_code/errorCode/success` 等非标准顶层形态违规，camelCase wire 键形态违规）。没有输出协议版本字段。

## Agent 审阅用法

```bash
python3 scripts/agent/scan_unified_result_surface.py \
  --output docs/agent-scans/unified-result-surface-YYYYMMDD.md
```

包装器在临时目录构建本轮二进制、运行三个检查器及其 self-test，并且只保存 Markdown
证据。若 Agent 需要定位某个样本，可单独运行下列底层命令；它们的退出码只是审阅输入，
不是 CI 判定：`0` = 无发现，`1` = 发现违规，`2` = 环境/用法无法审阅。

```bash
./scripts/policy/check-stdout-json.sh --scope dev
./scripts/policy/check-string-bool.sh --scope dev
./scripts/policy/check-envelope-keys.sh --self-test
```

## 样本与 --scope（B166）

脚本对**代表性命令的真实 stdout** 做扫描，样本表定义在 `unified-result-lib.sh`。
的 `unified_result_samples()`，按 scope 分两档：

| scope | 样本 | 说明 |
|---|---|---|
| `dev`（默认） | `dev connect list`、`dev connect status`、`dev connect stop --dry-run` | 全部是**离线确定性**统一结果 terminal 样本：隔离新建 HOME + `DWS_DISABLE_KEYCHAIN=1`，无需登录、无网络、无副作用。`dev connect` 根同时承载 stream 与 daemon start，当前整体 legacy，不纳入统一结果信封样本。 |
| `all` | dev 档 + `dev app list` + `devapp +list` + `schema list` + `auth status` + `version` | 两个 devapp 入口需要登录态（继承真实 HOME），并覆盖原子命令与 shortcut adapter；后三个是**未迁移的非信封 json 输出**（legacy class），只做可解析性与字符串布尔检查，**不做信封形状检查**（那是迁移前已知形态，不是回归） |

样本分两类（`unified_result_samples()` 首列）：

- `envelope` —— 已迁移统一信封的输出，三个脚本的全部规则适用；
- `legacy` —— 迁移前的非信封 json（如 `auth status` 顶层 `success` 键），
  `check-envelope-keys.sh` 对其**豁免信封形状检查**，只保留可解析性与
  字符串布尔检查；`dev` scope 下不含 legacy 样本，因此默认档对信封形状零豁免。

扫描纪律（硬纪律 3 落点）：

- 每个样本**失败自动重试一次**再下结论（并行在途改动可能造成偶发异常）。
  非零退出但 stdout 非空时仍扫描该文档（统一结果 `failure` / `partial_failure` 本来就
  使用非零退出码）；仅非零退出且 stdout 为空时视为环境/鉴权不可用，记
  `[skip]` 并打印 stderr 尾部。
- **至少一个样本成功验证**，否则脚本拒绝通过（防止空跑假绿）。
- `DWS_SCAN_HOME` 环境变量可覆盖样本 HOME（仅供本地 Agent 审阅定制环境）。

## B164 误报核查记录

核查方式：`make build` 后对真实二进制输出跑三个脚本，两档 scope 分别执行：

1. **`--scope dev`（默认档）**：当前扫描 3 个离线统一结果 terminal 样本。样本真实输出形态核对：`dev connect list` 输出
   `{ok,outcome,data:[],meta.count:0}` 信封；`status`/`stop` 对一次性
   probe id 输出 `{ok:true,outcome:success,...}`；stop 使用 dry-run 预览，保证策略扫描不发进程信号。
2. **`--scope all`**：覆盖 9 个样本，包含 `dev app list` 原子入口与
   `devapp +list` shortcut 入口。三个脚本均对非零统一结果继续扫描 stdout。
   legacy 样本（`schema list`/`auth status`/`version`）顶层含 `success` 等
   非信封键，被 legacy class 正确豁免信封形状检查；`dev app list` 信封
   （含 `meta.pagination.endpoint_exhausted:true`）全规则通过。
3. **检出能力反向验证**：用一个故意违规的 stub 二进制（`DWS_BIN` 覆盖）
   验证三脚本确实能抓到违规——stdout 混入 `[INFO]` 行且不可解析、
   `"ok":"true"` 字符串布尔、`errorCode` 顶层历史键、`meta.timedOut`
   camelCase 形态均被对应脚本以非零退出码报出；`--self-test` 模式下
   全部违规 fixture（`string_bool_violation.json`、`envelope_legacy_keys.json`、
   `envelope_camel_keys.json`、`legacy_envelope_keys.json`、`stdout_*` 系列）
   判 fail、全部合法 fixture 判 pass。

核查结论：**三脚本对现有合法输出零误报，对契约违规形态可检出**。
已知边界（原型立场，见下「设计取舍」）：`data` 载荷内的 camelCase /
legacy 命名业务字段（如服务端返回的 `hasMore`/`success` 业务字段）不视为
信封违规——信封契约只约束信封结构层（顶层键 + meta/error 子树）。

## 设计取舍（供 DESIGN 评审）

1. **扫描面是运行时 stdout 而非源码静态 grep**：三个脚本都消费真实二进制的
   json 模式输出（B161/B162/B163 的批次描述均以「命令输出样例」为扫描对象）。
   源码静态扫描（如 Go 文件里的 json tag 审计）是另一个维度，与 B158~B160
   的 Go 侧 fixture 扫描互补，不在本原型范围。
2. **legacy class 豁免**：`--scope all` 纳入的 `schema list`/`auth status`/
   `version` 是未迁移输出，豁免信封形状检查而非删除样本——保留其可解析性与
   字符串布尔检查，等 Phase F/E 迁移完成后这些样本升级为 envelope class，
   豁免自然消失。
3. **失败重试一次 + 有限 skip 机制**：对齐硬纪律 3；非零且有 stdout 的统一结果
   结果必须扫描，只有无 stdout 的环境/鉴权失败可 skip。skip 计数显式打印在
   结果行，全 skip 时拒绝通过。

## 与自动化测试的边界

- `check-schema-catalog.sh`、`check-cli-smoke.sh` 等自动化测试仍可验证命令树和
  装配约束；它们不替代本工具的运行时结果审阅。
- 这三个检查器及其 self-test 永不升级为 policy / CI gate。样本覆盖、真实账号、
  服务端响应形状和“是否可如实宣称终态”由 Agent 在评测或发布前取证。
- shell 脚本保持 POSIX `sh`；本地 Agent 审阅应先跑 `sh -n`，再运行包装器。
