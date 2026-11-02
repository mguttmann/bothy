// SPDX-License-Identifier: Apache-2.0

// A mouse selection in the history scrolls the history and spans messages (01 6.15 item 1,
// R-PR-29; ticket FIX-drag-select-autoscroll). On macOS and Windows a drag that holds the
// pointer beyond the top or bottom edge of the history scrolls it continuously, the selection
// grows over the messages that come into view, including messages the list builds only then,
// and the copy (Cmd+C or Ctrl+C, "Copy" of the message menu) is the whole selected range in
// order, one message apart from the next by an empty line. Shift and a click extend a
// selection to a message reached with the scroll wheel.

import 'package:client/thread/message_tile.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';

const _count = 120;

String _line(int n) => 'line ${n.toString().padLeft(2, '0')} of the history';

List<String> _clipboard(WidgetTester tester) {
  final writes = <String>[];
  tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
    SystemChannels.platform,
    (call) async {
      if (call.method == 'Clipboard.setData') {
        writes.add((call.arguments as Map)['text'] as String);
      }
      return null;
    },
  );
  return writes;
}

Future<ColumnHost> _open(WidgetTester tester, String platform) async {
  final host = await ColumnHost.open(tester);
  await host.run(
    tester,
    () => seedThread(
      host.db,
      messages: [
        for (var i = 1; i <= _count; i++)
          msg(i, author: i.isEven ? sam : null, text: _line(i), minutes: i),
      ],
    ),
  );
  final phone = platform == 'ios' || platform == 'android';
  await pumpThread(
    tester,
    host,
    FakeThreadReads(host.db),
    platform: platform,
    density: phone ? DsDensityMode.touch : DsDensityMode.comfortable,
  );
  return host;
}

Finder _paragraph(String text) => find.byWidgetPredicate(
  (w) => w is RichText && w.text.toPlainText() == text,
);

/// The global position of the character [index] of the paragraph [text].
Offset _at(WidgetTester tester, String text, int index) {
  final paragraph = tester.renderObject<RenderParagraph>(_paragraph(text));
  final caret = paragraph.getOffsetForCaret(
    TextPosition(offset: index),
    Rect.zero,
  );
  return paragraph.localToGlobal(
    caret + Offset(0, paragraph.preferredLineHeight / 2),
  );
}

Rect _list(WidgetTester tester) =>
    tester.getRect(find.byKey(const ValueKey('history.list')));

ScrollPosition _position(WidgetTester tester) => tester
    .state<ScrollableState>(
      find.descendant(
        of: find.byKey(const ValueKey('history.list')),
        matching: find.byType(Scrollable),
      ),
    )
    .position;

/// Scrolls the history to its oldest message; the extent grows while rows are built.
Future<void> _toOldest(WidgetTester tester, ColumnHost host) async {
  final position = _position(tester);
  for (var i = 0; i < 20 && position.pixels < position.maxScrollExtent; i++) {
    position.jumpTo(position.maxScrollExtent);
    await host.settle(tester);
  }
  expect(_paragraph(_line(1)), findsOneWidget);
}

Future<void> _copyKeys(WidgetTester tester) async {
  final mod = defaultTargetPlatform == TargetPlatform.macOS
      ? LogicalKeyboardKey.meta
      : LogicalKeyboardKey.control;
  await tester.sendKeyDownEvent(mod);
  await tester.sendKeyEvent(LogicalKeyboardKey.keyC);
  await tester.sendKeyUpEvent(mod);
  await tester.pump();
}

/// Holds a mouse drag from [from] at [to] for [frames] frames, then releases it.
Future<void> _holdDrag(
  WidgetTester tester,
  Offset from,
  Offset to, {
  int frames = 900,
}) async {
  final gesture = await tester.startGesture(
    from,
    kind: PointerDeviceKind.mouse,
  );
  await tester.pump();
  // Two steps, as a hand moves: first inside the text, then out over the edge.
  await gesture.moveTo(Offset.lerp(from, to, 0.3)!);
  await tester.pump();
  await gesture.moveTo(to);
  for (var i = 0; i < frames; i++) {
    await tester.pump(const Duration(milliseconds: 16));
  }
  await gesture.up();
  await tester.pump();
}

/// The text of the messages [first] to [last], one message apart from the next by an empty
/// line.
String _range(int first, int last) =>
    [for (var i = first; i <= last; i++) _line(i)].join('\n\n');

void main() {
  for (final (name, target) in [
    ('macos', TargetPlatform.macOS),
    ('windows', TargetPlatform.windows),
  ]) {
    final variant = TargetPlatformVariant.only(target);

    testWidgets('$name: a drag held above the history scrolls it up and '
        'selects up to the oldest message', (tester) async {
      final copied = _clipboard(tester);
      final host = await _open(tester, name);
      // The oldest messages are not built at the start.
      expect(_paragraph(_line(1)), findsNothing);
      final start = _line(_count - 1);
      await _holdDrag(
        tester,
        _at(tester, start, 4),
        Offset(_list(tester).center.dx, _list(tester).top - 40),
      );
      // The history scrolled all the way up while the pointer was held there.
      final position = _position(tester);
      expect(position.pixels, position.maxScrollExtent);
      expect(_paragraph(_line(1)), findsOneWidget);
      await _copyKeys(tester);
      expect(copied, ['${_range(1, _count - 2)}\n\nline']);
      await host.dispose(tester);
    }, variant: variant);

    testWidgets('$name: a drag held below the history scrolls it down, and the '
        'message the drag started in still counts once it is far away', (
      tester,
    ) async {
      final copied = _clipboard(tester);
      final host = await _open(tester, name);
      await _toOldest(tester, host);
      expect(_paragraph(_line(_count)), findsNothing);
      await _holdDrag(
        tester,
        _at(tester, _line(2), 5),
        Offset(_list(tester).center.dx, _list(tester).bottom + 40),
      );
      expect(_position(tester).pixels, _position(tester).minScrollExtent);
      expect(_paragraph(_line(_count)), findsOneWidget);
      await _copyKeys(tester);
      expect(copied, ['${_line(2).substring(5)}\n\n${_range(3, _count)}']);

      // "Copy" in the message menu of a right click copies the same range.
      await tester.tapAt(
        _at(tester, _line(_count), 2),
        buttons: kSecondaryButton,
        kind: PointerDeviceKind.mouse,
      );
      await tester.pump(const Duration(milliseconds: 500));
      await host.settle(tester);
      final copy = find.byKey(const ValueKey('action.copySelection'));
      // A right click inside the selection keeps it.
      expect(copy, findsOneWidget);
      await tester.tap(copy);
      await tester.pump(const Duration(milliseconds: 500));
      await host.settle(tester);
      expect(copied.last, copied.first);
      await host.dispose(tester);
    }, variant: variant);

    testWidgets('$name: shift and a click extend a selection to a message '
        'reached with the scroll wheel', (tester) async {
      final copied = _clipboard(tester);
      final host = await _open(tester, name);
      final position = _position(tester);
      await _toOldest(tester, host);
      // A selection inside message 3 ("03 of").
      await _holdDrag(
        tester,
        _at(tester, _line(3), 5),
        _at(tester, _line(3), 10),
        frames: 2,
      );
      // Down to the newest message with the wheel, never with the selection.
      final wheel = TestPointer(9, PointerDeviceKind.mouse);
      tester.binding.handlePointerEvent(wheel.hover(_list(tester).center));
      for (var i = 0; i < 40 && position.pixels > 0; i++) {
        tester.binding.handlePointerEvent(wheel.scroll(const Offset(0, 120)));
        await tester.pump(const Duration(milliseconds: 50));
      }
      await host.settle(tester);
      expect(position.pixels, position.minScrollExtent);
      await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
      await tester.tapAt(
        _at(tester, _line(_count), 4),
        kind: PointerDeviceKind.mouse,
      );
      await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
      await tester.pump(const Duration(milliseconds: 500));
      await _copyKeys(tester);
      expect(copied, [
        '${_line(3).substring(5)}\n\n${_range(4, _count - 1)}\n\nline',
      ]);
      await host.dispose(tester);
    }, variant: variant);
  }

  for (final (name, target) in [
    ('ios', TargetPlatform.iOS),
    ('android', TargetPlatform.android),
  ]) {
    testWidgets('$name: the history keeps no selection of its own, so a long '
        'press stays the message menu', (tester) async {
      _clipboard(tester);
      final host = await _open(tester, name);
      expect(find.byType(SelectionArea), findsNothing);
      await tester.longPress(_paragraph(_line(_count)));
      await tester.pump(const Duration(milliseconds: 500));
      await host.settle(tester);
      expect(find.byKey(const ValueKey('action.copyText')), findsOneWidget);
      expect(find.byType(MessageTile), findsWidgets);
      await host.dispose(tester);
    }, variant: TargetPlatformVariant.only(target));
  }
}
