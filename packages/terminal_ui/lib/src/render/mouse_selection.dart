// SPDX-License-Identifier: Apache-2.0

/// The mouse selection of the terminal screen on the desktop (ticket
/// FIX-drag-select-autoscroll): a drag held near or beyond the top or bottom edge scrolls the
/// scrollback, and shift with a click extends a selection.
///
/// xterm 4.0.0 selects from the local position where a drag began to the pointer
/// (`TerminalGestureHandler.onDragUpdate` maps both through the current scroll offset), so
/// its selection loses its start as soon as the view scrolls, and it never scrolls while a
/// drag is held. This class keeps the start as a cell of the buffer, scrolls while the pointer
/// is held at an edge and puts its own range back whenever xterm's handler replaces it during
/// the drag.
library;

import 'dart:async';
import 'dart:math' as math;

import 'package:flutter/gestures.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter/services.dart';
import 'package:flutter/widgets.dart';
import 'package:xterm/xterm.dart' as xterm show TerminalController;
import 'package:xterm/xterm.dart' show CellOffset, Terminal, TerminalViewState;

/// The speed of the scroll at the edge, in lines per second: [edgeLinesPerSecond] with the
/// pointer at the edge, growing by [linesPerSecondPerLine] for every line height the pointer
/// is beyond it, at most [maxLinesPerSecond].
abstract final class TerminalAutoScroll {
  /// Lines per second with the pointer at the edge or just inside it.
  static const double edgeLinesPerSecond = 6;

  /// Additional lines per second for every line height beyond the edge.
  static const double linesPerSecondPerLine = 12;

  /// The upper bound.
  static const double maxLinesPerSecond = 240;

  /// The speed in pixels per second with the pointer [beyond] pixels past the edge (zero or
  /// less inside the edge zone) and lines [lineHeight] high.
  static double pixelsPerSecond(double beyond, double lineHeight) {
    final lines = math.max(0, beyond) / lineHeight;
    final speed = math.min(
      edgeLinesPerSecond + lines * linesPerSecondPerLine,
      maxLinesPerSecond,
    );
    return speed * lineHeight;
  }
}

/// The mouse selection of one terminal screen; see the library comment.
final class TerminalMouseSelection {
  /// Creates the selection of the view behind [view] with [controller] and [scroll];
  /// [terminal] is the terminal shown now, [vsync] drives the scroll at the edge.
  TerminalMouseSelection({
    required this.view,
    required this.controller,
    required this.scroll,
    required this.terminal,
    required TickerProvider vsync,
  }) {
    _ticker = vsync.createTicker(_tick);
    controller.addListener(_keep);
  }

  /// The xterm view.
  final GlobalKey<TerminalViewState> view;

  /// The selection of the view.
  final xterm.TerminalController controller;

  /// The scroll position of the view.
  final ScrollController scroll;

  /// The terminal shown now.
  final Terminal Function() terminal;

  late final Ticker _ticker;
  Duration _lastTick = Duration.zero;

  /// Where the selection started, in cells of the buffer, while the button is down.
  CellOffset? _anchor;

  /// The pointer of the held button, in the view's coordinates.
  Offset? _pointer;
  Offset? _down;

  /// Whether this class owns the selection: the pointer moved past the slop, or shift and a
  /// click extended a selection.
  bool _owning = false;

  /// The range this class set last; xterm's handler replacing it is undone while owning.
  (CellOffset, CellOffset)? _wanted;
  bool _applying = false;

  /// After shift and a click, the range is kept until xterm's tap handling has run.
  Timer? _settle;

  /// Hears a pointer down on the screen.
  void down(PointerDownEvent event) {
    _release();
    if (event.kind != PointerDeviceKind.mouse ||
        event.buttons != kPrimaryMouseButton) {
      return;
    }
    final state = _state;
    if (state == null) return;
    final cell = state.renderTerminal.getCellOffset(event.localPosition);
    final range = controller.selection?.normalized;
    _down = event.localPosition;
    _pointer = event.localPosition;
    if (HardwareKeyboard.instance.isShiftPressed && range != null) {
      // Extends from the far end of the selection, as a text view does.
      _anchor = _before(cell, range.begin) ? range.end : range.begin;
      _owning = true;
      _apply();
    } else {
      _anchor = cell;
    }
  }

  /// Hears a pointer move on the screen.
  void move(PointerMoveEvent event) {
    final down = _down;
    if (_anchor == null || down == null) return;
    if (event.buttons != kPrimaryMouseButton) {
      up();
      return;
    }
    _pointer = event.localPosition;
    if (!_owning && (event.localPosition - down).distance <= kTouchSlop) {
      return;
    }
    _owning = true;
    _apply();
    if (_beyond() != null) {
      if (!_ticker.isActive) {
        _lastTick = Duration.zero;
        unawaited(_ticker.start());
      }
    } else if (_ticker.isActive) {
      _ticker.stop();
    }
  }

  /// Hears the end of the pointer.
  void up() {
    if (_ticker.isActive) _ticker.stop();
    final wanted = _wanted;
    final owned = _owning;
    _anchor = null;
    _pointer = null;
    _down = null;
    if (!owned || wanted == null) {
      _release();
      return;
    }
    // xterm's tap handling (a tap down clears the selection) may still run for this click,
    // at once or after the double tap timeout; the range stays until then.
    _settle?.cancel();
    _settle = Timer(kDoubleTapTimeout + const Duration(milliseconds: 50), () {
      _settle = null;
      _release();
    });
  }

  /// Stops the scroll and listening.
  void dispose() {
    _settle?.cancel();
    controller.removeListener(_keep);
    _ticker.dispose();
  }

  void _release() {
    _settle?.cancel();
    _settle = null;
    _owning = false;
    _wanted = null;
  }

  /// The view, once its render object is laid out.
  TerminalViewState? get _state {
    final state = view.currentState;
    if (state == null || state.context.findRenderObject() == null) return null;
    return state.renderTerminal.hasSize ? state : null;
  }

  /// How far the pointer is beyond the edge zone (one line inside each edge): negative for
  /// the top, positive for the bottom, null inside.
  double? _beyond() {
    final state = _state;
    final pointer = _pointer;
    if (state == null || pointer == null) return null;
    final render = state.renderTerminal;
    final height = render.size.height;
    final line = render.lineHeight;
    if (pointer.dy < line) return -(line - pointer.dy);
    if (pointer.dy > height - line) return pointer.dy - (height - line);
    return null;
  }

  void _tick(Duration elapsed) {
    final dt = (elapsed - _lastTick).inMicroseconds / 1e6;
    _lastTick = elapsed;
    final beyond = _beyond();
    final state = _state;
    if (beyond == null || state == null || !scroll.hasClients) {
      _ticker.stop();
      return;
    }
    final line = state.renderTerminal.lineHeight;
    final position = scroll.position;
    // Past the edge zone of one line, measured from the edge itself.
    final distance = beyond.abs() - line;
    final step = TerminalAutoScroll.pixelsPerSecond(distance, line) * dt;
    final target = (position.pixels + (beyond < 0 ? -step : step)).clamp(
      position.minScrollExtent,
      position.maxScrollExtent,
    );
    if (target != position.pixels) position.jumpTo(target);
    _apply();
  }

  /// Selects from the anchor to the cell under the pointer, the pointer held inside the
  /// screen, so the line at the edge joins as the view scrolls.
  void _apply() {
    final state = _state;
    final anchor = _anchor;
    final pointer = _pointer;
    if (state == null || anchor == null || pointer == null) return;
    final render = state.renderTerminal;
    final size = render.size;
    final inside = Offset(
      pointer.dx.clamp(0, math.max(0, size.width - 1)).toDouble(),
      pointer.dy.clamp(0, math.max(0, size.height - 1)).toDouble(),
    );
    var extent = render.getCellOffset(inside);
    // The cell under the pointer joins a selection that runs forward.
    if (!_before(extent, anchor)) extent = CellOffset(extent.x + 1, extent.y);
    _set((anchor, extent));
  }

  void _set((CellOffset, CellOffset) range) {
    final buffer = terminal().buffer;
    final last = buffer.lines.length - 1;
    CellOffset fit(CellOffset c) => CellOffset(
      c.x.clamp(0, buffer.viewWidth),
      c.y.clamp(0, math.max(0, last)),
    );
    final (base, extent) = (fit(range.$1), fit(range.$2));
    _wanted = (base, extent);
    _applying = true;
    try {
      controller.setSelection(
        buffer.createAnchorFromOffset(base),
        buffer.createAnchorFromOffset(extent),
      );
    } finally {
      _applying = false;
    }
  }

  /// Puts the range back when xterm's gesture handler replaced it while this class owns the
  /// selection.
  void _keep() {
    final wanted = _wanted;
    if (_applying || !_owning || wanted == null) return;
    final now = controller.selection?.normalized;
    final (begin, end) = _before(wanted.$2, wanted.$1)
        ? (wanted.$2, wanted.$1)
        : wanted;
    if (now != null && now.begin == begin && now.end == end) return;
    _set(wanted);
  }

  static bool _before(CellOffset a, CellOffset b) =>
      a.y < b.y || (a.y == b.y && a.x < b.x);
}
