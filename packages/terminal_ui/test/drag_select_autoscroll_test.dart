// SPDX-License-Identifier: Apache-2.0

// A mouse selection in the terminal scrolls the scrollback (ticket FIX-drag-select-autoscroll):
// on macOS and Windows a drag held near or beyond the top or bottom edge of the screen scrolls
// it continuously, faster the farther the pointer is out, the selection grows over the lines
// that come into view and keeps its start where the drag began, shift and a click extend a
// selection to a line reached with the scroll wheel, and the copy is plain text with a line
// break between the lines.

import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';
import 'package:xterm/xterm.dart' as xterm;

import 'support/fakes.dart';
import 'support/widgets.dart';

const _lines = 300;

const _desktop = TargetPlatformVariant({
  TargetPlatform.macOS,
  TargetPlatform.windows,
});

Future<Harness> _open(WidgetTester tester) async {
  final h = (await tester.runAsync(Harness.create))!;
  await pumpApp(
    tester,
    Padding(
      // Room above and below the screen, where the pointer goes beyond its edges.
      padding: const EdgeInsets.symmetric(vertical: 120),
      child: TerminalScreenView(controller: h.controller, fontSize: 14),
    ),
  );
  await tester.pump();
  h.controller.screen.write(
    Uint8List.fromList(
      utf8.encode(List.generate(_lines, (i) => 'line $i').join('\r\n')),
    ),
  );
  await tester.pump();
  await tester.pump();
  return h;
}

xterm.TerminalViewState _view(WidgetTester tester) =>
    tester.state<xterm.TerminalViewState>(find.byType(xterm.TerminalView));

ScrollPosition _scroll(WidgetTester tester) => tester
    .state<ScrollableState>(
      find.descendant(
        of: find.byType(TerminalScreenView),
        matching: find.byType(Scrollable),
      ),
    )
    .position;

Rect _screen(WidgetTester tester) =>
    tester.getRect(find.byType(xterm.TerminalView));

/// The global center of the cell [x], [y] (a line of the whole buffer).
Offset _cell(WidgetTester tester, int x, int y) {
  final render = _view(tester).renderTerminal;
  final local =
      render.getOffset(xterm.CellOffset(x, y)) +
      Offset(render.cellSize.width / 2, render.cellSize.height / 2);
  return render.localToGlobal(local);
}

/// The first line of the buffer the screen shows.
int _topLine(WidgetTester tester) =>
    (_scroll(tester).pixels / _view(tester).renderTerminal.lineHeight).ceil();

List<String> _clipboard() {
  final writes = <String>[];
  TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
      .setMockMethodCallHandler(SystemChannels.platform, (call) async {
        if (call.method == 'Clipboard.setData') {
          writes.add((call.arguments as Map)['text'] as String);
        }
        return null;
      });
  addTearDown(
    () => TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(SystemChannels.platform, null),
  );
  return writes;
}

Future<void> _copy(WidgetTester tester) async {
  final apple = defaultTargetPlatform == TargetPlatform.macOS;
  final mod = apple ? LogicalKeyboardKey.meta : LogicalKeyboardKey.control;
  await tester.sendKeyDownEvent(mod);
  if (!apple) await tester.sendKeyDownEvent(LogicalKeyboardKey.shift);
  await tester.sendKeyEvent(LogicalKeyboardKey.keyC);
  if (!apple) await tester.sendKeyUpEvent(LogicalKeyboardKey.shift);
  await tester.sendKeyUpEvent(mod);
  await tester.pump();
}

/// Drags with the mouse from [from] to [to] and holds there for [frames] frames.
Future<TestGesture> _hold(
  WidgetTester tester,
  Offset from,
  Offset to, {
  int frames = 0,
}) async {
  final gesture = await tester.startGesture(
    from,
    kind: PointerDeviceKind.mouse,
  );
  await tester.pump();
  await gesture.moveTo(Offset.lerp(from, to, 0.2)!);
  await tester.pump();
  await gesture.moveTo(to);
  await tester.pump();
  for (var i = 0; i < frames; i++) {
    await tester.pump(const Duration(milliseconds: 16));
  }
  return gesture;
}

String _range(int first, int last) =>
    [for (var i = first; i <= last; i++) 'line $i'].join('\n');

Future<void> _toTop(WidgetTester tester) async {
  _scroll(tester).jumpTo(0);
  await tester.pump();
}

void main() {
  testWidgets('a drag held above the screen scrolls the scrollback up to '
      'its first line and copies every line in order', (tester) async {
    final copied = _clipboard();
    final h = await _open(tester);
    final start = _topLine(tester) + 2;
    expect(start, greaterThan(100));
    final gesture = await _hold(
      tester,
      _cell(tester, 4, start),
      Offset(_screen(tester).left + 2, _screen(tester).top - 40),
      frames: 600,
    );
    expect(_scroll(tester).pixels, 0);
    await gesture.up();
    await tester.pump();
    await _copy(tester);
    expect(copied, ['${_range(0, start - 1)}\nline']);
    await h.controller.detach();
  }, variant: _desktop);

  testWidgets('a drag held below the screen scrolls down to the last line, '
      'and the line the drag began in keeps its start', (tester) async {
    final copied = _clipboard();
    final h = await _open(tester);
    await _toTop(tester);
    final gesture = await _hold(
      tester,
      _cell(tester, 4, 2),
      Offset(_screen(tester).right + 40, _screen(tester).bottom + 40),
      frames: 600,
    );
    final position = _scroll(tester);
    expect(position.pixels, position.maxScrollExtent);
    await gesture.up();
    await tester.pump();
    await _copy(tester);
    expect(copied, [' 2\n${_range(3, _lines - 1)}']);
    await h.controller.detach();
  }, variant: _desktop);

  testWidgets('the farther the pointer is beyond the edge, the faster the '
      'scrollback scrolls', (tester) async {
    _clipboard();
    final h = await _open(tester);
    Future<double> scrolled(double beyond) async {
      final before = _scroll(tester).pixels;
      final gesture = await _hold(
        tester,
        _cell(tester, 4, _topLine(tester) + 3),
        Offset(_screen(tester).center.dx, _screen(tester).top - beyond),
        frames: 20,
      );
      final moved = before - _scroll(tester).pixels;
      await gesture.up();
      await tester.pump();
      return moved;
    }

    final slow = await scrolled(4);
    final fast = await scrolled(100);
    expect(slow, greaterThan(0));
    expect(fast, greaterThan(slow * 2));
    // Released, the scrollback stays where it is.
    final rest = _scroll(tester).pixels;
    await tester.pump(const Duration(milliseconds: 500));
    expect(_scroll(tester).pixels, rest);
    await h.controller.detach();
  }, variant: _desktop);

  testWidgets('shift and a click extend a selection to a line reached with '
      'the scroll wheel', (tester) async {
    final copied = _clipboard();
    final h = await _open(tester);
    await _toTop(tester);
    final gesture = await _hold(
      tester,
      _cell(tester, 4, 3),
      _cell(tester, 0, 4),
    );
    await gesture.up();
    await tester.pump();
    final wheel = TestPointer(9, PointerDeviceKind.mouse);
    tester.binding.handlePointerEvent(wheel.hover(_screen(tester).center));
    final position = _scroll(tester);
    for (
      var i = 0;
      i < 100 && position.pixels < position.maxScrollExtent;
      i++
    ) {
      tester.binding.handlePointerEvent(wheel.scroll(const Offset(0, 300)));
      await tester.pump();
    }
    expect(position.pixels, position.maxScrollExtent);
    await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
    await tester.tapAt(
      _cell(tester, 3, _lines - 1),
      kind: PointerDeviceKind.mouse,
    );
    await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
    await tester.pump(const Duration(milliseconds: 500));
    await _copy(tester);
    expect(copied, [' 3\n${_range(4, _lines - 2)}\nline']);
    await h.controller.detach();
  }, variant: _desktop);
}
