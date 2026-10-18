// SPDX-License-Identifier: Apache-2.0

// The export (brief M4-29 Tests, Design note 3): the NDJSON file equals the server's bytes; the
// CSV has the columns, quotes per RFC 4180 and guards cells that start with `=`, `+`, `-` or
// `@`; an export needs a time range; a stream that breaks off says the file is incomplete.

import 'dart:async';
import 'dart:convert';

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void main() {
  group('the CSV', () {
    test('cells quote and guard', () {
      expect(auditCsvCell('plain'), 'plain');
      expect(auditCsvCell(null), '');
      expect(auditCsvCell(42), '42');
      expect(auditCsvCell('a,b'), '"a,b"');
      expect(auditCsvCell('say "hi"'), '"say ""hi"""');
      expect(auditCsvCell('two\nlines'), '"two\nlines"');
      expect(auditCsvCell('cr\rhere'), '"cr\rhere"');
      for (final lead in ['=', '+', '-', '@']) {
        expect(auditCsvCell('${lead}SUM(A1)'), "'${lead}SUM(A1)");
      }
      expect(auditCsvCell('=HYPERLINK("x")'), '"\'=HYPERLINK(""x"")"');
    });

    test('the converter writes the header and one row per line', () async {
      final events = [for (final e in auditFixture()) auditJson(e)];
      final ndjson = events.map((e) => '${jsonEncode(e)}\n').join();
      final bytes = utf8.encode(ndjson);
      // Chunks that cut lines and characters apart.
      final chunks = [
        for (var i = 0; i < bytes.length; i += 7)
          bytes.sublist(i, (i + 7).clamp(0, bytes.length)),
      ];
      final csv = utf8.decode(
        await auditNdjsonToCsv(Stream.fromIterable(chunks))
            .expand((c) => c)
            .toList(),
      );
      final lines = csv.split('\r\n');
      expect(lines.first, auditCsvColumns.join(','));
      expect(lines.first, startsWith('occurred_at,actor_principal_id,action'));
      expect(lines.first, endsWith('seq,hash,prev_hash'));
      expect(
        lines,
        hasLength(5),
        reason: 'header, 3 rows, empty after the last CRLF',
      );
      expect(
        lines[2],
        '2026-10-17T06:12:00.000Z,sam,agent.invoke,deny,run,'
        '0192f4a0-7b1c-7000-8000-000000000202,missing_grant,'
        '4bf92f35-77b3-4da6-a3ce-000000000002,2,${'b2' * 32},${'a1' * 32}',
      );
    });

    test('a line that is not a JSON object fails', () {
      expect(
        auditNdjsonToCsv(Stream.value(utf8.encode('[1]\n'))).drain<void>(),
        throwsFormatException,
      );
    });
  });

  group('the export bar', () {
    late FakeAdminRepository repository;
    late MemoryTarget target;

    setUp(() {
      repository = FakeAdminRepository(prompt: RecordingPrompt())
        ..stepUpFresh = true;
      target = MemoryTarget();
    });

    Future<void> range(WidgetTester tester) async {
      await tester.enterText(
        find.byKey(const ValueKey('audit.filter.from')),
        '2026-10-16',
      );
      await tester.enterText(
        find.byKey(const ValueKey('audit.filter.to')),
        '2026-10-17',
      );
      await tester.testTextInput.receiveAction(TextInputAction.done);
      await tester.pumpAndSettle();
    }

    Future<void> export(WidgetTester tester, String key) async {
      await tester.tap(find.byKey(ValueKey(key)));
      await tester.pumpAndSettle();
      await tester.runAsync(() => Future<void>.delayed(Duration.zero));
      await tester.pumpAndSettle();
    }

    testWidgets('the NDJSON file equals the server bytes', (tester) async {
      await pumpAudit(tester, repository, saveTarget: target.call);
      await range(tester);
      await export(tester, 'audit.export');

      expect(repository.exportRequests, [
        '2026-10-15T22:00:00.000Z..2026-10-17T22:00:00.000Z',
      ]);
      expect(target.names, ['audit-2026-10-15-2026-10-17.ndjson']);
      expect(target.bytes, repository.exportedBytes);
      expect(target.bytes, isNotEmpty);
      expect(target.closed, isTrue);
      expect(find.text('Export saved.'), findsOneWidget);
      // Fixture size for the report (brief M4-29 Report item 4).
      // ignore: avoid_print
      print('NDJSON fixture export: ${target.bytes.length} bytes');
    });

    testWidgets('the CSV is converted from the same stream', (tester) async {
      await pumpAudit(tester, repository, saveTarget: target.call);
      await range(tester);
      await export(tester, 'audit.exportCsv');

      expect(target.names, ['audit-2026-10-15-2026-10-17.csv']);
      final csv = utf8.decode(target.bytes);
      final rows = csv.split('\r\n');
      expect(rows.first, auditCsvColumns.join(','));
      expect(rows, hasLength(5));
      expect(rows[3], contains('owner.add'));
      expect(target.closed, isTrue);
      // ignore: avoid_print
      print('CSV fixture export: ${target.bytes.length} bytes');
    });

    testWidgets('an export needs a time range', (tester) async {
      await pumpAudit(tester, repository, saveTarget: target.call);
      await export(tester, 'audit.export');

      expect(
        find.text('Choose a time range (from and to) to export.'),
        findsOneWidget,
      );
      expect(target.names, isEmpty);
      expect(repository.exportRequests, isEmpty);
    });

    testWidgets('a stream that breaks off says the file is incomplete', (
      tester,
    ) async {
      repository.exportBreaksAfter = 1;
      await pumpAudit(tester, repository, saveTarget: target.call);
      await range(tester);
      await export(tester, 'audit.export');

      expect(
        find.text('The export broke off; the file is incomplete.'),
        findsOneWidget,
      );
      expect(target.closed, isTrue);
    });

    testWidgets('a cancelled save dialog exports nothing', (tester) async {
      await pumpAudit(tester, repository, saveTarget: (_) async => null);
      await range(tester);
      await export(tester, 'audit.export');
      expect(repository.exportRequests, isEmpty);
    });

    testWidgets('without a save dialog there is no export', (tester) async {
      await pumpAudit(tester, repository);
      expect(find.byKey(const ValueKey('audit.export')), findsNothing);
      expect(find.byKey(const ValueKey('audit.exportCsv')), findsNothing);
    });
  });
}
