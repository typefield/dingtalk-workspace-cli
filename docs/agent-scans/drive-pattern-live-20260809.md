# drive list --pattern Agent 实测

扫描日期：2026-08-09

> 当前源码临时构建；真实 Drive JSON 只在内存解析。本文件不保存文件名、ID、路径、token 或原始响应，也不接入 CI / policy。

## Result: PASS

| 检查项 | 结果 | 脱敏证据 |
|---|---|---|
| pattern 投影与 continuation 回归 | PASS | `rc=0` |
| 临时构建当前源码 | PASS | `rc=0` |
| Help 公开 --pattern | PASS | `rc=0` |
| 通配正例 | PASS | `baseline=41, matched=41` |
| 精确正例 | PASS | `expected=1, matched=1` |
| 无命中反例 | PASS | `matched=0` |
| 当前页过滤保留续页令牌 | PASS | `page_count=5, token_preserved=yes` |

## 结论

- `--pattern` 对服务端选定的当前页做客户端名称过滤；通配、精确和无命中三类实测均符合预期。
- 过滤不吞掉服务端 continuation；它不把当前页匹配结果扩大成整个 Drive 目录完整性。
- 本次正常根页只证明当前账号、当前 endpoint 页的行为，不证明子目录、权限受限、分页后续页或服务端目录覆盖。
