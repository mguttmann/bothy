// SPDX-License-Identifier: Apache-2.0

/// The duration of an MP4 file as the server reads it (12 8.1; brief M5-07 Design note 1): the
/// `mvhd` duration (ISO/IEC 14496-12, 8.2.2), else the `mehd` duration of a fragmented movie
/// (8.8.2), in whole milliseconds rounded down, the same rule as the server's box reader
/// (`files/adapters/isobmff`). The app measures a recording this way before it sends it, so a
/// voice message it sends never exceeds `voice.max_duration`.
library;

import 'dart:io';
import 'dart:typed_data';

/// The duration of the MP4 file at [path], or null when the file has no readable duration
/// (not an MP4, cut off, a time scale of zero, or a duration of zero or all ones, which the
/// format uses for "unknown").
///
/// Reads only box headers and the small `mvhd` and `mehd` boxes, never the media data, so it
/// is cheap for a long recording. Throws a [FileSystemException] when the file cannot be
/// opened.
///
/// Example: `final d = await mp4Duration(recording.path); if (d != null && d > limit) ...`
Future<Duration?> mp4Duration(String path) async {
  final file = await File(path).open();
  try {
    final moov = await _findBox(file, 0, await file.length(), 'moov');
    if (moov == null) return null;
    final mvhd = await _findBox(file, moov.$1, moov.$2, 'mvhd');
    if (mvhd == null) return null;
    final header = await _read(file, mvhd);
    final movie = _movieHeader(header);
    if (movie == null) return null;
    final (timescale, duration) = movie;
    if (duration != null) return _millis(duration, timescale);
    final mvex = await _findBox(file, moov.$1, moov.$2, 'mvex');
    if (mvex == null) return null;
    final mehd = await _findBox(file, mvex.$1, mvex.$2, 'mehd');
    if (mehd == null) return null;
    final fragment = _extendsDuration(await _read(file, mehd));
    return fragment == null ? null : _millis(fragment, timescale);
  } finally {
    await file.close();
  }
}

/// The payload range (start, end) of the first box of [type] between [from] and [to].
Future<(int, int)?> _findBox(
  RandomAccessFile file,
  int from,
  int to,
  String type,
) async {
  var at = from;
  while (at + 8 <= to) {
    await file.setPosition(at);
    final head = await file.read(16);
    if (head.length < 8) return null;
    final view = ByteData.sublistView(head);
    var size = view.getUint32(0);
    var header = 8;
    if (size == 1) {
      // A 64-bit size follows the type.
      if (head.length < 16) return null;
      size = view.getUint64(8);
      header = 16;
    } else if (size == 0) {
      // The box runs to the end of its parent.
      size = to - at;
    }
    if (size < header || at + size > to) return null;
    if (String.fromCharCodes(head.sublist(4, 8)) == type) {
      return (at + header, at + size);
    }
    at += size;
  }
  return null;
}

/// The first bytes of the payload [box], enough for the fields of `mvhd` and `mehd`.
Future<Uint8List> _read(RandomAccessFile file, (int, int) box) async {
  await file.setPosition(box.$1);
  final length = box.$2 - box.$1;
  return file.read(length < _fieldBytes ? length : _fieldBytes);
}

/// Version 1 of `mvhd` ends its duration at byte 32 of the payload.
const int _fieldBytes = 32;

const int _allOnes32 = 0xFFFFFFFF;

/// The time scale and the duration (null when unknown) of an `mvhd` payload, or null when it
/// is cut off or has an unknown version.
(int, int?)? _movieHeader(Uint8List b) {
  if (b.isEmpty) return null;
  final view = ByteData.sublistView(b);
  final int timescale;
  final int duration;
  final bool unknown;
  switch (b[0]) {
    case 0 when b.length >= 20:
      // version and flags (4), creation (4), modification (4), time scale (4), duration (4)
      timescale = view.getUint32(12);
      duration = view.getUint32(16);
      unknown = duration == _allOnes32;
    case 1 when b.length >= 32:
      // version and flags (4), creation (8), modification (8), time scale (4), duration (8)
      timescale = view.getUint32(20);
      duration = view.getUint64(24);
      // All 64 bits set reads as -1 in a Dart int.
      unknown = duration == -1;
    default:
      return null;
  }
  if (timescale == 0) return null;
  return (timescale, duration == 0 || unknown ? null : duration);
}

/// The fragment duration of a `mehd` payload, or null when unknown or cut off.
int? _extendsDuration(Uint8List b) {
  if (b.isEmpty) return null;
  final view = ByteData.sublistView(b);
  switch (b[0]) {
    case 0 when b.length >= 8:
      final d = view.getUint32(4);
      return d == 0 || d == _allOnes32 ? null : d;
    case 1 when b.length >= 12:
      final d = view.getUint64(4);
      return d == 0 || d == -1 ? null : d;
    default:
      return null;
  }
}

/// [duration] time scale units in whole milliseconds, rounded down like the server; null when
/// a negative 64-bit value (above the range of a Dart int) was read.
Duration? _millis(int duration, int timescale) {
  if (duration < 0) return null;
  // Whole seconds first, so a large duration cannot overflow the multiplication.
  final ms =
      duration ~/ timescale * Duration.millisecondsPerSecond +
      duration % timescale * Duration.millisecondsPerSecond ~/ timescale;
  return Duration(milliseconds: ms);
}
