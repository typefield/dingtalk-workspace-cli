# Multi AITable 写脚本 dry-run Agent 探针

扫描入口：`scripts/agent/probe_multi_aitable_dry_run.py`。

探针在临时目录创建 CSV、字段配置、附件和 sentinel `dws` 可执行文件；逐个
运行四个脚本的 `--dry-run`，要求 stdout 是可解析计划信封，且 sentinel 未被调用。
临时 fixture 和 sentinel 在进程退出后删除，不写入仓库。

实测结果：

```text
multi AITable dry-run probe: 4/4 passed; no dws or OSS write observed
```

这只证明脚本层在该输入下没有远端写入；不等于正式执行成功，也不替代真实账号下的
服务端终态校验。
