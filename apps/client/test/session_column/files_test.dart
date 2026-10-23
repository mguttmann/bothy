// SPDX-License-Identifier: Apache-2.0

// The section "Files" of the session column (brief M4-22 Scope; 01 5.1 D1): the files of the
// thread's messages, newest message first, each with its name and state; "No files" without.

import 'package:client/session_column/session_column.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/file_fakes.dart';
import '../support/file_widgets.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/widgets.dart';

void main() {
  Future<ColumnHost> open(
    WidgetTester tester,
    Map<int, List<FileMeta>> files,
  ) async {
    final host = await ColumnHost.open(tester);
    final services = FileTestServices(host.db, host.root, fakeFilePorts());
    await host.run(tester, () async {
      await seedThread(
        host.db,
        messages: [msg(1), msg(2, minutes: 5), msg(3, minutes: 9)],
      );
      for (final MapEntry(:key, :value) in files.entries) {
        await attachFiles(host.db, key, value);
      }
    });
    await host.pump(
      tester,
      SessionColumn(threadId: threadId(1)),
      overrides: [
        ...threadOverrides(FakeThreadReads(host.db)),
        ...services.overrides,
      ],
    );
    return host;
  }

  testWidgets('lists the files of the thread, newest message first', (
    tester,
  ) async {
    final host = await open(tester, {
      1: [fileOf(1, name: 'first.pdf')],
      3: [
        fileOf(
          2,
          name: 'latest.pdf',
          scan: ScanStatus.pending,
          upload: UploadState.pending,
        ),
        fileOf(3, name: 'notes.txt', mime: 'text/plain'),
      ],
    });
    await settleUntil(tester, host, find.text(en.sessionFiles(3)));
    expect(find.text(en.sessionFiles(3)), findsOneWidget);
    final rows = [
      for (final n in [2, 3, 1])
        tester.getTopLeft(find.byKey(ValueKey('file.${fileId(n).value}'))).dy,
    ];
    expect(rows, orderedEquals([...rows]..sort()));
    expect(
      find.descendant(
        of: find.byKey(ValueKey('file.${fileId(2).value}')),
        matching: find.text(en.fileStateChecking),
      ),
      findsOneWidget,
    );
    await host.dispose(tester);
  });

  testWidgets('a thread without files says so', (tester) async {
    final host = await open(tester, const {});
    await settleUntil(tester, host, find.text(en.sessionFilesEmpty));
    expect(find.text(en.sessionFiles(0)), findsOneWidget);
    expect(find.text(en.sessionFilesEmpty), findsOneWidget);
    await host.dispose(tester);
  });
}
