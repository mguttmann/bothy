// SPDX-License-Identifier: Apache-2.0

// A mouse drag in a list selection scrolls (01 6.15 item 1; ticket FIX-drag-select-autoscroll):
// held below the list, it scrolls the list through a code block taller than the list; held
// beyond the right edge of a code block without wrap, it scrolls the block sideways. The copy
// is the whole selected range.

import 'package:design_system/design_system.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import 'support.dart';

const _desktop = TargetPlatformVariant({
  TargetPlatform.macOS,
  TargetPlatform.windows,
});

final _lines = [for (var i = 1; i <= 16; i++) 'code line $i'];

const _long =
    'start of a very long line that runs far beyond the right edge of the '
    'block and ends here';

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

const _listKey = ValueKey('list');

Future<void> _pump(
  WidgetTester tester,
  String content, {
  bool wrap = true,
}) async {
  await tester.pumpWidget(
    MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: DsThemeScope(
        dsContext: testContext(),
        child: Material(
          child: Align(
            alignment: Alignment.topLeft,
            child: Padding(
              padding: const EdgeInsets.only(top: 60),
              child: SizedBox(
                width: 420,
                height: 200,
                child: DsCodeBlockScope(
                  wrap: wrap,
                  collapseLines: 40,
                  child: DsListSelection(
                    child: ListView(
                      key: _listKey,
                      children: [
                        DsSelectableRow(
                          child: DsMessage(
                            DsMessageData(
                              id: 'm1',
                              authorName: 'Robin',
                              origin: DsMessageOrigin.human,
                              kind: DsMessageKind.text,
                              timeLabel: '10:42',
                              content: content,
                            ),
                            collapseLines: 80,
                            onOpenLink: (_) {},
                            selectable: true,
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    ),
  );
  // The selectables join the selection after the first frame.
  await tester.pump();
}

Finder _paragraph(String text) => find.byWidgetPredicate(
  (w) => w is RichText && w.text.toPlainText().contains(text),
);

Offset _at(WidgetTester tester, String text, int index) {
  final paragraph = tester.renderObject<RenderParagraph>(
    _paragraph(text).first,
  );
  final caret = paragraph.getOffsetForCaret(
    TextPosition(offset: index),
    Rect.zero,
  );
  return paragraph.localToGlobal(
    caret + Offset(0, paragraph.preferredLineHeight / 2),
  );
}

Future<void> _holdDrag(WidgetTester tester, Offset from, Offset to) async {
  final gesture = await tester.startGesture(
    from,
    kind: PointerDeviceKind.mouse,
  );
  await tester.pump();
  await gesture.moveTo(Offset.lerp(from, to, 0.3)!);
  await tester.pump();
  await gesture.moveTo(to);
  for (var i = 0; i < 300; i++) {
    await tester.pump(const Duration(milliseconds: 16));
  }
  await gesture.up();
  await tester.pump();
}

Future<void> _copy(WidgetTester tester) async {
  final mod = defaultTargetPlatform == TargetPlatform.macOS
      ? LogicalKeyboardKey.meta
      : LogicalKeyboardKey.control;
  await tester.sendKeyDownEvent(mod);
  await tester.sendKeyEvent(LogicalKeyboardKey.keyC);
  await tester.sendKeyUpEvent(mod);
  await tester.pump();
}

ScrollPosition _scrollOf(WidgetTester tester, Finder of) => tester
    .state<ScrollableState>(
      find.descendant(of: of, matching: find.byType(Scrollable)).first,
    )
    .position;

void main() {
  testWidgets('a drag held below the list scrolls through a code block '
      'taller than the list and copies all of it', (tester) async {
    final copied = _clipboard(tester);
    await _pump(tester, 'Run:\n\n```\n${_lines.join('\n')}\n```');
    final list = tester.getRect(find.byKey(_listKey));
    final position = _scrollOf(tester, find.byKey(_listKey));
    expect(position.maxScrollExtent, greaterThan(0));
    await _holdDrag(
      tester,
      _at(tester, 'Run:', 0),
      Offset(list.center.dx, list.bottom + 30),
    );
    expect(position.pixels, position.maxScrollExtent);
    await _copy(tester);
    expect(copied, ['Run:\n\n${_lines.join('\n')}']);
  }, variant: _desktop);

  testWidgets('a drag held beyond the right edge of a code block without '
      'wrap scrolls it sideways and copies the end of the line', (
    tester,
  ) async {
    final copied = _clipboard(tester);
    await _pump(tester, '```\n$_long\n```', wrap: false);
    final block = find.byType(DsMarkdownCodeBlock);
    final sideways = tester
        .state<ScrollableState>(
          find.descendant(of: block, matching: find.byType(Scrollable)).first,
        )
        .position;
    expect(sideways.maxScrollExtent, greaterThan(0));
    final start = _at(tester, 'start of', 0);
    await _holdDrag(
      tester,
      start,
      Offset(tester.getRect(block).right + 30, start.dy),
    );
    expect(sideways.pixels, sideways.maxScrollExtent);
    await _copy(tester);
    expect(copied, [_long]);
  }, variant: _desktop);
}
