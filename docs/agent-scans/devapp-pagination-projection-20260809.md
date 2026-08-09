# DevApp native / shortcut shared pagination projection — Agent review

扫描时间：2026-08-09T15:19:09+08:00

> 这是 Agent 的语义审阅取证，不是 CI / policy gate。扫描只调用内存中的 Go 测试，并输出 Markdown；不会保存任何上游响应或 JSON fixture。

## Result: PASS

- 已审阅分页能力：**4/4，两个既有命令前缀共用一个 projector**
- 覆盖入口：`dev app list/permission list/event list/version list` 与对应
  `devapp +list/+permission-list/+event-list/+version-list`
- 焦点测试：`TestDevAppSharedListProjection*`、
  `TestDevAppListResultSpecMatchesSharedProjection`、
  `TestDevRepresentativeResultContractsReachContractFinal`、
  `TestDevAppPaginatedShortcutContractsMatchActiveData`、
  `TestDevAppNativeAndShortcutListToolsShareProjectedResult`、
  `TestDevAppPaginatedShortcutsEmitUnifiedResumableResults` 和
  `TestDevAppListPaginationProjectsMeta`
- 测试退出码：`0`

## Required behavior

1. 保留顶层及嵌套 `result/data/content/pageInfo/pagination` 中的有效 `hasMore` / `nextCursor`。
2. `hasMore` 非布尔值必须返回 `validation/pagination_invalid`，不能投影成空分页。
3. `nextCursor` 非字符串、`hasMore=true` 无游标必须返回 `validation/pagination_incomplete`。
4. 多层 `hasMore` 或非空 `nextCursor` 互相冲突必须返回 `validation/pagination_conflict`；
   DevApp 实际终页会保留一个位置 cursor，`hasMore=false` 为权威终态，该 cursor 不得投影为 `next_token`。
5. 上述失败均为 `response_projection`、不可安全重试；不得输出成功列表。
6. 只有显式的空数组才可投影为成功空列表；缺容器、非数组、非法行或仅展示字段的行必须为 `api/projection_unknown`。
7. 经本项 Agent 审阅的八条 terminal command 在各自既有路径直接输出统一结果；不改
   `dev` / `devapp` 前缀，调用者只传 `--format json`。
8. 非末页必须在 `meta.pagination` 中保留 `endpoint_exhausted:false` 与 `next_token`，且不输出任何协议版本标记。
9. 末页必须输出 `endpoint_exhausted:true`，即使原始响应携带位置 cursor，也不能诱导 Agent 继续请求。
10. 业务 `data` 只保留资源数组；`count/hasMore/nextCursor` 不得与 `meta.count/meta.pagination` 重复发布。
11. 八条命令的 live Runtime Schema 必须只声明对应 active data 键；NDJSON record path
    必须相同，且不得把框架拥有的 `meta.pagination` 重新声明成业务 data 分页。

## Source coverage

- PASS：四类列表的 native 与 Shortcut 都调用
  `helpers.ProjectDevAppListPage`，字段、稳定 ID、空态与分页判断不再各自实现。
- PASS：八条已审阅的 terminal command 均在原命令路径直接输出统一结果；没有公开
  版本/协议选择参数。
- PASS：共享 `helpers.DevAppListResultSpec` 与 projector 同模块维护；native 与 shortcut
  的八条 live Schema 分别只声明 `apps/permissions/events/versions`，required 与 NDJSON
  record path 完全一致，不再发布旧 `items/hasMore/nextCursor` 声明。

## Live pagination evidence

真实只读、脱敏探针确认：应用首屏 11 项，`hasMore=false` 且位置 cursor 非空；使用该
cursor 再次读取得到 0 项、同一 cursor、仍为 `hasMore=false`。修复后四条 active 列表
均只在 `meta` 发布 count/分页，业务 `data` 不含 legacy 分页键：

| 能力（两入口同形） | 页数 | 结构计数合计 | 终态 |
|---|---:|---:|---|
| 应用 list | 1 | 11 | exhausted，无 next_token |
| permission list | 5 | 249 | exhausted，无 token 缺失/循环 |
| event list | 3 | 255 | exhausted，无 token 缺失/循环 |
| version list | 1 | 0 | exhausted，已知空 |

探针不保存原始 JSON，也不打印应用、权限、事件、版本或 cursor 值。计数只证明当前账号
本次 endpoint 响应，不能扩大成企业权限覆盖或服务端业务完整性。

共享 projector 接入后又对八条命令逐一执行首屏脱敏对拍：两入口的业务键分别严格为
`apps/permissions/events/versions`，`meta.count` 分别为 `11/20/20/0`，应用和版本为
已耗尽、权限和事件为可续页；八条均为 `ok:true/outcome:success`，且不含版本标记。


## Focused test transcript

```text
$ DWS_PACKAGE_VERSION=0.0.0-test go test -count=1 ./internal/helpers ./internal/shortcut/devapp \
  -run 'TestDevApp(ListResultSpecMatchesSharedProjection|SharedListProjection|NativeAndShortcutListToolsShareProjectedResult|PaginatedShortcutContractsMatchActiveData|PaginatedShortcutsEmitUnifiedResumableResults|ListPaginationProjectsMeta)|TestDevRepresentativeResultContractsReachContractFinal'
ok  .../internal/helpers
ok  .../internal/shortcut/devapp
```

## Boundary

这证明本地投影不会再静默吞掉异常分页字段，并已对拍真实终页位置 cursor、四类列表
完整续翻以及两前缀首屏等价；权限受限、其他账号和上游异常响应仍需单独 Agent 实测。
