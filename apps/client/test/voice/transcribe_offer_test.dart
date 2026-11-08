// SPDX-License-Identifier: Apache-2.0

// "Transcribe" only where the server can transcribe (issue 570; 12 8.2): the button of a voice
// message whose transcript is `none` or `failed` shows only while `voice.enabled` is on in the
// stored flags and `getInstance` reports `voice_transcription_mode = service`. With the switch
// off the client asks nothing; in the modes `agent` and `off`, and from a server without the
// field, the button stays hidden. The view is shared by desktop and phone (VoicePlayer of the
// attachment tile), so these tests cover every platform. A scripted server; no socket.

import 'package:client/thread/history_column.dart';
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

/// Settles [host] until the view asked `getInstance`, at most twenty rounds, then five more
/// for the answer to reach the view.
Future<void> settleUntilAsked(
  WidgetTester tester,
  ColumnHost host,
  FileTestServices services,
) async {
  for (
    var i = 0;
    i < 20 && !services.adapter.requested.contains('/instance');
    i++
  ) {
    await tester.pump(const Duration(milliseconds: 10));
    await host.settle(tester);
  }
  for (var i = 0; i < 5; i++) {
    await tester.pump(const Duration(milliseconds: 10));
    await host.settle(tester);
  }
}

void main() {
  final request = find.byKey(const ValueKey('transcript.request'));

  Future<(ColumnHost, FileTestServices)> open(
    WidgetTester tester, {
    required bool voiceOn,
    String? mode,
    TranscriptStatus status = TranscriptStatus.none,
  }) async {
    final host = await ColumnHost.open(tester);
    final services = FileTestServices(host.db, host.root, fakeFilePorts());
    services.adapter.on(
      '/instance',
      ScriptedAnswer.json(200, instanceBody(mode)),
    );
    await host.run(tester, () async {
      await seedThread(
        host.db,
        messages: [msg(1, text: '', kind: MessageKind.voice)],
      );
      await host.db.instanceDao.put(
        instanceWith().copyWith(features: {'voice.enabled': voiceOn}),
      );
      await attachFiles(host.db, 1, [
        fileOf(
          1,
          name: 'voice.m4a',
          mime: 'audio/mp4',
          durationMs: 4000,
          transcript: status,
        ),
      ]);
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
    await settleUntil(tester, host, find.byKey(const ValueKey('voice.play')));
    return (host, services);
  }

  testWidgets('mode service with voice.enabled on offers Transcribe', (
    tester,
  ) async {
    final (host, services) = await open(tester, voiceOn: true, mode: 'service');
    await settleUntil(tester, host, request);
    expect(request, findsOneWidget);
    expect(services.adapter.requested, contains('/instance'));
    await host.dispose(tester);
  });

  testWidgets('a failed transcript in mode service offers Transcribe again', (
    tester,
  ) async {
    final (host, _) = await open(
      tester,
      voiceOn: true,
      mode: 'service',
      status: TranscriptStatus.failed,
    );
    await settleUntil(tester, host, request);
    expect(find.text(en.transcriptFailed), findsOneWidget);
    expect(request, findsOneWidget);
    await host.dispose(tester);
  });

  for (final (name, mode) in [
    ('agent', 'agent'),
    ('off', 'off'),
    ('absent (a server before the field)', null),
  ]) {
    testWidgets('mode $name hides Transcribe', (tester) async {
      final (host, services) = await open(tester, voiceOn: true, mode: mode);
      await settleUntilAsked(tester, host, services);
      expect(services.adapter.requested, contains('/instance'));
      expect(request, findsNothing);
      await host.dispose(tester);
    });
  }

  testWidgets('a failed transcript in mode agent keeps its text, no button', (
    tester,
  ) async {
    final (host, services) = await open(
      tester,
      voiceOn: true,
      mode: 'agent',
      status: TranscriptStatus.failed,
    );
    await settleUntilAsked(tester, host, services);
    expect(services.adapter.requested, contains('/instance'));
    expect(find.text(en.transcriptFailed), findsOneWidget);
    expect(request, findsNothing);
    await host.dispose(tester);
  });

  testWidgets('voice.enabled off hides Transcribe without asking the server', (
    tester,
  ) async {
    final (host, services) = await open(
      tester,
      voiceOn: false,
      mode: 'service',
    );
    for (var i = 0; i < 5; i++) {
      await host.settle(tester);
    }
    expect(request, findsNothing);
    expect(services.adapter.requested, isNot(contains('/instance')));
    await host.dispose(tester);
  });

  testWidgets('voice.enabled turned off in a snapshot hides Transcribe', (
    tester,
  ) async {
    final (host, _) = await open(tester, voiceOn: true, mode: 'service');
    await settleUntil(tester, host, request);
    expect(request, findsOneWidget);
    await host.run(
      tester,
      () => host.db.instanceDao.put(
        instanceWith().copyWith(features: {'voice.enabled': false}),
      ),
    );
    for (var i = 0; i < 5; i++) {
      await host.settle(tester);
    }
    expect(request, findsNothing);
    await host.dispose(tester);
  });
}
