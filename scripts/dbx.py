#!/usr/bin/env python3
"""Thin stdio client for the local dbx MCP server. Usage: dbx.py <tool_name> [json_args]"""
import subprocess, json, time, os, sys

exe = os.path.expanduser(r"~/.dbx/bin/dbx-mcp.exe")
p = subprocess.Popen([exe], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True, encoding='utf-8')

def send(msg):
    p.stdin.write(json.dumps(msg) + "\n"); p.stdin.flush()

def drain(seconds=20):
    lines, end = [], time.time() + seconds
    while time.time() < end:
        line = p.stdout.readline()
        if line:
            lines.append(line.strip())
            if '"id":' in line and ('"result"' in line or '"error"' in line):
                break
        else:
            time.sleep(0.05)
    return lines

send({"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"cli","version":"1.0"}}})
drain(8)
send({"jsonrpc":"2.0","method":"notifications/initialized"})

tool = sys.argv[1]
args = json.loads(sys.argv[2]) if len(sys.argv) > 2 else {}
send({"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":tool,"arguments":args}})
out = drain(60)
for l in out:
    try:
        m = json.loads(l)
        if m.get("id") == 2:
            r = m.get("result") or m.get("error")
            for c in (r.get("content") if isinstance(r, dict) and "content" in r else [{"text": json.dumps(r, ensure_ascii=False)}]):
                print(c.get("text", ""))
            break
    except Exception:
        pass
p.kill()
