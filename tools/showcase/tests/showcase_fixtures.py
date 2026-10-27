# SPDX-License-Identifier: Apache-2.0
"""Builders for the showcase test media: small PNG and MP4 files made in code.

No binary fixture files: every image and video of the tests is built here from its
chunks or boxes, so a test states exactly which metadata it carries.
"""

from __future__ import annotations

import struct
import zlib

PNG_SIGNATURE = b"\x89PNG\r\n\x1a\n"


def chunk(kind: bytes, data: bytes) -> bytes:
    """One PNG chunk with a valid CRC."""
    return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data))


def png(*extra: bytes, after_idat: bytes = b"", trailer: bytes = b"") -> bytes:
    """A 2x2 RGB PNG; `extra` chunks sit between IHDR and IDAT."""
    ihdr = struct.pack(">IIBBBBB", 2, 2, 8, 2, 0, 0, 0)
    rows = b"".join(b"\x00" + bytes([40, 80, 120]) * 2 for _ in range(2))
    return (
        PNG_SIGNATURE
        + chunk(b"IHDR", ihdr)
        + b"".join(extra)
        + chunk(b"IDAT", zlib.compress(rows))
        + after_idat
        + chunk(b"IEND", b"")
        + trailer
    )


def box(kind: bytes, payload: bytes = b"") -> bytes:
    """One ISO BMFF box with a 32-bit size."""
    return struct.pack(">I", 8 + len(payload)) + kind + payload


def large_box(kind: bytes, payload: bytes = b"") -> bytes:
    """One ISO BMFF box with a 64-bit size (`size` 1 and a `largesize`)."""
    return struct.pack(">I", 1) + kind + struct.pack(">Q", 16 + len(payload)) + payload


def full_box(kind: bytes, version: int, payload: bytes) -> bytes:
    """A full box: version, three bytes of flags, then the payload."""
    return box(kind, bytes([version, 0, 0, 0]) + payload)


def times(version: int, created: int, modified: int) -> bytes:
    """Creation and modification time in the width of the box version."""
    return struct.pack(">QQ" if version == 1 else ">II", created, modified)


XMP_UUID = bytes.fromhex("be7acfcb97a942e89c71999491e3afac")


def mp4(
    *,
    created: int = 0,
    version: int = 0,
    udta: bool = False,
    moov_extra: bytes = b"",
    top_extra: bytes = b"",
) -> bytes:
    """A small MP4 whose `stco` points into `mdat`, so offsets can be compared after a strip.

    Args:
        created: creation and modification time written into mvhd, tkhd and mdhd.
        version: full box version of the three header boxes (0: 32-bit times, 1: 64-bit).
        udta: add a `udta` with a `meta`/`ilst` title to the movie box.
        moov_extra: further boxes appended inside the movie box.
        top_extra: further top-level boxes between the movie box and `mdat`.
    """
    stamp = times(version, created, created)
    ftyp = box(b"ftyp", b"isom" + struct.pack(">I", 512) + b"isomiso2mp41")
    mvhd = full_box(b"mvhd", version, stamp + struct.pack(">I", 1000) + bytes(84))
    tkhd = full_box(b"tkhd", version, stamp + struct.pack(">I", 1) + bytes(68))
    mdhd = full_box(b"mdhd", version, stamp + struct.pack(">I", 1000) + bytes(8))
    hdlr = full_box(b"hdlr", 0, bytes(4) + b"vide" + bytes(12) + b"Video\x00")
    meta = b""
    if udta:
        ilst = box(b"ilst", box(b"\xa9nam", box(b"data", bytes(8) + b"Alex title")))
        meta = box(b"udta", full_box(b"meta", 0, hdlr + ilst))

    def build(offset: int) -> bytes:
        stco = full_box(b"stco", 0, struct.pack(">II", 1, offset))
        stbl = box(b"stbl", stco)
        minf = box(b"minf", stbl)
        mdia = box(b"mdia", mdhd + hdlr + minf)
        trak = box(b"trak", tkhd + box(b"edts", full_box(b"elst", 0, bytes(4))) + mdia)
        moov = box(b"moov", mvhd + trak + meta + moov_extra)
        return ftyp + moov + top_extra

    head = build(0)
    head = build(len(head) + 8)
    return head + box(b"mdat", b"frame-bytes")
