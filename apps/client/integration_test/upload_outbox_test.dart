// SPDX-License-Identifier: Apache-2.0

// Uploads in the outbox against the fake server (brief M4-22 Tests; 12 3; 15 7.5): two files
// are attached and sent offline; on reconnect the uploads run before the message, the message
// appears with both attachments, and they show "being checked" until the fake reports `clean`.

import 'dart:io';

import 'package:client/files/file_ports.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:sync_engine/testing.dart';

import '../test/support/fakes.dart';
import '../test/support/file_fakes.dart';
import '../test/support/thread_host.dart' show composerField;
import '../test/support/widgets.dart';
import 'support/test_app.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('two files sent offline upload before their message', (
    tester,
  ) async {
    final picker = FakePicker();
    final app = await TestApp.start(
      tester,
      files: fakeFilePorts(picker: picker),
    );
    final world = app.fake.server.world;
    final seed = app.fake.seed;
    world.fileSettings
      ..partBytes = 8
      ..holdScan = true;
    final forum = world.addForum(seed.space, 'files', {
      seed.alex.id: 'write',
      seed.sam.id: 'write',
    });
    final thread = world.addThread(forum, 'Logs', createdBy: seed.alex.id);
    final first = seed.post(seed.sam, thread, 'Please send the logs.');
    final dir = await Directory.systemTemp.createTemp('upload-outbox');
    Future<void> until(String what, bool Function() condition) async {
      final end = DateTime.now().add(const Duration(seconds: 30));
      while (!condition()) {
        if (DateTime.now().isAfter(end)) fail('timed out: $what');
        await tester.pump(const Duration(milliseconds: 50));
      }
    }

    try {
      await app.connect();
      await app.loginWithPassword();
      await app.waitLive();
      await app.waitFor(find.byKey(ValueKey('sidebar.channel.$forum')));
      await tester.tap(find.byKey(ValueKey('sidebar.channel.$forum')));
      await app.waitFor(find.byKey(ValueKey('postList.post.$thread')));
      await tester.tap(find.byKey(ValueKey('postList.post.$thread')));
      await app.waitFor(find.byKey(ValueKey('message.${first['id']}')));

      // Offline: nothing reaches the server.
      (app.fake.ports.network as SwitchNetwork).set(false);
      app.fake.server.disconnect(deviceId: app.deviceId);
      final a = File('${dir.path}/server.log')
        ..writeAsStringSync('line one of the server log\n');
      final b = File('${dir.path}/client.log')
        ..writeAsStringSync('the client log\n');
      picker.next = [
        PickedFile(path: a.path, name: 'server.log'),
        PickedFile(path: b.path, name: 'client.log'),
      ];
      await tester.tap(
        find.byWidgetPredicate(
          (w) => w is DsTappable && w.label == en.composerAttach,
        ),
      );
      await until(
        'two chips',
        () =>
            find
                .descendant(
                  of: find.byKey(const ValueKey('composer.attachments')),
                  matching: find.byType(Text),
                )
                .evaluate()
                .length >=
            4,
      );
      await tester.enterText(composerField, 'Here are both logs');
      await tester.pump();
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      await until(
        'sending',
        () => find.text(en.messageSending).evaluate().isNotEmpty,
      );
      expect(app.fake.server.requestsOf('createUpload'), isEmpty);

      // Back online: catch-up, then the uploads, then the message.
      (app.fake.ports.network as SwitchNetwork).set(true);
      await until(
        'message stored',
        () => world
            .messagesOf(thread)
            .any((m) => m['content_md'] == 'Here are both logs'),
      );
      final ops = [
        for (final r in app.fake.server.requests)
          if (r.status >= 200 && r.status < 300) r.operationId,
      ];
      expect(ops.where((o) => o == 'completeUpload'), hasLength(2));
      expect(
        ops.lastIndexOf('completeUpload'),
        lessThan(ops.indexOf('createMessage')),
      );
      final stored = world
          .messagesOf(thread)
          .firstWhere((m) => m['content_md'] == 'Here are both logs');
      final ids = [
        for (final att in stored['attachments']! as List)
          ((att as Map)['file']! as Map)['id']! as String,
      ];
      expect(ids, hasLength(2));

      // Both show "being checked" until the fake reports clean.
      Finder tile(String id) => find.byKey(ValueKey('file.$id'));
      await until(
        'tiles',
        () => ids.every((id) => tile(id).evaluate().isNotEmpty),
      );
      for (final id in ids) {
        expect(
          find.descendant(
            of: tile(id),
            matching: find.text(en.fileStateChecking),
          ),
          findsOneWidget,
        );
      }
      for (final id in ids) {
        world.finishScan(id);
      }
      await until(
        'clean',
        () => ids.every(
          (id) => find
              .descendant(
                of: tile(id),
                matching: find.byKey(ValueKey('file.$id.open')),
              )
              .evaluate()
              .isNotEmpty,
        ),
      );
      expect(find.text(en.fileStateChecking), findsNothing);
      expect(app.fake.server.violations, isEmpty);
    } finally {
      await app.stop();
      if (dir.existsSync()) await dir.delete(recursive: true);
    }
  });
}
