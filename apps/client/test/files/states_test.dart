// SPDX-License-Identifier: Apache-2.0

// The states of 12 2 in the history (brief M4-22 Tests): a tile for every view of a stored
// file and of an upload entry, `file_not_ready` shows "being checked" and loads again on
// `file.updated`, `file_blocked` shows "blocked".

import 'dart:convert';
import 'dart:io';

import 'package:client/files/upload_executor.dart';
import 'package:client/files/upload_planner.dart';
import 'package:client/thread/history_column.dart';
import 'package:client/thread/thread_actions.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/column_host.dart';
import '../support/file_fakes.dart';
import '../support/file_widgets.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/widgets.dart';

void main() {
  Future<(ColumnHost, FileTestServices)> open(
    WidgetTester tester, {
    List<FileMeta> files = const [],
  }) async {
    final host = await ColumnHost.open(tester);
    final services = FileTestServices(host.db, host.root, fakeFilePorts());
    await host.run(tester, () async {
      await seedThread(host.db, messages: [msg(1, text: 'See the files')]);
      await attachFiles(host.db, 1, files);
    });
    return (host, services);
  }

  Future<void> pump(
    WidgetTester tester,
    ColumnHost host,
    FileTestServices services,
  ) => host.pump(
    tester,
    HistoryColumn(threadId: threadId(1)),
    location: '/t/${threadId(1).value}',
    overrides: [
      ...threadOverrides(FakeThreadReads(host.db)),
      ...services.overrides,
    ],
  );

  Finder inTile(FileId id, Finder f) => find.descendant(
    of: find.byKey(ValueKey('file.${id.value}')),
    matching: f,
  );

  testWidgets('a tile for every view of a stored file (12 2)', (tester) async {
    final files = [
      fileOf(
        1,
        name: 'checking.pdf',
        upload: UploadState.pending,
        scan: ScanStatus.pending,
      ),
      fileOf(
        2,
        name: 'failed.pdf',
        upload: UploadState.failed,
        scan: ScanStatus.pending,
      ),
      fileOf(3, name: 'ready.pdf'),
      fileOf(4, name: 'infected.pdf', scan: ScanStatus.infected),
      fileOf(5, name: 'error.pdf', scan: ScanStatus.error),
      fileOf(6, name: 'skipped.pdf', scan: ScanStatus.skipped),
      deletedFile(7),
    ];
    final (host, services) = await open(tester, files: files);
    await pump(tester, host, services);

    expect(inTile(fileId(1), find.text(en.fileStateChecking)), findsOneWidget);
    expect(
      inTile(fileId(2), find.text(en.fileStateUploadFailed)),
      findsOneWidget,
    );
    expect(inTile(fileId(3), find.textContaining('ready.pdf')), findsOneWidget);
    expect(
      inTile(fileId(3), find.byKey(ValueKey('file.${fileId(3).value}.open'))),
      findsOneWidget,
    );
    expect(inTile(fileId(4), find.text(en.fileStateBlocked)), findsOneWidget);
    // A failed check is its own state, not "blocked" (FIX-scan-state-stale).
    expect(
      inTile(fileId(5), find.text(en.fileStateCheckFailed)),
      findsOneWidget,
    );
    expect(
      inTile(fileId(6), find.byKey(ValueKey('file.${fileId(6).value}.save'))),
      findsOneWidget,
    );
    expect(inTile(fileId(7), find.text(en.fileDeleted)), findsOneWidget);
    // Only a servable file offers Open and Save as.
    for (final n in [1, 2, 4, 5, 7]) {
      expect(
        find.byKey(ValueKey('file.${fileId(n).value}.open')),
        findsNothing,
      );
    }
    // No request: nothing here is an image.
    expect(services.adapter.requested, isEmpty);
    await host.dispose(tester);
  });

  testWidgets('Open shows the reason when the download or the system refuses', (
    tester,
  ) async {
    final report = fileOf(1, name: 'report.pdf');
    final program = fileOf(
      2,
      name: 'setup.exe',
      mime: 'application/octet-stream',
    );
    final (host, services) = await open(tester, files: [report, program]);
    services.adapter.on(
      '/files/${report.id.value}/content',
      ScriptedAnswer.problem(409, 'file_blocked'),
    );
    await pump(tester, host, services);
    Finder notice(FileId id) => find.byKey(ValueKey('file.${id.value}.notice'));

    await tester.tap(find.byKey(ValueKey('file.${report.id.value}.open')));
    await settleUntil(tester, host, notice(report.id));
    expect(inTile(report.id, find.text(en.fileStateBlocked)), findsWidgets);

    await tester.tap(find.byKey(ValueKey('file.${program.id.value}.open')));
    await settleUntil(tester, host, notice(program.id));
    expect(notice(program.id), findsOneWidget);
    // A program is refused before any request (D-M4-22-14).
    expect(
      services.adapter.requested,
      isNot(contains('/files/${program.id.value}/content')),
    );
    await host.dispose(tester);
  });

  testWidgets(
    'upload entries show waiting, uploading, failed with Upload again '
    'and canceled',
    (tester) async {
      final (host, services) = await open(tester);
      final ids = <String>[];
      await host.run(tester, () async {
        final dir = host.root.path;
        for (final name in ['wait.txt', 'up.txt', 'fail.txt', 'gone.txt']) {
          final path = '$dir/$name';
          await File(path).writeAsBytes(utf8.encode('data of $name'));
          final e = await host.engine.outbox.enqueue(
            UploadOps.request(
              threadId: threadId(1),
              channelId: channelId(1),
              source: await prepareUpload(path),
              key: name,
            ),
          );
          ids.add(e.id);
        }
        await host.db.outboxDao.markState(ids[1], OutboxState.sending);
        await host.db.outboxDao.recordFailure(
          ids[2],
          code: 'conflict',
          reason: 'part_conflict',
        );
        await host.db.outboxDao.recordFailure(ids[3], code: 'gone');
        await host.engine.outbox.enqueue(
          ThreadActions.messageRequest(
            threadId(1),
            'With four files',
            attachments: MessageAttachments(
              keys: const ['wait.txt', 'up.txt', 'fail.txt', 'gone.txt'],
              uploads: ids,
              preview: [
                for (final k in ['wait.txt', 'up.txt', 'fail.txt', 'gone.txt'])
                  {'key': k, 'name': k, 'size_bytes': 16},
              ],
            ),
          ),
        );
      });
      await pump(tester, host, services);

      Finder chip(String key, Finder f) =>
          find.descendant(of: find.byKey(ValueKey('upload.$key')), matching: f);
      expect(chip('wait.txt', find.text(en.fileStateWaiting)), findsOneWidget);
      expect(
        chip('up.txt', find.text(en.fileStateUploading(0))),
        findsOneWidget,
      );
      expect(
        chip('fail.txt', find.text(en.fileStateUploadFailed)),
        findsOneWidget,
      );
      expect(chip('fail.txt', find.text(en.fileUploadAgain)), findsOneWidget);
      expect(chip('gone.txt', find.text(en.fileStateCanceled)), findsOneWidget);
      // The message names the reason of its failed upload.
      expect(find.textContaining(en.fileFailPartConflict), findsWidgets);
      // An upload is not a refused write of its own.
      expect(find.byKey(ValueKey('outbox.${ids[2]}')), findsNothing);
      await host.dispose(tester);
    },
  );

  testWidgets('file_not_ready shows "being checked" and loads again on '
      'file.updated', (tester) async {
    final image = fileOf(1, name: 'shot.png', mime: 'image/png');
    final (host, services) = await open(tester, files: [image]);
    final path = '/files/${image.id.value}/thumbnails/medium';
    services.adapter
      ..on(path, ScriptedAnswer.problem(409, 'file_not_ready'))
      ..on(path, ScriptedAnswer.file(onePixelPng));
    await pump(tester, host, services);
    await settleUntil(
      tester,
      host,
      inTile(image.id, find.text(en.fileStateChecking)),
    );
    expect(inTile(image.id, find.text(en.fileStateChecking)), findsOneWidget);
    expect(inTile(image.id, find.byType(Image)), findsNothing);

    // file.updated: the server reports the file again (here with its final size).
    await host.run(
      tester,
      () => host.db.messagesDao.putFile(image.copyWith(width: 2, height: 2)),
    );
    await settleUntil(tester, host, inTile(image.id, find.byType(Image)));
    expect(services.adapter.requested.where((p) => p == path), hasLength(2));
    expect(inTile(image.id, find.byType(Image)), findsOneWidget);
    expect(inTile(image.id, find.text(en.fileStateChecking)), findsNothing);
    await host.dispose(tester);
  });

  testWidgets('file_blocked shows "blocked"', (tester) async {
    final image = fileOf(1, name: 'shot.png', mime: 'image/png');
    final (host, services) = await open(tester, files: [image]);
    services.adapter.on(
      '/files/${image.id.value}/thumbnails/medium',
      ScriptedAnswer.problem(409, 'file_blocked'),
    );
    await pump(tester, host, services);
    await settleUntil(
      tester,
      host,
      inTile(image.id, find.text(en.fileStateBlocked)),
    );
    expect(inTile(image.id, find.text(en.fileStateBlocked)), findsOneWidget);
    expect(inTile(image.id, find.byType(Image)), findsNothing);
    await host.dispose(tester);
  });
}
