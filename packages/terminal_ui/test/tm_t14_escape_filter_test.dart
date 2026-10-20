// SPDX-License-Identifier: Apache-2.0

// TM-T14 end to end through the view (docs/spec/24-terminal.md 11.2 and 14, SEC-TERM-6): host
// output with OSC 52 (BEL and ST) puts nothing into the clipboard, OSC 8 shows its target and
// opens only after a click and a confirmation, a title or clipboard query gets no answer on
// the stream, and sequences split across two frames are handled. The filter's own unit tests
// are in escape_filter_test.dart.

import 'dart:convert';

import 'package:bothy_noise/bothy_noise.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';
import 'package:xterm/xterm.dart' show TerminalView;

import 'support/fakes.dart';
import 'support/widgets.dart';

const String _esc = '\x1b';
const String _bel = '\x07';
const String _st = '$_esc\\';

/// Records every write to the system clipboard.
List<Object?> _fakeClipboard(WidgetTester tester) {
  final writes = <Object?>[];
  tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
    SystemChannels.platform,
    (call) async {
      if (call.method == 'Clipboard.setData') writes.add(call.arguments);
      return null;
    },
  );
  addTearDown(
    () => tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
      SystemChannels.platform,
      null,
    ),
  );
  return writes;
}

Future<Harness> _connected(WidgetTester tester) async {
  final h = (await tester.runAsync(Harness.create))!;
  await pumpApp(tester, TerminalPane(controller: h.controller, fontSize: 14));
  final opening = h.open();
  await pumpUntil(
    tester,
    () => h.controller.state.phase == TerminalPhase.connected,
  );
  await opening;
  return h;
}

Future<void> _send(WidgetTester tester, Harness h, String text) async {
  await h.host.sendData(utf8.encode(text));
  await tester.pump(const Duration(milliseconds: 5));
  await tester.pump(const Duration(milliseconds: 5));
}

/// Taps [at] and waits out the double tap timeout of the renderer's gesture handler.
Future<void> _click(WidgetTester tester, Offset at) async {
  await tester.tapAt(at);
  await tester.pump(kDoubleTapTimeout + const Duration(milliseconds: 50));
  await tester.pumpAndSettle();
}

Iterable<DataFrame> _answers(Harness h) => h.host.frames.whereType<DataFrame>();

void main() {
  testWidgets(
    'OSC 52 with BEL or ST, as set and as query, puts nothing into the clipboard',
    (tester) async {
      final clipboard = _fakeClipboard(tester);
      final h = await _connected(tester);
      await _send(tester, h, 'a$_esc]52;c;c2VjcmV0$_bel');
      await _send(tester, h, 'b$_esc]52;c;c2VjcmV0$_st');
      await _send(tester, h, 'c$_esc]52;c;?$_bel');
      await _send(tester, h, 'd$_esc]52;p;?$_st');
      await tester.pump();
      expect(clipboard, isEmpty);
      expect(h.controller.screen.visibleText(), 'abcd');
      expect(_answers(h), isEmpty);
    },
  );

  testWidgets(
    'OSC 8 shows the target and opens only after a click and a confirmation',
    (tester) async {
      final h = await _connected(tester);
      await _send(
        tester,
        h,
        '$_esc]8;;https://example.com/docs${_st}docs$_esc]8;;$_st rest',
      );
      expect(h.controller.screen.visibleText(), 'docs rest');
      expect(
        h.controller.screen.links.single.target,
        'https://example.com/docs',
      );

      final view = tester.getTopLeft(find.byType(TerminalView));
      final onLink = view + const Offset(6, 6);
      final mouse = await tester.createGesture(kind: PointerDeviceKind.mouse);
      addTearDown(mouse.removePointer);
      await mouse.addPointer(location: onLink);
      await mouse.moveTo(onLink);
      await tester.pump();
      expect(find.text('Link: https://example.com/docs'), findsOneWidget);
      expect(h.links.opened, isEmpty);

      await _click(tester, onLink);
      expect(find.text('Open link?'), findsOneWidget);
      expect(find.text('https://example.com/docs'), findsOneWidget);
      await tester.tap(find.text('Cancel'));
      await tester.pumpAndSettle();
      expect(h.links.opened, isEmpty);

      await _click(tester, onLink);
      await tester.tap(find.text('Open link'));
      await tester.pumpAndSettle();
      expect(h.links.opened.map((u) => u.toString()), [
        'https://example.com/docs',
      ]);
    },
  );

  testWidgets('a link to another scheme cannot be opened', (tester) async {
    final h = await _connected(tester);
    await _send(tester, h, '$_esc]8;;file:///etc/passwd${_st}x$_esc]8;;$_st');
    final onLink =
        tester.getTopLeft(find.byType(TerminalView)) + const Offset(6, 6);
    await _click(tester, onLink);
    expect(find.text('Open link?'), findsOneWidget);
    expect(find.text('Open link'), findsNothing);
    expect(
      find.text('Only https and http links open. This address stays closed.'),
      findsOneWidget,
    );
  });

  testWidgets(
    'title, icon, size, status and clipboard queries get no answer on the stream',
    (tester) async {
      final h = await _connected(tester);
      await _send(
        tester,
        h,
        '$_esc[21t$_esc[20t$_esc[18t$_esc[14t$_esc[5n$_esc[=c'
        '$_esc]52;c;?$_bel$_esc]11;?$_bel${_esc}P\$qm$_st$_esc[>q',
      );
      await tester.pump(const Duration(milliseconds: 20));
      expect(_answers(h), isEmpty);
      // The allowed answers still work: a cursor position report goes back.
      await _send(tester, h, '$_esc[6n');
      await pumpUntil(tester, () => _answers(h).isNotEmpty);
      expect(utf8.decode(_answers(h).single.bytes), startsWith('$_esc['));
    },
  );

  testWidgets('sequences split across two frames are filtered the same', (
    tester,
  ) async {
    final clipboard = _fakeClipboard(tester);
    final h = await _connected(tester);
    await _send(tester, h, 'x$_esc]5');
    await _send(tester, h, '2;c;c2VjcmV0${_bel}y');
    await _send(tester, h, '$_esc[2');
    await _send(tester, h, '1t');
    await _send(tester, h, '$_esc]8;;https://example.');
    await _send(tester, h, 'com/a$_esc');
    await _send(tester, h, '\\z$_esc]8;;$_st');
    await tester.pump(const Duration(milliseconds: 20));
    expect(clipboard, isEmpty);
    expect(_answers(h), isEmpty);
    expect(h.controller.screen.visibleText(), 'xyz');
    expect(h.controller.screen.links.single.target, 'https://example.com/a');
  });

  testWidgets('the link confirmation shows the host before a long target', (
    tester,
  ) async {
    final h = await _connected(tester);
    final target = 'https://evil.example/${'a' * 1900}';
    await _send(tester, h, '$_esc]8;;$target${_st}x$_esc]8;;$_st');
    final onLink =
        tester.getTopLeft(find.byType(TerminalView)) + const Offset(6, 6);
    await _click(tester, onLink);
    final host = find.text('Host: evil.example');
    expect(host, findsOneWidget);
    expect(
      tester.getRect(host).bottom,
      lessThan(tester.view.physicalSize.height),
    );
  });
}
