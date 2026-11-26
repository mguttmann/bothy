# SPDX-License-Identifier: Apache-2.0
"""Fixture app of the computer use tests (docs/spec/27-computer-use.md 15, CU-T24).

A Tk window at a fixed place with known controls, so a scripted agent can click, type and check
pixels without a model: the window covers the screen from (0, 0) at 400 by 300 pixels, the
button "Alpha" lies at (20, 20) with 120 by 40 pixels, "Beta" at (160, 20), the text field at
(20, 100) with 360 by 30 pixels, and a pure red square at (300, 200) with 60 by 60 pixels. Every
click of a button and every change of the text field is appended to the event file as one JSON
line ({"event": "click", "button": "Alpha"} or {"event": "text", "value": "..."}).

Example::

    python3 fixture_app.py /tmp/events.jsonl
"""

from __future__ import annotations

import json
import sys
import tkinter as tk

GEOMETRY = "400x300+0+0"
ALPHA = (20, 20, 120, 40)
BETA = (160, 20, 120, 40)
ENTRY = (20, 100, 360, 30)
RED = (300, 200, 60, 60)


def main(argv: list[str]) -> int:
    """Show the window and write the events to argv[1] until the window closes."""
    path = argv[1]

    def emit(obj: dict[str, str]) -> None:
        with open(path, "a", encoding="utf-8") as f:
            f.write(json.dumps(obj, ensure_ascii=False) + "\n")

    root = tk.Tk(className="ComputerFixture")
    root.title("Computer fixture")
    root.geometry(GEOMETRY)
    root.configure(background="white")
    for name, (x, y, w, h) in (("Alpha", ALPHA), ("Beta", BETA)):
        b = tk.Button(
            root,
            text=name,
            command=lambda n=name: emit({"event": "click", "button": n}),
        )
        b.place(x=x, y=y, width=w, height=h)
    text = tk.StringVar()
    text.trace_add("write", lambda *_: emit({"event": "text", "value": text.get()}))
    entry = tk.Entry(root, textvariable=text)
    x, y, w, h = ENTRY
    entry.place(x=x, y=y, width=w, height=h)
    x, y, w, h = RED
    tk.Frame(root, background="#ff0000").place(x=x, y=y, width=w, height=h)
    root.after(100, lambda: emit({"event": "ready"}))
    root.mainloop()
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
