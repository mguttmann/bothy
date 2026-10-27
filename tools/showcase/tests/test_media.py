# SPDX-License-Identifier: Apache-2.0
"""Tests for the metadata rule of the showcase media (PNG and MP4)."""

from __future__ import annotations

import struct
import zlib

import media
import pytest
from showcase_fixtures import (
    XMP_UUID,
    box,
    chunk,
    full_box,
    large_box,
    mp4,
    png,
)

# --- PNG -----------------------------------------------------------------------


def idat(data: bytes) -> list[bytes]:
    found, pos = [], 8
    while pos < len(data):
        (length,) = struct.unpack(">I", data[pos : pos + 4])
        if data[pos + 4 : pos + 8] == b"IDAT":
            found.append(data[pos + 8 : pos + 8 + length])
        pos += 12 + length
    return found


def test_clean_png_has_no_metadata() -> None:
    assert media.png_metadata(png()) == []


def test_png_colour_chunks_are_codec_data() -> None:
    data = png(
        chunk(b"sRGB", b"\x00"),
        chunk(b"gAMA", struct.pack(">I", 45455)),
        chunk(b"cHRM", bytes(32)),
    )
    assert media.png_metadata(data) == []
    assert media.strip_png(data) == data


def test_png_text_chunk_is_metadata() -> None:
    assert media.png_metadata(png(chunk(b"tEXt", b"Author\x00Alex"))) == ["tEXt chunk"]


def test_png_exif_chunk_is_metadata() -> None:
    assert media.png_metadata(png(chunk(b"eXIf", b"MM\x00*"))) == ["eXIf chunk"]


def test_png_every_metadata_chunk_is_named_in_order() -> None:
    data = png(
        chunk(b"iCCP", b"p\x00\x00" + zlib.compress(b"x")),
        chunk(b"pHYs", bytes(9)),
        chunk(b"zTXt", b"k\x00\x00" + zlib.compress(b"v")),
        after_idat=chunk(b"tIME", bytes(7))
        + chunk(b"iTXt", b"k\x00\x00\x00\x00\x00v")
        + chunk(b"prVt", b"private"),
    )
    assert media.png_metadata(data) == [
        "iCCP chunk",
        "pHYs chunk",
        "zTXt chunk",
        "tIME chunk",
        "iTXt chunk",
        "prVt chunk",
    ]


def test_png_bytes_after_iend_are_metadata() -> None:
    assert media.png_metadata(png(trailer=b"hidden")) == ["6 bytes after IEND"]


def test_strip_png_keeps_pixels_and_valid_crcs() -> None:
    clean = png(chunk(b"sRGB", b"\x00"))
    dirty = png(
        chunk(b"sRGB", b"\x00"),
        chunk(b"tEXt", b"Comment\x00made on a host"),
        chunk(b"eXIf", b"MM\x00*"),
        after_idat=chunk(b"tIME", bytes(7)),
        trailer=b"tail",
    )
    stripped = media.strip_png(dirty)
    assert stripped == clean
    assert idat(stripped) == idat(dirty)
    assert media.png_metadata(stripped) == []


def test_strip_png_is_idempotent() -> None:
    once = media.strip_png(png(chunk(b"tEXt", b"a\x00b")))
    assert media.strip_png(once) == once


@pytest.mark.parametrize(
    ("data", "message"),
    [
        (b"GIF89a" + bytes(20), "not a PNG"),
        (png()[:-6], "truncated"),
        (png()[:20], "truncated"),
        (png()[:-12], "no IEND"),
    ],
)
def test_png_malformed_is_refused(data: bytes, message: str) -> None:
    with pytest.raises(media.MediaError, match=message):
        media.png_metadata(data)
    with pytest.raises(media.MediaError, match=message):
        media.strip_png(data)


def test_png_bad_crc_is_refused() -> None:
    data = bytearray(png())
    data[29] ^= 0xFF  # first byte of the IHDR CRC
    with pytest.raises(media.MediaError, match="CRC"):
        media.png_metadata(bytes(data))


def test_png_must_start_with_ihdr() -> None:
    data = media.PNG_SIGNATURE + chunk(b"IEND", b"")
    with pytest.raises(media.MediaError, match="IHDR"):
        media.png_metadata(data)


# --- MP4 -----------------------------------------------------------------------


def stco_offset(data: bytes) -> int:
    pos = data.index(b"stco")
    return int.from_bytes(data[pos + 12 : pos + 16])


def test_clean_mp4_has_no_metadata() -> None:
    assert media.mp4_metadata(mp4()) == []


def test_mp4_fixture_offset_points_at_mdat_payload() -> None:
    data = mp4(udta=True, created=7)
    assert data[stco_offset(data) :] == b"frame-bytes"


def test_mp4_udta_and_times_are_metadata() -> None:
    assert media.mp4_metadata(mp4(udta=True, created=3_000_000_000)) == [
        "moov/mvhd creation and modification time",
        "moov/trak/tkhd creation and modification time",
        "moov/trak/mdia/mdhd creation and modification time",
        "moov/udta box",
    ]


@pytest.mark.parametrize("version", [0, 1])
def test_strip_mp4_keeps_sizes_and_offsets(version: int) -> None:
    created = 2**40 if version == 1 else 3_000_000_000
    dirty = mp4(udta=True, created=created, version=version)
    stripped = media.strip_mp4(dirty)
    assert len(stripped) == len(dirty)
    assert stco_offset(stripped) == stco_offset(dirty)
    assert stripped[stco_offset(stripped) :] == b"frame-bytes"
    assert media.mp4_metadata(stripped) == []
    # The udta box became a free box of the same size at the same place.
    pos = dirty.index(b"udta") - 4
    size = int.from_bytes(dirty[pos : pos + 4])
    assert stripped[pos : pos + 8] == struct.pack(">I", size) + b"free"
    assert stripped[pos + 8 : pos + size] == bytes(size - 8)
    assert b"Alex title" not in stripped


def test_strip_mp4_is_idempotent() -> None:
    once = media.strip_mp4(mp4(udta=True, created=5))
    assert media.strip_mp4(once) == once


def test_mp4_top_level_meta_uuid_and_free_payload_are_metadata() -> None:
    data = mp4(
        top_extra=full_box(b"meta", 0, box(b"hdlr", bytes(4)))
        + box(b"uuid", XMP_UUID + b"<x:xmpmeta/>")
        + box(b"free", b"leftover")
        + box(b"skip", bytes(4))
        + box(b"free"),
    )
    assert media.mp4_metadata(data) == ["meta box", "uuid box", "free box with content"]
    stripped = media.strip_mp4(data)
    assert len(stripped) == len(data)
    assert media.mp4_metadata(stripped) == []
    assert b"xmpmeta" not in stripped
    assert b"leftover" not in stripped


def test_mp4_quicktime_meta_without_version_is_walked() -> None:
    # Android's screen recorder writes `moov/meta` in the QuickTime form: the first child box
    # follows the header at once, without version and flags.
    keys = box(b"keys", bytes(8)) + box(b"ilst", box(b"\x00\x00\x00\x01", b"Alex phone"))
    data = mp4(moov_extra=box(b"meta", box(b"hdlr", bytes(25)) + keys))
    assert media.mp4_metadata(data) == ["moov/meta box"]
    stripped = media.strip_mp4(data)
    assert len(stripped) == len(data)
    assert media.mp4_metadata(stripped) == []
    assert b"Alex phone" not in stripped


def test_mp4_nested_metadata_box_is_found_in_a_track() -> None:
    data = mp4(moov_extra=box(b"trak", box(b"udta", box(b"name", b"Sam"))))
    assert media.mp4_metadata(data) == ["moov/trak/udta box"]
    assert b"Sam" not in media.strip_mp4(data)


def test_mp4_64_bit_box_sizes_are_walked() -> None:
    data = mp4(top_extra=large_box(b"udta", box(b"name", b"Robin")))
    assert media.mp4_metadata(data) == ["udta box"]
    stripped = media.strip_mp4(data)
    pos = data.index(b"udta") - 4
    assert stripped[pos : pos + 16] == data[pos : pos + 4] + b"free" + data[pos + 8 : pos + 16]
    assert b"Robin" not in stripped
    assert media.mp4_metadata(stripped) == []


def test_mp4_size_zero_runs_to_the_end() -> None:
    data = mp4() + struct.pack(">I", 0) + b"udta" + box(b"name", b"Alex")
    assert media.mp4_metadata(data) == ["udta box"]
    stripped = media.strip_mp4(data)
    assert stripped.endswith(struct.pack(">I", 0) + b"free" + bytes(len(box(b"name", b"Alex"))))
    assert media.mp4_metadata(stripped) == []


@pytest.mark.parametrize(
    ("data", "message"),
    [
        (box(b"moov", b""), "not an MP4"),
        (b"\x00\x00", "not an MP4"),
        (mp4() + b"\x00\x00\x00", "truncated"),
        (mp4() + struct.pack(">I", 100) + b"mdat", "runs past"),
        (mp4() + struct.pack(">I", 4) + b"mdat", "smaller than its header"),
        (mp4() + struct.pack(">I", 1) + b"mdat" + struct.pack(">Q", 8), "smaller than its header"),
        (mp4(moov_extra=box(b"mvhd", b"\x00")), "too short"),
        (mp4(moov_extra=full_box(b"mvhd", 1, bytes(8))), "too short"),
        (mp4() + struct.pack(">I", 1) + b"mdat" + bytes(4), "64-bit size"),
        (mp4(moov_extra=full_box(b"mvhd", 2, bytes(16))), "unknown version"),
        (mp4(top_extra=box(b"meta", b"\x00\x00")), "too short"),
        (mp4(top_extra=box(b"uuid", b"short")), "too short"),
    ],
)
def test_mp4_malformed_is_refused(data: bytes, message: str) -> None:
    with pytest.raises(media.MediaError, match=message):
        media.mp4_metadata(data)
    with pytest.raises(media.MediaError, match=message):
        media.strip_mp4(data)


# --- detection -----------------------------------------------------------------


@pytest.mark.parametrize(
    ("data", "kind"),
    [(png(), "png"), (mp4(), "mp4"), (b"\xff\xd8\xff\xe0JFIF", None), (b"", None)],
)
def test_detect(data: bytes, kind: str | None) -> None:
    assert media.detect(data) == kind


def test_strip_and_metadata_dispatch_on_content() -> None:
    assert media.metadata(png(chunk(b"tEXt", b"a\x00b"))) == ["tEXt chunk"]
    assert media.metadata(mp4(created=1)) != []
    assert media.strip(png(chunk(b"tEXt", b"a\x00b"))) == png()
    assert media.metadata(media.strip(mp4(created=1))) == []
    with pytest.raises(media.MediaError, match="neither PNG nor MP4"):
        media.strip(b"\xff\xd8\xff\xe0JFIF")
    with pytest.raises(media.MediaError, match="neither PNG nor MP4"):
        media.metadata(b"GIF89a")
