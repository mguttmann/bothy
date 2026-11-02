// SPDX-License-Identifier: Apache-2.0

// What the document viewer of attachments needs from the one Markdown renderer (ticket
// FEAT-markdown-viewer): syntax colors in code blocks (messages too), the marks of a search with
// the same count as the renderer draws, links that a caller activates (a relative link to
// another attachment, an anchor), a document parsed once and rendered block by block, and its
// headings with their anchors.

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

/// Every [TextSpan] with text below the root, flattened.
List<TextSpan> _spans(WidgetTester tester) {
  final out = <TextSpan>[];
  for (final rich in tester.widgetList<RichText>(find.byType(RichText))) {
    rich.text.visitChildren((span) {
      if (span is TextSpan && (span.text?.isNotEmpty ?? false)) out.add(span);
      return true;
    });
  }
  return out;
}

DsColors _colors(WidgetTester tester) =>
    tester.element(find.byType(DsMarkdown).first).dsColors;

void main() {
  group('dsCodeTokens', () {
    String kinds(String code, String language) => dsCodeTokens(code, language)
        .where((r) => r.token != DsCodeToken.plain)
        .map((r) => '${r.token.name}:${code.substring(r.start, r.end)}')
        .join(' ');

    test('colors keywords, types, strings, numbers and comments of Dart', () {
      expect(
        kinds("final Foo x = 'a'; // note\nreturn 42;", 'dart'),
        "keyword:final type:Foo string:'a' comment:// note keyword:return "
        'number:42',
      );
    });

    test('knows the comments of Python and shell and the words of SQL', () {
      expect(kinds('def f(): # hi', 'python'), 'keyword:def comment:# hi');
      expect(
        kinds('echo "x" # done', 'bash'),
        'keyword:echo string:"x" '
        'comment:# done',
      );
      expect(
        kinds('SELECT 1 -- n', 'sql'),
        'keyword:SELECT number:1 '
        'comment:-- n',
      );
    });

    test('colors the lines of a diff and leaves unknown languages plain', () {
      expect(
        kinds('@@ -1 +1 @@\n-old\n+new\n same', 'diff'),
        'keyword:@@ -1 +1 @@\n deleted:-old\n inserted:+new\n',
      );
      expect(kinds('final x = 1;', 'brainfog'), isEmpty);
      expect(kinds('final x = 1;', ''), isEmpty);
    });

    test('runs cover the code without gaps, also for broken input', () {
      for (final code in ['"open string\nnext', '/* never closed', r'a\', '']) {
        final runs = dsCodeTokens(code, 'js');
        var at = 0;
        for (final r in runs) {
          expect(r.start, at);
          at = r.end;
        }
        expect(at, code.length);
      }
    });
  });

  testWidgets('a code block of a known language is colored, the copy is not', (
    tester,
  ) async {
    await pumpDs(
      tester,
      DsMarkdown("```dart\nfinal x = 'a';\n```", onOpenLink: (_) {}),
    );
    final colors = _colors(tester);
    final spans = _spans(tester);
    expect(
      spans.where(
        (s) => s.text == 'final' && s.style?.color == colors.categoryHeatherFg,
      ),
      isNotEmpty,
    );
    expect(
      spans.where(
        (s) => s.text == "'a'" && s.style?.color == colors.categoryMossFg,
      ),
      isNotEmpty,
    );
  });

  group('search', () {
    const source =
        '# Token guide\n\n'
        'A token and `token()` and [the token](https://example.com).\n\n'
        '| Col | Token |\n|---|---|\n| token | x |\n\n'
        '```\ntoken = 1\n```\n';

    testWidgets('marks every match, the current one stronger, and counts as it '
        'draws', (tester) async {
      final doc = DsMarkdownDocument.parse(source);
      await pumpDs(
        tester,
        Builder(
          builder: (context) {
            final blocks = <Widget>[];
            var first = 0;
            for (var i = 0; i < doc.length; i++) {
              blocks.add(
                DsMarkdown.block(
                  doc,
                  i,
                  onOpenLink: (_) {},
                  search: DsTextSearch('TOKEN', active: 2, first: first),
                ),
              );
              first += DsMarkdown.countMatches(context, doc, i, 'token');
            }
            return Column(children: blocks);
          },
        ),
      );
      final colors = _colors(tester);
      final marked = _spans(tester).where(
        (s) =>
            s.style?.backgroundColor == colors.attentionSubtle ||
            s.style?.backgroundColor == colors.attentionBg,
      );
      // Heading 1, paragraph 3 (text, inline code, link), table 2, code block 1.
      expect(marked, hasLength(7));
      expect(marked.every((s) => s.text!.toLowerCase() == 'token'), isTrue);
      final active = marked.where(
        (s) => s.style?.backgroundColor == colors.attentionBg,
      );
      expect(active, hasLength(1));
      expect(active.single.text, 'token');
      expect(active.single.style?.color, colors.attentionOnBg);
      var total = 0;
      final context = tester.element(find.byType(Column).first);
      for (var i = 0; i < doc.length; i++) {
        total += DsMarkdown.countMatches(context, doc, i, 'token');
      }
      expect(total, 7);
    });

    testWidgets('an empty query marks nothing', (tester) async {
      await pumpDs(
        tester,
        DsMarkdown(source, onOpenLink: (_) {}, search: const DsTextSearch('')),
      );
      final colors = _colors(tester);
      expect(
        _spans(tester)
            .where((s) => s.style?.backgroundColor == colors.attentionSubtle),
        isEmpty,
      );
    });

    test('dsMatchRanges ignores case and never overlaps', () {
      expect(dsMatchRanges('aAa', 'aa'), [(0, 2)]);
      expect(dsMatchRanges('Straße STRASSE', 'straße'), [(0, 6)]);
      expect(dsMatchRanges('x', ''), isEmpty);
    });
  });

  group('links', () {
    testWidgets('a relative link stays text unless the caller resolves it', (
      tester,
    ) async {
      final opened = <Uri>[];
      await pumpDs(
        tester,
        DsMarkdown('[notes](notes.md)', onOpenLink: opened.add),
      );
      expect(find.byType(DsMarkdownLink), findsNothing);

      await pumpDs(
        tester,
        DsMarkdown(
          '[notes](notes.md) and [up](#intro)',
          onOpenLink: opened.add,
          resolveLink: (href) => href == 'notes.md'
              ? DsLinkTarget(Uri.parse('attachment:42'), 'notes.md')
              : null,
        ),
      );
      expect(find.byType(DsMarkdownLink), findsOneWidget);
      await tester.tap(find.byType(DsMarkdownLink));
      expect(opened, [Uri.parse('attachment:42')]);
    });

    testWidgets('a javascript link stays inactive even with a resolver that '
        'refuses it', (tester) async {
      await pumpDs(
        tester,
        DsMarkdown(
          '[x](javascript:alert(1))',
          onOpenLink: (_) => fail('opened'),
          resolveLink: (_) => null,
        ),
      );
      expect(find.byType(DsMarkdownLink), findsNothing);
    });
  });

  group('DsMarkdownDocument', () {
    test('lists the headings with level, block and GitHub anchors', () {
      final doc = DsMarkdownDocument.parse(
        '# Intro\n\ntext\n\n## Set up, then run!\n\n## Intro\n\n### Intro\n',
      );
      expect(doc.length, 5);
      expect(
        [for (final h in doc.headings) (h.level, h.text, h.block, h.anchor)],
        [
          (1, 'Intro', 0, 'intro'),
          (2, 'Set up, then run!', 2, 'set-up-then-run'),
          (2, 'Intro', 3, 'intro-1'),
          (3, 'Intro', 4, 'intro-2'),
        ],
      );
    });

    testWidgets('DsMarkdown.block renders one top-level block', (tester) async {
      final doc = DsMarkdownDocument.parse('# One\n\nTwo\n\n- three\n');
      await pumpDs(tester, DsMarkdown.block(doc, 2, onOpenLink: (_) {}));
      expect(allText(tester), contains('three'));
      expect(allText(tester), isNot(contains('One')));
      expect(allText(tester), isNot(contains('Two')));
    });
  });
}
