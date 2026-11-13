// SPDX-License-Identifier: Apache-2.0

// Files in the new chat view of a forum (FEAT-NEW-CHAT-ATTACH; rulings FNCA/1 to FNCA/5): before
// the chat exists the person pastes an image, picks or drops files on the desktop and takes
// them from the camera, the library or the files on the phone, with or without text. The files
// wait in the composer (staged, nothing in the outbox and nothing on the server) and the send
// writes their uploads and the post that attaches them to its first message in one batch.
// Removing them leaves nothing; the files of an earlier run are deleted; a post whose upload
// failed after the send shows why, sends it again or is discarded without leaving an upload.

import 'dart:convert';
import 'dart:io';

import 'package:client/files/file_ports.dart';
import 'package:client/files/upload_executor.dart';
import 'package:client/phone/compose/media_picker.dart';
import 'package:design_system/design_system.dart';
import 'package:desktop_drop/desktop_drop.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:image/image.dart' as img;
import 'package:local_store/local_store.dart';

import '../support/column_host.dart';
import '../support/file_fakes.dart';
import '../support/file_widgets.dart';
import '../support/new_chat_fixtures.dart';
import '../support/thread_fixtures.dart' show instanceWith;
import '../support/widgets.dart';

void main() {
  Finder inComposer(String label) => find.descendant(
    of: find.byKey(const ValueKey('newChat.composer')),
    matching: find.byWidgetPredicate(
      (w) => w is DsTappable && w.label == label,
    ),
  );

  Finder chip(String key) => find.descendant(
    of: find.byKey(const ValueKey('newChat.attachments')),
    matching: find.byKey(ValueKey('upload.$key')),
  );

  Future<List<OutboxEntry>> outbox(WidgetTester tester, ColumnHost host) =>
      host.run(tester, host.db.outboxDao.allEntries);

  List<OutboxEntry> uploads(List<OutboxEntry> all) =>
      ofKind(all, UploadOps.upload);

  /// The files below [dir] (recursively), or none when it does not exist.
  List<String> filesIn(Directory dir) => dir.existsSync()
      ? [
          for (final e in dir.listSync(recursive: true))
            if (e is File) e.path,
        ]
      : const [];

  Future<
    ({ColumnHost host, FakePicker picker, FakePaste paste, FakeMediaPort media})
  >
  open(
    WidgetTester tester, {
    Channel? forum,
    String platform = 'macos',
    Future<void> Function(Directory root)? before,
  }) async {
    final host = await ColumnHost.open(tester);
    final picker = FakePicker();
    final paste = FakePaste();
    final media = FakeMediaPort();
    final temporary = Directory('${host.root.path}/tmp')..createSync();
    final services = FileTestServices(
      host.db,
      host.root,
      fakeFilePorts(
        picker: picker,
        paste: paste,
        media: media,
        temporary: temporary,
      ),
    );
    await tester.runAsync(() async {
      await fillForums(host.db);
      await host.db.instanceDao.put(instanceWith());
      await before?.call(host.root);
    });
    await host.pump(
      tester,
      shell,
      location: newChatPath(forum ?? server),
      platform: platform,
      overrides: [...shellOverrides(), ...services.overrides],
    );
    return (host: host, picker: picker, paste: paste, media: media);
  }

  Future<File> write(ColumnHost host, String name, List<int> bytes) async {
    final f = File('${host.root.path}/$name');
    await f.writeAsBytes(bytes, flush: true);
    return f;
  }

  Directory staged(ColumnHost host) => Directory('${host.root.path}/staged');

  Future<void> pasteClipboard(WidgetTester tester) async {
    await tester.tap(composerField);
    final command = Platform.isMacOS
        ? LogicalKeyboardKey.meta
        : LogicalKeyboardKey.control;
    await tester.sendKeyDownEvent(command);
    await tester.sendKeyEvent(LogicalKeyboardKey.keyV);
    await tester.sendKeyUpEvent(command);
  }

  // The post once the send wrote it: the batch copies the files with real IO first.
  Future<List<OutboxEntry>> sent(WidgetTester tester, ColumnHost host) async {
    final all = await host.outboxUntil(
      tester,
      (all) => creates(all).isNotEmpty,
      what: 'the send',
    );
    await host.settle(tester);
    return all;
  }

  group('before the chat exists', () {
    testWidgets('a pasted image alone is staged and becomes the first message '
        'of the new chat', (tester) async {
      final (:host, :paste, picker: _, media: _) = await open(tester);
      final png = await host.run(
        tester,
        () => write(host, 'screenshot.png', onePixelPng),
      );
      paste.files = [PickedFile(path: png.path, name: 'screenshot.png')];
      await pasteClipboard(tester);
      await settleUntil(tester, host, chip('attach-0'));
      expect(chip('attach-0'), findsOneWidget);
      expect(
        find.byKey(const ValueKey('upload.attach-0.preview')),
        findsOneWidget,
        reason: 'the local preview of an image (15 14)',
      );
      // Staged: nothing in the outbox and nothing sent before the send.
      expect(uploads(await outbox(tester, host)), isEmpty);
      expect(filesIn(staged(host)), hasLength(1));

      // Send without text: the send button works with files only.
      await tester.tap(inComposer(en.composerSend));
      final all = await sent(tester, host);
      final post = creates(all).single;
      final up = uploads(all).single;
      final p = payloadOf(post);
      expect(p['content_md'], '');
      expect(p['attachment_keys'], ['attach-0']);
      expect(p['clear_draft'], isTrue);
      expect(post.dependsOn, up.id);
      expect(post.threadId, isNull);
      final u = payloadOf(up);
      expect(u['channel_id'], server.id.value);
      expect(u['attachment_key'], 'attach-0');
      expect(u['declared_mime_type'], 'image/png');
      expect(u['delete_after'], isTrue);
      // The staged copy moved into the folder the upload reads.
      expect(u['path'], startsWith('${host.root.path}/outgoing/attach-0/'));
      expect(File(u['path']! as String).existsSync(), isTrue);
      expect(filesIn(staged(host)), isEmpty);
      expect(find.byKey(const ValueKey('newChat.sending')), findsOneWidget);
      await host.dispose(tester);
    });

    testWidgets('text and two picked files go out as one post with both files '
        'in their order', (tester) async {
      final (:host, :picker, paste: _, media: _) = await open(tester);
      final a = await host.run(
        tester,
        () => write(host, 'server.log', utf8.encode('server log\n')),
      );
      final b = await host.run(
        tester,
        () => write(host, 'client.log', utf8.encode('client log\n')),
      );
      picker.next = [
        PickedFile(path: a.path, name: 'server.log'),
        PickedFile(path: b.path, name: 'client.log'),
      ];
      await tester.tap(inComposer(en.composerAttach));
      await settleUntil(tester, host, chip('attach-1'));
      expect(chip('attach-0'), findsOneWidget);
      expect(uploads(await outbox(tester, host)), isEmpty);

      await tester.enterText(composerField, 'Both logs of the outage');
      await host.settle(tester);
      await tester.showKeyboard(composerField);
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      final all = await sent(tester, host);
      final post = creates(all).single;
      expect(payloadOf(post)['content_md'], 'Both logs of the outage');
      expect(payloadOf(post)['attachment_keys'], ['attach-0', 'attach-1']);
      expect(post.dependsOn, 'attach-0,attach-1');
      expect(
        [for (final u in uploads(all)) payloadOf(u)['name']],
        ['server.log', 'client.log'],
      );
      await host.dispose(tester);
    });

    testWidgets('files dropped on the composer are staged', (tester) async {
      final (:host, picker: _, paste: _, media: _) = await open(tester);
      final f = await host.run(
        tester,
        () => write(host, 'trace.txt', utf8.encode('trace')),
      );
      final drop = find.descendant(
        of: find.byKey(const ValueKey('newChat.view')),
        matching: find.byType(DropTarget),
      );
      expect(drop, findsOneWidget);
      tester.widget<DropTarget>(drop).onDragDone!(
        DropDoneDetails(
          files: [DropItemFile(f.path, name: 'trace.txt')],
          localPosition: Offset.zero,
          globalPosition: Offset.zero,
        ),
      );
      await settleUntil(tester, host, chip('attach-0'));
      expect(chip('attach-0'), findsOneWidget);
      expect(uploads(await outbox(tester, host)), isEmpty);
      await host.dispose(tester);
    });

    testWidgets('a file over file_max_upload_bytes is refused with the reason, '
        'as in an existing chat', (tester) async {
      final (:host, :picker, paste: _, media: _) = await open(tester);
      await host.run(
        tester,
        () => host.db.instanceDao.put(
          instanceWith().copyWith(
            limits: instanceWith().limits.copyWith(fileMaxUploadBytes: 4),
          ),
        ),
      );
      await host.settle(tester);
      final f = await host.run(
        tester,
        () => write(host, 'big.bin', List.filled(10, 0x41)),
      );
      picker.next = [PickedFile(path: f.path, name: 'big.bin')];
      await tester.tap(inComposer(en.composerAttach));
      await settleUntil(
        tester,
        host,
        find.byKey(const ValueKey('newChat.attachError')),
      );
      expect(
        find.text(en.composerFileTooLarge(dsIsolate('big.bin'), '4 B')),
        findsOneWidget,
      );
      expect(find.byKey(const ValueKey('newChat.attachments')), findsNothing);
      expect(filesIn(staged(host)), isEmpty);
      await host.dispose(tester);
    });

    testWidgets('on the phone the menu offers camera, library and file, and a '
        'photo of the library is staged', (tester) async {
      final (:host, :media, picker: _, paste: _) = await open(
        tester,
        platform: 'ios',
      );
      final photo = await host.run(
        tester,
        () => write(
          host,
          'IMG_0042.jpg',
          img.encodeJpg(img.Image(width: 4, height: 4)),
        ),
      );
      media.next = RawMedia(path: photo.path, name: 'IMG_0042.jpg');
      await tester.tap(inComposer(en.composerAttach));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('composer.attachMenu')), findsOneWidget);
      for (final item in ['camera', 'library', 'file']) {
        expect(find.byKey(ValueKey('composer.attach.$item')), findsOneWidget);
      }
      // The new chat view has no templates (as on the desktop).
      expect(
        find.byKey(const ValueKey('composer.attach.templates')),
        findsNothing,
      );
      await tester.tap(find.byKey(const ValueKey('composer.attach.library')));
      await tester.pumpAndSettle();
      await settleUntil(tester, host, chip('attach-0'));
      expect(media.asked, [MediaSource.library]);
      expect(uploads(await outbox(tester, host)), isEmpty);
      await tester.tap(inComposer(en.composerSend));
      final all = await sent(tester, host);
      expect(
        payloadOf(uploads(all).single)['declared_mime_type'],
        'image/jpeg',
      );
      expect(payloadOf(creates(all).single)['attachment_keys'], ['attach-0']);
      await host.dispose(tester);
    });

    testWidgets('a forum without an operator agent needs a title for a post of '
        'files only; the files stay', (tester) async {
      final (:host, :paste, picker: _, media: _) = await open(
        tester,
        forum: ideas,
      );
      final png = await host.run(
        tester,
        () => write(host, 'shot.png', onePixelPng),
      );
      paste.files = [PickedFile(path: png.path, name: 'shot.png')];
      await pasteClipboard(tester);
      await settleUntil(tester, host, chip('attach-0'));
      expect(
        find.byKey(const ValueKey('newChat.titleMissing')),
        findsOneWidget,
      );
      await tester.tap(inComposer(en.composerSend));
      await host.settle(tester);
      expect(creates(await outbox(tester, host)), isEmpty);
      expect(chip('attach-0'), findsOneWidget, reason: 'nothing was taken');
      expect(filesIn(staged(host)), hasLength(1));
      await host.dispose(tester);
    });
  });

  group('error paths', () {
    testWidgets('removing the files and leaving leaves nothing behind', (
      tester,
    ) async {
      final (:host, :picker, paste: _, media: _) = await open(tester);
      final a = await host.run(
        tester,
        () => write(host, 'a.txt', utf8.encode('a')),
      );
      final b = await host.run(
        tester,
        () => write(host, 'b.txt', utf8.encode('b')),
      );
      picker.next = [
        PickedFile(path: a.path, name: 'a.txt'),
        PickedFile(path: b.path, name: 'b.txt'),
      ];
      await tester.tap(inComposer(en.composerAttach));
      await settleUntil(tester, host, chip('attach-1'));
      for (final key in ['attach-0', 'attach-1']) {
        await tester.tap(find.byKey(ValueKey('upload.$key.remove')));
        await host.settle(tester);
      }
      expect(find.byKey(const ValueKey('newChat.attachments')), findsNothing);
      // Esc leaves the view (01 6.8 item 8).
      await tester.sendKeyEvent(LogicalKeyboardKey.escape);
      await host.settle(tester);
      final all = await outbox(tester, host);
      expect(uploads(all), isEmpty);
      expect(creates(all), isEmpty);
      expect(ofKind(all, UploadOps.releaseUpload), isEmpty);
      expect(filesIn(staged(host)), isEmpty);
      expect(
        filesIn(Directory('${host.root.path}/outgoing')),
        isEmpty,
        reason: 'no copy for an upload',
      );
      await host.dispose(tester);
    });

    testWidgets('the files staged by an earlier run are deleted when the view '
        'opens; nothing of them was sent', (tester) async {
      late File left;
      final (:host, picker: _, paste: _, media: _) = await open(
        tester,
        before: (root) async {
          final dir = Directory('${root.path}/staged/${server.id.value}/k1')
            ..createSync(recursive: true);
          left = File('${dir.path}/screenshot.png')
            ..writeAsBytesSync(onePixelPng);
        },
      );
      await host.settle(tester);
      expect(left.existsSync(), isFalse);
      expect(filesIn(staged(host)), isEmpty);
      expect(await outbox(tester, host), isEmpty);
      await host.dispose(tester);
    });

    testWidgets('an upload that failed after the send: the reason, "Try again" '
        'sends it again, "Discard" leaves no upload', (tester) async {
      final (:host, :paste, picker: _, media: _) = await open(tester);
      final png = await host.run(
        tester,
        () => write(host, 'screenshot.png', onePixelPng),
      );
      paste.files = [PickedFile(path: png.path, name: 'screenshot.png')];
      await pasteClipboard(tester);
      await settleUntil(tester, host, chip('attach-0'));
      await tester.tap(inComposer(en.composerSend));
      final all = await sent(tester, host);
      final post = creates(all).single;
      var up = uploads(all).single;

      // The upload announced its session, then failed for good.
      Future<void> failUpload(OutboxEntry e, String fileId) => host.run(
        tester,
        () async {
          await host.db.outboxDao.markState(e.id, OutboxState.sending);
          await host.db.outboxDao.updatePayload(
            e.id,
            jsonEncode({
              ...payloadOf(e),
              UploadOps.sessionMember: {
                'entry': e.id,
                'file_id': fileId,
                'mode': 'direct',
              },
            }),
          );
          await host.db.outboxDao.recordFailure(e.id, code: 'file_unreadable');
        },
      );
      const first = '0192f4a0-7b1c-7000-8a00-0000000f1001';
      const second = '0192f4a0-7b1c-7000-8a00-0000000f1002';
      await failUpload(up, first);
      final notice = find.byKey(ValueKey('postList.failed.${post.id}'));
      await settleUntil(tester, host, notice);
      expect(find.text(en.fileFailUnreadable), findsWidgets);
      expect(
        find.descendant(
          of: find.byKey(const ValueKey('newChat.view')),
          matching: find.text(en.fileStateUploadFailed),
        ),
        findsOneWidget,
        reason: 'the chip of the file says it',
      );
      expect(
        find.byKey(ValueKey('postList.failed.edit.${post.id}')),
        findsNothing,
        reason: 'the uploaded files cannot go back into the composer',
      );

      // Try again: the failed session is released and the upload goes again with a new one.
      await tester.tap(
        find.byKey(ValueKey('postList.failed.retry.${post.id}')),
      );
      await host.settle(tester);
      var now = await outbox(tester, host);
      up = uploads(now).single;
      expect(up.id, isNot('attach-0'));
      expect(up.state, OutboxState.pending);
      expect(creates(now).single.dependsOn, up.id);
      expect(
        [
          for (final r in ofKind(now, UploadOps.releaseUpload))
            payloadOf(r)['file_id'],
        ],
        [first],
      );
      expect(notice, findsNothing);

      // It fails again; Discard removes the post and its upload and releases the session.
      await failUpload(up, second);
      await settleUntil(tester, host, notice);
      final copy = File(payloadOf(up)['path']! as String);
      expect(copy.existsSync(), isTrue);
      await tester.tap(
        find.byKey(ValueKey('postList.failed.discard.${post.id}')),
      );
      await host.settle(tester);
      now = await outbox(tester, host);
      expect(creates(now), isEmpty);
      expect(uploads(now), isEmpty);
      expect(
        [
          for (final r in ofKind(now, UploadOps.releaseUpload))
            payloadOf(r)['file_id'],
        ],
        [first, second],
      );
      expect(copy.existsSync(), isFalse);
      expect(host.visited.last, forumPath(server));
      await host.dispose(tester);
    });
  });
}
