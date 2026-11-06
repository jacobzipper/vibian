#!/usr/bin/env python3
"""Drive `vibian mcp computer` the way a harness does, from a JSON list of steps.

  computer-drive.py STEPS.json

Steps: {"tool": NAME, "args": {...}, "save": PNG-PATH} calls a tool,
{"sh": CMD} runs a shell command, {"sleep": SECONDS} waits. Every tool call
prints `CALL <tool> OK|ERR <text or image WxH>`, every command `SH => <stdout>`.
One server process serves all steps, as in a real session.
"""
import base64, json, subprocess, sys, time

steps = json.load(open(sys.argv[1]))
p = subprocess.Popen(["vibian", "mcp", "computer", "--agent", "test"],
                     stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True)
n = 0


def rpc(method, params=None):
    global n
    n += 1
    p.stdin.write(json.dumps({"jsonrpc": "2.0", "id": n, "method": method, "params": params or {}}) + "\n")
    p.stdin.flush()
    return json.loads(p.stdout.readline())


rpc("initialize", {"protocolVersion": "2025-06-18", "capabilities": {}, "clientInfo": {"name": "t", "version": "1"}})
print("TOOLS", len(rpc("tools/list")["result"]["tools"]))
for s in steps:
    if "sleep" in s:
        time.sleep(s["sleep"])
        continue
    if "sh" in s:
        out = subprocess.run(["bash", "-c", s["sh"]], capture_output=True, text=True).stdout.strip()
        print("SH =>", out.replace("\n", " "))
        continue
    r = rpc("tools/call", {"name": s["tool"], "arguments": s.get("args", {})})["result"]
    parts = []
    for item in r["content"]:
        if item["type"] == "image":
            data = base64.b64decode(item["data"])
            open(s.get("save", "/tmp/shot.png"), "wb").write(data)
            parts.append("image %dx%d" % (int.from_bytes(data[16:20], "big"), int.from_bytes(data[20:24], "big")))
        else:
            parts.append(item["text"].replace("\n", " "))
    print("CALL", s["tool"], "ERR" if r.get("isError") else "OK", " | ".join(parts))
    time.sleep(0.3)
p.stdin.close()
p.wait()
