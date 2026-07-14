# 适用范围

## 适用范围

`sheet` 仅支持钉钉在线电子表格（`contentType=ALIDOC`、`extension=axls`）。

| 文件类型 | 处理方式 |
|---------|---------|
| 在线电子表格（`axls`） | 走 `sheet` 全部命令 |
| `xlsx` / `xls` / `xlsm` / `csv` | `dws drive download --node <ID> --output <路径>` 下载到本地处理，禁止调用任何 `sheet` 子命令 |
| 本地 xlsx 导入为在线表格 | `dws drive upload --file <路径> --convert`（上传并转换为在线电子表格，转换后可用 `sheet` 命令操作） |
| 在线表格导出为 xlsx | `dws sheet export`（axls → xlsx 格式转换） |

用户贴原始 `alidocs` URL 时必须先 probe：`dws doc info --node <URL> --format json`，按 [链接规范](../url-patterns.md#alidocs-url-类型探测流程) 校验：
- `contentType=ALIDOC` + `extension=axls` → 继续走 `sheet`
- `extension=xlsx` / `xls` / `xlsm` / `csv` → 转 `dws drive download`，告知用户"这是本地表格文件，已为你下载到本地处理"
