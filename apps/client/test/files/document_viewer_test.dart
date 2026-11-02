// SPDX-License-Identifier: Apache-2.0

// The in-app document viewer of text attachments (ticket FEAT-markdown-viewer): a tap on a
// Markdown attachment opens the viewer instead of the system app, a blocked file never opens,
// relative links and images reach the other attachments of the message, the toolbar toggles raw
// text, searches with marks, copies, saves and hands the file to another app, the table of
// contents jumps to a heading, and the keys of a reader work.

import 'dart:convert';

import 'package:client/files/image_preview.dart';
import 'package:client/files/viewer/document_content.dart';
import 'package:client/files/viewer/document_view.dart';
import 'package:client/thread/history_column.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/document_samples.dart';
import '../support/file_fakes.dart';
import '../support/file_widgets.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/widgets.dart';

Finder _key(String key) => find.byKey(ValueKey(key));

/// Lets a dialog or a sheet run its transition: the first frame starts its clock.
Future<void> _animate(WidgetTester tester) async {
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 500));
  await tester.pump();
}

void main() {
  group('from the history', () {
    Future<(ColumnHost, FileTestServices, FakeOpener)> open(
      WidgetTester tester, {
      required List<FileMeta> files,
      Map<FileMeta, String> bytes = const {},
      String platform = 'macos',
    }) async {
      final host = await ColumnHost.open(tester);
      final opener = FakeOpener();
      final services = FileTestServices(
        host.db,
        host.root,
        fakeFilePorts(opener: opener),
      );
      await host.run(tester, () async {
        await seedThread(host.db, messages: [msg(1, text: 'The notes')]);
        await attachFiles(host.db, 1, files);
      });
      for (final MapEntry(key: f, value: text) in bytes.entries) {
        services.adapter.on(
          '/files/${f.id.value}/content',
          ScriptedAnswer.file(utf8.encode(text)),
        );
      }
      await host.pump(
        tester,
        HistoryColumn(threadId: threadId(1)),
        location: '/t/${threadId(1).value}',
        platform: platform,
        overrides: [
          ...threadOverrides(FakeThreadReads(host.db)),
          ...services.overrides,
        ],
      );
      return (host, services, opener);
    }

    final notes = fileOf(1, name: 'notes.md', mime: 'text/markdown');
    final setup = fileOf(2, name: 'setup.md', mime: 'text/markdown');
    final diagram = fileOf(3, name: 'diagram.png', mime: 'image/png');

    testWidgets('Open on a Markdown attachment opens the viewer, not the '
        'system app', (tester) async {
      final (host, services, opener) = await open(
        tester,
        files: [notes],
        bytes: {notes: sampleMarkdown},
      );
      await tester.tap(_key('file.${notes.id.value}.open'));
      await settleUntil(tester, host, _key('document.rendered'));

      expect(_key('document.viewer'), findsOneWidget);
      expect(opener.opened, isEmpty);
      expect(
        services.adapter.requested,
        contains('/files/${notes.id.value}/content'),
      );
      // Every element of the sample is rendered by the renderer of messages.
      expect(find.text('Release notes'), findsWidgets);
      expect(find.byType(DsMarkdownCodeBlock), findsOneWidget);
      expect(find.byType(DsMarkdownQuoteBlock), findsOneWidget);
      expect(find.byType(Table), findsOneWidget);
      expect(find.byType(Checkbox), findsNWidgets(2));
      expect(find.byType(Divider), findsWidgets);
      expect(find.textContaining('<b>no HTML</b>'), findsOneWidget);
      // A web image never loads without a click.
      expect(_key('document.remoteImage'), findsOneWidget);
      expect(_key('document.remoteImage.loaded'), findsNothing);
      await host.dispose(tester);
    });

    testWidgets('a tap on the tile opens a text file too, and Esc closes', (
      tester,
    ) async {
      final answer = fileOf(1, name: 'answer.txt', mime: 'text/plain');
      final (host, _, opener) = await open(
        tester,
        files: [answer],
        bytes: {answer: 'The long answer\nin two lines'},
      );
      await tester.tap(find.textContaining('answer.txt'));
      await settleUntil(tester, host, _key('document.raw'));
      expect(find.textContaining('The long answer'), findsOneWidget);
      // Plain text has no formatted form.
      expect(_key('document.toggle'), findsNothing);
      expect(opener.opened, isEmpty);

      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      // The dialog leaves with its exit transition.
      await _animate(tester);
      expect(_key('document.viewer'), findsNothing);
      await host.dispose(tester);
    });

    testWidgets('a blocked Markdown file does not open', (tester) async {
      final blocked = fileOf(
        1,
        name: 'notes.md',
        mime: 'text/markdown',
        scan: ScanStatus.infected,
      );
      final (host, services, _) = await open(tester, files: [blocked]);
      expect(_key('file.${blocked.id.value}.open'), findsNothing);
      await tester.tap(find.textContaining('notes.md'));
      await host.settle(tester);
      expect(_key('document.viewer'), findsNothing);
      expect(services.adapter.requested, isEmpty);
      await host.dispose(tester);
    });

    testWidgets('relative links and images reach the other attachments of '
        'the message', (tester) async {
      final (host, services, _) = await open(
        tester,
        files: [notes, setup, diagram],
        bytes: {notes: sampleMarkdown, setup: '# Setup\n\nRun it.'},
      );
      services.adapter.on(
        '/files/${diagram.id.value}/thumbnails/medium',
        ScriptedAnswer.file(onePixelPng),
      );
      await tester.tap(_key('file.${notes.id.value}.open'));
      await settleUntil(tester, host, _key('document.rendered'));
      final image = find.descendant(
        of: _key('document.rendered'),
        matching: find.byType(ImagePreview),
      );
      await tester.scrollUntilVisible(
        find.text('the setup'),
        200,
        scrollable: find
            .descendant(
              of: _key('document.rendered'),
              matching: find.byType(Scrollable),
            )
            .first,
      );
      expect(image, findsOneWidget);

      await tester.tap(find.text('the setup'));
      await settleUntil(tester, host, find.text('Run it.'));
      expect(_key('document.viewer'), findsNWidgets(2));
      expect(find.text('Run it.'), findsOneWidget);
      await host.dispose(tester);
    });

    testWidgets('Open in another app hands the checked file to the system', (
      tester,
    ) async {
      final (host, _, opener) = await open(
        tester,
        files: [notes],
        bytes: {notes: sampleMarkdown},
      );
      await tester.tap(_key('file.${notes.id.value}.open'));
      await settleUntil(tester, host, _key('document.rendered'));
      await tester.tap(_key('document.external'));
      await host.settle(tester);
      expect(opener.opened, hasLength(1));
      expect(opener.opened.single, endsWith('notes.md'));
      await host.dispose(tester);
    });

    testWidgets('on the phone the viewer is full screen and the contents are '
        'a sheet', (tester) async {
      final (host, _, _) = await open(
        tester,
        files: [notes],
        bytes: {notes: sampleMarkdown},
        platform: 'ios',
      );
      await tester.tap(_key('file.${notes.id.value}.open'));
      await settleUntil(tester, host, _key('document.rendered'));
      expect(find.byType(Dialog), findsOneWidget);
      expect(
        tester.getSize(_key('document.viewer')).width,
        tester.view.physicalSize.width / tester.view.devicePixelRatio,
      );
      // No side panel on a phone.
      expect(_key('document.toc'), findsNothing);
      await tester.tap(_key('document.contents'));
      await _animate(tester);
      expect(_key('document.toc'), findsOneWidget);
      await tester.tap(_key('document.toc.2'));
      await _animate(tester);
      expect(_key('document.toc'), findsNothing);
      await host.dispose(tester);
    });
  });

  group('the view', () {
    String? copied;

    Future<List<int>> pumpView(
      WidgetTester tester,
      DocumentContent content, {
      String name = 'notes.md',
      Size size = const Size(1440, 1000),
    }) async {
      tester.view
        ..physicalSize = size
        ..devicePixelRatio = 1;
      addTearDown(tester.view.reset);
      copied = null;
      tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
        SystemChannels.platform,
        (call) async {
          if (call.method == 'Clipboard.setData') {
            copied = (call.arguments as Map)['text'] as String;
          }
          return null;
        },
      );
      final closed = <int>[];
      await tester.pumpWidget(
        hostScreen(
          DocumentView(
            name: name,
            content: content,
            onClose: () => closed.add(1),
            onOpenLink: (_) {},
          ),
        ),
      );
      await tester.pump();
      return closed;
    }

    DocumentContent md(String text) => DocumentContent.prepare(
      text,
      DocumentKind.markdown,
      totalBytes: utf8.encode(text).length,
    );

    Future<void> search(WidgetTester tester, String query) async {
      await tester.sendKeyDownEvent(LogicalKeyboardKey.meta);
      await tester.sendKeyEvent(LogicalKeyboardKey.keyF);
      await tester.sendKeyUpEvent(LogicalKeyboardKey.meta);
      await tester.pump();
      await tester.pump();
      expect(_key('document.search.field'), findsOneWidget);
      await tester.enterText(_key('document.search.field'), query);
      await tester.pump();
      await tester.pump();
    }

    String count(WidgetTester tester) =>
        tester.widget<Text>(_key('document.search.count')).data!;

    List<TextSpan> marked(WidgetTester tester, Color background) {
      final out = <TextSpan>[];
      for (final rich in tester.widgetList<RichText>(find.byType(RichText))) {
        rich.text.visitChildren((span) {
          if (span is TextSpan && span.style?.backgroundColor == background) {
            out.add(span);
          }
          return true;
        });
      }
      return out;
    }

    testWidgets('the toggle switches between formatted and raw text', (
      tester,
    ) async {
      await pumpView(tester, md(sampleMarkdown));
      expect(_key('document.rendered'), findsOneWidget);
      expect(find.text(en.documentShowRaw), findsOneWidget);

      await tester.tap(_key('document.toggle'));
      await tester.pump();
      expect(_key('document.raw'), findsOneWidget);
      expect(find.textContaining('| Step | Owner | Status |'), findsOneWidget);
      expect(find.textContaining('# Release notes'), findsOneWidget);

      await tester.tap(_key('document.toggle'));
      await tester.pump();
      expect(_key('document.rendered'), findsOneWidget);
    });

    testWidgets('Cmd+F searches with marks, Enter and the buttons step, Esc '
        'closes the search and then the viewer', (tester) async {
      final closed = await pumpView(tester, md(sampleMarkdown));
      await search(tester, 'step');
      final colors = tester.element(_key('document.rendered')).dsColors;
      // "Steps", "check" no, "Step" in the table header, "step" in the quote and the code.
      final total = marked(tester, colors.attentionSubtle).length + 1;
      expect(count(tester), en.documentSearchCount(1, total));
      expect(marked(tester, colors.attentionBg), hasLength(1));
      expect(marked(tester, colors.attentionBg).single.text, 'Step');

      await tester.testTextInput.receiveAction(TextInputAction.search);
      await tester.pump();
      expect(count(tester), en.documentSearchCount(2, total));
      await tester.tap(_key('document.search.previous'));
      await tester.pump();
      expect(count(tester), en.documentSearchCount(1, total));
      await tester.tap(_key('document.search.previous'));
      await tester.pump();
      expect(count(tester), en.documentSearchCount(total, total));

      await tester.enterText(_key('document.search.field'), 'nothing like it');
      await tester.pump();
      expect(count(tester), en.documentSearchNone);

      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await tester.pump();
      expect(_key('document.search.field'), findsNothing);
      expect(marked(tester, colors.attentionSubtle), isEmpty);
      expect(closed, isEmpty);
      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await tester.pump();
      expect(closed, [1]);
    });

    testWidgets('the search of the raw text finds the Markdown source', (
      tester,
    ) async {
      await pumpView(tester, md(sampleMarkdown));
      await tester.tap(_key('document.toggle'));
      await tester.pump();
      await search(tester, '| Build |');
      expect(count(tester), en.documentSearchCount(1, 1));
    });

    testWidgets('a match far down scrolls into view', (tester) async {
      final long = [
        for (var i = 0; i < 300; i++) 'Paragraph $i of filler text.',
        'The needle is here.',
      ].join('\n\n');
      await pumpView(tester, md(long));
      expect(find.textContaining('needle'), findsNothing);
      await search(tester, 'needle');
      for (var i = 0; i < 10; i++) {
        await tester.pump();
      }
      final needle = find.descendant(
        of: _key('document.rendered'),
        matching: find.textContaining('needle'),
      );
      expect(needle, findsOneWidget);
      expect(tester.getTopLeft(needle).dy, lessThan(1000));
    });

    testWidgets('the contents are a side panel on a wide screen and jump to '
        'a heading', (tester) async {
      final long = [
        '# Start',
        for (var i = 0; i < 200; i++) 'Filler paragraph $i.',
        '## Middle',
        for (var i = 0; i < 200; i++) 'More filler $i.',
        '## End',
        'The last words.',
      ].join('\n\n');
      await pumpView(tester, md(long));
      expect(_key('document.toc'), findsOneWidget);
      expect(find.text('The last words.'), findsNothing);
      await tester.tap(_key('document.toc.2'));
      for (var i = 0; i < 10; i++) {
        await tester.pump();
      }
      expect(find.text('The last words.'), findsOneWidget);

      // The toolbar hides and shows the panel.
      await tester.tap(_key('document.contents'));
      await tester.pump();
      expect(_key('document.toc'), findsNothing);
    });

    testWidgets('arrows, space and End scroll the document', (tester) async {
      final long = [for (var i = 0; i < 200; i++) 'Line paragraph $i.']
          .join('\n\n');
      await pumpView(tester, md(long));
      ScrollPosition position() => tester
          .state<ScrollableState>(
            find
                .descendant(
                  of: _key('document.rendered'),
                  matching: find.byType(Scrollable),
                )
                .first,
          )
          .position;
      expect(position().pixels, 0);
      await tester.sendKeyEvent(LogicalKeyboardKey.arrowDown);
      await tester.pump();
      final afterArrow = position().pixels;
      expect(afterArrow, greaterThan(0));
      await tester.sendKeyEvent(LogicalKeyboardKey.space);
      await tester.pump();
      expect(position().pixels, greaterThan(afterArrow + 400));
      await tester.sendKeyEvent(LogicalKeyboardKey.end);
      await tester.pump();
      expect(position().pixels, position().maxScrollExtent);
    });

    testWidgets('Copy all copies the whole source', (tester) async {
      await pumpView(tester, md(sampleMarkdown));
      await tester.tap(_key('document.copy'));
      await tester.pump();
      expect(copied, sampleMarkdown);
      expect(_key('document.notice'), findsOneWidget);
    });

    testWidgets('a large file opens raw with a notice and renders on request', (
      tester,
    ) async {
      await pumpView(
        tester,
        DocumentContent.prepare(
          '# Big\n\nbody',
          DocumentKind.markdown,
          totalBytes: documentRenderLimitBytes + 1,
        ),
      );
      expect(_key('document.raw'), findsOneWidget);
      expect(_key('document.large'), findsOneWidget);
      await tester.tap(_key('document.render'));
      await tester.pump();
      expect(_key('document.rendered'), findsOneWidget);
      expect(_key('document.large'), findsNothing);
    });

    testWidgets('JSON is pretty-printed, raw shows the file', (tester) async {
      await pumpView(
        tester,
        DocumentContent.prepare(
          '{"name":"bothy","tags":["a"]}',
          DocumentKind.json,
          totalBytes: 29,
        ),
        name: 'data.json',
      );
      expect(find.textContaining('  "name": "bothy"'), findsOneWidget);
      await tester.tap(_key('document.toggle'));
      await tester.pump();
      expect(
        find.textContaining('{"name":"bothy","tags":["a"]}'),
        findsOneWidget,
      );
    });

    testWidgets('a small CSV is a table, raw shows the file', (tester) async {
      await pumpView(
        tester,
        DocumentContent.prepare(
          'name,role\nAlex,admin\nSam,member\n',
          DocumentKind.csv,
          totalBytes: 32,
        ),
        name: 'people.csv',
      );
      expect(find.byType(Table), findsOneWidget);
      expect(find.text('member'), findsOneWidget);
      await tester.tap(_key('document.toggle'));
      await tester.pump();
      expect(find.byType(Table), findsNothing);
      expect(find.textContaining('Alex,admin'), findsOneWidget);
    });
  });
}
