# DevApp member-list unified output Agent review

扫描时间：2026-08-09（Asia/Shanghai）

## 范围

- 目标命令：`dws devapp +member-list`
- 迁移前：`dual_validate` / legacy active contract
- 迁移后：`unified_active` / unified active contract
- 对照入口：`dws dev app member list`

本次只审阅一个只读、幂等 terminal command；没有批量切换其他 DevApp 写命令，
也没有引入公开协议选择 flag 或结果版本标记。Agent 仍只使用
`--format json`。

## 受控对拍

专项 Go 回归通过以下约束：

1. shortcut 只调用一次 `devapp/list_dev_app_members`；
2. 顶层结果为 `ok:true`、`outcome:success`，业务结果位于 `data`；
3. 输出不包含已经删除的 `contract_version`；
4. shortcut 与 native `dev app member list` 共用
   `DevAppCommandResultFromPayload`，同一上游载荷得到同一 outcome；
5. rollout 声明审阅确认该命令是单步
   `dual_validate -> unified_active`，当前 inventory 为
   `legacy_only=1343`、`dual_validate=19`、`unified_active=121`。

专项命令：

```text
DWS_PACKAGE_VERSION=0.0.0-test go test -count=1 ./internal/shortcut/devapp ./internal/helpers
```

结果：PASS。

## 真实只读探针

源码构建二进制先尝试通过 `devapp +list` 在内存中取得一个稳定
`unifiedAppId`，再查询成员；探针不保存原始 JSON，也不打印应用、人员或查询标识。

前置列表在真实账号上返回：

```text
rc=3 ok=false outcome=failure error_type=validation error_subtype=pagination_conflict
```

因此 Agent 遵循 fail-closed，没有从矛盾分页结果中提取应用 ID，也没有发出成员
查询。这个结果证明分页冲突会被诚实表达，但**不构成 member-list 真实正向证明**。

## 结论与边界

- 命令级统一输出迁移和受控结果对拍通过；
- 真实环境的成员正向、已知空和权限受限场景仍待有可信
  `unifiedAppId` 的受控账号复验；
- `devapp +list` 的真实 `pagination_conflict` 需由 DevApp 服务适配层确认：不能为
  方便发现 ID 而忽略矛盾 cursor，也不能把当前列表扩大成完整应用目录。
