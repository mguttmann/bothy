---
status: accepted
date: 2026-10-10
decision-makers: Planning lead
consulted: Team G (platform services), Team C (security)
informed: Team E (apps)
---

# Remove image metadata losslessly

## Context and Problem Statement

Photos carry metadata: EXIF with location, device and time, XMP, IPTC, text chunks in PNG. Otherwise
this data leaves the sender with every shared image without the sender noticing
([19-security.md](../spec/19-security.md), SEC-UP-10, R-SE-06). The planning basis originally
planned removal by re-encoding (ruling R-Objektspeicher). But re-encoding costs quality for every
original, and pure Go discards the color profile and the orientation while doing so. How does the
server remove metadata so that the original is kept in full quality (ruling L16)?

## Decision Drivers

- Location data never leaves the sender; no switch to turn it off (R-SE-06).
- The original stays available as a download in full quality (ruling L16).
- Color and orientation are correct in the original and in the preview.
- When in doubt, remove: what the server does not know is dropped (allowlist instead of denylist,
  R-SE-03).
- Pure Go in the standard build, no cgo (ADR 0005, Baseline R-Objektspeicher).
- No decoding of oversized images (decompression bombs, SEC-T08).

## Considered Options

- Lossless segment and chunk filter with an allowlist for JPEG and PNG, thumbnails re-encoded,
  other formats only as a download
- Re-encoding every inline image including the original
- Denylist: remove only known metadata fields (EXIF GPS, XMP), keep the rest
- Removal in the client before the upload
- External tool (libvips or exiftool) as a separate process for all formats

## Decision Outcome

Chosen option: "Lossless segment and chunk filter with an allowlist", because it is the only one
that reliably removes location data and keeps the original byte for byte in the image data. The
rule is in [12-files.md](../spec/12-files.md), section 7 (R-FI-07).

1. **JPEG.** When the upload completes, the server writes a new original (`orig-2`) from the
   allowlist: all markers that decoding according to ITU-T T.81 needs (header, tables, frame,
   scans with their data, end), `APP0` (JFIF), `APP14` (Adobe), `APP2` only with the identifier
   `ICC_PROFILE` (all parts of a multi-part profile) and a newly written `APP1` with exactly
   the EXIF tag "Orientation". Everything else is dropped: other `APPn` (EXIF including the
   embedded thumbnail, XMP, IPTC, Multi-Picture Format), `COM` and every byte after the first
   end of image. Reason for the last item: formats such as Ultra HDR append further images after
   the first one [Q3, verified]; these carry their own metadata and are not needed for the first
   image.
2. **PNG.** The critical chunks and the chunks for color, transparency, dimensions and
   animation from the list in 12, section 7.1 are kept. Text chunks, `eXIf`, `tIME` and every
   unknown chunk are dropped, even if it is marked as safe to copy.
3. **If the filter fails** on a file that was detected as JPEG or PNG (damaged structure), the
   file is not stored (`upload_state = 'failed'`). Reason: the promise applies to every stored
   JPEG and PNG; a file stored unchanged could carry location data.
4. **Thumbnails** are created by decoding and re-encoding in pure Go, with the orientation
   applied. They carry no metadata except the unchanged color profile of the original. Reason:
   the standard library does not convert color spaces; without the profile, photos with a wide
   color gamut would look washed out in the timeline and in full screen. The profile contains no
   location data, and the original carries it anyway.
5. **Other formats** stay as in 12, section 7.1: GIF is re-encoded, still WebP is converted to
   PNG or JPEG, HEIC, AVIF, TIFF and animated WebP are only download cards, unless
   libvips is turned on. SVG is never inline.

### Consequences

- Good, because originals stay without generation loss and `sha256` describes the stored object;
  the declared checksum stays in the upload session.
- Good, because the allowlist removes new kinds of metadata without a code change.
- Good, because the filter only reads structure and decodes no pixels; bombs only hit the
  preview, and that has `file.image.max_pixels` as a limit.
- Bad, because HDR extra images (gain maps) are dropped; the original then shows only the
  standard image. That is the price of the allowlist.
- Bad, because still WebP without alpha is stored as JPEG and thereby converted lossily; according
  to L16 the filter applies only to JPEG and PNG.
- Bad, because files in other formats (HEIC, PDF, Office, archives) keep their metadata. The limit
  of the promise is stated in 12, section 7.1; there the apps convert photos from the photo
  library to JPEG before the upload, so that HEIC arrives only as a deliberately sent file.
- Neutral, because a damaged JPEG or PNG is now rejected instead of ending up as a download.

### Confirmation

- Test "JPEG with GPS EXIF, XMP, IPTC, `COM`, `APP2` with Multi-Picture Format, appended image and
  orientation 6": the stored original has exactly one `APP1` with only the tag "Orientation" = 6,
  `APP2` only with `ICC_PROFILE` (profile identical byte for byte), no other `APPn`, no `COM`, no
  byte after the end of image; the scan data is identical to the input byte for byte.
- Test "PNG with `tEXt`, `iTXt`, `zTXt`, `eXIf`, `tIME` and an unknown ancillary chunk": the
  result contains only chunks of the allowlist; `IDAT` is identical byte for byte; the decoded
  pixels are identical.
- Fuzz test (`go test -fuzz`) of the filter: for arbitrary inserted segments and chunks, the
  output contains only entries of the allowlist, or the upload fails.
- Test "truncated JPEG": `upload_state = 'failed'`, no object in storage.
- Test of the preview: no EXIF, XMP or text data in any variant, orientation applied in the pixels,
  color profile identical to the original.
- SEC-T08 from [19-security.md](../spec/19-security.md) and the architecture rule "no cgo in
  the standard build"; libvips only in a separate build with `file.image.libvips_enabled`.
- Build ticket M3-06 in [PLAN.md](../PLAN.md).

## Pros and Cons of the Options

### Lossless filter with an allowlist

- Good, because `APPn` segments and `COM` are reserved for the application and are not needed for
  decoding [Q1, verified]; removing them does not change the image data [derived].
- Good, because PNG keeps text and Exif in their own chunks [Q2, verified] and they can therefore
  be separated cleanly.
- Bad, because two parsers (JPEG segments, PNG chunks) must be built and hardened against
  fuzzing.

### Re-encoding every inline image

- Good, because one path for all formats and nothing but pixels remains.
- Bad, because every JPEG original loses quality and the color profile and orientation must be
  rebuilt by hand; pure Go decodes without either [Q5, derived].
- Bad, because L16 requires the original in full quality.

### Denylist of known fields

- Good, because the original stays almost unchanged.
- Bad, because every new kind of metadata (vendor fields, embedded thumbnails with their
  own EXIF, appended images) slips through unnoticed. That is the wrong failure class (R-SE-03).

### Removal in the client

- Good, because the data does not leave the device in the first place.
- Bad, because agents, imports and third-party clients upload as well; the server would have to
  check anyway. A second implementation per platform would only bring duplicate code.

### External tool for all formats

- Good, because HEIC, AVIF and TIFF would be covered as well.
- Bad, because libvips brings cgo and LGPL-2.1 [Q4, verified] and exiftool another runtime;
  both stay optional (`file.image.libvips_enabled`), not the default.

## More Information

- Sources (retrieved 2026-10-10): Q1 ITU-T T.81, Annex B,
  https://www.w3.org/Graphics/JPEG/itu-t81.pdf [verified]; Q2 PNG Third Edition,
  https://www.w3.org/TR/png-3/ [verified]; Q3 Ultra HDR Image Format v1.1,
  https://developer.android.com/media/platform/hdr-image-format [verified]; Q4 govips,
  https://github.com/davidbyttow/govips [verified]; Q5 Go `image/jpeg`,
  https://pkg.go.dev/image/jpeg [verified]; identifier `ICC_PROFILE` in `APP2`:
  https://www.color.org/profile_embedding.html and libjpeg-turbo `jcicc.c`,
  https://skia.googlesource.com/external/github.com/libjpeg-turbo/libjpeg-turbo/+/HEAD/jcicc.c
  [verified].
- Extension to WebP: the same pattern would work with a filter of the RIFF chunks `EXIF` and
  `XMP `. That is a separate decision, because L16 limits the filter to JPEG and PNG; until then
  the conversion from 12 applies.
- Related: ADR 0010 (object storage), ADR 0033 (malware scan), ADR 0048 (download).
