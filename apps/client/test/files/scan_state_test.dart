// SPDX-License-Identifier: Apache-2.0

// The state of a file follows the server once its upload is done (FIX-scan-state-stale, 12 2):
// "Being checked" only while `scan_status` is `pending`, "Preparing" while a clean file is still
// prepared (an image's cleaned original and thumbnails), the normal view once it is servable, and
// a failed check shown as such. The composer chips and the chips of a message waiting in the
// outbox read the stored file of a confirmed upload, which `file.updated` keeps current, instead
// of showing "Being checked" for as long as they exist.

import 'dart:convert';
import 'dart:io';

import 'package:client/files/attachment_mapping.dart';
import 'package:client/files/file_ports.dart';
import 'package:client/files/upload_executor.dart';
import 'package:client/files/upload_planner.dart';
import 'package:client/thread/history_column.dart';
import 'package:client/thread/thread_actions.dart';
import 'package:design_system/design_system.dart';
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
  group('stateOfFile (12 2)', () {
    test('"being checked" only while the scan is pending', () {
      expect(
        stateOfFile(
          fileOf(1, upload: UploadState.pending, scan: ScanStatus.pending),
        ),
        AttachmentState.checking,
      );
    });

    test('a clean image the server still prepares is "preparing"', () {
      // The scan pass leaves a clean image pending for files.thumbnail (12 3.4): the scan is
      // over, so it is no longer "being checked".
      final image = fileOf(
        1,
        mime: 'image/png',
        upload: UploadState.pending,
        scan: ScanStatus.clean,
        thumbnails: false,
      );
      expect(stateOfFile(image), AttachmentState.preparing);
      expect(
        stateOfFile(
          fileOf(1, upload: UploadState.pending, scan: ScanStatus.skipped),
        ),
        AttachmentState.preparing,
      );
    });

    test('servable is ready, a found threat is blocked, a failed check is '
        'its own state', () {
      expect(stateOfFile(fileOf(1)), AttachmentState.ready);
      expect(
        stateOfFile(fileOf(1, scan: ScanStatus.infected)),
        AttachmentState.blocked,
      );
      expect(
        stateOfFile(fileOf(1, scan: ScanStatus.error)),
        AttachmentState.checkFailed,
      );
      expect(
        attachmentStateLabel(en, AttachmentState.checkFailed),
        en.fileStateCheckFailed,
      );
      expect(
        attachmentStateLabel(en, AttachmentState.preparing),
        en.fileStatePreparing,
      );
    });
  });

  Finder chip(String key, Finder f) =>
      find.descendant(of: find.byKey(ValueKey('upload.$key')), matching: f);

  testWidgets('the chips of a waiting message follow the stored file of a '
      'confirmed upload', (tester) async {
    final host = await ColumnHost.open(tester);
    final services = FileTestServices(host.db, host.root, fakeFilePorts());
    final pending = fileOf(
      1,
      name: 'shot.png',
      mime: 'image/png',
      upload: UploadState.pending,
      scan: ScanStatus.pending,
      thumbnails: false,
    );
    await host.run(tester, () async {
      await seedThread(host.db, messages: [msg(1)]);
      await host.db.messagesDao.putFile(pending);
      final path = '${host.root.path}/wait.txt';
      await File(path).writeAsBytes(utf8.encode('data of wait.txt'));
      final upload = await host.engine.outbox.enqueue(
        UploadOps.request(
          threadId: threadId(1),
          channelId: channelId(1),
          source: await prepareUpload(path),
          key: 'wait.txt',
        ),
      );
      // shot.png is confirmed (its slot holds the file ID), wait.txt still uploads, so the
      // message waits in the outbox.
      await host.engine.outbox.enqueue(
        ThreadActions.messageRequest(
          threadId(1),
          'Two screenshots',
          attachments: MessageAttachments(
            keys: const ['shot.png', 'wait.txt'],
            uploads: [upload.id],
            fileIds: {'shot.png': pending.id.value},
            preview: const [
              {'key': 'shot.png', 'name': 'shot.png', 'size_bytes': 48213},
              {'key': 'wait.txt', 'name': 'wait.txt', 'size_bytes': 16},
            ],
          ),
        ),
      );
    });
    await host.pump(
      tester,
      HistoryColumn(threadId: threadId(1)),
      location: '/t/${threadId(1).value}',
      overrides: [
        ...threadOverrides(FakeThreadReads(host.db)),
        ...services.overrides,
      ],
    );
    await settleUntil(
      tester,
      host,
      chip('shot.png', find.text(en.fileStateChecking)),
    );
    expect(chip('shot.png', find.text(en.fileStateChecking)), findsOneWidget);

    // file.updated: the scan is clean, the image is still prepared.
    await host.run(
      tester,
      () => host.db.messagesDao.putFile(
        pending.copyWith(scanStatus: ScanStatus.clean),
      ),
    );
    await settleUntil(
      tester,
      host,
      chip('shot.png', find.text(en.fileStatePreparing)),
    );
    expect(chip('shot.png', find.text(en.fileStateChecking)), findsNothing);

    // file.updated: servable.
    await host.run(
      tester,
      () => host.db.messagesDao.putFile(
        pending.copyWith(
          scanStatus: ScanStatus.clean,
          uploadState: UploadState.complete,
        ),
      ),
    );
    await host.settle(tester);
    expect(chip('shot.png', find.text(en.fileStatePreparing)), findsNothing);
    expect(chip('shot.png', find.text(en.fileStateChecking)), findsNothing);
    expect(chip('shot.png', find.textContaining('shot.png')), findsOneWidget);
    await host.dispose(tester);
  });

  testWidgets('the composer chip of an uploaded file follows the server and '
      'shows a failed check', (tester) async {
    final host = await ColumnHost.open(tester);
    final picker = FakePicker();
    final services = FileTestServices(
      host.db,
      host.root,
      fakeFilePorts(picker: picker),
    );
    await host.run(tester, () async {
      await seedThread(host.db, messages: [msg(1)]);
      await host.db.instanceDao.put(instanceWith());
    });
    await host.pump(
      tester,
      HistoryColumn(threadId: threadId(1)),
      location: '/t/${threadId(1).value}',
      overrides: [
        ...threadOverrides(FakeThreadReads(host.db)),
        ...services.overrides,
      ],
    );
    final picked = await host.run(tester, () async {
      final f = File('${host.root.path}/shot.png');
      await f.writeAsBytes(List.filled(32, 0x41), flush: true);
      return f.path;
    });
    picker.next = [PickedFile(path: picked, name: 'shot.png')];
    await tester.tap(
      find.byWidgetPredicate(
        (w) => w is DsTappable && w.label == en.composerAttach,
      ),
    );
    await settleUntil(
      tester,
      host,
      find.byKey(const ValueKey('upload.attach-0')),
    );

    // The upload is confirmed: its entry leaves the outbox and the server checks the file.
    final file = fileOf(
      1,
      name: 'shot.png',
      mime: 'image/png',
      upload: UploadState.pending,
      scan: ScanStatus.pending,
      thumbnails: false,
    );
    await host.run(tester, () async {
      for (final e in await host.db.outboxDao.allEntries()) {
        if (e.kind == UploadOps.upload) await host.db.outboxDao.delete(e.id);
      }
      await host.db.messagesDao.putFile(file);
    });
    services.progress.debugComplete('attach-0', file.id.value);
    await settleUntil(
      tester,
      host,
      chip('attach-0', find.text(en.fileStateChecking)),
    );
    expect(chip('attach-0', find.text(en.fileStateChecking)), findsOneWidget);

    // file.updated: the check failed. The chip says so instead of "Being checked".
    await host.run(
      tester,
      () => host.db.messagesDao.putFile(
        file.copyWith(
          uploadState: UploadState.complete,
          scanStatus: ScanStatus.error,
        ),
      ),
    );
    await settleUntil(
      tester,
      host,
      chip('attach-0', find.text(en.fileStateCheckFailed)),
    );
    expect(
      chip('attach-0', find.text(en.fileStateCheckFailed)),
      findsOneWidget,
    );
    expect(chip('attach-0', find.text(en.fileStateChecking)), findsNothing);

    // file.updated of another upload: clean and servable, the normal chip.
    await host.run(
      tester,
      () => host.db.messagesDao.putFile(
        file.copyWith(
          uploadState: UploadState.complete,
          scanStatus: ScanStatus.clean,
        ),
      ),
    );
    await host.settle(tester);
    expect(chip('attach-0', find.text(en.fileStateCheckFailed)), findsNothing);
    expect(chip('attach-0', find.text(en.fileStateChecking)), findsNothing);
    await host.dispose(tester);
  });
}
