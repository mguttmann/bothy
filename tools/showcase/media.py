# SPDX-License-Identifier: Apache-2.0
"""Metadata of the showcase media formats: find it and strip it (M7-28, requirement 14).

The showcase carries PNG images and MP4 videos only. Both formats can carry data beside the
pixels and frames that tells who made the file, when and on which machine: PNG text chunks,
EXIF, a modification time, an embedded colour profile with a device name; MP4 user data boxes,
iTunes-style tags, XMP and the creation times of the movie, its tracks and their media.

The rule: a file keeps exactly what a decoder needs to show it and nothing else.

- PNG keeps `IHDR`, `PLTE`, `tRNS`, `IDAT`, `IEND` and the colour decoding chunks `sRGB`,
  `gAMA` and `cHRM`. Every other chunk is metadata (`tEXt`, `zTXt`, `iTXt`, `eXIf`, `tIME`,
  `iCCP`, `pHYs`, any private chunk), and so are bytes after `IEND`. Stripping rebuilds the file
  from the kept chunks, which keep their CRCs.
- MP4 (ISO BMFF) metadata boxes are `udta`, `meta`, `ilst`, every `uuid` box (XMP and other
  vendor data no player needs) and a `free` or `skip` box whose payload is not all zero; the
  creation and modification times of `mvhd`, `tkhd` and `mdhd` are metadata too. Stripping
  must not move a byte, because `stco` and `co64` hold absolute file offsets into `mdat`: a
  metadata box becomes a `free` box of the same size with a zeroed payload, and the times
  become zero, in place.

The MP4 walk descends into the boxes that hold the movie structure (`moov`, `trak`, `mdia`,
`minf`, `stbl`, `edts`, `dinf`) and into `udta` and `meta` (after its version and flags) to
check that they are well formed. Sample data in `mdat` and fragmented files (`moof`) are not
inspected: the capture drivers write a plain, non-fragmented MP4.

Errors: a file that is not the format it claims, is truncated, or has a box or chunk that runs
past its parent raises `MediaError` with the byte position.
"""

from __future__ import annotations

import struct
import zlib
from collections.abc import Iterator
from dataclasses import dataclass

PNG_SIGNATURE = b"\x89PNG\r\n\x1a\n"
# Chunks a decoder needs for the pixels and their colours; everything else is metadata.
PNG_KEEP = frozenset({b"IHDR", b"PLTE", b"tRNS", b"IDAT", b"IEND", b"sRGB", b"gAMA", b"cHRM"})

# Boxes that only group other boxes of the movie structure.
MP4_CONTAINERS = frozenset({b"moov", b"trak", b"mdia", b"minf", b"stbl", b"edts", b"dinf"})
# Metadata boxes that are containers themselves; they are walked to prove they are well formed.
MP4_METADATA_CONTAINERS = frozenset({b"udta", b"meta"})
MP4_METADATA = frozenset({b"udta", b"meta", b"ilst", b"uuid"})
MP4_PADDING = frozenset({b"free", b"skip"})
# Full boxes whose payload starts with a creation and a modification time.
MP4_TIMED = frozenset({b"mvhd", b"tkhd", b"mdhd"})


class MediaError(ValueError):
    """A media file that is not the claimed format, is truncated or is otherwise malformed."""


def detect(data: bytes) -> str | None:
    """Name the showcase format of a file by its content.

    Args:
        data: the whole file.

    Returns:
        `"png"` for the PNG signature, `"mp4"` when the first box is `ftyp`, else `None`.
    """
    if data.startswith(PNG_SIGNATURE):
        return "png"
    if data[4:8] == b"ftyp":
        return "mp4"
    return None


def metadata(data: bytes) -> list[str]:
    """Name the metadata of a PNG or MP4 file, chosen by content.

    Args:
        data: the whole file.

    Returns:
        One description per metadata item; empty when the file is clean.

    Raises:
        MediaError: the file is neither PNG nor MP4, or is malformed.
    """
    match detect(data):
        case "png":
            return png_metadata(data)
        case "mp4":
            return mp4_metadata(data)
    raise MediaError("neither PNG nor MP4")


def strip(data: bytes) -> bytes:
    """Remove the metadata of a PNG or MP4 file, chosen by content.

    Args:
        data: the whole file.

    Returns:
        The file without metadata (`strip_png` or `strip_mp4`).

    Raises:
        MediaError: the file is neither PNG nor MP4, or is malformed.
    """
    match detect(data):
        case "png":
            return strip_png(data)
        case "mp4":
            return strip_mp4(data)
    raise MediaError("neither PNG nor MP4")


# --- PNG -----------------------------------------------------------------------


@dataclass(frozen=True)
class _Chunk:
    kind: bytes
    start: int
    end: int


def _png_chunks(data: bytes) -> tuple[list[_Chunk], int]:
    """Split a PNG into its chunks; return them and the number of bytes after `IEND`."""
    if not data.startswith(PNG_SIGNATURE):
        raise MediaError("not a PNG: signature missing")
    chunks: list[_Chunk] = []
    pos = len(PNG_SIGNATURE)
    while pos < len(data):
        if pos + 8 > len(data):
            raise MediaError(f"PNG truncated: chunk header at byte {pos} is incomplete")
        (length,) = struct.unpack(">I", data[pos : pos + 4])
        kind = data[pos + 4 : pos + 8]
        end = pos + 12 + length
        if end > len(data):
            raise MediaError(f"PNG truncated: chunk at byte {pos} runs past the end")
        (crc,) = struct.unpack(">I", data[end - 4 : end])
        if zlib.crc32(data[pos + 4 : end - 4]) != crc:
            raise MediaError(f"PNG chunk {_name(kind)} at byte {pos} has a wrong CRC")
        if not chunks and kind != b"IHDR":
            raise MediaError("PNG does not start with IHDR")
        chunks.append(_Chunk(kind, pos, end))
        pos = end
        if kind == b"IEND":
            return chunks, len(data) - end
    raise MediaError("PNG has no IEND chunk")


def png_metadata(data: bytes) -> list[str]:
    """Name every metadata chunk of a PNG and any bytes after `IEND`.

    Args:
        data: the whole file.

    Returns:
        `"<type> chunk"` per metadata chunk in file order, then `"<n> bytes after IEND"`;
        empty when the file is clean.

    Raises:
        MediaError: not a PNG, truncated, a wrong CRC, no leading IHDR or no IEND.
    """
    chunks, trailing = _png_chunks(data)
    found = [f"{_name(c.kind)} chunk" for c in chunks if c.kind not in PNG_KEEP]
    if trailing:
        found.append(f"{trailing} bytes after IEND")
    return found


def strip_png(data: bytes) -> bytes:
    """Rebuild a PNG from the chunks that carry pixels and colour decoding.

    Args:
        data: the whole file.

    Returns:
        The signature and the kept chunks, byte for byte with their CRCs; the pixel data
        (`IDAT`) is unchanged.

    Raises:
        MediaError: as `png_metadata`.
    """
    chunks, _ = _png_chunks(data)
    kept = (data[c.start : c.end] for c in chunks if c.kind in PNG_KEEP)
    return PNG_SIGNATURE + b"".join(kept)


# --- MP4 -----------------------------------------------------------------------


@dataclass(frozen=True)
class _Box:
    kind: bytes
    start: int
    # Size of the size fields: 8, or 16 with a 64-bit `largesize`.
    size_header: int
    # First byte of the payload, after the size fields and a `uuid` user type.
    payload: int
    end: int


def _boxes(data: bytes, start: int, end: int) -> Iterator[_Box]:
    """Yield the boxes between `start` and `end`, checking every size against the parent."""
    pos = start
    while pos < end:
        if pos + 8 > end:
            raise MediaError(f"MP4 truncated: box header at byte {pos} is incomplete")
        (size,) = struct.unpack(">I", data[pos : pos + 4])
        kind = data[pos + 4 : pos + 8]
        size_header = 8
        if size == 1:
            if pos + 16 > end:
                raise MediaError(f"MP4 truncated: 64-bit size at byte {pos} is incomplete")
            (size,) = struct.unpack(">Q", data[pos + 8 : pos + 16])
            size_header = 16
        elif size == 0:
            size = end - pos
        if size < size_header:
            raise MediaError(f"MP4 box {_name(kind)} at byte {pos} is smaller than its header")
        if pos + size > end:
            raise MediaError(f"MP4 box {_name(kind)} at byte {pos} runs past its parent")
        payload = pos + size_header
        if kind == b"uuid":
            payload += 16
            if payload > pos + size:
                raise MediaError(f"MP4 uuid box at byte {pos} is too short for its user type")
        yield _Box(kind, pos, size_header, payload, pos + size)
        pos += size


def _walk(data: bytes, start: int, end: int, path: str) -> Iterator[tuple[_Box, str]]:
    """Yield every box of the walked tree with its parent path (`moov/trak/`)."""
    for b in _boxes(data, start, end):
        yield b, path
        if b.kind in MP4_CONTAINERS or b.kind in MP4_METADATA_CONTAINERS:
            first = b.payload
            if b.kind == b"meta":
                # ISO `meta` is a full box: version and flags (zero) come before its children.
                # The QuickTime form, which Android's recorder writes, starts with a child box.
                if first + 4 > b.end:
                    raise MediaError(f"MP4 meta box at byte {b.start} is too short")
                if data[first : first + 4] == b"\0\0\0\0":
                    first += 4
            yield from _walk(data, first, b.end, f"{path}{_name(b.kind)}/")


def _times(data: bytes, b: _Box) -> tuple[int, int]:
    """Return the offset and width of the two time fields of a `mvhd`, `tkhd` or `mdhd`."""
    if b.payload + 4 > b.end:
        raise MediaError(f"MP4 {_name(b.kind)} box at byte {b.start} is too short")
    version = data[b.payload]
    if version not in (0, 1):
        raise MediaError(f"MP4 {_name(b.kind)} box at byte {b.start} has unknown version {version}")
    width = 16 if version == 1 else 8
    if b.payload + 4 + width > b.end:
        raise MediaError(f"MP4 {_name(b.kind)} box at byte {b.start} is too short")
    return b.payload + 4, width


def _items(data: bytes) -> Iterator[tuple[_Box, str, str]]:
    """Yield `(box, path, what)` for each metadata box and each header with times.

    Boxes inside a metadata box are walked for their structure but not reported: stripping
    the outer box clears them too.
    """
    if detect(data) != "mp4":
        raise MediaError("not an MP4: the first box is not ftyp")
    inside: int = -1
    for b, path in _walk(data, 0, len(data), ""):
        if b.start < inside:
            continue
        if b.kind in MP4_METADATA:
            inside = b.end
            yield b, path, "box"
        elif b.kind in MP4_PADDING and any(data[b.payload : b.end]):
            yield b, path, "box with content"
        elif b.kind in MP4_TIMED:
            offset, width = _times(data, b)
            if any(data[offset : offset + width]):
                yield b, path, "creation and modification time"


def mp4_metadata(data: bytes) -> list[str]:
    """Name every metadata box and every non-zero time of an MP4.

    Args:
        data: the whole file.

    Returns:
        `"<path><type> box"`, `"<path><type> box with content"` (padding that is not zero) or
        `"<path><type> creation and modification time"` in file order, the path naming the
        parent boxes (`moov/trak/`); empty when the file is clean.

    Raises:
        MediaError: not an MP4 (no leading `ftyp`) or a malformed box tree.
    """
    return [f"{path}{_name(b.kind)} {what}" for b, path, what in _items(data)]


def strip_mp4(data: bytes) -> bytes:
    """Clear the metadata of an MP4 in place, without changing any size or offset.

    Each metadata box becomes a `free` box of the same size (its size fields stay as they
    were, the rest is zero), and the creation and modification times of `mvhd`, `tkhd` and
    `mdhd` become zero. Sample offsets in `stco` and `co64` therefore stay valid.

    Args:
        data: the whole file.

    Returns:
        The file of the same length without metadata.

    Raises:
        MediaError: as `mp4_metadata`.
    """
    out = bytearray(data)
    for b, _, _ in list(_items(data)):
        if b.kind in MP4_TIMED:
            offset, width = _times(data, b)
            out[offset : offset + width] = bytes(width)
            continue
        out[b.start + 4 : b.start + 8] = b"free"
        out[b.start + b.size_header : b.end] = bytes(b.end - b.start - b.size_header)
    return bytes(out)


def _name(kind: bytes) -> str:
    """A four-character type as text; a non-ASCII byte (the 0xA9 of `©nam`) keeps its code point."""
    return kind.decode("latin-1")
