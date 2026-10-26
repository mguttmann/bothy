// SPDX-License-Identifier: Apache-2.0

// Recording a voice message on the phone (brief M5-07 Tests; 01 5.3 M5; 15 13; 12 8.1): press,
// hold and release sends one upload entry and then the message with `kind = voice`; a swipe
// left discards and deletes the file; a swipe up locks; the notice comes
// `client.voice.limit_notice_lead` before `voice_max_duration_ms`; at the limit the recording
// stops by itself with the notice and waits in ready (PLAN row); the background stops a running
// recording and keeps it. Fake recorder, fake clock; nothing touches a microphone.

import 'dart:async';
import 'dart:io';

import 'package:client/files/upload_executor.dart';
import 'package:client/phone/compose/voice_recorder.dart';
import 'package:client/thread/history_column.dart';
import 'package:client/voice/recorder.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../../support/column_host.dart';
import '../../support/file_fakes.dart';
import '../../support/file_widgets.dart';
import '../../support/recording_clock.dart';
import '../../support/structure.dart';
import '../../support/thread_fixtures.dart';
import '../../support/thread_host.dart';
import '../../support/until.dart';
import '../../support/widgets.dart';

void main() {
  group('controller', () {
    late Directory scratch;
    late FakeRecordingClock clock;
    late FakeRecorder recorder;
    late List<RecordedFile> sent;

    setUp(() {
      scratch = Directory.systemTemp.createTempSync('phone-voice');
      clock = FakeRecordingClock();
      recorder = FakeRecorder();
      sent = [];
    });
    tearDown(() => scratch.deleteSync(recursive: true));

    PhoneVoiceRecorder voice({
      Duration? limit,
      Duration lead = const Duration(seconds: 30),
      Future<void> Function(RecordedFile file)? onSend,
    }) => PhoneVoiceRecorder(
      recorder: () => recorder,
      bitRate: 64000,
      noticeLead: lead,
      limit: limit,
      scratch: () async => scratch,
      clock: clock,
      onSend: onSend ?? (f) async => sent.add(f),
    );

    List<FileSystemEntity> files() => scratch.listSync();

    test('press and release sends the recording once', () async {
      final v = voice(limit: const Duration(minutes: 15));
      await v.press();
      expect(v.phase, PhoneVoicePhase.recording);
      expect(recorder.calls, ['permission', 'start']);
      expect(recorder.bitRate, 64000);
      clock.advance(const Duration(seconds: 4));
      expect(v.elapsed, const Duration(seconds: 4));
      await v.release();
      expect(v.phase, PhoneVoicePhase.idle);
      expect(sent, hasLength(1));
      expect(sent.single.path, endsWith('.m4a'));
      expect(recorder.calls, ['permission', 'start', 'stop']);
      v.dispose();
    });

    test('a swipe left discards and deletes the recording at once', () async {
      final v = voice();
      await v.press();
      await v.discard();
      expect(v.phase, PhoneVoicePhase.discarded);
      expect(recorder.calls.last, 'cancel');
      expect(sent, isEmpty);
      expect(files(), isEmpty);
      v.dispose();
    });

    test('Discard of a finished recording deletes its file', () async {
      final v = voice();
      await v.startLocked();
      await v.stop();
      expect(v.phase, PhoneVoicePhase.ready);
      expect(File(v.recorded!.path).existsSync(), isTrue);
      final path = v.recorded!.path;
      await v.discard();
      expect(File(path).existsSync(), isFalse);
      expect(v.phase, PhoneVoicePhase.discarded);
      expect(sent, isEmpty);
      v.dispose();
    });

    test('a swipe up locks: releasing then sends nothing', () async {
      final v = voice();
      await v.press();
      v.lock();
      expect(v.phase, PhoneVoicePhase.locked);
      await v.release();
      expect(v.phase, PhoneVoicePhase.locked);
      expect(sent, isEmpty);
      await v.send();
      expect(sent, hasLength(1));
      expect(v.phase, PhoneVoicePhase.idle);
      v.dispose();
    });

    test('Stop in locked shows the preview, which sends on Send', () async {
      final v = voice();
      await v.startLocked();
      expect(v.phase, PhoneVoicePhase.locked);
      await v.stop();
      expect(v.phase, PhoneVoicePhase.ready);
      expect(sent, isEmpty);
      await v.send();
      expect(sent, hasLength(1));
      v.dispose();
    });

    test('the notice comes limit_notice_lead before the stop', () async {
      const limit = Duration(minutes: 2);
      const lead = Duration(seconds: 30);
      final v = voice(limit: limit, lead: lead);
      await v.press();
      final noticeAt = voiceStopAt(limit) - lead;
      clock.advance(noticeAt - const Duration(milliseconds: 1));
      expect(v.noticeShown, isFalse);
      clock.advance(const Duration(milliseconds: 1));
      expect(v.noticeShown, isTrue);
      expect(v.noticeAt, lead);
      expect(v.phase, PhoneVoicePhase.recording);
      v.dispose();
    });

    test('a limit shorter than the lead shows the notice at once', () async {
      final v = voice(
        limit: const Duration(seconds: 5),
        lead: const Duration(seconds: 30),
      );
      await v.press();
      clock.advance(Duration.zero);
      expect(v.noticeShown, isTrue);
      expect(v.noticeAt, voiceStopAt(const Duration(seconds: 5)));
      v.dispose();
    });

    test(
      'at the limit the recording stops by itself and waits in ready',
      () async {
        const limit = Duration(seconds: 20);
        final v = voice(limit: limit);
        await v.press();
        final stopAt = voiceStopAt(limit);
        expect(stopAt, lessThan(limit));
        clock.advance(stopAt - const Duration(milliseconds: 1));
        expect(recorder.calls, isNot(contains('stop')));
        clock.advance(const Duration(milliseconds: 1));
        await waitUntil(
          () => v.phase == PhoneVoicePhase.ready,
          what: 'the stop at the limit',
        );
        expect(recorder.calls, contains('stop'));
        expect(v.phase, PhoneVoicePhase.ready);
        expect(v.limitReached, isTrue);
        expect(v.noticeShown, isTrue);
        // The finger still on the microphone: lifting it does not send what the limit stopped.
        await v.release();
        expect(sent, isEmpty);
        expect(v.phase, PhoneVoicePhase.ready);
        await v.send();
        expect(sent, hasLength(1));
        v.dispose();
      },
    );

    test('a recording over the limit is never sent', () async {
      const limit = Duration(seconds: 10);
      recorder.duration = const Duration(seconds: 11);
      final v = voice(limit: limit);
      await v.startLocked();
      await v.stop();
      expect(v.overLimit, isTrue);
      await v.send();
      expect(sent, isEmpty);
      expect(v.phase, PhoneVoicePhase.ready);
      v.dispose();
    });

    test('the background stops a running recording and keeps it', () async {
      for (final locked in [false, true]) {
        final v = voice();
        if (locked) {
          await v.startLocked();
        } else {
          await v.press();
        }
        await v.interrupt();
        expect(v.phase, PhoneVoicePhase.ready);
        expect(v.recorded, isNotNull);
        expect(File(v.recorded!.path).existsSync(), isTrue);
        expect(sent, isEmpty);
        await v.discard();
        v.dispose();
      }
    });

    test('a press released while the system asks records nothing', () async {
      recorder.startGate = null;
      final v = voice();
      final asked = v.press();
      await v.release();
      await asked;
      expect(v.phase, PhoneVoicePhase.idle);
      expect(recorder.calls, ['permission']);
      expect(sent, isEmpty);
      v.dispose();
    });

    test('a denied microphone ends in denied; Discard closes it', () async {
      recorder.permitted = false;
      final v = voice();
      await v.press();
      expect(v.phase, PhoneVoicePhase.denied);
      expect(recorder.calls, ['permission']);
      await v.discard();
      expect(v.phase, PhoneVoicePhase.idle);
      v.dispose();
    });

    test('a failed send keeps the recording in ready', () async {
      final v = voice(
        onSend: (_) async => throw const FileSystemException('offline'),
      );
      await v.startLocked();
      await v.stop();
      final file = v.recorded;
      await v.send();
      expect(v.phase, PhoneVoicePhase.ready);
      expect(v.recorded, same(file));
      v.dispose();
    });

    test(
      'a release while a start after a discard waits records nothing',
      () async {
        final v = voice();
        await v.press();
        await v.discard();
        expect(v.phase, PhoneVoicePhase.discarded);
        recorder.startGate = Completer<void>();
        final pressed = v.press();
        await waitUntil(
          () => recorder.calls.where((c) => c == 'start').length == 2,
          what: 'the second start call',
        );
        await v.release();
        recorder.startGate!.complete();
        await pressed;
        expect(v.phase, PhoneVoicePhase.idle);
        expect(recorder.calls.last, 'cancel');
        expect(files(), isEmpty);
        v.dispose();
      },
    );

    test('a slow start counts against the limit', () async {
      const limit = Duration(seconds: 20);
      recorder.startGate = Completer<void>();
      final v = voice(limit: limit);
      final pressed = v.press();
      while (!recorder.calls.contains('start')) {
        await Future<void>.delayed(Duration.zero);
      }
      // The recorder takes a second to start; the microphone records from the call on.
      clock.advance(const Duration(seconds: 1));
      recorder.startGate!.complete();
      await pressed;
      clock.advance(voiceStopAt(limit) - const Duration(seconds: 1));
      await waitUntil(
        () => v.phase == PhoneVoicePhase.ready,
        what: 'the stop at the limit',
      );
      expect(recorder.calls, contains('stop'));
      expect(v.phase, PhoneVoicePhase.ready);
      v.dispose();
    });

    test('Discard while the recorder finishes deletes the file', () async {
      final v = voice();
      await v.startLocked();
      recorder.stopGate = Completer<void>();
      final stopping = v.stop();
      expect(v.phase, PhoneVoicePhase.stopping);
      await v.discard();
      expect(v.phase, PhoneVoicePhase.discarded);
      recorder.stopGate!.complete();
      await stopping;
      expect(v.phase, PhoneVoicePhase.discarded);
      expect(v.recorded, isNull);
      expect(files(), isEmpty);
      v.dispose();
    });

    test('a recorder that fails to stop ends in failed', () async {
      final v = voice();
      await v.startLocked();
      recorder.stopError = StateError('recorder gone');
      await v.stop();
      expect(v.phase, PhoneVoicePhase.failed);
      await v.discard();
      expect(v.phase, PhoneVoicePhase.idle);
      v.dispose();
    });

    test(
      'the composer going while the recorder finishes deletes the file',
      () async {
        final v = voice();
        await v.startLocked();
        recorder.stopGate = Completer<void>();
        final stopping = v.stop();
        v.dispose();
        recorder.stopGate!.complete();
        await stopping;
        expect(files(), isEmpty);
      },
    );

    test('the shown time counts on the injected clock', () async {
      final v = voice(limit: const Duration(minutes: 1));
      await v.press();
      clock.advance(const Duration(seconds: 3));
      expect(v.elapsed, const Duration(seconds: 3));
      expect(v.remaining, voiceStopAt(const Duration(minutes: 1)) - v.elapsed);
      v.dispose();
    });
  });

  group('in the phone composer', () {
    Future<List<OutboxEntry>> entries(WidgetTester tester, ColumnHost host) =>
        host.run(tester, () async {
          final all = await host.db.outboxDao.allEntries();
          return [
            for (final e in all)
              if (e.kind == UploadOps.upload ||
                  e.kind == MessageOps.createMessage)
                e,
          ];
        });

    // The outbox entries once [done] holds: the stop writes a real file, so the entries are
    // awaited by condition, bounded only by the patience, never by a fixed number of rounds.
    Future<List<OutboxEntry>> entriesWhen(
      WidgetTester tester,
      ColumnHost host,
      bool Function(List<OutboxEntry> all) done, {
      required String what,
      Duration patience = const Duration(minutes: 3),
    }) async {
      final watch = Stopwatch()..start();
      while (true) {
        await host.settle(tester);
        final all = await entries(tester, host);
        if (done(all)) return all;
        if (watch.elapsed >= patience) {
          fail('$what did not happen within $patience');
        }
      }
    }

    Future<(ColumnHost, FakeRecorder, Directory)> open(
      WidgetTester tester,
    ) async {
      final host = await ColumnHost.open(tester);
      final recorder = FakeRecorder();
      final temporary = Directory('${host.root.path}/tmp')..createSync();
      final services = FileTestServices(
        host.db,
        host.root,
        fakeFilePorts(recorder: recorder, temporary: temporary),
      );
      await host.run(tester, () async {
        await seedThread(host.db, messages: [msg(1)]);
        await host.db.instanceDao.put(instanceWith());
      });
      await host.pump(
        tester,
        HistoryColumn(threadId: threadId(1)),
        location: '/t/${threadId(1).value}',
        platform: 'ios',
        overrides: [
          ...threadOverrides(FakeThreadReads(host.db)),
          ...services.overrides,
        ],
      );
      return (host, recorder, temporary);
    }

    Future<TestGesture> hold(WidgetTester tester, ColumnHost host) async {
      final mic = find.byKey(const ValueKey('voice.mic'));
      expect(mic, findsOneWidget);
      final gesture = await tester.startGesture(tester.getCenter(mic));
      await settleUntil(
        tester,
        host,
        find.byKey(const ValueKey('voice.panel')),
      );
      return gesture;
    }

    testWidgets(
      'press, hold and release writes the upload, then the voice message',
      (tester) async {
        final (host, recorder, temporary) = await open(tester);
        final gesture = await hold(tester, host);
        expect(find.text(en.voicePhoneSwipeHint), findsOneWidget);
        await gesture.up();
        final all = await entriesWhen(
          tester,
          host,
          (all) => all.any((e) => e.kind == MessageOps.createMessage),
          what: 'the voice message in the outbox',
        );
        expect(all.map((e) => e.kind), [
          UploadOps.upload,
          MessageOps.createMessage,
        ]);
        final upload = all.first;
        final message = all.last;
        expect(
          decodePayload(upload.payload)['declared_mime_type'],
          'audio/mp4',
        );
        expect(decodePayload(message.payload)['kind'], 'voice');
        expect(message.dependsOn, upload.id);
        expect(recorder.calls, ['permission', 'start', 'stop']);
        // Requirement 16: the outbox took the recording, the temporary folder is empty.
        expect(Directory('${temporary.path}/voice').listSync(), isEmpty);
        await host.dispose(tester);
      },
    );

    testWidgets('a swipe left discards and deletes the file', (tester) async {
      final (host, recorder, temporary) = await open(tester);
      final gesture = await hold(tester, host);
      for (var i = 0; i < 10; i++) {
        await gesture.moveBy(const Offset(-20, 0));
        await tester.pump();
      }
      await gesture.up();
      await host.settle(tester);
      expect(recorder.calls.last, 'cancel');
      expect(find.byKey(const ValueKey('voice.panel')), findsNothing);
      expect(await entries(tester, host), isEmpty);
      expect(Directory('${temporary.path}/voice').listSync(), isEmpty);
      await host.dispose(tester);
    });

    testWidgets('a swipe up locks; the microphone stops, then Send sends', (
      tester,
    ) async {
      final (host, recorder, _) = await open(tester);
      final gesture = await hold(tester, host);
      for (var i = 0; i < 10; i++) {
        await gesture.moveBy(const Offset(0, -20));
        await tester.pump();
      }
      await gesture.up();
      await host.settle(tester);
      expect(recorder.calls, isNot(contains('stop')));
      expect(find.byKey(const ValueKey('voice.stop')), findsOneWidget);
      expect(await entries(tester, host), isEmpty);
      // The microphone shows Stop while the locked recording runs.
      await tester.tap(find.byKey(const ValueKey('voice.mic')));
      await settleUntil(
        tester,
        host,
        find.byKey(const ValueKey('voice.preview')),
      );
      await tester.tap(find.byKey(const ValueKey('voice.send')));
      final all = await entriesWhen(
        tester,
        host,
        (all) => all.any((e) => e.kind == MessageOps.createMessage),
        what: 'the voice message in the outbox',
      );
      expect(
        all.where((e) => e.kind == MessageOps.createMessage),
        hasLength(1),
      );
      await host.dispose(tester);
    });

    testWidgets('the background stops the recording and keeps it unsent', (
      tester,
    ) async {
      final (host, recorder, _) = await open(tester);
      final gesture = await hold(tester, host);
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.hidden);
      await host.settle(tester);
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await gesture.up();
      await settleUntil(
        tester,
        host,
        find.byKey(const ValueKey('voice.preview')),
      );
      expect(recorder.calls, contains('stop'));
      expect(await entries(tester, host), isEmpty);
      await host.dispose(tester);
    });
  });
}
