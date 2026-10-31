// SPDX-License-Identifier: Apache-2.0

// Free text selection in a message (01 6.15, R-PR-29; brief M4-50): under a system selection the
// renderer copies what it shows as plain text with the structure of the message (blocks apart by
// an empty line, list items on their own lines with their marker, table cells by a tab, code
// lines by a line break), a partial selection of a code block copies exactly its part without
// fences and without the display wrap of M4-49, the chrome of a block (language label, "Copy
// code", "Wrap lines", "Show all") is never part of it, links stay clickable, the copy buttons,
// the toggle and the expand keep working, and `DsMessage` makes only its content selectable.

import 'package:design_system/design_system.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import 'support.dart';

/// Records what the copy buttons write.
final class _Recorder implements DsClipboard {
  final List<DsCopyData> writes = [];

  @override
  Future<void> write(DsCopyData data) async => writes.add(data);
}

const _desktop = TargetPlatformVariant({
  TargetPlatform.macOS,
  TargetPlatform.windows,
});

const _sentence =
    'Please copy this long sentence into the ticket, so that everyone involved '
    'knows exactly which steps are still open tonight and who takes them over '
    'before the release on Monday.';

/// A message with every block kind a selection crosses.
const _rich =
    'First paragraph with a [link](https://example.com/a) and `code` inside.\n'
    '\n'
    '- one\n'
    '- two\n'
    '\n'
    '3. three\n'
    '4. four\n'
    '\n'
    '> quoted **line**\n'
    '\n'
    '```sh\n'
    'alpha\n'
    '\n'
    '  beta\n'
    '$_sentence\n'
    '```\n'
    '\n'
    '| a | b |\n'
    '|---|---|\n'
    '| c | d |\n'
    '\n'
    'Last.';

const _richCopy =
    'First paragraph with a link and code inside.\n'
    '\n'
    '- one\n'
    '- two\n'
    '\n'
    '3. three\n'
    '4. four\n'
    '\n'
    'quoted line\n'
    '\n'
    'alpha\n'
    '\n'
    '  beta\n'
    '$_sentence\n'
    '\n'
    'a\tb\n'
    'c\td\n'
    '\n'
    'Last.';

/// The clipboard writes of the platform channel (the system copy of a selection).
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

/// Pumps [source] below a [SelectionArea] at [width]; [wrap] is the device choice of M4-49.
Future<_Recorder> _pump(
  WidgetTester tester,
  String source, {
  double width = 420,
  bool wrap = true,
  int collapseLines = DsCodeBlockScope.defaultCollapseLines,
  void Function(Uri)? onOpenLink,
}) async {
  final recorder = _Recorder();
  await tester.pumpWidget(
    MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: DsThemeScope(
        dsContext: testContext(),
        child: Scaffold(
          body: Align(
            alignment: Alignment.topLeft,
            child: SizedBox(
              width: width,
              child: SingleChildScrollView(
                child: DsClipboardScope(
                  clipboard: recorder,
                  child: DsCodeBlockScope(
                    wrap: wrap,
                    collapseLines: collapseLines,
                    child: SelectionArea(
                      child: DsMarkdown(
                        source,
                        onOpenLink: onOpenLink ?? (_) {},
                      ),
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
  await tester.pump();
  await tester.pump();
  return recorder;
}

LogicalKeyboardKey get _mod => defaultTargetPlatform == TargetPlatform.macOS
    ? LogicalKeyboardKey.meta
    : LogicalKeyboardKey.control;

Future<void> _shortcut(WidgetTester tester, LogicalKeyboardKey key) async {
  await tester.sendKeyDownEvent(_mod);
  await tester.sendKeyEvent(key);
  await tester.sendKeyUpEvent(_mod);
  await tester.pump();
}

/// The global position of the character [index] of the text [text] (a whole line of code or a
/// paragraph that the finder matches), vertically in its middle.
Offset _at(WidgetTester tester, Finder text, int index) {
  final paragraph = tester.renderObject<RenderParagraph>(text);
  final caret = paragraph.getOffsetForCaret(
    TextPosition(offset: index),
    Rect.zero,
  );
  return paragraph.localToGlobal(
    caret + Offset(0, paragraph.preferredLineHeight / 2),
  );
}

/// A mouse drag from [from] to [to].
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

/// A mouse click at [at], [times] in a row.
Future<void> _click(WidgetTester tester, Offset at, {int times = 1}) async {
  for (var i = 0; i < times; i++) {
    final gesture = await tester.startGesture(
      at,
      kind: PointerDeviceKind.mouse,
    );
    await gesture.up();
    await tester.pump(const Duration(milliseconds: 50));
  }
  await tester.pump(const Duration(milliseconds: 500));
}

/// The paragraph whose plain text is [text].
Finder _text(String text) => find.byWidgetPredicate(
  (w) => w is RichText && w.text.toPlainText() == text,
);

/// The paragraph whose plain text contains [text].
Finder _containing(String text) => find.byWidgetPredicate(
  (w) => w is RichText && w.text.toPlainText().contains(text),
);

void main() {
  group('copy of a selection', () {
    testWidgets('select all copies the shown text with the structure of the '
        'message and no chrome', (tester) async {
      final copied = _clipboard(tester);
      await _pump(tester, _rich);
      await _click(tester, _at(tester, _text('Last.'), 1));
      await _shortcut(tester, LogicalKeyboardKey.keyA);
      await _shortcut(tester, LogicalKeyboardKey.keyC);
      expect(copied, [_richCopy]);
      final l = await en();
      for (final label in [
        l.markdownCopyCode,
        l.markdownCopyQuote,
        l.markdownCodeWrap,
        'sh',
      ]) {
        expect(copied.single, isNot(contains('$label\n')));
      }
    }, variant: _desktop);

    testWidgets('a drag from a paragraph into the middle of a code line copies '
        'exactly that part', (tester) async {
      final copied = _clipboard(tester);
      await _pump(tester, _rich);
      final from = _at(tester, _containing('First paragraph'), 0);
      final to = _at(tester, _text('beta'), 2);
      await _drag(tester, from, to);
      await _shortcut(tester, LogicalKeyboardKey.keyC);
      expect(copied, [
        'First paragraph with a link and code inside.\n'
            '\n'
            '- one\n'
            '- two\n'
            '\n'
            '3. three\n'
            '4. four\n'
            '\n'
            'quoted line\n'
            '\n'
            'alpha\n'
            '\n'
            '  be',
      ]);
    }, variant: _desktop);

    testWidgets('a partial selection inside a code block has no fences and '
        'keeps its line breaks and indentation', (tester) async {
      final copied = _clipboard(tester);
      await _pump(tester, '```\nalpha\n  beta\ngamma\n```');
      await _drag(
        tester,
        _at(tester, _text('alpha'), 2),
        _at(tester, _text('gamma'), 3),
      );
      await _shortcut(tester, LogicalKeyboardKey.keyC);
      expect(copied, ['pha\n  beta\ngam']);
    }, variant: _desktop);

    testWidgets('the display wrap of a long line inserts no line break', (
      tester,
    ) async {
      final copied = _clipboard(tester);
      await _pump(tester, '```\n$_sentence\nnext\n```', width: 300);
      final line = _text(_sentence);
      // The sentence wraps on screen.
      expect(
        tester.getSize(line).height,
        greaterThan(
          tester.renderObject<RenderParagraph>(line).preferredLineHeight * 2,
        ),
      );
      await _drag(tester, _at(tester, line, 0), _at(tester, _text('next'), 4));
      await _shortcut(tester, LogicalKeyboardKey.keyC);
      expect(copied, ['$_sentence\nnext']);
    }, variant: _desktop);

    testWidgets('without wrap a partial selection of a code block is exact', (
      tester,
    ) async {
      final copied = _clipboard(tester);
      await _pump(tester, '```\nalpha\n  beta\ngamma\n```', wrap: false);
      final code = _text('alpha\n  beta\ngamma');
      await _drag(tester, _at(tester, code, 3), _at(tester, code, 11));
      await _shortcut(tester, LogicalKeyboardKey.keyC);
      expect(copied, ['ha\n  bet']);
    }, variant: _desktop);

    testWidgets('a nested list keeps its items on their own lines', (
      tester,
    ) async {
      final copied = _clipboard(tester);
      await _pump(tester, '- one\n  - inner\n- two');
      await _click(tester, _at(tester, _text('two'), 1));
      await _shortcut(tester, LogicalKeyboardKey.keyA);
      await _shortcut(tester, LogicalKeyboardKey.keyC);
      expect(copied, ['- one\n- inner\n- two']);
    }, variant: _desktop);
  });

  group('mouse gestures', () {
    testWidgets('a double click selects a word, a triple click the paragraph '
        'or the code line', (tester) async {
      final copied = _clipboard(tester);
      await _pump(tester, 'Hello brave world.\n\n```\nalpha beta\ngamma\n```');
      await _click(
        tester,
        _at(tester, _text('Hello brave world.'), 8),
        times: 2,
      );
      await _shortcut(tester, LogicalKeyboardKey.keyC);
      expect(copied.last, 'brave');
      await _click(
        tester,
        _at(tester, _text('Hello brave world.'), 8),
        times: 3,
      );
      await _shortcut(tester, LogicalKeyboardKey.keyC);
      expect(copied.last, 'Hello brave world.');
      await _click(tester, _at(tester, _text('alpha beta'), 2), times: 3);
      await _shortcut(tester, LogicalKeyboardKey.keyC);
      expect(copied.last, 'alpha beta');
    }, variant: _desktop);

    testWidgets('shift and a click extend the selection', (tester) async {
      final copied = _clipboard(tester);
      await _pump(tester, 'Hello brave world.\n\nSecond line here.');
      await _click(tester, _at(tester, _text('Hello brave world.'), 6));
      await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
      await _click(tester, _at(tester, _text('Second line here.'), 6));
      await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
      await _shortcut(tester, LogicalKeyboardKey.keyC);
      expect(copied, ['brave world.\n\nSecond']);
    }, variant: _desktop);
  });

  group('targets under a selection', () {
    testWidgets('a click on a link opens it, a drag over it selects', (
      tester,
    ) async {
      final copied = _clipboard(tester);
      final opened = <Uri>[];
      await _pump(
        tester,
        'Go to [the site](https://example.com/a) now.',
        onOpenLink: opened.add,
      );
      await tester.tap(find.text('the site'), kind: PointerDeviceKind.mouse);
      await tester.pump(const Duration(milliseconds: 500));
      expect(opened, [Uri.parse('https://example.com/a')]);

      // From the end of the link back to the start of the paragraph.
      await _drag(
        tester,
        tester.getRect(find.text('the site')).centerRight - const Offset(1, 0),
        _at(tester, _containing('Go to'), 0),
      );
      await _shortcut(tester, LogicalKeyboardKey.keyC);
      expect(opened, hasLength(1));
      expect(copied, ['Go to the site']);
    }, variant: _desktop);

    testWidgets('the copy button, the toggle and Show all keep working', (
      tester,
    ) async {
      final l = await en();
      final source =
          '```\n${[for (var i = 1; i <= 6; i++) 'line $i'].join('\n')}\n```';
      final recorder = await _pump(tester, source, collapseLines: 3);
      await tester.tap(find.bySemanticsLabel(l.markdownCopyCode));
      await tester.pump();
      expect(recorder.writes, [
        DsCopyData([for (var i = 1; i <= 6; i++) 'line $i'].join('\n')),
      ]);
      await tester.tap(find.text(l.markdownCodeShowAll));
      await tester.pump();
      await tester.pump();
      expect(find.text(l.markdownCodeShowLess), findsOneWidget);
      await tester.tap(find.bySemanticsLabel(l.markdownCodeWrap));
      await tester.pump();
      await tester.pump();
      expect(
        find.descendant(
          of: find.byType(DsMarkdownCodeBlock),
          matching: find.byWidgetPredicate(
            (w) => w is Scrollable && w.axisDirection == AxisDirection.right,
          ),
        ),
        findsOneWidget,
      );
    }, variant: _desktop);
  });

  group('DsMessage', () {
    const data = DsMessageData(
      id: 'm1',
      authorName: 'Robin',
      origin: DsMessageOrigin.human,
      kind: DsMessageKind.text,
      timeLabel: '10:42',
      content: 'The build is green.\n\nSecond paragraph.',
    );

    testWidgets('with a selection callback only the content is selectable and '
        'the callback hears the selected text', (tester) async {
      final copied = _clipboard(tester);
      final heard = <String>[];
      await pumpDs(
        tester,
        DsMessage(
          data,
          collapseLines: 20,
          onOpenLink: (_) {},
          onSelectionChanged: heard.add,
        ),
      );
      expect(find.byType(SelectionArea), findsOneWidget);
      await _click(tester, _at(tester, _text('The build is green.'), 2));
      await _shortcut(tester, LogicalKeyboardKey.keyA);
      await _shortcut(tester, LogicalKeyboardKey.keyC);
      expect(copied, ['The build is green.\n\nSecond paragraph.']);
      expect(heard.last, 'The build is green.\n\nSecond paragraph.');
      // A right click shows no system menu of its own: the message menu is the app's.
      await tester.tapAt(
        _at(tester, _text('Second paragraph.'), 2),
        buttons: kSecondaryButton,
        kind: PointerDeviceKind.mouse,
      );
      await tester.pump(const Duration(milliseconds: 500));
      expect(find.byType(AdaptiveTextSelectionToolbar), findsNothing);
    }, variant: _desktop);

    testWidgets('without a selection callback nothing is selectable', (
      tester,
    ) async {
      await pumpDs(
        tester,
        DsMessage(data, collapseLines: 20, onOpenLink: (_) {}),
      );
      expect(find.byType(SelectionArea), findsNothing);
    }, variant: _desktop);

    testWidgets('Show more stays outside the selection', (tester) async {
      final copied = _clipboard(tester);
      final l = await en();
      await pumpDs(
        tester,
        DsMessage(
          DsMessageData(
            id: 'm1',
            authorName: 'Robin',
            origin: DsMessageOrigin.human,
            kind: DsMessageKind.text,
            timeLabel: '10:42',
            content: [for (var i = 1; i <= 8; i++) 'row $i'].join('\n\n'),
          ),
          collapseLines: 3,
          onOpenLink: (_) {},
          onSelectionChanged: (_) {},
        ),
      );
      expect(find.text(l.messageShowMore), findsOneWidget);
      await _click(tester, _at(tester, _text('row 1'), 1));
      await _shortcut(tester, LogicalKeyboardKey.keyA);
      await _shortcut(tester, LogicalKeyboardKey.keyC);
      expect(copied.single, startsWith('row 1\n\nrow 2'));
      expect(copied.single, isNot(contains(l.messageShowMore)));
    }, variant: _desktop);
  });
}
