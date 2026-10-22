// SPDX-License-Identifier: Apache-2.0

// Voice messages (brief M4-22 Tests; 15 13, 12 8): the recording stops at
// `voice_max_duration_ms`, the message has `kind = voice` and exactly one attachment, the
// player plays a ticket URL at speed 1, 1.5 and 2, the transcript appears on `file.updated`,
// and `transcription_unavailable` shows its short text. Fake recorder, fake player, a
// scripted server; nothing touches a microphone or a socket.

import 'dart:async';
import 'dart:io';

import 'package:audioplayers/audioplayers.dart' show AudioLogLevel, AudioLogger;
import 'package:client/files/attach_controller.dart';
import 'package:client/files/ticket_player.dart';
import 'package:client/files/upload_executor.dart';
import 'package:client/thread/history_column.dart';
import 'package:client/voice/recorder.dart';
import 'package:client/voice/voice_button.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/column_host.dart';
import '../support/file_fakes.dart';
import '../support/file_widgets.dart';
import '../support/structure.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';
import '../support/widgets.dart';

void main() {
  group('recording', () {
    late Directory scratch;
    setUp(() => scratch = Directory.systemTemp.createTempSync('voice'));
    tearDown(() => scratch.deleteSync(recursive: true));

    testWidgets('the recording stops at voice_max_duration_ms', (tester) async {
      final recorder = FakeRecorder();
      RecordedFile? sent;
      final voice = VoiceController(
        recorder: () => recorder,
        bitRate: 64000,
        limit: const Duration(seconds: 3),
        scratch: () async => scratch,
        onSend: (f) async => sent = f,
      );
      // In fake time, so the limit timer runs on the fake clock; the folder and the file of
      // the recording are real IO, which the real zone completes.
      Future<void> settle() async {
        for (var i = 0; i < 10; i++) {
          await tester.runAsync(
            () => Future<void>.delayed(const Duration(milliseconds: 5)),
          );
          await tester.pump();
        }
      }

      unawaited(voice.start());
      await settle();
      expect(voice.phase, VoicePhase.recording);
      expect(recorder.bitRate, 64000);
      await tester.pump(const Duration(seconds: 2));
      expect(recorder.calls, isNot(contains('stop')));
      await tester.pump(const Duration(seconds: 1));
      await settle();
      expect(recorder.calls, contains('stop'));
      expect(voice.phase, VoicePhase.recorded);
      expect(voice.limitReached, isTrue);
      await tester.runAsync(voice.send);
      expect(sent, isNotNull);
      expect(sent!.path, endsWith('.m4a'));
      voice.dispose();
    });

    testWidgets('two stops at once keep the recording', (tester) async {
      final voice = VoiceController(
        recorder: FakeRecorder.new,
        bitRate: 64000,
        scratch: () async => scratch,
        onSend: (_) async {},
      );
      await tester.runAsync(() async {
        await voice.start();
        await Future.wait([voice.stop(), voice.stop()]);
      });
      expect(voice.phase, VoicePhase.recorded);
      expect(voice.recorded, isNotNull);
      voice.dispose();
    });

    testWidgets('two starts at once record once', (tester) async {
      final recorder = FakeRecorder();
      final voice = VoiceController(
        recorder: () => recorder,
        bitRate: 64000,
        scratch: () async => scratch,
        onSend: (_) async {},
      );
      await tester.runAsync(() => Future.wait([voice.start(), voice.start()]));
      expect(recorder.calls.where((c) => c == 'start'), hasLength(1));
      expect(voice.phase, VoicePhase.recording);
      voice.dispose();
    });

    testWidgets('the microphone keeps a recording that waits to be sent', (
      tester,
    ) async {
      final recorder = FakeRecorder();
      final voice = VoiceController(
        recorder: () => recorder,
        bitRate: 64000,
        scratch: () async => scratch,
        onSend: (_) async {},
      );
      await tester.runAsync(() async {
        await voice.start();
        await voice.stop();
      });
      final first = voice.recorded;
      expect(first, isNotNull);
      await tester.runAsync(voice.start);
      expect(voice.phase, VoicePhase.recorded);
      expect(voice.recorded, same(first));
      expect(File(first!.path).existsSync(), isTrue);
      expect(recorder.calls.where((c) => c == 'start'), hasLength(1));
      voice.dispose();
    });

    test('the audio player logs nothing, so no ticket URL reaches a log', () {
      AudioTicketPlayer.silenceLogs();
      expect(AudioLogger.logLevel, AudioLogLevel.none);
    });

    testWidgets('a denied microphone explains the system settings', (
      tester,
    ) async {
      final voice = VoiceController(
        recorder: () => FakeRecorder(permitted: false),
        bitRate: 64000,
        scratch: () async => scratch,
        onSend: (_) async {},
      );
      await tester.runAsync(voice.start);
      expect(voice.phase, VoicePhase.denied);
      expect(find.text(en.voiceMicrophoneDenied), findsNothing);
      voice.dispose();
    });
  });

  testWidgets('a voice message has kind voice and exactly one attachment', (
    tester,
  ) async {
    final host = await ColumnHost.open(tester);
    final scratch = Directory('${host.root.path}/rec')..createSync();
    final recorded = File('${scratch.path}/voice.m4a')
      ..writeAsBytesSync(List.filled(32, 7));
    await host.run(tester, () async {
      final attach = AttachController(
        thread: threadId(1),
        channel: channelId(1),
        outbox: host.engine.outbox,
        progress: UploadProgress(),
        newKey: () => 'voice-key',
        outgoing: () async => scratch,
      );
      await sendVoice(
        host.engine.outbox,
        attach,
        threadId(1),
        RecordedFile(
          path: recorded.path,
          duration: const Duration(seconds: 2),
          sizeBytes: 32,
        ),
      );
      attach.dispose();
    });
    final entries = await host.run(tester, host.db.outboxDao.allEntries);
    final upload = entries.firstWhere((e) => e.kind == UploadOps.upload);
    final message = entries.firstWhere(
      (e) => e.kind == MessageOps.createMessage,
    );
    expect(decodePayload(upload.payload)['declared_mime_type'], 'audio/mp4');
    final payload = decodePayload(message.payload);
    expect(payload['kind'], 'voice');
    expect(payload['attachment_keys'], ['voice-key']);
    expect(payload.containsKey('clear_draft'), isFalse);
    expect(message.dependsOn, upload.id);
    await host.dispose(tester);
  });

  group('player and transcript', () {
    final voiceFile = fileOf(
      1,
      name: 'voice.m4a',
      mime: 'audio/mp4',
      durationMs: 4000,
      transcript: TranscriptStatus.pending,
    );

    Future<(ColumnHost, FileTestServices)> open(WidgetTester tester) async {
      FakePlayer.created.clear();
      final host = await ColumnHost.open(tester);
      final services = FileTestServices(host.db, host.root, fakeFilePorts());
      await host.run(tester, () async {
        await seedThread(
          host.db,
          messages: [msg(1, text: '', kind: MessageKind.voice)],
        );
        await attachFiles(host.db, 1, [voiceFile]);
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
      return (host, services);
    }

    String path(String tail) => '/files/${voiceFile.id.value}/$tail';

    testWidgets('plays a ticket URL at speed 1, 1.5 and 2', (tester) async {
      final (host, services) = await open(tester);
      services.adapter.on(
        path('tickets'),
        ScriptedAnswer.json(201, {
          'url': '/api/v1/files/${voiceFile.id.value}/content?ticket=abc',
          'expires_at': '2026-10-17T09:05:00.000Z',
        }),
      );
      await settleUntil(tester, host, find.byKey(const ValueKey('voice.play')));
      await tester.tap(find.byKey(const ValueKey('voice.play')));
      await settleUntil(tester, host, find.text(en.messageVoicePause));
      final player = FakePlayer.created.single;
      expect(player.played.single.queryParameters['ticket'], 'abc');
      expect(player.played.single.host, 'chat.example.com');
      for (final speed in ['1.5', '2', '1']) {
        await tester.tap(find.byKey(const ValueKey('voice.speed')));
        await host.settle(tester);
        expect(find.text(en.voiceSpeedValue(speed)), findsOneWidget);
      }
      expect(player.speeds, [1.5, 2.0, 1.0]);
      await host.dispose(tester);
      expect(player.disposed, isTrue);
    });

    testWidgets('the transcript appears when file.updated reports it', (
      tester,
    ) async {
      final (host, services) = await open(tester);
      services.adapter.on(
        path('transcript'),
        ScriptedAnswer.json(200, {
          'file_id': voiceFile.id.value,
          'status': 'done',
          'text': 'Please call Sam back',
          'language': 'en',
        }),
      );
      await settleUntil(
        tester,
        host,
        find.text(en.messageVoiceTranscriptPending),
      );
      expect(find.text(en.messageVoiceTranscriptPending), findsOneWidget);
      expect(services.adapter.requested, isNot(contains(path('transcript'))));
      await host.run(
        tester,
        () => host.db.messagesDao.putFile(
          voiceFile.copyWith(transcriptStatus: TranscriptStatus.done),
        ),
      );
      await settleUntil(tester, host, find.text('Please call Sam back'));
      expect(find.text('Please call Sam back'), findsOneWidget);
      await host.dispose(tester);
    });

    testWidgets('Transcribe shows transcription_unavailable as a short text', (
      tester,
    ) async {
      final (host, services) = await open(tester);
      await host.run(
        tester,
        () => host.db.messagesDao.putFile(
          voiceFile.copyWith(transcriptStatus: TranscriptStatus.none),
        ),
      );
      await host.settle(tester);
      services.adapter.on(
        path('transcribe'),
        ScriptedAnswer.problem(
          409,
          'conflict',
          reason: 'transcription_unavailable',
        ),
      );
      await settleUntil(
        tester,
        host,
        find.byKey(const ValueKey('transcript.request')),
      );
      await tester.tap(find.byKey(const ValueKey('transcript.request')));
      await settleUntil(tester, host, find.text(en.transcriptUnavailable));
      expect(find.text(en.transcriptUnavailable), findsOneWidget);
      expect(
        tester
            .widget<DsButton>(find.byKey(const ValueKey('transcript.request')))
            .onPressed,
        isNotNull,
      );
      await host.dispose(tester);
    });
  });
}
