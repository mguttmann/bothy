// SPDX-License-Identifier: Apache-2.0

/// The screen of one tab: the terminal model of `xterm` behind the escape filter, the
/// hyperlink records and the replay swap (docs/spec/15-client-architektur.md 23.2 item 6, 23.4).
///
/// Everything here lives only in memory (R-CL-17, SEC-TERM-1): no log, no cache, no metric.
library;

import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:terminal_ui/src/render/escape_filter.dart';
import 'package:xterm/xterm.dart'
    show BufferRangeLine, CellAnchor, CellOffset, Terminal;

/// The most hyperlink records one screen keeps; older ones are dropped first, so a program
/// that prints links forever cannot grow the memory of the view.
const int maxLinkRecords = 512;

/// Whether a link [target] may open at all: an absolute `https` or `http` URI with a host
/// (docs/spec/24-terminal.md 11.2); every other scheme stays closed.
bool isOpenableLink(String target) {
  if (target.codeUnits.any((c) => c < 0x21 || c > 0x7e)) return false;
  final uri = Uri.tryParse(target);
  return uri != null &&
      uri.hasAuthority &&
      uri.host.isNotEmpty &&
      (uri.isScheme('https') || uri.isScheme('http'));
}

/// Returns [text] without ESC, DEL and the C0 and C1 controls other than tab, line feed and
/// carriage return; what remains is what the person can see in the paste confirmation.
String sanitizePaste(String text) => String.fromCharCodes(
  text.runes.where(
    (r) =>
        r == 0x09 ||
        r == 0x0a ||
        r == 0x0d ||
        (r >= 0x20 && r != 0x7f && !(r >= 0x80 && r <= 0x9f)),
  ),
);

/// A hyperlink of OSC 8, rewritten into inert text and this record (Design note 2).
final class TerminalLink {
  TerminalLink._(this.target, this._start);

  /// The full target, shown on hover and in the confirmation.
  final String target;

  final CellAnchor _start;
  CellAnchor? _end;

  void _release() {
    _start.dispose();
    _end?.dispose();
  }

  /// Whether the cell [offset] lies inside the visible text of the link.
  bool contains(CellOffset offset) {
    final end = _end;
    if (end == null || !_start.attached || !end.attached) return false;
    final s = _start.offset;
    final e = end.offset;
    if (offset.y < s.y || offset.y > e.y) return false;
    if (offset.y == s.y && offset.x < s.x) return false;
    if (offset.y == e.y && offset.x >= e.x) return false;
    return true;
  }
}

/// The terminal model of one tab with the escape filter in front of it.
///
/// [onInput] receives what the renderer sends to the host: the keys of the person and the
/// answers the filter lets through (cursor position and device attributes).
final class TerminalScreen extends ChangeNotifier {
  /// Creates an empty screen with [scrollbackLines] lines of scrollback
  /// (`client.terminal.scrollback_lines`).
  TerminalScreen({required this._scrollbackLines, required this._onInput}) {
    _terminal = _newTerminal();
  }

  final int _scrollbackLines;
  final void Function(String data) _onInput;
  final EscapeFilter _filter = EscapeFilter();
  late Terminal _terminal;
  final List<TerminalLink> _links = [];
  TerminalLink? _openLink;
  ByteConversionSink? _decoder;
  final StringBuffer _decoded = StringBuffer();

  /// Receives the size in cells whenever the view resizes the terminal.
  void Function(int cols, int rows)? onResize;

  /// The current terminal model; a replay replaces it ([replaceWith]).
  Terminal get terminal => _terminal;

  /// The hyperlink records of the current screen, oldest first.
  List<TerminalLink> get links => List.unmodifiable(_links);

  /// The link whose text covers [offset], or null.
  TerminalLink? linkAt(CellOffset offset) {
    for (final link in _links.reversed) {
      if (link.contains(offset)) return link;
    }
    return null;
  }

  /// Writes output of the host through the filter.
  void write(Uint8List bytes) {
    _writeTo(_terminal, bytes);
    notifyListeners();
  }

  /// Replaces the whole screen with [replay], the scrollback the host played between
  /// `replay_begin` and `replay_end`, in a fresh model (15 23.2 item 6).
  void replaceWith(Uint8List replay) {
    final size = (_terminal.viewWidth, _terminal.viewHeight);
    _filter.reset();
    _decoder = null;
    _decoded.clear();
    for (final link in _links) {
      link._release();
    }
    _links.clear();
    _openLink = null;
    _terminal = _newTerminal()..resize(size.$1, size.$2);
    _writeTo(_terminal, replay);
    notifyListeners();
  }

  /// The text of the line at the cursor, for screen readers (01 8).
  String cursorLine() {
    final buffer = _terminal.buffer;
    return buffer.lines[buffer.absoluteCursorY].getText().trimRight();
  }

  /// The visible lines, for "Read screen" (01 8).
  String visibleText() {
    final buffer = _terminal.buffer;
    final top = buffer.height - _terminal.viewHeight;
    final lines = <String>[
      for (var y = top < 0 ? 0 : top; y < buffer.height; y++)
        buffer.lines[y].getText().trimRight(),
    ];
    while (lines.isNotEmpty && lines.last.isEmpty) {
      lines.removeLast();
    }
    return lines.join('\n');
  }

  /// The text of [range] for copying by the person.
  String textOf(BufferRangeLine range) => _terminal.buffer.getText(range);

  /// xterm 4.0.0 answers the cursor position report zero-based (`ESC[0;0R` at the home
  /// position); the host expects rows and columns from 1 (D-M4-35-5).
  static String _fixCursorReport(String data) {
    final m = _cursorReport.firstMatch(data);
    if (m == null) return data;
    final row = int.parse(m.group(1)!) + 1;
    final col = int.parse(m.group(2)!) + 1;
    return '\x1b[$row;${col}R';
  }

  static final RegExp _cursorReport = RegExp(r'^\x1b\[(\d{1,5});(\d{1,5})R$');

  /// Pastes [text] of the person through the renderer (bracketed when the program asked for
  /// it), without ESC and the other C0 controls except tab, line feed and carriage return:
  /// they could end the bracketed paste early or run a line without the confirmation of
  /// 15 23.4 (red team M4-35).
  void paste(String text) => _terminal.paste(sanitizePaste(text));

  Terminal _newTerminal() => Terminal(
    maxLines: _scrollbackLines,
    // The second layer of 24 11.2: title, icon and private OSC handlers stay unset, so even
    // a sequence the filter missed changes nothing and answers nothing; xterm has no
    // clipboard handler at all.
    onOutput: (data) => _onInput(_fixCursorReport(data)),
    onResize: (cols, rows, _, _) => onResize?.call(cols, rows),
  );

  void _writeTo(Terminal terminal, Uint8List bytes) {
    for (final chunk in _filter.process(bytes)) {
      switch (chunk) {
        case FilteredText(:final bytes):
          _decode(bytes);
        case LinkStart(:final target):
          _flush(terminal);
          _closeLink(terminal);
          final buffer = terminal.buffer;
          final link = TerminalLink._(
            target,
            buffer.createAnchor(buffer.cursorX, buffer.absoluteCursorY),
          );
          _openLink = link;
          _links.add(link);
          // A dropped record releases its anchors; xterm keeps every anchor on its line and
          // walks them on each edit (red team M4-35: the view froze otherwise).
          if (_links.length > maxLinkRecords) _links.removeAt(0)._release();
        case LinkEnd():
          _flush(terminal);
          _closeLink(terminal);
      }
    }
    _flush(terminal);
  }

  void _closeLink(Terminal terminal) {
    final link = _openLink;
    if (link == null) return;
    final buffer = terminal.buffer;
    link._end = buffer.createAnchor(buffer.cursorX, buffer.absoluteCursorY);
    _openLink = null;
  }

  void _decode(Uint8List bytes) {
    // UTF-8 characters may be split across frames; the chunked decoder keeps the rest.
    final decoder = _decoder ??= const Utf8Decoder(allowMalformed: true)
        .startChunkedConversion(StringConversionSink.fromStringSink(_decoded));
    decoder.add(bytes);
  }

  void _flush(Terminal terminal) {
    if (_decoded.isEmpty) return;
    final text = _decoded.toString();
    _decoded.clear();
    try {
      terminal.write(text);
    } on Object {
      // A renderer bug on hostile output (xterm 4.0.0 throws on some sequences the filter
      // passes) must not end the session as tampering; the rest of this chunk is dropped and
      // the error, which may quote output, goes nowhere (R-CL-17).
    }
  }
}
