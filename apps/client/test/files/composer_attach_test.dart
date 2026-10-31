// SPDX-License-Identifier: Apache-2.0

// Attaching in the composer (brief M4-22 Scope and Design note 2; 15 14, 04 13.2): the
// attachment button and paste put one upload entry per file into the outbox, before the
// message that attaches it; a file over `file_max_upload_bytes` is refused with the reason
// before any entry; a message of attachments only can be sent; the voice button records and
// sends a voice message.

import 'dart:io';

import 'package:client/files/file_ports.dart';
import 'package:client/files/upload_executor.dart';
import 'package:client/thread/history_column.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/column_host.dart';
import '../support/file_fakes.dart';
import '../support/file_widgets.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/widgets.dart';

void main() {
  Finder button(String label) =>
      find.byWidgetPredicate((w) => w is DsTappable && w.label == label);

  // The writes of files and messages; opening the thread also writes its read state.
  Future<List<OutboxEntry>> entries(
    WidgetTester tester,
    ColumnHost host,
  ) => host.run(tester, () async {
    final all = await host.db.outboxDao.allEntries();
    return [
      for (final e in all)
        if (e.kind == UploadOps.upload || e.kind == MessageOps.createMessage) e,
    ];
  });

  Future<(ColumnHost, FakePicker, FakePaste, FakeRecorder)> open(
    WidgetTester tester, {
    int maxUpload = 1000000,
  }) async {
    final host = await ColumnHost.open(tester);
    final picker = FakePicker();
    final paste = FakePaste();
    final recorder = FakeRecorder();
    final services = FileTestServices(
      host.db,
      host.root,
      fakeFilePorts(picker: picker, paste: paste, recorder: recorder),
    );
    await host.run(tester, () async {
      await seedThread(host.db, messages: [msg(1)]);
      final base = instanceWith();
      await host.db.instanceDao.put(
        base.copyWith(
          limits: base.limits.copyWith(fileMaxUploadBytes: maxUpload),
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
    return (host, picker, paste, recorder);
  }

  Future<PickedFileFixture> file(ColumnHost host, String name, int size) async {
    final f = File('${host.root.path}/$name');
    await f.writeAsBytes(List.filled(size, 0x41), flush: true);
    return PickedFileFixture(f.path, name);
  }

  testWidgets('the attachment button attaches uploads before the message', (
    tester,
  ) async {
    final (host, picker, _, _) = await open(tester);
    final f = await host.run(tester, () => file(host, 'notes.txt', 12));
    picker.next = [f.picked];
    await tester.tap(button(en.composerAttach));
    await settleUntil(
      tester,
      host,
      find.byKey(const ValueKey('upload.attach-0')),
    );
    expect(
      find.descendant(
        of: find.byKey(const ValueKey('upload.attach-0')),
        matching: find.textContaining('notes.txt'),
      ),
      findsOneWidget,
    );
    final uploads = await entries(tester, host);
    expect(uploads.single.kind, UploadOps.upload);

    // A message of attachments only: the send button works without text.
    await tester.tap(button(en.composerSend));
    await host.settle(tester);
    final all = await entries(tester, host);
    final message = all.firstWhere((e) => e.kind == MessageOps.createMessage);
    expect(message.dependsOn, uploads.single.id);
    expect(decodePayload(message.payload)['attachment_keys'], ['attach-0']);
    expect(
      find.descendant(
        of: find.byKey(const ValueKey('composer.attachments')),
        matching: find.byKey(const ValueKey('upload.attach-0')),
      ),
      findsNothing,
    );
    await host.dispose(tester);
  });

  testWidgets('a file over file_max_upload_bytes is refused with the reason', (
    tester,
  ) async {
    final (host, picker, _, _) = await open(tester, maxUpload: 4);
    final f = await host.run(tester, () => file(host, 'big.bin', 10));
    picker.next = [f.picked];
    await tester.tap(button(en.composerAttach));
    await settleUntil(
      tester,
      host,
      find.byKey(const ValueKey('composer.attachError')),
    );
    expect(
      find.text(en.composerFileTooLarge(dsIsolate('big.bin'), '4 B')),
      findsOneWidget,
    );
    expect(await entries(tester, host), isEmpty);
    await host.dispose(tester);
  });

  testWidgets('paste attaches the files of the clipboard', (tester) async {
    final (host, _, paste, _) = await open(tester);
    final f = await host.run(tester, () => file(host, 'pasted.png', 8));
    paste.files = [f.picked];
    await tester.tap(composerField);
    final command = Platform.isMacOS
        ? LogicalKeyboardKey.meta
        : LogicalKeyboardKey.control;
    await tester.sendKeyDownEvent(command);
    await tester.sendKeyEvent(LogicalKeyboardKey.keyV);
    await tester.sendKeyUpEvent(command);
    await settleUntil(
      tester,
      host,
      find.byKey(const ValueKey('upload.attach-0')),
    );
    expect(find.byKey(const ValueKey('upload.attach-0')), findsOneWidget);
    expect((await entries(tester, host)).single.kind, UploadOps.upload);
    await host.dispose(tester);
  });

  testWidgets('the voice button records and sends a voice message', (
    tester,
  ) async {
    final (host, _, _, recorder) = await open(tester);
    await tester.tap(button(en.composerVoice));
    await settleUntil(tester, host, find.byKey(const ValueKey('voice.bar')));
    expect(recorder.calls, containsAllInOrder(['permission', 'start']));
    await tester.tap(find.byKey(const ValueKey('voice.send')));
    // The recorder writes its file and the send copies it with real IO before the message:
    // wait for the message, never for a number of settle rounds (issue #238).
    await host.outboxUntil(
      tester,
      (all) => all.any((e) => e.kind == MessageOps.createMessage),
      what: 'the voice message',
    );
    final all = await entries(tester, host);
    final message = all.firstWhere((e) => e.kind == MessageOps.createMessage);
    expect(decodePayload(message.payload)['kind'], 'voice');
    final upload = all.firstWhere((e) => e.kind == UploadOps.upload);
    expect(decodePayload(upload.payload)['declared_mime_type'], 'audio/mp4');
    expect(message.dependsOn, upload.id);
    await host.dispose(tester);
  });
}

/// A file written for a test and its picked form.
class PickedFileFixture {
  /// Creates the fixture.
  PickedFileFixture(this.path, this.name);

  /// The path.
  final String path;

  /// The name.
  final String name;

  /// As a picked file.
  PickedFile get picked => PickedFile(path: path, name: name);
}
