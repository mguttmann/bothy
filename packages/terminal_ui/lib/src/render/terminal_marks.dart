// SPDX-License-Identifier: Apache-2.0

/// The cursor and the selection of the terminal screen, painted by the view itself
/// (docs/design/components.md 12, docs/decisions.d/M4-35.md D-M4-35-18).
///
/// `xterm` 4.0.0 paints both as overlays after the text: the block cursor covers the character
/// and has no colour for it, the underline and the bar ignore the row of the cursor, and the
/// selection is a filled rectangle over the text. The theme of the view therefore gives xterm
/// a transparent cursor and selection, and [TerminalMarksPainter] paints every marked cell
/// opaque with the pairing of the tokens: the cursor cell in `color.terminal.cursor` with its
/// character in `color.terminal.cursor-text`, a selected cell in `color.terminal.selection-bg`
/// with its character in `color.terminal.fg`. Without the focus the cursor is a frame in
/// `color.terminal.cursor` around the unchanged cell.
library;

import 'dart:ui' as ui;

import 'package:flutter/rendering.dart';
import 'package:xterm/xterm.dart'
    show
        BufferRange,
        CellContent,
        CellData,
        CellFlags,
        CellOffset,
        Terminal,
        TerminalStyle;

/// The geometry of the renderer the marks follow: xterm's `RenderTerminal`.
abstract interface class TerminalGeometry {
  /// The size of one cell in logical pixels.
  Size get cellSize;

  /// The top left corner of [cell] (absolute buffer row) in the view, scroll included.
  Offset offsetOf(CellOffset cell);
}

/// The colours of the marks, from the tokens `color.terminal.*`.
final class TerminalMarkColors {
  /// Creates the colours.
  const TerminalMarkColors({
    required this.cursor,
    required this.cursorText,
    required this.selection,
    required this.selectionText,
  });

  /// `color.terminal.cursor`.
  final Color cursor;

  /// `color.terminal.cursor-text`.
  final Color cursorText;

  /// `color.terminal.selection-bg`.
  final Color selection;

  /// `color.terminal.fg`, the text on the selection.
  final Color selectionText;
}

/// Paints the cursor and the selection of [terminal] over xterm's view.
///
/// [geometry] returns null while the renderer has no size; [selection] is the selection of
/// the view, [focused] whether the screen has the focus, [devicePixelRatio] snaps the filled
/// cells to whole device pixels so no edge is painted at partial coverage.
final class TerminalMarksPainter extends CustomPainter {
  /// Creates the painter; [repaint] fires when the terminal, the selection, the focus or the
  /// scroll position change.
  TerminalMarksPainter({
    required this.terminal,
    required this.geometry,
    required this.selection,
    required this.focused,
    required this.colors,
    required this.textStyle,
    required this.textScaler,
    required this.devicePixelRatio,
    super.repaint,
  });

  /// The terminal model.
  final Terminal terminal;

  /// The renderer's geometry, null before its first layout.
  final TerminalGeometry? Function() geometry;

  /// The current selection, null without one.
  final BufferRange? Function() selection;

  /// Whether the screen has the focus.
  final bool Function() focused;

  /// The token colours.
  final TerminalMarkColors colors;

  /// The text style of the view.
  final TerminalStyle textStyle;

  /// The text scaler of the view.
  final TextScaler textScaler;

  /// The device pixel ratio of the view.
  final double devicePixelRatio;

  @override
  void paint(Canvas canvas, Size size) {
    final g = geometry();
    if (g == null) return;
    canvas.save();
    canvas.clipRect(Offset.zero & size);
    final buffer = terminal.buffer;
    final lines = buffer.lines;
    final cell = CellData.empty();
    final range = selection()?.normalized;
    if (range != null) {
      for (final segment in range.toSegments()) {
        if (segment.line < 0 || segment.line >= lines.length) continue;
        // Only the visible rows, as xterm paints (a selection over the whole scrollback
        // would lay out every selected cell on each frame, red team round 3).
        final top = g.offsetOf(CellOffset(0, segment.line)).dy;
        if (top + g.cellSize.height <= 0 || top >= size.height) continue;
        final start = segment.start ?? 0;
        final end = segment.end ?? terminal.viewWidth;
        final line = lines[segment.line];
        for (var x = start; x < end && x < line.length; x++) {
          line.getCellData(x, cell);
          _paintCell(
            canvas,
            g,
            CellOffset(x, segment.line),
            cell,
            colors.selection,
            colors.selectionText,
          );
          if (cell.content >> CellContent.widthShift == 2) x++;
        }
      }
    }
    final y = buffer.absoluteCursorY;
    if (terminal.cursorVisibleMode && y >= 0 && y < lines.length) {
      final x = buffer.cursorX;
      lines[y].getCellData(x, cell);
      if (focused()) {
        _paintCell(
          canvas,
          g,
          CellOffset(x, y),
          cell,
          colors.cursor,
          colors.cursorText,
        );
      } else {
        _paintFrame(canvas, _cellRect(g, CellOffset(x, y), cell));
      }
    }
    canvas.restore();
  }

  Rect _cellRect(TerminalGeometry g, CellOffset at, CellData cell) {
    final o = g.offsetOf(at);
    final wide = cell.content >> CellContent.widthShift == 2;
    // xterm draws each line at a whole logical pixel (`truncateToDouble`).
    return Rect.fromLTWH(
      o.dx,
      o.dy.truncateToDouble(),
      g.cellSize.width * (wide ? 2 : 1),
      g.cellSize.height,
    );
  }

  // Outward to whole device pixels, so the fill covers xterm's glyph completely.
  Rect _snap(Rect r) {
    final d = devicePixelRatio;
    return Rect.fromLTRB(
      (r.left * d).floorToDouble() / d,
      (r.top * d).floorToDouble() / d,
      (r.right * d).ceilToDouble() / d,
      (r.bottom * d).ceilToDouble() / d,
    );
  }

  void _paintCell(
    Canvas canvas,
    TerminalGeometry g,
    CellOffset at,
    CellData cell,
    Color fill,
    Color text,
  ) {
    final rect = _cellRect(g, at, cell);
    canvas.drawRect(_snap(rect), Paint()..color = fill);
    final code = cell.content & CellContent.codepointMask;
    if (code == 0) return;
    final flags = cell.flags;
    final style = textStyle.toTextStyle(
      color: text,
      bold: flags & CellFlags.bold != 0,
      italic: flags & CellFlags.italic != 0,
      underline: flags & CellFlags.underline != 0,
    );
    final builder = ui.ParagraphBuilder(style.getParagraphStyle())
      ..pushStyle(style.getTextStyle(textScaler: textScaler))
      ..addText(String.fromCharCode(code));
    final paragraph = builder.build()
      ..layout(const ui.ParagraphConstraints(width: double.infinity));
    canvas.drawParagraph(paragraph, rect.topLeft);
    paragraph.dispose();
  }

  // A frame inside the cell, at least one device pixel at full coverage.
  void _paintFrame(Canvas canvas, Rect cell) {
    final r = _snap(cell);
    // One logical pixel is at least one whole device pixel.
    const w = 1.0;
    final paint = Paint()..color = colors.cursor;
    canvas
      ..drawRect(Rect.fromLTWH(r.left, r.top, r.width, w), paint)
      ..drawRect(Rect.fromLTWH(r.left, r.bottom - w, r.width, w), paint)
      ..drawRect(Rect.fromLTWH(r.left, r.top, w, r.height), paint)
      ..drawRect(Rect.fromLTWH(r.right - w, r.top, w, r.height), paint);
  }

  @override
  bool shouldRepaint(TerminalMarksPainter oldDelegate) =>
      oldDelegate.terminal != terminal ||
      oldDelegate.colors != colors ||
      oldDelegate.textStyle != textStyle ||
      oldDelegate.textScaler != textScaler ||
      oldDelegate.devicePixelRatio != devicePixelRatio;
}
