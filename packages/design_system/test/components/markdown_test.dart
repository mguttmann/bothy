// SPDX-License-Identifier: Apache-2.0

// SEC-OUT-1 and SEC-OUT-2 at component level: the Markdown renderer executes nothing, loads
// nothing, shows link targets and hands a link to the caller only on activation.

import 'dart:io';

import 'package:design_system/design_system.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

/// Fails the test on any HTTP request.
class _NoNetwork extends HttpOverrides {
  int requests = 0;

  @override
  HttpClient createHttpClient(SecurityContext? context) {
    requests++;
    throw StateError('network request from a component');
  }
}

void main() {
  final opened = <Uri>[];

  Widget md(String source) => DsMarkdown(source, onOpenLink: opened.add);

  setUp(opened.clear);

  testWidgets('raw HTML renders as its literal text', (tester) async {
    await pumpDs(tester, md('<script>alert(1)</script>\n\nok <b>x</b>'));
    expect(allText(tester), contains('<script>alert(1)</script>'));
    expect(allText(tester), contains('<b>x</b>'));
  });

  testWidgets('an img tag renders as text and builds no Image', (tester) async {
    await pumpDs(tester, md('<img src="https://example.com/x.png">'));
    expect(allText(tester), contains('<img src="https://example.com/x.png">'));
    expect(find.byType(Image), findsNothing);
  });

  testWidgets('a Markdown image is a text with the domain and loads nothing', (
    tester,
  ) async {
    final net = _NoNetwork();
    await HttpOverrides.runZoned(() async {
      await pumpDs(tester, md('![x](https://example.com/p.png?d=secret)'));
      await tester.pumpAndSettle();
    }, createHttpClient: net.createHttpClient);
    final l = await en();
    expect(allText(tester), contains(l.markdownImage('x', 'example.com')));
    expect(allText(tester), isNot(contains('secret')));
    expect(find.byType(Image), findsNothing);
    expect(net.requests, 0);
  });

  testWidgets('an image goes to the attachment resolver and nowhere else', (
    tester,
  ) async {
    final asked = <Uri>[];
    await pumpDs(
      tester,
      DsMarkdown(
        '![plan](/files/1)',
        onOpenLink: opened.add,
        attachments: (uri) {
          asked.add(uri);
          return const Text('resolved');
        },
      ),
    );
    expect(asked, [Uri.parse('/files/1')]);
    expect(find.text('resolved'), findsOneWidget);
  });

  testWidgets('javascript: and data: links are plain text', (tester) async {
    await pumpDs(tester, md('[a](javascript:alert(1)) [b](data:text/html,x)'));
    expect(find.byType(DsMarkdownLink), findsNothing);
    expect(allText(tester), contains('a'));
    expect(allText(tester), contains('b'));
    for (final r in tester.widgetList<RichText>(find.byType(RichText))) {
      r.text.visitChildren((span) {
        expect((span as TextSpan).recognizer, isNull);
        return true;
      });
    }
  });

  testWidgets('file: and relative links are plain text', (tester) async {
    await pumpDs(
      tester,
      md('[f](file:///etc/passwd) [r](../x) [p](//example.com/x)'),
    );
    expect(find.byType(DsMarkdownLink), findsNothing);
  });

  testWidgets('a link shows its domain on hover and opens only on tap', (
    tester,
  ) async {
    await pumpDs(tester, md('[c](https://example.com/path)'));
    final link = find.byType(DsMarkdownLink);
    expect(link, findsOneWidget);
    expect(opened, isEmpty);

    final gesture = await tester.createGesture(kind: PointerDeviceKind.mouse);
    await gesture.addPointer(location: Offset.zero);
    await gesture.moveTo(tester.getCenter(link));
    await tester.pumpAndSettle();
    expect(find.text('example.com'), findsOneWidget);
    expect(opened, isEmpty);
    await gesture.removePointer();

    await tester.tap(link);
    await tester.pumpAndSettle();
    expect(opened, [Uri.parse('https://example.com/path')]);
  });

  testWidgets('a focused link shows its domain and opens on Enter', (
    tester,
  ) async {
    await pumpDs(
      tester,
      md('[c](https://example.com/path) and [m](mailto:sam@example.com)'),
    );
    await tester.sendKeyEvent(LogicalKeyboardKey.tab);
    await tester.pumpAndSettle();
    expect(find.text('example.com'), findsOneWidget);
    expect(opened, isEmpty);
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pumpAndSettle();
    expect(opened, [Uri.parse('https://example.com/path')]);
  });

  testWidgets('internal references are active and show their path', (
    tester,
  ) async {
    await pumpDs(tester, md('[t](/t/0190?m=1) [c](/c/0191)'));
    expect(find.byType(DsMarkdownLink), findsNWidgets(2));
    await tester.tap(find.byType(DsMarkdownLink).first);
    expect(opened, [Uri.parse('/t/0190?m=1')]);
  });

  testWidgets('a link whose text names another domain shows the real one', (
    tester,
  ) async {
    await pumpDs(
      tester,
      md('[https://example.org](https://evil.example.com@example.net/x)'),
    );
    final link = tester.widget<DsMarkdownLink>(find.byType(DsMarkdownLink));
    expect(link.target, 'example.net');
  });

  testWidgets('headings render at most in type.heading', (tester) async {
    await pumpDs(tester, md('# Title\n\n###### Small'));
    TextStyle? styleOf(String text) {
      TextStyle? found;
      for (final r in tester.widgetList<RichText>(find.byType(RichText))) {
        r.text.visitChildren((span) {
          if (span is TextSpan && span.text == text) found = span.style;
          return found == null;
        });
      }
      return found;
    }

    expect(styleOf('Title')?.fontSize, DsType.heading.fontSize);
    expect(styleOf('Title')?.fontWeight, DsType.heading.fontWeight);
    expect(
      styleOf('Small')!.fontSize!,
      lessThanOrEqualTo(DsType.heading.fontSize!),
    );
  });

  testWidgets(
    'a fenced block scrolls horizontally and copies exactly the code',
    (tester) async {
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
      const code =
          'make check && echo "a very long line that does not wrap at all"';
      await pumpDs(tester, md('```sh\n$code\n```'), width: 240);
      final scroll = tester.widget<SingleChildScrollView>(
        find.descendant(
          of: find.byType(DsMarkdownCodeBlock),
          matching: find.byType(SingleChildScrollView),
        ),
      );
      expect(scroll.scrollDirection, Axis.horizontal);
      expect(find.text('sh'), findsOneWidget);
      final l = await en();
      await tester.tap(find.bySemanticsLabel(l.markdownCopyCode));
      expect(copied, code);
    },
  );

  testWidgets('lists, quotes, tables, tasks, rules and emphasis render', (
    tester,
  ) async {
    await pumpDs(
      tester,
      md(
        '**bold** *em* ~~gone~~ `code`\n\n'
        '> quote\n\n'
        '- one\n- two\n\n'
        '1. first\n2. second\n\n'
        '- [x] done\n- [ ] open\n\n'
        '---\n\n'
        '| a | b |\n|---|---|\n| 1 | 2 |\n',
      ),
    );
    final text = allText(tester);
    for (final part in [
      'bold',
      'em',
      'gone',
      'code',
      'quote',
      'one',
      'two',
      'first',
      'second',
      'done',
      'open',
      'a',
      '2',
    ]) {
      expect(text, contains(part));
    }
    expect(find.byType(Checkbox), findsNWidgets(2));
    expect(
      tester
          .widgetList<Checkbox>(find.byType(Checkbox))
          .every((c) => c.onChanged == null),
      isTrue,
    );
    expect(find.byType(Table), findsOneWidget);
  });

  testWidgets('an image inside a link keeps its text form in the link', (
    tester,
  ) async {
    final l = await en();
    await pumpDs(
      tester,
      md(
        'see [![build](https://example.com/b.svg)](https://example.com/x) end',
      ),
    );
    final link = tester.widget<DsMarkdownLink>(find.byType(DsMarkdownLink));
    expect(link.text, l.markdownImage('build', 'example.com'));
  });

  testWidgets('a loose task list keeps its boxes', (tester) async {
    await pumpDs(tester, md('- [x] done\n\n- [ ] open'));
    final boxes = tester.widgetList<Checkbox>(find.byType(Checkbox)).toList();
    expect(boxes.map((b) => b.value), [true, false]);
    expect(allText(tester), isNot(contains('•')));
  });
}
