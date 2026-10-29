// SPDX-License-Identifier: Apache-2.0

// Copying single blocks of a message in the history (01 6.12, R-PR-25; brief M4-48): on the
// desktop (macOS and Windows) the button at the top right of a code block or a quote copies
// exactly that block and the message menu's "Copy text" the whole message; on the phone (iOS
// and Android) a tap on a block copies it with "Copied" and a haptic, and a long press on the
// message opens the message menu, whose "Copy text" copies the whole message.

import 'package:client/thread/message_tile.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/widgets.dart';

const _content =
    'Run this:\n\n'
    '```sh\n\tmake check  \n\techo "ü \u{1F600}"\n```\n\n'
    '> Quoted **line**\n'
    '> second line';

const _code = '\tmake check  \n\techo "ü \u{1F600}"';
const _quote = 'Quoted line\nsecond line';

/// The clipboard writes and the haptics of the platform channel.
final class _Platform {
  final List<String> clipboard = [];
  final List<String> haptics = [];
}

_Platform _platform(WidgetTester tester) {
  final p = _Platform();
  tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
    SystemChannels.platform,
    (call) async {
      if (call.method == 'Clipboard.setData') {
        p.clipboard.add((call.arguments as Map)['text'] as String);
      }
      if (call.method == 'HapticFeedback.vibrate') {
        p.haptics.add(call.arguments as String);
      }
      return null;
    },
  );
  return p;
}

Future<ColumnHost> _open(
  WidgetTester tester, {
  required String platform,
  required bool phone,
}) async {
  final host = await ColumnHost.open(tester);
  await host.run(
    tester,
    () => seedThread(
      host.db,
      messages: [msg(1, author: sam, text: _content)],
    ),
  );
  await pumpThread(
    tester,
    host,
    FakeThreadReads(host.db),
    platform: platform,
    density: phone ? DsDensityMode.touch : DsDensityMode.comfortable,
  );
  return host;
}

/// Chooses "Copy text" in the open menu.
Future<void> _copyText(WidgetTester tester, ColumnHost host) async {
  final item = find.byKey(const ValueKey('action.copyText'));
  await tester.ensureVisible(item);
  await tester.pump(const Duration(milliseconds: 300));
  await tester.tap(item);
  await tester.pump(const Duration(milliseconds: 500));
  await host.settle(tester);
}

Finder get _message => find.byKey(ValueKey('message.${messageId(1).value}'));

void main() {
  for (final platform in ['macos', 'windows']) {
    testWidgets('$platform: the buttons copy exactly their block, the menu '
        'the whole message', (tester) async {
      final p = _platform(tester);
      final host = await _open(tester, platform: platform, phone: false);
      expect(find.byType(MessageTile), findsOneWidget);

      await tester.tap(find.bySemanticsLabel(en.markdownCopyCode));
      await tester.pump();
      await tester.tap(find.bySemanticsLabel(en.markdownCopyQuote));
      await tester.pump();
      expect(p.clipboard, [_code, _quote]);
      expect(find.text(en.messageCopied), findsWidgets);

      await tester.tap(_message, buttons: kSecondaryButton);
      await tester.pump(const Duration(milliseconds: 500));
      await host.settle(tester);
      await _copyText(tester, host);
      expect(p.clipboard.last, _content);
      expect(p.haptics, isEmpty);
      await host.dispose(tester);
    });
  }

  for (final platform in ['ios', 'android']) {
    testWidgets('$platform: a tap copies the block, a long press opens the '
        'menu whose Copy text copies the whole message', (tester) async {
      final p = _platform(tester);
      final host = await _open(tester, platform: platform, phone: true);
      // No copy buttons on the phone.
      expect(find.text(en.markdownCopyCode), findsNothing);

      await tester.tap(find.textContaining('make check'));
      await tester.pump();
      expect(p.clipboard, [_code]);
      expect(find.text(en.messageCopied), findsOneWidget);
      expect(p.haptics, isNotEmpty);

      await tester.tap(find.textContaining('second line'));
      await tester.pump();
      expect(p.clipboard.last, _quote);

      // A long press on a block or on the text opens the menu; nothing is copied by it.
      await tester.longPress(find.textContaining('make check'));
      await tester.pump(const Duration(milliseconds: 500));
      await host.settle(tester);
      expect(p.clipboard, hasLength(2));
      expect(find.byKey(const ValueKey('action.copyText')), findsOneWidget);
      await _copyText(tester, host);
      expect(p.clipboard.last, _content);

      await tester.longPress(find.textContaining('Run this'));
      await tester.pump(const Duration(milliseconds: 500));
      await host.settle(tester);
      expect(find.byKey(const ValueKey('action.copyText')), findsOneWidget);
      await host.dispose(tester);
    });
  }

  testWidgets('on the desktop a long press opens no menu', (tester) async {
    _platform(tester);
    final host = await _open(tester, platform: 'macos', phone: false);
    await tester.longPress(find.textContaining('Run this'));
    await tester.pump(const Duration(milliseconds: 500));
    await host.settle(tester);
    expect(find.byKey(const ValueKey('action.copyText')), findsNothing);
    await host.dispose(tester);
  });
}
