// SPDX-License-Identifier: Apache-2.0

// Free text selection in the history (01 6.15, R-PR-29; brief M4-50). On macOS and Windows a
// mouse drag selects text across the paragraphs, the code block and the quote of a message,
// Cmd+C (Ctrl+C) and "Copy" in the message menu of a right click copy it as exact plain text,
// the selection triggers no message action, links and the copy buttons keep working. On iOS and
// Android the history keeps its long press menu, whose "Select text" opens a view of the message
// with the system selection.

import 'package:client/thread/message_tile.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/widgets.dart';

String get _link => '/t/${threadId(2).value}';

String get _content =>
    'Run this:\n\n'
    '```sh\n\tmake check  \n\techo "ñ \u{1F600}"\n```\n\n'
    '> Quoted **line**\n'
    '> second line\n\n'
    'See [the guide]($_link) first.';

/// What a selection from the first to the last character copies: blocks apart by an empty
/// line, the code exactly, the quote without markers, the link as its text.
const _all =
    'Run this:\n\n'
    '\tmake check  \n\techo "ñ \u{1F600}"\n\n'
    'Quoted line\nsecond line\n\n'
    'See the guide first.';

const _code = '\tmake check  \n\techo "ñ \u{1F600}"';

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

Future<ColumnHost> _open(
  WidgetTester tester, {
  required String platform,
}) async {
  final host = await ColumnHost.open(tester);
  final phone = platform == 'ios' || platform == 'android';
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

/// The writes of message actions in the outbox (the read position and the draft are the
/// view's own).
Future<List<OutboxEntry>> _actions(
  WidgetTester tester,
  ColumnHost host,
) async => [
  for (final e in await host.run(tester, () => host.db.outboxDao.allEntries()))
    if (e.kind != PersonalOps.putReadState &&
        e.kind != PersonalOps.putThreadDraft)
      e,
];

/// The paragraph whose plain text contains [text].
Finder _paragraph(String text) => find.byWidgetPredicate(
  (w) => w is RichText && w.text.toPlainText().contains(text),
);

/// The global position of the character [index] of the paragraph [text].
Offset _at(WidgetTester tester, Finder text, int index) {
  final paragraph = tester.renderObject<RenderParagraph>(text.first);
  final caret = paragraph.getOffsetForCaret(
    TextPosition(offset: index),
    Rect.zero,
  );
  return paragraph.localToGlobal(
    caret + Offset(0, paragraph.preferredLineHeight / 2),
  );
}

Future<void> _drag(WidgetTester tester, Offset from, Offset to) async {
  final gesture = await tester.startGesture(
    from,
    kind: PointerDeviceKind.mouse,
  );
  await tester.pump();
  await gesture.moveTo(to);
  await tester.pump();
  await gesture.up();
  await tester.pump();
}

/// Selects from the first character of the message to its last one.
Future<void> _selectAll(WidgetTester tester) => _drag(
  tester,
  _at(tester, _paragraph('Run this:'), 0),
  _at(tester, _paragraph('See '), 'See the guide first.'.length + 1),
);

Future<void> _copyKeys(WidgetTester tester) async {
  final mod = defaultTargetPlatform == TargetPlatform.macOS
      ? LogicalKeyboardKey.meta
      : LogicalKeyboardKey.control;
  await tester.sendKeyDownEvent(mod);
  await tester.sendKeyEvent(LogicalKeyboardKey.keyC);
  await tester.sendKeyUpEvent(mod);
  await tester.pump();
}

Future<void> _rightClick(
  WidgetTester tester,
  ColumnHost host,
  Offset at,
) async {
  await tester.tapAt(
    at,
    buttons: kSecondaryButton,
    kind: PointerDeviceKind.mouse,
  );
  await tester.pump(const Duration(milliseconds: 500));
  await host.settle(tester);
}

Future<void> _choose(
  WidgetTester tester,
  ColumnHost host,
  String action,
) async {
  final item = find.byKey(ValueKey('action.$action'));
  await tester.ensureVisible(item);
  await tester.pump(const Duration(milliseconds: 300));
  await tester.tap(item);
  await tester.pump(const Duration(milliseconds: 500));
  await host.settle(tester);
}

void main() {
  for (final (name, target) in [
    ('macos', TargetPlatform.macOS),
    ('windows', TargetPlatform.windows),
  ]) {
    final variant = TargetPlatformVariant.only(target);

    testWidgets('$name: a drag selects across the blocks of a message and the '
        'copy shortcut copies it exactly', (tester) async {
      final copied = _clipboard(tester);
      final host = await _open(tester, platform: name);
      await _selectAll(tester);
      await _copyKeys(tester);
      expect(copied, [_all]);

      // A part of the code block alone: no fence, no language label.
      await _drag(
        tester,
        _at(tester, _paragraph('make check'), 5),
        _at(tester, _paragraph('echo'), 4),
      );
      await _copyKeys(tester);
      expect(copied.last, 'check  \n\techo');
      await host.dispose(tester);
    }, variant: variant);

    testWidgets('$name: the message menu of a right click copies the '
        'selection, and the selection triggers no message action', (
      tester,
    ) async {
      final copied = _clipboard(tester);
      final host = await _open(tester, platform: name);
      // Without a selection the menu has no "Copy" (a right click on the avatar).
      await _rightClick(
        tester,
        host,
        tester.getCenter(find.byType(DsAvatar).first),
      );
      expect(find.byKey(const ValueKey('action.copySelection')), findsNothing);
      expect(find.byKey(const ValueKey('action.copyText')), findsOneWidget);
      await tester.tapAt(const Offset(5, 5));
      await host.settle(tester);

      await _selectAll(tester);
      // The drag opened no menu and wrote nothing.
      expect(find.byKey(const ValueKey('action.copyText')), findsNothing);
      expect(await _actions(tester, host), isEmpty);
      expect(copied, isEmpty);

      await _rightClick(tester, host, _at(tester, _paragraph('Quoted'), 2));
      expect(
        find.byKey(const ValueKey('action.copySelection')),
        findsOneWidget,
      );
      expect(find.text(en.messageActionCopySelection), findsOneWidget);
      await _choose(tester, host, 'copySelection');
      expect(copied, [_all]);
      expect(find.text(en.messageCopied), findsOneWidget);
      expect(await _actions(tester, host), isEmpty);
      await host.dispose(tester);
    }, variant: variant);

    testWidgets('$name: links and the copy button keep working beside the '
        'selection', (tester) async {
      final copied = _clipboard(tester);
      final host = await _open(tester, platform: name);
      await _selectAll(tester);
      await tester.tap(find.text('the guide'), kind: PointerDeviceKind.mouse);
      await tester.pump(const Duration(milliseconds: 500));
      await host.settle(tester);
      expect(host.visited.last, _link);
      await host.dispose(tester);

      final again = await _open(tester, platform: name);
      await _selectAll(tester);
      await tester.tap(find.bySemanticsLabel(en.markdownCopyCode));
      await tester.pump();
      expect(copied.last, _code);
      await again.dispose(tester);
    }, variant: variant);
  }

  for (final (name, target) in [
    ('ios', TargetPlatform.iOS),
    ('android', TargetPlatform.android),
  ]) {
    testWidgets('$name: the long press menu keeps its actions and Select text '
        'opens a selectable view of the message', (tester) async {
      final copied = _clipboard(tester);
      final host = await _open(tester, platform: name);
      // The history itself has no selection: a long press is the message menu's.
      expect(
        find.descendant(
          of: find.byType(MessageTile),
          matching: find.byType(SelectionArea),
        ),
        findsNothing,
      );
      await tester.longPress(find.textContaining('Run this'));
      await tester.pump(const Duration(milliseconds: 500));
      await host.settle(tester);
      expect(find.byKey(const ValueKey('action.copyText')), findsOneWidget);
      expect(find.byKey(const ValueKey('action.copySelection')), findsNothing);
      expect(find.text(en.messageActionSelectText), findsOneWidget);
      await _choose(tester, host, 'selectText');
      // The sheet slides in after the menu closed.
      await tester.pump(const Duration(milliseconds: 500));

      final view = find.byKey(const ValueKey('selectText.view'));
      expect(view, findsOneWidget);
      expect(
        find.descendant(of: view, matching: find.byType(SelectionArea)),
        findsOneWidget,
      );
      // A long press on a word selects it with the system handles and its menu.
      final word = find.descendant(
        of: view,
        matching: _paragraph('second line'),
      );
      await tester.longPressAt(_at(tester, word, 'Quoted line\nsec'.length));
      await tester.pump(const Duration(milliseconds: 500));
      final selectAll = find.text(
        target == TargetPlatform.iOS ? 'Select All' : 'Select all',
      );
      expect(selectAll, findsOneWidget);
      await tester.tap(selectAll);
      await tester.pump(const Duration(milliseconds: 500));
      final copy = find.text('Copy');
      expect(copy, findsOneWidget);
      await tester.tap(copy);
      await tester.pump(const Duration(milliseconds: 500));
      expect(copied, [_all]);

      // The view closes with its button, back to the history.
      await tester.tap(find.bySemanticsLabel(en.messageSelectTextClose));
      await tester.pump(const Duration(milliseconds: 500));
      await tester.pump(const Duration(milliseconds: 500));
      await host.settle(tester);
      expect(view, findsNothing);
      expect(find.byType(MessageTile), findsOneWidget);
      await host.dispose(tester);
    }, variant: TargetPlatformVariant.only(target));
  }

  testWidgets('a deleted message offers no Select text', (tester) async {
    _clipboard(tester);
    final host = await ColumnHost.open(tester);
    await host.run(
      tester,
      () => seedThread(
        host.db,
        messages: [msg(1, author: sam, text: 'gone', deleted: true)],
      ),
    );
    await pumpThread(
      tester,
      host,
      FakeThreadReads(host.db),
      platform: 'ios',
      density: DsDensityMode.touch,
    );
    await tester.longPress(find.byType(MessageTile));
    await tester.pump(const Duration(milliseconds: 500));
    await host.settle(tester);
    expect(find.text(en.messageActionSelectText), findsNothing);
    await host.dispose(tester);
  });
}
