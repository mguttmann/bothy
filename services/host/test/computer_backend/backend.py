# SPDX-License-Identifier: Apache-2.0
"""Linux test backend of the computer helper (docs/spec/27-computer-use.md 9.4, CU-03).

Linux has no computer helper; this backend stands in for it in tests only. It speaks the protocol
of the local channel between the host service and the helper over the same endpoint the host
service opens on Linux (``/run/bothy-host-computer/computer.sock``, admitted only for the user of
``host_service.computer_test_peer``), and drives an X server (Xvfb in the tests) with python-xlib
and the XTEST extension. The Compose end-to-end test of CU-06 runs it next to the fixture app.

Behaviour that stands in for the native helpers:

* ``hello`` on connect with the enablement given on the command line (``--generation``,
  ``--disabled``); permissions are ``granted``, the signature ``development``.
* ``capture`` takes an image of the root window (or a region) in memory, scales it so the long
  edge is at most ``max_edge`` and encodes JPEG or PNG with Pillow; nothing touches the disk.
* ``do`` injects pointer and keyboard input with XTEST; the lock state is checked right before
  every injection (``screen_locked``), like the helpers do.
* SIGUSR1 simulates a lock, SIGUSR2 the unlock (``state`` with ``locked``); SIGHUP stops the live
  session like the stop button of the indicator (``stop`` with ``indicator_stop``).
* ``release`` and ``hide`` release every key and button this backend pressed.

Example::

    python3 backend.py --endpoint /run/bothy-host-computer/computer.sock --generation 1
"""

from __future__ import annotations

import argparse
import getpass
import io
import json
import select
import signal
import socket
import struct
import sys
import time
from typing import Any

KIND_JSON = 0x01
KIND_IMAGE = 0x02
LEN_SIZE = 4
DEFAULT_MAX_BYTES = 64 << 20
HELPER_VERSION = "0.0.0-test"
# MAPPING_PAUSE gives the X clients the time to read a changed keyboard mapping before keys of the
# new mapping arrive; a constant of the test backend, not a product value.
MAPPING_PAUSE = 0.1

MODIFIER_KEYSYMS = {
    "shift": "Shift_L",
    "ctrl": "Control_L",
    "alt": "Alt_L",
    "cmd": "Super_L",
    "win": "Super_L",
}

# The aliases of key names that agents use and the X keysym names.
KEY_ALIASES = {
    "enter": "Return",
    "return": "Return",
    "esc": "Escape",
    "escape": "Escape",
    "tab": "Tab",
    "space": "space",
    "backspace": "BackSpace",
    "delete": "Delete",
    "up": "Up",
    "down": "Down",
    "left": "Left",
    "right": "Right",
    "home": "Home",
    "end": "End",
    "pageup": "Prior",
    "pagedown": "Next",
}

SCROLL_BUTTONS = {"up": 4, "down": 5, "left": 6, "right": 7}
CLICK_BUTTONS = {
    "left_click": (1, 1),
    "right_click": (3, 1),
    "middle_click": (2, 1),
    "double_click": (1, 2),
    "triple_click": (1, 3),
}


class SockReader:
    """A buffered reader of a socket whose ``pending`` tells whether a frame can be read without
    waiting for the host service; the backend waits with select, so signals are handled between
    frames and never in the middle of one."""

    def __init__(self, sock: socket.socket) -> None:
        self.sock = sock
        self.buf = b""

    def read(self, n: int) -> bytes:
        """Return up to n bytes, waiting for at least one; b"" when the channel closed."""
        if not self.buf:
            self.buf = self.sock.recv(65536)
        out, self.buf = self.buf[:n], self.buf[n:]
        return out

    def pending(self, timeout: float) -> bool:
        """Report whether bytes are buffered or arrive within timeout seconds."""
        if self.buf:
            return True
        readable, _, _ = select.select([self.sock], [], [], timeout)
        return bool(readable)


class ProtocolError(Exception):
    """A frame against the protocol of the channel; the connection closes."""


def encode_frame(kind: int, body: bytes, max_bytes: int = DEFAULT_MAX_BYTES) -> bytes:
    """Return the wire form of one channel frame.

    Args:
        kind: KIND_JSON or KIND_IMAGE.
        body: the content.
        max_bytes: host_service.computer_frame_max_bytes.

    Raises:
        ProtocolError: the frame is longer than max_bytes.
    """
    n = 1 + len(body)
    if n > max_bytes:
        raise ProtocolError("frame too long")
    return struct.pack(">I", n) + bytes([kind]) + body


def encode_msg(obj: dict[str, Any], max_bytes: int = DEFAULT_MAX_BYTES) -> bytes:
    """Return the channel frame of one JSON object."""
    return encode_frame(
        KIND_JSON, json.dumps(obj, ensure_ascii=False).encode("utf-8"), max_bytes
    )


def read_exact(reader: Any, n: int) -> bytes:
    """Read exactly n bytes from a file-like reader (``read``) or raise EOFError."""
    out = b""
    while len(out) < n:
        chunk = reader.read(n - len(out))
        if not chunk:
            raise EOFError("channel closed")
        out += chunk
    return out


def read_frame(reader: Any, max_bytes: int = DEFAULT_MAX_BYTES) -> tuple[int, bytes]:
    """Read one channel frame and return its kind and content.

    Raises:
        ProtocolError: a length of zero or above max_bytes, or an unknown kind.
        EOFError: the channel closed.
    """
    (n,) = struct.unpack(">I", read_exact(reader, LEN_SIZE))
    if n == 0 or n > max_bytes:
        raise ProtocolError("frame length out of range")
    kind = read_exact(reader, 1)[0]
    if kind not in (KIND_JSON, KIND_IMAGE):
        raise ProtocolError("unknown frame kind")
    return kind, read_exact(reader, n - 1)


class Backend:
    """The helper side of the channel, driving one X display."""

    def __init__(
        self, sock: socket.socket, generation: int, enabled: bool, max_bytes: int
    ) -> None:
        from Xlib import XK, X, display
        from Xlib.ext import xtest

        self.X, self.XK, self.xtest = X, XK, xtest
        self.d = display.Display()
        self.root = self.d.screen().root
        self.sock = sock
        self.reader = SockReader(sock)
        self.max_bytes = max_bytes
        self.generation = generation
        self.enabled = enabled
        self.locked = False
        self.session = ""
        self.keys_down: list[int] = []
        self.buttons_down: list[int] = []
        self.pending_lock: bool | None = None
        self.pending_stop = False
        self.spares = self._spare_keycodes()
        self.spare = self.spares[-1] if self.spares else self.d.display.info.max_keycode

    # Channel.

    def send(self, obj: dict[str, Any]) -> None:
        """Write one JSON message."""
        self.sock.sendall(encode_msg(obj, self.max_bytes))

    def hello(self) -> None:
        """Send hello, the first frame of the connection."""
        self.send(
            {
                "op": "hello",
                "enabled": self.enabled,
                "generation": self.generation,
                "os_account": getpass.getuser(),
                "permissions": {"capture": "granted", "input": "granted"},
                "helper_version": HELPER_VERSION,
                "signature": "development",
                "locked": self.locked,
            }
        )

    def screen(self) -> dict[str, Any]:
        """Return the geometry of the screen; X has scale 1."""
        s = self.d.screen()
        return {
            "x": 0,
            "y": 0,
            "width": s.width_in_pixels,
            "height": s.height_in_pixels,
            "scale": 1,
        }

    def app(self) -> dict[str, str]:
        """Return the identity of the window with the input focus (WM_CLASS and WM_NAME)."""
        try:
            w = self.d.get_input_focus().focus
            for _ in range(8):
                if not hasattr(w, "get_wm_class"):
                    break
                cls = w.get_wm_class()
                if cls:
                    return {
                        "id": cls[1],
                        "name": w.get_wm_name() or cls[1],
                        "signer": "",
                    }
                tree = w.query_tree()
                if tree.parent == 0 or tree.parent == self.root:
                    break
                w = tree.parent
        except Exception:  # noqa: BLE001 - an identity is best effort; the action goes on
            return {"id": "", "name": "", "signer": ""}
        return {"id": "", "name": "", "signer": ""}

    def done(self, seq: int, code: str, **extra: Any) -> dict[str, Any]:
        """Return a done message."""
        msg = {
            "op": "done",
            "seq": seq,
            "code": code,
            "app": self.app(),
            "screen": self.screen(),
        }
        msg.update(extra)
        return msg

    def serve(self) -> None:
        """Answer the host service until the channel closes."""
        self.hello()
        while True:
            self._signals()
            if not self.reader.pending(0.2):
                continue
            try:
                kind, body = read_frame(self.reader, self.max_bytes)
            except EOFError:
                self.release()
                return
            if kind != KIND_JSON:
                raise ProtocolError("image frame from the host service")
            self.handle(json.loads(body))

    def _signals(self) -> None:
        if self.pending_lock is not None:
            self.locked, self.pending_lock = self.pending_lock, None
            if self.locked:
                self.release()
            self.send({"op": "state", "locked": self.locked})
        if self.pending_stop:
            self.pending_stop = False
            if self.session:
                self.send(
                    {
                        "op": "stop",
                        "session_id": self.session,
                        "detail": "indicator_stop",
                    }
                )

    def handle(self, m: dict[str, Any]) -> None:
        """Handle one message of the host service."""
        op = m.get("op")
        if op == "show":
            self.session = m["session_id"]
        elif op == "hide":
            self.release()
            self.session = ""
        elif op == "release":
            self.release()
        elif op == "disable":
            self.enabled = False
            self.release()
            self.send({"op": "state", "enabled": False})
        elif op == "capture":
            self.capture(m)
        elif op == "position":
            p = self.root.query_pointer()
            self.send(self.done(m["seq"], "ok", cursor={"x": p.root_x, "y": p.root_y}))
        elif op == "do":
            self.send(self.done(m["seq"], self.do(m)))
        else:
            raise ProtocolError("unknown op")

    # Capture.

    def capture(self, m: dict[str, Any]) -> None:
        """Answer capture with done and the image frame; the image stays in memory."""
        from PIL import Image

        if self.locked:
            self.send(self.done(m["seq"], "screen_locked"))
            return
        scr = self.screen()
        r = m.get("region") or {
            "x": 0,
            "y": 0,
            "width": scr["width"],
            "height": scr["height"],
        }
        x, y = round(r["x"]), round(r["y"])
        w, h = max(1, round(r["width"])), max(1, round(r["height"]))
        raw = self.root.get_image(x, y, w, h, self.X.ZPixmap, 0xFFFFFFFF)
        img = Image.frombytes("RGB", (w, h), raw.data, "raw", "BGRX")
        edge = int(m["max_edge"])
        if max(w, h) > edge:
            f = edge / max(w, h)
            img = img.resize(
                (max(1, round(w * f)), max(1, round(h * f))), Image.Resampling.LANCZOS
            )
        buf = io.BytesIO()
        if m["format"] == "png":
            img.save(buf, format="PNG")
        else:
            img.save(buf, format="JPEG", quality=int(m["quality"]))
        data = buf.getvalue()
        info = {
            "format": m["format"],
            "width": img.width,
            "height": img.height,
            "bytes": len(data),
        }
        self.sock.sendall(
            encode_msg(self.done(m["seq"], "ok", image=info), self.max_bytes)
            + encode_frame(KIND_IMAGE, data, self.max_bytes)
        )

    # Input.

    def do(self, m: dict[str, Any]) -> str:
        """Inject one action; returns the code of done."""
        if self.locked:
            return "screen_locked"
        action = m["action"]
        mods = [self._keycode(MODIFIER_KEYSYMS[x]) for x in m.get("modifiers", [])]
        if "x" in m and "y" in m:
            self.xtest.fake_input(
                self.d, self.X.MotionNotify, x=round(m["x"]), y=round(m["y"])
            )
        if action in CLICK_BUTTONS:
            button, times = CLICK_BUTTONS[action]
            self._with(mods, lambda: self._click(button, times))
        elif action == "scroll":
            self._click(SCROLL_BUTTONS[m["direction"]], int(m["amount"]))
        elif action == "mouse_move":
            pass
        elif action == "type":
            self._type_text(m["text"])
        elif action == "key":
            for _ in range(int(m.get("repeat", 1) or 1)):
                self._combo(m["keys"])
        else:
            return "invalid_action"
        self.d.sync()
        return "ok"

    def _with(self, mods: list[int], f: Any) -> None:
        for kc in mods:
            self._key(kc, True)
        f()
        for kc in reversed(mods):
            self._key(kc, False)

    def _click(self, button: int, times: int) -> None:
        for _ in range(times):
            self.xtest.fake_input(self.d, self.X.ButtonPress, button)
            self.buttons_down.append(button)
            self.xtest.fake_input(self.d, self.X.ButtonRelease, button)
            self.buttons_down.remove(button)
        self.d.sync()

    def _key(self, keycode: int, down: bool) -> None:
        self.xtest.fake_input(
            self.d, self.X.KeyPress if down else self.X.KeyRelease, keycode
        )
        if down:
            self.keys_down.append(keycode)
        elif keycode in self.keys_down:
            self.keys_down.remove(keycode)

    def _combo(self, keys: str) -> None:
        names = [k for k in keys.split("+") if k]
        codes = []
        for n in names:
            if n.lower() in MODIFIER_KEYSYMS:
                codes.append(self._keycode(MODIFIER_KEYSYMS[n.lower()]))
            else:
                codes.append(self._keysym_code(self._keysym(n)))
        for kc in codes:
            self._key(kc, True)
        for kc in reversed(codes):
            self._key(kc, False)
        self.d.sync()

    def _keysym(self, name: str) -> int:
        name = KEY_ALIASES.get(name.lower(), name)
        ks = self.XK.string_to_keysym(name)
        if ks == 0 and len(name) == 1:
            ks = self._char_keysym(name)
        return ks

    @staticmethod
    def _char_keysym(ch: str) -> int:
        cp = ord(ch)
        return cp if 0x20 <= cp <= 0xFF else 0x01000000 | cp

    def _keycode(self, keysym_name: str) -> int:
        return self._keysym_code(self.XK.string_to_keysym(keysym_name))

    def _keysym_code(self, ks: int) -> int:
        kc = self.d.keysym_to_keycode(ks)
        if kc:
            return kc
        self.d.change_keyboard_mapping(self.spare, [(ks, ks)])
        self.d.sync()
        time.sleep(MAPPING_PAUSE)
        return self.spare

    def _type_text(self, text: str) -> None:
        """Type text as Unicode input, whatever the keyboard layout: a character whose keysym the
        layout has is typed with its keycode (and shift for the second column); every other
        character gets an empty keycode mapped to its keysym, in batches, with a pause so the
        clients read the new mapping first (MappingNotify)."""
        shift = self._keycode("Shift_L")
        keysyms = [self._text_keysym(ch) for ch in text]
        start = 0
        while start < len(keysyms):
            mapped: dict[int, tuple[int, bool]] = {}
            free = list(self.spares)
            end = start
            while end < len(keysyms):
                ks = keysyms[end]
                if ks not in mapped:
                    pairs = [
                        (kc, idx)
                        for kc, idx in self.d.keysym_to_keycodes(ks)
                        if idx in (0, 1)
                    ]
                    if pairs:
                        mapped[ks] = (pairs[0][0], pairs[0][1] == 1)
                    elif free:
                        kc = free.pop()
                        self.d.change_keyboard_mapping(kc, [(ks, ks)])
                        mapped[ks] = (kc, False)
                    else:
                        break
                end += 1
            self.d.sync()
            time.sleep(MAPPING_PAUSE)
            for ks in keysyms[start:end]:
                kc, shifted = mapped[ks]
                if shifted:
                    self._key(shift, True)
                self._key(kc, True)
                self._key(kc, False)
                if shifted:
                    self._key(shift, False)
                self.d.sync()
            start = end

    def _text_keysym(self, ch: str) -> int:
        if ch == "\n":
            return self.XK.string_to_keysym("Return")
        if ch == "\t":
            return self.XK.string_to_keysym("Tab")
        return self._char_keysym(ch)

    def _spare_keycodes(self) -> list[int]:
        lo, hi = self.d.display.info.min_keycode, self.d.display.info.max_keycode
        mapping = self.d.get_keyboard_mapping(lo, hi - lo + 1)
        return [lo + i for i in range(len(mapping)) if not any(mapping[i])]

    def release(self) -> None:
        """Release every key and button this backend pressed."""
        for kc in list(self.keys_down):
            self._key(kc, False)
        for b in list(self.buttons_down):
            self.xtest.fake_input(self.d, self.X.ButtonRelease, b)
            self.buttons_down.remove(b)
        self.d.sync()


def main(argv: list[str] | None = None) -> int:
    """Connect to the endpoint and serve until the host service closes the channel."""
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("--endpoint", default="/run/bothy-host-computer/computer.sock")
    p.add_argument("--generation", type=int, default=1)
    p.add_argument("--disabled", action="store_true")
    p.add_argument("--max-bytes", type=int, default=DEFAULT_MAX_BYTES)
    a = p.parse_args(argv)
    sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    sock.connect(a.endpoint)
    b = Backend(sock, a.generation, not a.disabled, a.max_bytes)

    def lock(_s: int, _f: Any) -> None:
        b.pending_lock = True

    def unlock(_s: int, _f: Any) -> None:
        b.pending_lock = False

    def stop(_s: int, _f: Any) -> None:
        b.pending_stop = True

    signal.signal(signal.SIGUSR1, lock)
    signal.signal(signal.SIGUSR2, unlock)
    signal.signal(signal.SIGHUP, stop)
    try:
        b.serve()
    finally:
        b.release()
        sock.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
