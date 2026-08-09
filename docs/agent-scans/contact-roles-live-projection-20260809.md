# Contact roles 投影形状 Agent 审阅

扫描日期：2026-08-09

> 本探针只读调用 `contact +list-roles`，原始下层响应仅在进程内解析；报告不保存角色名称、ID、成员资料或 JSON fixture，也不接入 CI。

## 结果

**PASS：当前响应未触发角色投影未知错误**

## 结构事实

- 命令 exit code：0；捕获的 contact 下层响应数：1。
- 上层结果：统一 success。
- 下层工具：`contact/get_org_labels`；仅记录结构签名：`{result:array(len=6; item={groupName:string, labels:array(len=1; item={labelId:null, name:null})} | {groupName:string, labels:array(len=28; item={labelId:number, name:string}, …)} | {groupName:string, labels:array(len=4; item={labelId:number, name:string})} | {groupName:string, labels:array(len=6; item={labelId:number, name:string})}), success:bool}`。
- 下层标签行数：57；成对空占位行数：1；可投影角色预期数：56。
- 上层已投影角色数：56。

## 下一步

仍需验证分组、空列表和权限受限形状；当前一次成功不代表组织角色目录完整。
