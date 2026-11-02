// SPDX-License-Identifier: Apache-2.0

// Which attachments the document viewer opens and how it prepares their text (ticket
// FEAT-markdown-viewer): the kinds by name and MIME type, the read limit, pretty JSON, a small
// CSV as an escaped Markdown table, and the size bounds above which a file opens raw.

import 'dart:convert';
import 'dart:io';

import 'package:client/files/viewer/document_content.dart';
import 'package:client/files/viewer/document_viewer.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/file_widgets.dart';

void main() {
  group('documentKindOf', () {
    test('knows the text kinds by extension, in any case', () {
      for (final (name, kind) in [
        ('notes.md', DocumentKind.markdown),
        ('README.MARKDOWN', DocumentKind.markdown),
        ('answer.txt', DocumentKind.text),
        ('build.log', DocumentKind.text),
        ('data.json', DocumentKind.json),
        ('config.yaml', DocumentKind.yaml),
        ('ci.yml', DocumentKind.yaml),
        ('export.csv', DocumentKind.csv),
      ]) {
        expect(
          documentKindOf(
            fileOf(1, name: name, mime: 'application/octet-stream'),
          ),
          kind,
          reason: name,
        );
      }
    });

    test('falls back to the MIME type, and opens nothing else', () {
      expect(
        documentKindOf(fileOf(1, name: 'answer', mime: 'text/markdown')),
        DocumentKind.markdown,
      );
      expect(
        documentKindOf(fileOf(1, name: 'x', mime: 'text/plain; charset=utf-8')),
        DocumentKind.text,
      );
      expect(documentKindOf(fileOf(1, name: 'report.pdf')), isNull);
      expect(
        documentKindOf(fileOf(1, name: 'setup.sh', mime: 'text/x-sh')),
        isNull,
      );
      expect(
        documentKindOf(fileOf(1, name: 'shot.png', mime: 'image/png')),
        isNull,
      );
    });

    test(
      'only a servable file opens in the viewer (scan_status respected)',
      () {
        expect(opensInDocumentViewer(fileOf(1, name: 'a.md')), isTrue);
        for (final scan in [
          ScanStatus.infected,
          ScanStatus.error,
          ScanStatus.pending,
        ]) {
          expect(
            opensInDocumentViewer(fileOf(1, name: 'a.md', scan: scan)),
            isFalse,
            reason: scan.name,
          );
        }
        expect(
          opensInDocumentViewer(
            fileOf(1, name: 'a.md', upload: UploadState.failed),
          ),
          isFalse,
        );
        expect(opensInDocumentViewer(deletedFile(1)), isFalse);
      },
    );
  });

  group('DocumentContent', () {
    test('pretty-prints JSON and JSON Lines, keeps invalid JSON raw', () {
      final json = DocumentContent.prepare(
        '{"a":[1,2],"b":{"c":null}}',
        DocumentKind.json,
        totalBytes: 26,
      );
      expect(json.renderable, isTrue);
      expect(
        json.pretty,
        '{\n  "a": [\n    1,\n    2\n  ],\n  "b": {\n    "c": null\n  }\n}',
      );
      expect(
        prettyJson('{"a":1}\n{"a":2}\n'),
        '{\n  "a": 1\n}\n{\n  "a": 2\n}',
      );
      final broken = DocumentContent.prepare(
        '{"a":',
        DocumentKind.json,
        totalBytes: 5,
      );
      expect(broken.pretty, isNull);
      expect(broken.renderable, isFalse);
    });

    test('turns a small CSV into an escaped table, a large one stays raw', () {
      expect(
        csvAsMarkdown('name,note\nAlex,"a, b"\nSam,"say ""hi"" *now*"\n'),
        '| name | note |\n|---|---|\n| Alex | a, b |\n'
        r'| Sam | say "hi" \*now\* |',
      );
      final big = List.generate(csvTableMaxRows + 1, (i) => '$i,x').join('\n');
      expect(csvAsMarkdown(big), isNull);
      expect(csvAsMarkdown('only one column\nhere'), isNull);
      expect(csvAsMarkdown('a,"never closed\n'), isNull);
    });

    test(
      'a file above the render bound opens raw; a truncated one says so',
      () {
        final large = DocumentContent.prepare(
          '# Big',
          DocumentKind.markdown,
          totalBytes: documentRenderLimitBytes + 1,
        );
        expect(large.large, isTrue);
        expect(large.renderable, isTrue);
        final jsonLarge = DocumentContent.prepare(
          '{}',
          DocumentKind.json,
          totalBytes: documentRenderLimitBytes + 1,
        );
        expect(jsonLarge.pretty, isNull);
        expect(
          DocumentContent.prepare(
            'plain',
            DocumentKind.text,
            totalBytes: 5,
          ).renderable,
          isFalse,
        );
      },
    );

    test('readDocument reads at most the load bound and drops a BOM', () async {
      final dir = Directory.systemTemp.createTempSync('document-content');
      addTearDown(() => dir.deleteSync(recursive: true));
      final small = File('${dir.path}/a.md')
        ..writeAsBytesSync([0xEF, 0xBB, 0xBF, ...utf8.encode('# Hi é')]);
      final read = await readDocument(small, DocumentKind.markdown);
      expect(read.text, '# Hi é');
      expect(read.truncated, isFalse);

      final big = File('${dir.path}/b.log')
        ..writeAsBytesSync(List.filled(documentMaxLoadBytes + 10, 0x61));
      final cut = await readDocument(big, DocumentKind.text);
      expect(cut.truncated, isTrue);
      expect(cut.text.length, documentMaxLoadBytes);
      expect(cut.totalBytes, documentMaxLoadBytes + 10);
      expect(cut.large, isTrue);
    });
  });
}
