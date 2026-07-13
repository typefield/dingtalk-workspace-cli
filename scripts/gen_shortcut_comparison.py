#!/usr/bin/env python3
# Generates docs/shortcut-comparison.html: a 3-way comparison for every built-in
# shortcut — the dws `+command`, the equivalent lark-cli command, and the raw
# dws native `dws mcp ...` combination it replaces. Data is extracted directly
# from source so it stays truthful.
import os, re, glob, html, json

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SC_DIR = os.path.join(ROOT, "internal", "shortcut")
LARK = "/Users/dennis/Projects/larksuite/cli/shortcuts"

SKIP_PKG = {"builtin", "usage", "userdef"}

# dws service -> lark service (None = DingTalk-specific, no lark counterpart)
LARK_MAP = {
    "chat": "im", "todo": "task", "aitable": "base", "sheet": "sheets",
    "devapp": "apps", "contact": "contact", "calendar": "calendar",
    "doc": "doc", "drive": "drive", "mail": "mail", "wiki": "wiki",
    "minutes": "minutes",
    # DingTalk-specific:
    "attendance": None, "oa": None, "report": None, "ding": None,
    "aisearch": None, "live": None, "devdoc": None,
}

def load_lark():
    out = {}
    for svc in set(v for v in LARK_MAP.values() if v):
        cmds = set()
        for f in glob.glob(os.path.join(LARK, svc, "*.go")):
            try:
                txt = open(f, encoding="utf-8").read()
            except OSError:
                continue
            for m in re.finditer(r'Command:\s*"(\+[a-z0-9-]+)"', txt):
                cmds.add(m.group(1))
        out[svc] = cmds
    return out

def split_vars(txt):
    """Yield source blocks for each `var X = shortcut.Shortcut{ ... }`."""
    lines = txt.splitlines()
    i, n = 0, len(lines)
    while i < n:
        if re.match(r'\s*var \w+ = shortcut\.Shortcut\{', lines[i]):
            buf = [lines[i]]
            i += 1
            while i < n and not re.match(r'^\}', lines[i]):
                buf.append(lines[i]); i += 1
            if i < n:
                buf.append(lines[i])
            yield "\n".join(buf)
        i += 1

def field(block, name):
    m = re.search(name + r':\s*"([^"]*)"', block)
    return m.group(1) if m else ""

def parse_block(block, pkg, consts=None):
    consts = consts or {}
    svc = field(block, "Service") or pkg
    cmd = field(block, "Command")
    product = field(block, "Product") or svc
    desc = field(block, "Description")
    rm = re.search(r'Risk:\s*shortcut\.Risk(\w+)', block)
    risk = {"Read": "read", "Write": "write", "HighWrite": "high-risk-write"}.get(rm.group(1), "read") if rm else "read"
    tm = re.search(r'CallMCP\("([^"]+)"', block)
    if tm:
        tool = tm.group(1)
    else:
        # tool passed as a const identifier, e.g. CallMCP(listeningNoteCmdTool, ...)
        im = re.search(r'CallMCP\(([A-Za-z_]\w*)', block)
        tool = consts.get(im.group(1), "") if im else ""
    # flags: each {Name: "x" ... Required: true?}
    flags = []
    for fm in re.finditer(r'\{Name:\s*"([^"]+)"(.*?)\}', block, re.S):
        req = "Required: true" in fm.group(2)
        flags.append((fm.group(1), req))
    # param keys from the Execute body (map literal keys + params["k"])
    exec_part = block.split("Execute:", 1)[-1]
    keys = []
    seen = set()
    for km in re.finditer(r'"([a-zA-Z_][a-zA-Z0-9_]*)":', exec_part):
        k = km.group(1)
        if k not in seen:
            seen.add(k); keys.append(k)
    for km in re.finditer(r'params\["([a-zA-Z_][a-zA-Z0-9_]*)"\]', exec_part):
        k = km.group(1)
        if k not in seen:
            seen.add(k); keys.append(k)
    return dict(service=svc, command=cmd, product=product, desc=desc, risk=risk,
               tool=tool, flags=flags, keys=keys)

def collect():
    items = []
    for d in sorted(glob.glob(os.path.join(SC_DIR, "*"))):
        pkg = os.path.basename(d)
        if not os.path.isdir(d) or pkg in SKIP_PKG:
            continue
        for f in glob.glob(os.path.join(d, "*.go")):
            if f.endswith("_test.go"):
                continue
            txt = open(f, encoding="utf-8").read()
            consts = dict(re.findall(r'(\w+)\s*=\s*"([^"]+)"', txt))
            for block in split_vars(txt):
                it = parse_block(block, pkg, consts)
                if it["command"]:
                    items.append(it)
    return items

def shortcut_cmd(it):
    s = f"dws {it['service']} {it['command']}"
    for name, req in it["flags"]:
        s += f" --{name} <{name}>"
    return s

def raw_cmd(it):
    if not it["tool"]:
        return f"dws mcp {it['product']} <tool>"
    if not it["keys"]:
        return f"dws mcp {it['product']} {it['tool']}"
    body = ", ".join(f'"{k}":<..>' for k in it["keys"])
    return f"dws mcp {it['product']} {it['tool']} --json '{{{body}}}'"

def lark_cmd(it, lark):
    lsvc = LARK_MAP.get(it["service"], "__none__")
    if lsvc is None:
        return ("none", "无对应（钉钉特有服务）")
    if lsvc == "__none__":
        return ("none", "—")
    cmds = lark.get(lsvc, set())
    if it["command"] in cmds:
        return ("hit", f"lark-cli {lsvc} {it['command']}")
    return ("miss", f"lark {lsvc} 无同名命令（能力/命名不同）")

RISK_CLS = {"read": "rk-r", "write": "rk-w", "high-risk-write": "rk-h"}

def gen_html(items, lark):
    by_svc = {}
    for it in items:
        by_svc.setdefault(it["service"], []).append(it)
    order = sorted(by_svc, key=lambda s: -len(by_svc[s]))

    hit = sum(1 for it in items if lark_cmd(it, lark)[0] == "hit")
    dss = sum(1 for it in items if LARK_MAP.get(it["service"]) is None)

    P = []
    P.append(f'''<!DOCTYPE html><html lang="zh-CN"><head><meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>DWS Shortcut 三方对照（dws vs lark-cli vs 原生 MCP）</title>
<style>
:root{{--bg:#0f1420;--card:#161d2c;--ink:#e6edf6;--muted:#93a1b5;--line:#26314a;--accent:#6ea8fe;--green:#3fb950;--yellow:#d5a429;--red:#f25c5c}}
*{{box-sizing:border-box}} body{{margin:0;background:#0f1420;color:var(--ink);font:14px/1.6 -apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC",sans-serif;padding-bottom:70px}}
header{{padding:40px 22px 22px;text-align:center;border-bottom:1px solid var(--line);background:radial-gradient(1000px 260px at 50% -50px,rgba(79,156,255,.15),transparent)}}
h1{{margin:0 0 6px;font-size:25px}} .sub{{color:var(--muted);font-size:13px}}
.wrap{{max-width:1280px;margin:0 auto;padding:0 18px}}
.stats{{display:flex;gap:12px;justify-content:center;flex-wrap:wrap;margin:20px 0 4px}}
.stat{{background:var(--card);border:1px solid var(--line);border-radius:12px;padding:12px 18px;min-width:120px}}
.stat .n{{font-size:24px;font-weight:700;color:var(--accent)}} .stat .l{{color:var(--muted);font-size:12px}}
.note{{background:var(--card);border:1px solid var(--line);border-left:3px solid var(--accent);border-radius:8px;padding:12px 16px;margin:22px 0;color:var(--muted);font-size:13px}}
.note b{{color:var(--ink)}}
h2{{font-size:18px;margin:34px 0 4px;padding-top:10px}} h2 .c{{color:var(--muted);font-size:14px;font-weight:400}}
table{{width:100%;border-collapse:collapse;margin:10px 0 6px;background:var(--card);border:1px solid var(--line);border-radius:10px;overflow:hidden;table-layout:fixed}}
th,td{{padding:9px 11px;text-align:left;border-bottom:1px solid var(--line);vertical-align:top;word-break:break-all}}
th{{background:#1b2536;color:var(--muted);font-size:12px;font-weight:600}}
tr:last-child td{{border-bottom:none}}
col.c1{{width:29%}} col.c2{{width:38%}} col.c3{{width:33%}}
code{{font-family:"SF Mono",Menlo,Consolas,monospace;font-size:12.5px}}
.sc code{{color:#8fd3ff}} .raw code{{color:#c7cfdb}} .lk-hit code{{color:#84e39a}}
.desc{{color:var(--muted);font-size:11.5px;margin-top:3px}}
.rk{{display:inline-block;font-size:10px;padding:0 6px;border-radius:10px;margin-left:6px;vertical-align:1px}}
.rk-r{{background:#12321d;color:#66d38a}} .rk-w{{background:#3a2f10;color:#e2b23c}} .rk-h{{background:#3a1414;color:#f27272}}
.lk-none{{color:#6b7890}} .lk-miss{{color:var(--yellow)}} .lk-hit{{color:var(--green)}}
footer{{color:var(--muted);text-align:center;font-size:12px;margin-top:36px}}
</style></head><body>
<header><h1>DWS Shortcut 三方指令对照</h1>
<div class="sub">每个 shortcut：<b style="color:#8fd3ff">dws +命令</b> vs <b style="color:#84e39a">lark-cli</b> vs <b style="color:#c7cfdb">dws 原生 MCP 组合</b>　·　源码自动提取</div>
<div class="stats">
<div class="stat"><div class="n">{len(items)}</div><div class="l">shortcut 总数</div></div>
<div class="stat"><div class="n">{len(order)}</div><div class="l">服务</div></div>
<div class="stat"><div class="n">{hit}</div><div class="l">lark 同名可对齐</div></div>
<div class="stat"><div class="n">{dss}</div><div class="l">钉钉特有(无 lark)</div></div>
</div></header>
<div class="wrap">
<div class="note"><b>读表说明</b>：① <b style="color:#8fd3ff">dws shortcut</b> = 新增的精选命令（<code>&lt;xxx&gt;</code> 为参数占位）。
② <b style="color:#c7cfdb">dws 原生 MCP 组合</b> = 该 shortcut 底层等价的裸命令，需手拼 JSON —— 直观体现 shortcut 的收敛价值。
③ <b style="color:#84e39a">lark-cli</b> = 飞书对应命令：<span class="lk-hit">绿色</span>=同名可对齐；<span class="lk-miss">黄色</span>=lark 有该服务但命名/能力不同；<span class="lk-none">灰色</span>=钉钉特有服务，lark 无对应。因两边 API 不同，仅同名项做逐一对齐。</div>
''')

    for svc in order:
        rows = by_svc[svc]
        lsvc = LARK_MAP.get(svc)
        lbl = f"↔ lark {lsvc}" if lsvc else "钉钉特有 · lark 无对应"
        P.append(f'<h2>{html.escape(svc)} <span class="c">· {len(rows)} 个 · {lbl}</span></h2>')
        P.append('<table><colgroup><col class="c1"><col class="c2"><col class="c3"></colgroup>')
        P.append('<thead><tr><th>dws shortcut（新增）</th><th>dws 原生 MCP 组合（等价裸命令）</th><th>lark-cli 对应</th></tr></thead><tbody>')
        for it in sorted(rows, key=lambda x: x["command"]):
            rk = RISK_CLS.get(it["risk"], "rk-r")
            lk_cls, lk_txt = lark_cmd(it, lark)
            lk_html = f'<span class="lk-{lk_cls}"><code>{html.escape(lk_txt)}</code></span>' if lk_cls == "hit" else f'<span class="lk-{lk_cls}">{html.escape(lk_txt)}</span>'
            P.append("<tr>")
            P.append(f'<td class="sc"><code>{html.escape(shortcut_cmd(it))}</code><span class="rk {rk}">{it["risk"]}</span><div class="desc">{html.escape(it["desc"])}</div></td>')
            P.append(f'<td class="raw"><code>{html.escape(raw_cmd(it))}</code></td>')
            P.append(f'<td>{lk_html}</td>')
            P.append("</tr>")
        P.append("</tbody></table>")

    P.append('<footer>由 scripts/gen_shortcut_comparison.py 从源码自动生成 · 数据随 shortcut 演进刷新</footer></div></body></html>')
    return "\n".join(P)

def main():
    lark = load_lark()
    items = collect()
    out = gen_html(items, lark)
    dst = os.path.join(ROOT, "docs", "shortcut-comparison.html")
    open(dst, "w", encoding="utf-8").write(out)
    hit = sum(1 for it in items if lark_cmd(it, lark)[0] == "hit")
    miss = sum(1 for it in items if lark_cmd(it, lark)[0] == "miss")
    none = sum(1 for it in items if lark_cmd(it, lark)[0] == "none")
    print(f"shortcuts={len(items)} lark_hit={hit} lark_miss={miss} dingtalk_specific={none}")
    print("written:", dst)

if __name__ == "__main__":
    main()
