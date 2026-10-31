// SPDX-License-Identifier: Apache-2.0

// Readable code blocks in the history (01 6.14, R-PR-28; brief M4-49), with the device choice
// of the app (`CodeBlockPreferenceScope`, `shell_view_state.code_blocks_unwrapped`) and the
// limit `ui.code_block_collapse_lines`: on macOS and Windows and on iOS and Android a long line
// wraps by default, the toggle "Wrap lines" switches every block of the device to one line per
// line with a horizontal scroll and the choice survives a restart, a long block shows collapsed
// with "Show all" and "Show less", and the copy stays the exact text.

import 'package:client/app/code_block_preference.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/thread/history_column.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/fakes.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/widgets.dart';

/// The live case: one long sentence as a fenced block "to copy".
const _sentence =
    'Please copy this long sentence into the ticket, so that everyone involved '
    'knows exactly which steps are still open tonight and who takes them over '
    'before the release on Monday.';

final String _long = [for (var i = 1; i <= 12; i++) 'step $i'].join('\n');

/// The clipboard writes of the platform channel.
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

/// Pumps the history of thread 1 below the app's code block scope, as the app root has it,
/// with `ui.code_block_collapse_lines` at 8.
Future<void> _pumpHistory(
  WidgetTester tester,
  ColumnHost host, {
  required String platform,
}) => host.pump(
  tester,
  CodeBlockPreferenceScope(child: HistoryColumn(threadId: threadId(1))),
  location: '/t/${threadId(1).value}',
  overrides: [
    ...threadOverrides(FakeThreadReads(host.db)),
    appConfigProvider.overrideWithValue(
      AppConfig.defaults().withOverrides({
        ...fastConfig,
        'ui.code_block_collapse_lines': '8',
      }),
    ),
  ],
  platform: platform,
  density: platform == 'ios' || platform == 'android'
      ? DsDensityMode.touch
      : DsDensityMode.comfortable,
);

Future<ColumnHost> _open(
  WidgetTester tester, {
  required String platform,
  required String content,
}) async {
  final host = await ColumnHost.open(tester);
  await host.run(
    tester,
    () => seedThread(
      host.db,
      messages: [msg(1, author: sam, text: content)],
    ),
  );
  await _pumpHistory(tester, host, platform: platform);
  return host;
}

Finder get _block => find.byType(DsMarkdownCodeBlock);

Finder get _scroll => find.descendant(
  of: _block,
  matching: find.byWidgetPredicate(
    (w) => w is Scrollable && w.axisDirection == AxisDirection.right,
  ),
);

double _offset(WidgetTester tester) =>
    tester.state<ScrollableState>(_scroll).position.pixels;

/// A point on the code, inside the visible part of the block.
Offset _onCode(WidgetTester tester) {
  final text = tester.getRect(
    find.descendant(of: _block, matching: find.textContaining('Please')).first,
  );
  return Offset(tester.getRect(_block).left + 60, text.top + 6);
}

Future<void> _toggle(WidgetTester tester, ColumnHost host) async {
  await tester.tap(find.bySemanticsLabel(en.markdownCodeWrap));
  await host.settle(tester);
}

void main() {
  for (final platform in ['macos', 'windows']) {
    testWidgets('$platform: a long line wraps, the toggle switches the device '
        'to a horizontal scroll that survives a restart, the copy is exact', (
      tester,
    ) async {
      final clipboard = _clipboard(tester);
      final host = await _open(
        tester,
        platform: platform,
        content: 'To copy:\n\n```\n$_sentence\n```',
      );
      // Wrapped: the whole sentence is inside the block.
      expect(_scroll, findsNothing);
      final text = tester.getRect(
        find.descendant(of: _block, matching: find.textContaining('Please')),
      );
      expect(text.right, lessThanOrEqualTo(tester.getRect(_block).right));
      expect(await host.db.forumViewDao.codeBlocksUnwrapped(), isFalse);

      await _toggle(tester, host);
      expect(await host.db.forumViewDao.codeBlocksUnwrapped(), isTrue);
      expect(_scroll, findsOneWidget);
      final bar = tester.widget<RawScrollbar>(
        find.descendant(
          of: _block,
          matching: find.byWidgetPredicate((w) => w is RawScrollbar),
        ),
      );
      expect(bar.thumbVisibility, isTrue);

      // Shift and the mouse wheel scroll the block sideways, not the history.
      final pointer = TestPointer(1, PointerDeviceKind.mouse);
      await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
      await tester.sendEventToBinding(pointer.hover(_onCode(tester)));
      await tester.sendEventToBinding(pointer.scroll(const Offset(0, 120)));
      await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
      await tester.pump();
      expect(_offset(tester), greaterThan(0));

      // A trackpad pan too.
      final before = _offset(tester);
      final pan = await tester.createGesture(kind: PointerDeviceKind.trackpad);
      await pan.panZoomStart(_onCode(tester));
      await pan.panZoomUpdate(_onCode(tester), pan: const Offset(-120, 0));
      await pan.panZoomEnd();
      await tester.pump(const Duration(milliseconds: 500));
      expect(_offset(tester), greaterThan(before));

      await tester.tap(find.bySemanticsLabel(en.markdownCopyCode));
      await tester.pump();
      expect(clipboard, [_sentence]);

      // A restart reads the choice of the device.
      await _pumpHistory(tester, host, platform: platform);
      expect(_scroll, findsOneWidget);
      await _toggle(tester, host);
      expect(await host.db.forumViewDao.codeBlocksUnwrapped(), isFalse);
      expect(_scroll, findsNothing);
      await host.dispose(tester);
    });
  }

  for (final platform in ['ios', 'android']) {
    testWidgets('$platform: a long line wraps, the toggle switches without '
        'copying, a drag scrolls, a tap copies the exact text', (tester) async {
      final clipboard = _clipboard(tester);
      final host = await _open(
        tester,
        platform: platform,
        content: 'To copy:\n\n```\n$_sentence\n```',
      );
      expect(_scroll, findsNothing);
      await _toggle(tester, host);
      expect(clipboard, isEmpty);
      expect(await host.db.forumViewDao.codeBlocksUnwrapped(), isTrue);
      expect(_scroll, findsOneWidget);

      await tester.dragFrom(_onCode(tester), const Offset(-150, 0));
      await tester.pump(const Duration(milliseconds: 500));
      expect(_offset(tester), greaterThan(0));
      expect(clipboard, isEmpty);

      await tester.tapAt(_onCode(tester));
      await tester.pump();
      expect(clipboard, [_sentence]);
      await host.dispose(tester);
    });
  }

  for (final platform in ['macos', 'windows', 'ios', 'android']) {
    testWidgets('$platform: a block above ui.code_block_collapse_lines shows '
        'collapsed and expands in place', (tester) async {
      final clipboard = _clipboard(tester);
      final host = await _open(
        tester,
        platform: platform,
        content: '```\n$_long\n```',
      );
      expect(find.text(en.markdownCodeShowAll), findsOneWidget);
      expect(find.byKey(DsMarkdownCodeBlock.fadeKey), findsOneWidget);
      final collapsed = tester.getRect(_block).height;

      await tester.tap(find.text(en.markdownCodeShowAll));
      await host.settle(tester);
      expect(tester.getRect(_block).height, greaterThan(collapsed));
      expect(find.text(en.markdownCodeShowLess), findsOneWidget);
      expect(find.byKey(DsMarkdownCodeBlock.fadeKey), findsNothing);

      await tester.tap(find.text(en.markdownCodeShowLess));
      await host.settle(tester);
      expect(tester.getRect(_block).height, collapsed);
      expect(clipboard, isEmpty);
      await host.dispose(tester);
    });
  }

  testWidgets('without a local database the toggle switches only its block', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          localDatabaseProvider.overrideWithValue(null),
          appConfigProvider.overrideWithValue(AppConfig.defaults()),
        ],
        child: hostScreen(
          CodeBlockPreferenceScope(
            child: DsMarkdown('```\n$_sentence\n```', onOpenLink: (_) {}),
          ),
        ),
      ),
    );
    await tester.pump();
    final scope = tester.widget<DsCodeBlockScope>(
      find.byType(DsCodeBlockScope),
    );
    expect(scope.wrap, isTrue);
    expect(scope.collapseLines, AppConfig.defaults().codeBlockCollapseLines);
    await tester.tap(find.bySemanticsLabel(en.markdownCodeWrap));
    await tester.pump();
    await tester.pump();
    expect(_scroll, findsOneWidget);
  });
}
