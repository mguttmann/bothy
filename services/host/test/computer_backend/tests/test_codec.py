# SPDX-License-Identifier: Apache-2.0
"""The codec of the backend against the channel vectors of docs/spec/27-computer-use.md 9.4."""

from __future__ import annotations

import io
import json
import pathlib
import sys
import unittest

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parents[1]))

import backend

VECTORS = (
    pathlib.Path(__file__).resolve().parents[3]
    / "testdata"
    / "computer"
    / "channel-vectors.json"
)


class CodecTest(unittest.TestCase):
    """Every vector decodes to its object or bytes, and the encoder's output decodes back."""

    def test_vectors(self) -> None:
        vectors = json.loads(VECTORS.read_text(encoding="utf-8"))["vectors"]
        self.assertTrue(vectors)
        for v in vectors:
            raw = bytes.fromhex(v["frame_hex"])
            kind, body = backend.read_frame(io.BytesIO(raw))
            self.assertEqual(kind, v["type"], v["name"])
            if kind == backend.KIND_IMAGE:
                self.assertEqual(body, bytes.fromhex(v["bytes_hex"]), v["name"])
                self.assertEqual(backend.encode_frame(kind, body), raw, v["name"])
                continue
            self.assertEqual(json.loads(body), v["object"], v["name"])
            again = backend.encode_msg(v["object"])
            _, body2 = backend.read_frame(io.BytesIO(again))
            self.assertEqual(json.loads(body2), v["object"], v["name"])

    def test_limits(self) -> None:
        frame = backend.encode_frame(backend.KIND_IMAGE, b"x" * 100, 1000)
        with self.assertRaises(backend.ProtocolError):
            backend.read_frame(io.BytesIO(frame), 100)
        with self.assertRaises(backend.ProtocolError):
            backend.encode_frame(backend.KIND_IMAGE, b"x" * 100, 100)
        with self.assertRaises(backend.ProtocolError):
            backend.read_frame(io.BytesIO(b"\x00\x00\x00\x00"))
        with self.assertRaises(backend.ProtocolError):
            backend.read_frame(io.BytesIO(b"\x00\x00\x00\x01\x07"))
        with self.assertRaises(EOFError):
            backend.read_frame(io.BytesIO(b"\x00\x00"))


if __name__ == "__main__":
    unittest.main()
