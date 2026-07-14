# 导入本地表格

## 导入本地表格

用户说"导入Excel/把xlsx转为在线表格/上传表格并在线编辑"时：
```bash
# 上传并转换为在线电子表格（转换后返回 nodeId，可用 sheet 命令操作）
dws drive upload --file ./data.xlsx --convert

# 指定上传到某个文件夹
dws drive upload --file ./data.xlsx --folder <FOLDER_ID> --convert

# 指定上传到知识库
dws drive upload --file ./data.xlsx --workspace <WS_ID> --convert
```
- `--convert` 是关键参数，不加则仅上传为附件，不会转换为在线电子表格
- 转换后的文档为 `axls` 格式，可用 `sheet` 全部命令操作
- 支持 `.xlsx` / `.xls` / `.csv` 等格式
