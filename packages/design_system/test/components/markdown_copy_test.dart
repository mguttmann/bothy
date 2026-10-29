// SPDX-License-Identifier: Apache-2.0

// Copying single blocks of a message (01 6.12, R-PR-25; brief M4-48): the content a code block
// and a quote block put on the clipboard, the copy button at the top right of each block on the
// desktop (macOS and Windows), the tap on a block on the phone (iOS and Android) with "Copied"
// and a haptic, nested blocks, and the system selection beside them.

import 'package:design_system/design_system.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:markdown/markdown.dart' as md;

import 'support.dart';

/// Records what the renderer writes.
final class _Recorder implements DsClipboard {
  final List<DsCopyData> writes = [];

  @override
  Future<void> write(DsCopyData data) async => writes.add(data);
}

md.Element _first(String source, String tag) {
  md.Element? found;
  void walk(md.Node n) {
    if (found != null || n is! md.Element) return;
    if (n.tag == tag) {
      found = n;
      return;
    }
    (n.children ?? const <md.Node>[]).forEach(walk);
  }

  DsMarkdown.parse(source).forEach(walk);
  return found!;
}

String _code(String source) => dsCodeBlockText(_first(source, 'pre'));

DsCopyData _quote(String source) => dsQuoteCopy(_first(source, 'blockquote'));

const _desktop = TargetPlatformVariant({
  TargetPlatform.macOS,
  TargetPlatform.windows,
});

const _phone = TargetPlatformVariant({
  TargetPlatform.iOS,
  TargetPlatform.android,
});

/// A message body with a code block, a quote with a nested code block and a nested quote, and
/// a link inside the quote.
const _message =
    'Intro\n\n'
    '```dart\n\tfinal x = 1;\n```\n\n'
    '> outer **bold** [site](https://example.com/a)\n'
    '>\n'
    '> ```\n'
    '> inner code\n'
    '> ```\n'
    '>\n'
    '> > inner quote\n';

/// Pumps [child] in a scaffold (the "Copied" confirmation is a snack bar of the app's
/// scaffold) at [width] in the density of a desktop or a phone.
Future<void> _host(
  WidgetTester tester,
  Widget child, {
  bool touch = false,
  double width = 400,
}) async {
  await tester.pumpWidget(
    MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: DsThemeScope(
        dsContext: testContext(
          density: touch ? DsDensityMode.touch : DsDensityMode.comfortable,
        ),
        child: Scaffold(
          body: Align(
            alignment: Alignment.topLeft,
            child: SizedBox(
              width: width,
              child: SingleChildScrollView(child: child),
            ),
          ),
        ),
      ),
    ),
  );
  await tester.pump();
}

Future<_Recorder> _pump(
  WidgetTester tester,
  String source, {
  required bool touch,
  List<Uri>? opened,
  VoidCallback? onLongPress,
}) async {
  final recorder = _Recorder();
  await _host(
    tester,
    DsClipboardScope(
      clipboard: recorder,
      child: GestureDetector(
        onLongPress: onLongPress,
        child: DsMarkdown(source, onOpenLink: (u) => opened?.add(u)),
      ),
    ),
    touch: touch,
  );
  return recorder;
}

/// The copy button of [block] itself: its last button, because the buttons of nested blocks
/// come first in the tree and a block's own button follows its content.
Finder _ownButton(Finder block) =>
    find.descendant(of: block, matching: find.byType(DsTappable)).last;

void _expectTopRight(WidgetTester tester, Finder button, Finder block) {
  final b = tester.getRect(button);
  final r = tester.getRect(block);
  // Within the hairline border of a code block.
  expect(b.top, moreOrLessEquals(r.top, epsilon: 1));
  expect(b.right, moreOrLessEquals(r.right, epsilon: 1));
}

/// Records the haptic calls of the platform channel.
List<String> _haptics(WidgetTester tester) {
  final calls = <String>[];
  tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
    SystemChannels.platform,
    (call) async {
      if (call.method == 'HapticFeedback.vibrate') {
        calls.add(call.arguments as String);
      }
      return null;
    },
  );
  return calls;
}

void main() {
  group('code block content', () {
    test(
      'keeps tabs, indentation, trailing spaces and unicode byte for byte',
      () {
        const body =
            '\tif (a) {\n'
            '\t\treturn "ü \u{1F600} אב é  x";  \n'
            '    }\n'
            '​\u{10FFFF}';
        expect(_code('```js\n$body\n```'), body);
      },
    );

    test('a long line stays one line', () {
      final line = List.filled(5000, 'abc\t').join();
      expect(_code('```\n$line\n```'), line);
      expect(_code('```\n$line\n```').contains('\n'), isFalse);
    });

    test('no language tag and no fence markers', () {
      expect(_code('```python title\nprint(1)\n```'), 'print(1)');
      expect(_code('~~~~sh\necho ```\n~~~~'), 'echo ```');
    });

    test('drops only the line break that ends the last line', () {
      expect(_code('```\na\n```'), 'a');
      expect(_code('```\na\n\n\n```'), 'a\n\n');
      expect(_code('```\n\n```'), '');
      expect(_code('```\n```'), '');
    });

    test('CR LF line breaks copy as LF', () {
      expect(_code('```\r\na\r\nb\r\n```'), 'a\nb');
    });

    test('an indented fence removes only its own indentation', () {
      expect(_code('  ```\n    a\n  b\n  ```'), '  a\nb');
    });

    test('a block inside a quote copies without the quote markers', () {
      expect(_code('> ```\n> \tx\n>   y\n> ```'), '\tx\n  y');
    });

    test('an unclosed fence copies to the end of the message', () {
      expect(_code('```\na\nb'), 'a\nb');
    });
  });

  group('quote block content', () {
    test('plain text without markers, with line breaks and inline text', () {
      final copy = _quote(
        '> line one\n'
        '> line **two** with `code` and *em*\n'
        '>\n'
        '> second [link](https://example.com/x) ![alt](https://example.com/i.png)',
      );
      expect(
        copy.text,
        'line one\nline two with code and em\n\nsecond link alt',
      );
    });

    test('hard breaks, lists, tasks, code and tables', () {
      final copy = _quote(
        '> a  \n> b\n>\n'
        '> - one\n> - [x] done\n>\n'
        '> 3. three\n> 4. four\n>\n'
        '> ```\n> \tcode\n> ```\n>\n'
        '> | h1 | h2 |\n> |---|---|\n> | 1 | 2 |',
      );
      expect(
        copy.text,
        'a\nb\n\n'
        '- one\n- [x] done\n\n'
        '3. three\n4. four\n\n'
        '\tcode\n\n'
        'h1\th2\n1\t2',
      );
    });

    test('a nested quote is text of its parent', () {
      expect(_quote('> outer\n>\n> > inner').text, 'outer\n\ninner');
    });

    test('the HTML form keeps the formatting and escapes text', () {
      final copy = _quote(
        '> **b** *i* ~~d~~ `<c>` a & b\n'
        '> next line [l](https://example.com/?q=1&r="2") [j](javascript:x)'
        ' [t](/t/1)\n'
        '>\n'
        '> ```\n> <tag>\n> ```',
      );
      expect(
        copy.html,
        '<p><strong>b</strong> <em>i</em> <del>d</del> <code>&lt;c&gt;</code> '
        'a &amp; b<br>next line '
        '<a href="https://example.com/?q=1&amp;r=%222%22">l</a> j t</p>'
        '<pre><code>&lt;tag&gt;</code></pre>',
      );
    });

    test('an image never becomes an img element', () {
      final copy = _quote('> ![x](https://example.com/p.png)');
      expect(copy.html, isNot(contains('<img')));
      expect(copy.html, '<p>x</p>');
    });
  });

  group('desktop', () {
    testWidgets(
      'every code and quote block has a copy button at its top right',
      (tester) async {
        await _pump(tester, _message, touch: false);
        final l = await en();
        final codeButtons = find.bySemanticsLabel(l.markdownCopyCode);
        final quoteButtons = find.bySemanticsLabel(l.markdownCopyQuote);
        // The top level code block and the code block inside the quote.
        expect(codeButtons, findsNWidgets(2));
        // The outer and the inner quote.
        expect(quoteButtons, findsNWidgets(2));

        final block = find.byType(DsMarkdownCodeBlock).first;
        _expectTopRight(tester, _ownButton(block), block);
        final outer = find.byType(DsMarkdownQuoteBlock).first;
        _expectTopRight(tester, _ownButton(outer), outer);
        final inner = find.byType(DsMarkdownQuoteBlock).last;
        _expectTopRight(tester, _ownButton(inner), inner);
      },
      variant: _desktop,
    );

    testWidgets('each button copies exactly its own block and says Copied', (
      tester,
    ) async {
      final haptics = _haptics(tester);
      final recorder = await _pump(tester, _message, touch: false);
      final l = await en();

      await tester.tap(_ownButton(find.byType(DsMarkdownCodeBlock).first));
      await tester.pump();
      expect(recorder.writes.last, const DsCopyData('\tfinal x = 1;'));
      expect(find.text(l.messageCopied), findsOneWidget);

      // The code block nested in the quote copies only itself.
      await tester.tap(_ownButton(find.byType(DsMarkdownCodeBlock).last));
      expect(recorder.writes.last, const DsCopyData('inner code'));

      // The outer quote copies all of its content, the inner only its own.
      await tester.tap(_ownButton(find.byType(DsMarkdownQuoteBlock).first));
      expect(
        recorder.writes.last.text,
        'outer bold site\n\ninner code\n\ninner quote',
      );
      expect(
        recorder.writes.last.html,
        contains('<a href="https://example.com/a">site</a>'),
      );
      await tester.tap(_ownButton(find.byType(DsMarkdownQuoteBlock).last));
      expect(recorder.writes.last.text, 'inner quote');
      expect(recorder.writes, hasLength(4));
      // A desktop copy gives no haptic.
      expect(haptics, isEmpty);
    }, variant: _desktop);

    testWidgets('a click on the text of a block copies nothing', (
      tester,
    ) async {
      final recorder = await _pump(tester, _message, touch: false);
      await tester.tap(find.textContaining('final x'));
      await tester.tap(find.textContaining('inner quote'));
      await tester.pump();
      expect(recorder.writes, isEmpty);
    }, variant: _desktop);

    testWidgets('the copy button is reachable by keyboard', (tester) async {
      final recorder = await _pump(tester, '```\nx\n```', touch: false);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await tester.pump();
      expect(recorder.writes, [const DsCopyData('x')]);
    }, variant: _desktop);

    testWidgets('the system selection still selects text across the blocks', (
      tester,
    ) async {
      String? copied;
      tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
        SystemChannels.platform,
        (call) async {
          if (call.method == 'Clipboard.setData') {
            copied = (call.arguments as Map)['text'] as String;
          }
          return null;
        },
      );
      await _host(
        tester,
        SelectionArea(
          child: DsMarkdown(
            'a\n\n```\ncode\n```\n\n> quoted',
            onOpenLink: (_) {},
          ),
        ),
      );
      // Drag from the first line to the quote.
      final start = tester.getTopLeft(find.text('a'));
      final end = tester.getBottomRight(find.textContaining('quoted'));
      final gesture = await tester.startGesture(
        start + const Offset(1, 2),
        kind: PointerDeviceKind.mouse,
      );
      await tester.pump();
      await gesture.moveTo(end - const Offset(1, 2));
      await tester.pump();
      await gesture.up();
      await tester.pump();
      final mod = defaultTargetPlatform == TargetPlatform.macOS
          ? LogicalKeyboardKey.meta
          : LogicalKeyboardKey.control;
      await tester.sendKeyDownEvent(mod);
      await tester.sendKeyEvent(LogicalKeyboardKey.keyC);
      await tester.sendKeyUpEvent(mod);
      await tester.pump();
      expect(copied, isNotNull);
      expect(copied, contains('code'));
      expect(copied, contains('quote'));
      // The labels of the copy buttons are not part of a selection.
      final l = await en();
      expect(copied, isNot(contains(l.markdownCopyCode)));
      expect(copied, isNot(contains(l.markdownCopyQuote)));
    }, variant: _desktop);
  });

  group('phone', () {
    testWidgets('a tap on a code block copies it with Copied and a haptic', (
      tester,
    ) async {
      final haptics = _haptics(tester);
      final recorder = await _pump(tester, _message, touch: true);
      final l = await en();
      // No copy button on the phone; the block itself is the target.
      expect(find.text(l.markdownCopyCode), findsNothing);
      await tester.tap(find.textContaining('final x'));
      await tester.pump();
      expect(recorder.writes, [const DsCopyData('\tfinal x = 1;')]);
      expect(find.text(l.messageCopied), findsOneWidget);
      expect(haptics, isNotEmpty);
    }, variant: _phone);

    testWidgets('a tap on a quote copies the quote; nested blocks only '
        'themselves', (tester) async {
      _haptics(tester);
      final recorder = await _pump(tester, _message, touch: true);
      await tester.tap(find.textContaining('outer'));
      await tester.pump();
      expect(
        recorder.writes.last.text,
        'outer bold site\n\ninner code\n\ninner quote',
      );
      expect(recorder.writes.last.html, contains('<strong>bold</strong>'));
      await tester.tap(find.textContaining('inner code'));
      await tester.pump();
      expect(recorder.writes.last, const DsCopyData('inner code'));
      await tester.tap(find.textContaining('inner quote'));
      await tester.pump();
      expect(recorder.writes.last.text, 'inner quote');
      expect(recorder.writes, hasLength(3));
    }, variant: _phone);

    testWidgets('a tap on a link inside a quote opens it and copies nothing', (
      tester,
    ) async {
      _haptics(tester);
      final opened = <Uri>[];
      final recorder = await _pump(
        tester,
        _message,
        touch: true,
        opened: opened,
      );
      await tester.tap(find.byType(DsMarkdownLink));
      await tester.pump();
      expect(opened, [Uri.parse('https://example.com/a')]);
      expect(recorder.writes, isEmpty);
    }, variant: _phone);

    testWidgets('a long press reaches the message menu and copies nothing', (
      tester,
    ) async {
      _haptics(tester);
      var menus = 0;
      final recorder = await _pump(
        tester,
        _message,
        touch: true,
        onLongPress: () => menus++,
      );
      await tester.longPress(find.textContaining('final x'));
      await tester.longPress(find.textContaining('outer'));
      await tester.pump();
      expect(menus, 2);
      expect(recorder.writes, isEmpty);
    }, variant: _phone);

    testWidgets('a horizontal drag scrolls a code block and copies nothing', (
      tester,
    ) async {
      _haptics(tester);
      final line = List.filled(60, 'wide').join(' ');
      final recorder = await _pump(tester, '```\n$line\n```', touch: true);
      await tester.drag(find.textContaining('wide'), const Offset(-200, 0));
      await tester.pumpAndSettle();
      expect(recorder.writes, isEmpty);
    }, variant: _phone);

    testWidgets('a screen reader can copy a block by its action', (
      tester,
    ) async {
      _haptics(tester);
      final handle = tester.ensureSemantics();
      final recorder = await _pump(tester, '```\nx\n```', touch: true);
      final l = await en();
      final node = tester.getSemantics(
        find.bySemanticsLabel(l.markdownCopyCode),
      );
      expect(node.getSemanticsData().hasAction(SemanticsAction.tap), isTrue);
      tester.semantics.tap(find.semantics.byLabel(l.markdownCopyCode));
      await tester.pump();
      expect(recorder.writes, [const DsCopyData('x')]);
      handle.dispose();
    }, variant: _phone);
  });

  testWidgets('without a scope the plain text goes to the system clipboard', (
    tester,
  ) async {
    String? copied;
    tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
      SystemChannels.platform,
      (call) async {
        if (call.method == 'Clipboard.setData') {
          copied = (call.arguments as Map)['text'] as String;
        }
        return null;
      },
    );
    await _host(tester, DsMarkdown('> a **b**', onOpenLink: (_) {}));
    final l = await en();
    await tester.tap(find.bySemanticsLabel(l.markdownCopyQuote));
    await tester.pump();
    expect(copied, 'a b');
  });
}
