# SPDX-License-Identifier: Apache-2.0
"""A ``stdio`` MCP server for the probes of 27 16: one tool that returns a known image.

Standard library only; started by the CLI from a ``--mcp-config`` file, as the runner starts
``bothy-computer-mcp`` (docs/spec/27-computer-use.md 10.1). It answers ``initialize``,
``tools/list`` and ``tools/call`` of the Model Context Protocol (the revisions with
``initialize``) and the request form of ``2026-07-28`` (``server/discover``), so it works with
whichever revision the CLI sends. The tool ``screenshot`` answers one text block and one PNG
image block whose bytes carry `SENTINEL`, so a probe can search every file the CLI wrote for
the image.

When it gets ``BOTHY_COMPUTER_DIR`` (the request folder of 27 10.2), the server writes the names
and values of the variables `RECORDED` it received into ``probe-env.json`` there (JSON), so a
probe sees the ``env`` of the MCP file reach the server. It writes nothing else to disk.

Usage (by the CLI): ``python3 -I image_mcp.py``.
"""

from __future__ import annotations

import base64
import json
import os
import struct
import sys
import zlib
from pathlib import Path
from typing import Any

SENTINEL = b"BOTHY-CU06-PROBE-SENTINEL-7f3a9c"
"""A byte pattern no CLI writes by itself; it stands in a ``tEXt`` chunk of the image."""

RECORDED = (
    "BOTHY_COMPUTER_DIR",
    "BOTHY_COMPUTER_NONCE",
    "BOTHY_COMPUTER_TIMEOUT_MS",
    "BOTHY_COMPUTER_PINS",
    "BOTHY_COMPUTER_SERVER",
)
"""The variables of 27 10.1 the server reports back to the probe."""

ENV_FILE = "probe-env.json"
"""The file in the request folder with the variables the server received."""

TOOL = {
    "name": "screenshot",
    "description": "Take a screenshot.",
    "inputSchema": {"type": "object", "properties": {}, "additionalProperties": False},
}


def _chunk(kind: bytes, data: bytes) -> bytes:
    body = kind + data
    return struct.pack(">I", len(data)) + body + struct.pack(">I", zlib.crc32(body) & 0xFFFFFFFF)


def png() -> bytes:
    """Return a 4 by 4 red PNG whose ``tEXt`` chunk holds `SENTINEL`."""
    width = height = 4
    rows = b"".join(b"\x00" + b"\xff\x00\x00" * width for _ in range(height))
    return b"".join(
        (
            b"\x89PNG\r\n\x1a\n",
            _chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0)),
            _chunk(b"tEXt", b"Comment\x00" + SENTINEL),
            _chunk(b"IDAT", zlib.compress(rows)),
            _chunk(b"IEND", b""),
        )
    )


def image_base64() -> str:
    """Return the image as the base64 text of the MCP image block."""
    return base64.b64encode(png()).decode("ascii")


def _result(request_id: Any, result: dict[str, Any]) -> None:
    sys.stdout.write(json.dumps({"jsonrpc": "2.0", "id": request_id, "result": result}) + "\n")
    sys.stdout.flush()


def _record_env() -> None:
    folder = os.environ.get("BOTHY_COMPUTER_DIR")
    if folder:
        target = Path(folder) / ENV_FILE
        values = {name: os.environ.get(name) for name in RECORDED}
        Path(target).write_text(json.dumps(values), encoding="utf-8")


def _answer(message: dict[str, Any]) -> None:
    method = message.get("method")
    request_id = message.get("id")
    if request_id is None:
        return  # a notification
    if method == "initialize":
        params = message.get("params") or {}
        _result(
            request_id,
            {
                "protocolVersion": params.get("protocolVersion", "2025-06-18"),
                "capabilities": {"tools": {}},
                "serverInfo": {"name": "probe-image", "version": "1"},
            },
        )
    elif method == "server/discover":
        _result(
            request_id,
            {
                "supportedVersions": ["2026-07-28"],
                "capabilities": {"tools": {}},
                "serverInfo": {"name": "probe-image", "version": "1"},
            },
        )
    elif method == "tools/list":
        _result(request_id, {"tools": [TOOL]})
    elif method == "tools/call":
        _result(
            request_id,
            {
                "content": [
                    {"type": "text", "text": "width 4, height 4"},
                    {"type": "image", "data": image_base64(), "mimeType": "image/png"},
                ],
                "isError": False,
            },
        )
    else:
        error = {"code": -32601, "message": "method not found"}
        sys.stdout.write(json.dumps({"jsonrpc": "2.0", "id": request_id, "error": error}) + "\n")
        sys.stdout.flush()


def main() -> int:
    """Serve requests on stdin until it ends."""
    _record_env()
    for line in sys.stdin:
        if line.strip():
            _answer(json.loads(line))
    return 0


if __name__ == "__main__":
    sys.exit(main())
