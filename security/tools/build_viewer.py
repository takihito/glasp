#!/usr/bin/env python3
"""Embed a Threat Composer .tc.json into the viewer template as a single HTML file.

usage: build_viewer.py <input.tc.json> <output.html>
"""
import html
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))


def main():
    src, out = sys.argv[1], sys.argv[2]
    with open(src, encoding="utf-8") as fp:
        doc = json.load(fp)
    with open(os.path.join(HERE, "viewer-template.html"), encoding="utf-8") as fp:
        tpl = fp.read()

    # JSON inside <script type="application/json">: write every "<" as the
    # JSON escape \u003c so the payload can never close the script element or
    # open an HTML comment, and JSON.parse still restores the original text.
    # "<" only appears inside JSON strings, where \u003c is a valid escape.
    payload = json.dumps(doc, ensure_ascii=False, separators=(",", ":"))
    payload = payload.replace("<", "\\u003c")

    name = (doc.get("applicationInfo") or {}).get("name") or "Threat Model"
    title = html.escape(f"{name} 脅威モデル")

    assert tpl.count("__DATA__") == 1 and tpl.count("__TITLE__") == 1
    result = tpl.replace("__TITLE__", title).replace("__DATA__", payload)
    with open(out, "w", encoding="utf-8") as fp:
        fp.write(result)
    print("wrote", out, f"({len(result.encode('utf-8'))} bytes)")


if __name__ == "__main__":
    main()
