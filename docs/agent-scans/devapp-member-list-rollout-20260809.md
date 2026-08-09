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

源码构建二进制先通过 `devapp +list` 在内存中取得一个稳定
`unifiedAppId`，再查询成员；探针不保存原始 JSON，也不打印应用、人员、cursor 或查询标识。

第一次探针发现 DevApp 原始响应为 11 项、`hasMore=false`，但仍携带长度为 10 的
`nextCursor`。将该 cursor 原样用于第二次只读调用后，服务端返回 0 项、同一 cursor、
`hasMore=false`。因此它是终页位置标记，不是 continuation。客户端据此仅在 DevApp
适配层改为：相信明确的 `hasMore=false`，输出 `endpoint_exhausted:true`，不暴露
`next_token`；其他产品的分页规则不变。

修复后脱敏结果：

```text
list_rc=0 list_ok=true list_outcome=success list_count=11 endpoint_exhausted=true has_next_token=false version_marker=false
member_rc=0 member_ok=true member_outcome=success data_type=object version_marker=false
member_count=1 stable_member_ids=1 business_success_type=boolean
```

## 结论与边界

- 命令级统一输出迁移、受控结果对拍和一组真实成员正向读取通过；
- 真实环境的已知空、权限受限、非终页续翻和异常字段仍待隔离账号复验；
- 当前 11 项只证明该账号本次 endpoint 已耗尽，不证明企业应用目录的权限覆盖或业务完整性。
