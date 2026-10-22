// SPDX-License-Identifier: Apache-2.0

// A voice message against the fake server (brief M4-22 Tests; 15 13; 12 8): recorded with
// the fake recorder, sent as `kind = voice` with one attachment, played back through a file
// ticket, and its transcript appears when the fake reports it.

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:sync_engine/testing.dart';

import '../test/support/file_fakes.dart';
import '../test/support/widgets.dart';
import 'support/test_app.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('a voice message is sent, played and transcribed', (
    tester,
  ) async {
    FakePlayer.created.clear();
    final recorder = FakeRecorder();
    final app = await TestApp.start(
      tester,
      files: fakeFilePorts(recorder: recorder),
    );
    final world = app.fake.server.world;
    final seed = app.fake.seed;
    final forum = world.addForum(seed.space, 'voice', {
      seed.alex.id: 'write',
      seed.sam.id: 'write',
    });
    final thread = world.addThread(
      forum,
      'Call notes',
      createdBy: seed.alex.id,
    );
    final first = seed.post(seed.sam, thread, 'Can you leave a note?');
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

      await tester.tap(
        find.byWidgetPredicate(
          (w) => w is DsTappable && w.label == en.composerVoice,
        ),
      );
      await app.waitFor(find.byKey(const ValueKey('voice.bar')));
      await tester.tap(find.byKey(const ValueKey('voice.send')));
      await until(
        'voice message stored',
        () => world.messagesOf(thread).any((m) => m['kind'] == 'voice'),
      );
      final voice = world
          .messagesOf(thread)
          .firstWhere((m) => m['kind'] == 'voice');
      final attachments = voice['attachments']! as List;
      expect(attachments, hasLength(1));
      final fileId = ((attachments.single as Map)['file']! as Map)['id']!;
      expect(world.files['$fileId']!.json['mime_type'], 'audio/mp4');
      expect(recorder.bitRate, isNotNull);

      await app.waitFor(find.byKey(const ValueKey('voice.play')));
      await tester.tap(find.byKey(const ValueKey('voice.play')));
      await until(
        'played',
        () =>
            FakePlayer.created.isNotEmpty &&
            FakePlayer.created.last.played.isNotEmpty,
      );
      final url = FakePlayer.created.last.played.single;
      expect(url.queryParameters['ticket'], isNotEmpty);
      expect(url.path, '/api/v1/files/$fileId/content');

      world.setTranscript('$fileId', 'Call Sam back after lunch');
      await app.waitFor(find.text('Call Sam back after lunch'));
      expect(app.fake.server.violations, isEmpty);
    } finally {
      await app.stop();
    }
  });
}
