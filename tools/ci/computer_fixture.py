# SPDX-License-Identifier: Apache-2.0
"""Fixture window of the computer helper probe (`computer_probe.py`, brief CU-02).

A Tk window filled with a known color (`computer_probe.FIXTURE_RGB`) with one entry field. Once
the window shows, it prints one JSON line with its geometry in screen coordinates (the space of
the helper's `done.screen`: points on macOS, pixels on Windows) and the center of the entry; then
it appends one JSON line per event to `--events`: clicks, keys (Tk key symbol) and the entry's
text after every key. It logs nothing else and keeps no image.

Usage: computer_fixture.py --events FILE
"""

from __future__ import annotations

import argparse
import json
import sys
import tkinter as tk
from pathlib import Path
from typing import Any

COLOR = "#2e8b57"
SIZE = "640x420+120+120"
# Milliseconds before the geometry is read, so the window manager has placed the window.
SHOW_DELAY_MS = 1500


def main(argv: list[str] | None = None) -> int:
    """Open the window and log its events until the process is ended."""
    parser = argparse.ArgumentParser(description="Fixture window of the computer probe.")
    parser.add_argument("--events", type=Path, required=True)
    args = parser.parse_args(argv)
    log = args.events.open("a", encoding="utf-8")

    def write(event: dict[str, Any]) -> None:
        log.write(json.dumps(event) + "\n")
        log.flush()

    root = tk.Tk()
    root.title("Bothy probe fixture")
    root.geometry(SIZE)
    root.configure(background=COLOR)
    root.attributes("-topmost", True)
    entry = tk.Entry(root, width=32)
    entry.place(relx=0.5, rely=0.25, anchor="center")

    def click(e: Any) -> None:
        write({"kind": "click", "x": e.x_root, "y": e.y_root, "button": e.num})

    def key(e: Any) -> None:
        write({"kind": "key", "keysym": e.keysym})

    def text(_: Any) -> None:
        write({"kind": "text", "value": entry.get()})

    root.bind_all("<ButtonPress>", click)
    root.bind_all("<KeyPress>", key)
    root.bind_all("<KeyRelease>", text)

    def ready() -> None:
        root.lift()
        root.focus_force()
        root.update_idletasks()
        geometry = {
            "x": root.winfo_rootx(),
            "y": root.winfo_rooty(),
            "width": root.winfo_width(),
            "height": root.winfo_height(),
            "entry": {
                "x": entry.winfo_rootx() + entry.winfo_width() // 2,
                "y": entry.winfo_rooty() + entry.winfo_height() // 2,
            },
            "color": COLOR,
        }
        sys.stdout.write(json.dumps(geometry) + "\n")
        sys.stdout.flush()

    root.after(SHOW_DELAY_MS, ready)
    root.mainloop()
    return 0


if __name__ == "__main__":
    sys.exit(main())
