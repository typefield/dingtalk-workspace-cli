# 安全配置

> 安全配置=应用的 IP 白名单 / 登录重定向 / 端内免登 URL；见 SKILL.md 概念地图。

## 查询

```bash
dws dev app security get --unified-app-id <unifiedAppId> --format json
```

返回 `ipWhitelist`、`redirectUrls`、`ssoUrls` 三个字段（已配置的非空列表）和 `configured`（是否已配置任一项）。

## 配置

`dws dev app security config` 配 IP 白名单（`--ip-whitelist`）、登录重定向（`--redirect-urls`）、端内免登（`--sso-urls`）。参数查 `dws schema dev.app.security.config`，至少给一个配置字段。

覆盖语义：未提供的字段不动；显式提供的列表是整组覆盖（传入即全量替换该项，不是追加）——要保留旧值就把旧值一起带上。

## 发现命令

调用任何方法前先查清楚再敲：

```
# 浏览命令组下的子命令与 flag
dws dev app security --help

# 查某方法的必填参数、类型、默认值
dws schema dev.app.security.config
```

按 `dws schema` 输出构造 `--flag`（flag 名 = schema 参数名）。

## 三方个人应用

三方个人应用的 `security config` 只能配登录重定向（`--redirect-urls`，即 OAuth 回调地址），其余字段一律不支持：

```bash
dws dev app security config --unified-app-id <id> --redirect-urls https://example.com/callback --yes --format json
```

- `--redirect-urls`：唯一支持项，整组覆盖（传入即全量替换，要保留旧值就一起带上）。
- `--ip-whitelist`、`--sso-urls`：三方个人应用不支持，禁止配置——用户要求配这两项时直接说明不支持，不要下发命令。

写完后用 `dev app security get` 回读确认（返回的 `redirectUrls` 应包含刚配置的地址），不要走企业内部网页应用配置命令验证。

不覆盖：小程序（miniapp）安全域名。那是另一套体系（按 miniAppId + requestType 配单个域名白名单，如 request/download/socket 域名），和本命令的应用级安全配置（unifiedAppId + IP 白名单 / 登录重定向 / 端内免登）互不相通。用户说「小程序安全域名 / 业务域名 / 服务器域名 / requestType」时，本 skill 不支持，别把 security config 套上去。
