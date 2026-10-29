// SPDX-License-Identifier: Apache-2.0

// Voice messages and photos on the phone (brief M5-07 Tests; 01 5.3 M5; 15 13, 14; 12 8.1)
// against the fake server: a recording held on the microphone and released arrives as an
// `audio/mp4` upload with `kind = voice`; with the instance limit set very low the recording
// stops by itself with the notice (PLAN row: a recording over the limit stops with a notice)
// and its `mvhd` duration stays within the limit; a photo of the library arrives as JPEG, and
// three photos picked at once arrive as one message with three JPEG attachments (M5-16).
//
// When requested (CLIENT_VOICE_REAL_RECORDER) on an iOS simulator or Android emulator whose
// microphone permission was granted before the run, the real recorder of package `record` records the simulator's or emulator's audio input,
// so the `mvhd` duration of the upload is checked; elsewhere (the Flutter tester, a device
// without the permission) the fake recorder stands in and the run prints a SKIP line for the
// real recording. The system photo picker is a system screen the test cannot drive, so the
// picker port is a fake that returns PNGs; the app turns them into the JPEGs it uploads.

import 'dart:io';

import 'package:client/files/attachment_tile.dart' show UploadChip;
import 'package:client/files/file_ports.dart';
import 'package:client/phone/compose/media_picker.dart';
import 'package:client/voice/mp4_duration.dart';
import 'package:client/voice/recorder.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:image/image.dart' as img;
import 'package:integration_test/integration_test.dart';
import 'package:record/record.dart' show AudioRecorder;

import '../../test/support/file_fakes.dart';
import '../../test/support/widgets.dart' show en;
import 'support/phone_flow.dart';

/// Whether this run records with the real recorder: only on request
/// (`--dart-define=CLIENT_VOICE_REAL_RECORDER=true`, set by `tool/run_ios_integration_tests.sh`
/// from the environment variable of the same name). The audio input of a hosted CI simulator
/// or emulator is not dependable: a start or a probe recording there can hang or come late, so
/// the default run uses the fake recorder and stays deterministic.
const _realRequested = bool.fromEnvironment('CLIENT_VOICE_REAL_RECORDER');

/// The recorder of this run: the real one where it was requested, the platform records, the
/// permission was granted before the run (no system dialog can be answered by the test) and a
/// probe recording gives an MP4 with a duration (a simulator or emulator without an audio input
/// does not); else the fake.
///
/// The fake reports [fakeDuration] as the duration of its file when set (a real recording's
/// `mvhd` duration), otherwise the real time it ran.
Future<(VoiceRecorder Function(), bool)> _recorder({
  Duration? fakeDuration,
}) async {
  VoiceRecorder fake() => FakeRecorder()..duration = fakeDuration;
  final why = !_realRequested
      ? 'the real recorder was not requested (CLIENT_VOICE_REAL_RECORDER)'
      : (!Platform.isIOS && !Platform.isAndroid)
      ? 'this platform has no phone recorder'
      : await _cannotRecord();
  if (why == null) return (RecordVoiceRecorder.new, true);
  // ignore: avoid_print
  print('SKIP client voice-device: $why; the fake recorder stands in');
  return (fake, false);
}

/// Why the platform cannot record here, or null when a probe recording works.
Future<String?> _cannotRecord() async {
  final probe = AudioRecorder();
  final dir = Directory.systemTemp.createTempSync('voice-probe');
  try {
    if (!await probe.hasPermission(request: false)) {
      return 'the microphone permission was not granted before the run';
    }
    final recorder = RecordVoiceRecorder();
    try {
      await recorder.start(path: '${dir.path}/probe.m4a', bitRate: 64000);
      await Future<void>.delayed(const Duration(milliseconds: 500));
      final file = await recorder.stop();
      if (file == null || await mp4Duration(file.path) == null) {
        return 'a probe recording gave no MP4 duration (no audio input)';
      }
    } on Object catch (e) {
      return 'the recorder failed to start ($e)';
    } finally {
      await recorder.dispose();
    }
    return null;
  } finally {
    await probe.dispose();
    dir.deleteSync(recursive: true);
  }
}

/// The remaining time the panel shows ("m:ss left"), in whole seconds.
int remainingSeconds(WidgetTester tester) {
  final text = tester
      .widget<Text>(find.byKey(const ValueKey('voice.time')))
      .data!;
  final m = RegExp(r'(\d+):(\d\d)').firstMatch(text)!;
  return int.parse(m.group(1)!) * 60 + int.parse(m.group(2)!);
}

/// Pumps until [shown] appears once the microphone is pressed. The first start of the audio
/// input on a loaded simulator or emulator can take long, so the wait is bounded only by the
/// repository's patience (`testkit.Patience`); a start that ended in a notice (failed or
/// denied) fails at once with its text.
Future<void> recordingStarts(WidgetTester tester, Finder shown) async {
  const patience = Duration(minutes: 3);
  final watch = Stopwatch()..start();
  final notice = find.byKey(const ValueKey('voice.notice'));
  while (shown.evaluate().isEmpty) {
    if (notice.evaluate().isNotEmpty) {
      final text = tester.widget<DsBanner>(notice).text;
      fail('the recording did not start: $text');
    }
    if (watch.elapsed >= patience) {
      fail('the recording did not start within $patience');
    }
    await tester.pump(const Duration(milliseconds: 50));
  }
}

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  late Directory temporary;
  setUp(() => temporary = Directory.systemTemp.createTempSync('phone-voice'));
  tearDown(() {
    if (temporary.existsSync()) temporary.deleteSync(recursive: true);
  });

  Future<(TestApp, String, FakeMediaPort, bool)> open(
    WidgetTester tester, {
    int? voiceLimitMs,
  }) async {
    // The fake's file is as long as the controller recorded up to its stop (the real time it
    // ran would add the lateness of a timer on a loaded machine, which the test must not
    // depend on); the real recorder's `mvhd` duration is checked against the limit below.
    final (recorder, real) = await _recorder(
      fakeDuration: voiceLimitMs == null
          ? null
          : voiceStopAt(Duration(milliseconds: voiceLimitMs)),
    );
    final media = FakeMediaPort();
    final app = await startPhone(
      tester,
      files: FilePorts(
        picker: FakePicker(),
        paste: FakePaste(),
        opener: FakeOpener(),
        recorder: recorder,
        player: FakePlayer.new,
        media: () => media,
        temporary: () async => temporary,
      ),
    );
    final world = app.fake.server.world;
    if (voiceLimitMs != null) {
      world.limits['voice_max_duration_ms'] = voiceLimitMs;
    }
    final seed = app.fake.seed;
    final forum = world.addForum(seed.space, 'notes', {seed.alex.id: 'write'});
    final thread = world.addThread(
      forum,
      'Call notes',
      createdBy: seed.alex.id,
    );
    final first = seed.post(seed.alex, thread, 'Leave a note here.');
    await signIn(app);
    await openThread(app, forum, thread, first['id']!);
    return (app, thread, media, real);
  }

  /// The `mvhd` duration of the uploaded file [fileId] of the fake.
  Future<Duration?> uploadedDuration(TestApp app, Object fileId) async {
    final bytes = app.fake.server.world.files['$fileId']!.bytes!;
    final f = File('${temporary.path}/uploaded.m4a')..writeAsBytesSync(bytes);
    return mp4Duration(f.path);
  }

  Map<String, Object?> fileOf(Map<String, Object?> message) {
    final attachments = message['attachments']! as List;
    expect(attachments, hasLength(1));
    return ((attachments.single as Map)['file']! as Map).cast();
  }

  testWidgets('a held recording arrives as audio/mp4 with kind voice', (
    tester,
  ) async {
    final (app, thread, _, real) = await open(tester);
    final world = app.fake.server.world;
    try {
      final mic = find.byKey(const ValueKey('voice.mic'));
      await untilStill(tester, mic);
      final gesture = await tester.startGesture(tester.getCenter(mic));
      await recordingStarts(
        tester,
        find.byKey(const ValueKey('voice.swipeHint')),
      );
      // Hold for a few seconds of real time: until the shown remaining time is two seconds
      // shorter than at the start.
      final start = remainingSeconds(tester);
      await until(
        tester,
        'two seconds recorded',
        () => remainingSeconds(tester) <= start - 2,
      );
      await gesture.up();
      await until(
        tester,
        'voice message stored',
        () => world.messagesOf(thread).any((m) => m['kind'] == 'voice'),
      );
      final voice = world
          .messagesOf(thread)
          .firstWhere((m) => m['kind'] == 'voice');
      final file = fileOf(voice);
      expect(world.files['${file['id']}']!.json['mime_type'], 'audio/mp4');
      if (real) {
        final d = await uploadedDuration(app, file['id']!);
        expect(d, isNotNull, reason: 'the recording has an mvhd duration');
        expect(d!, greaterThanOrEqualTo(const Duration(seconds: 1)));
      }
      expect(app.fake.server.violations, isEmpty);
    } finally {
      await app.stop();
    }
  });

  testWidgets('a recording over a low limit stops by itself with the notice', (
    tester,
  ) async {
    const limitMs = 3000;
    final (app, thread, _, real) = await open(tester, voiceLimitMs: limitMs);
    final world = app.fake.server.world;
    try {
      final mic = find.byKey(const ValueKey('voice.mic'));
      await untilStill(tester, mic);
      final gesture = await tester.startGesture(tester.getCenter(mic));
      await recordingStarts(tester, find.byKey(const ValueKey('voice.panel')));
      // The limit is shorter than `client.voice.limit_notice_lead`: the notice shows at once.
      await app.waitFor(find.byKey(const ValueKey('voice.limitNotice')));
      // The finger stays down; the limit stops the recording and says why.
      await app.waitFor(find.byKey(const ValueKey('voice.limitReached')));
      expect(find.text(en.voiceLimitReached('0:03')), findsOneWidget);
      await gesture.up();
      await tester.pump();
      // Nothing was sent by the release: the stopped recording waits for the person.
      expect(
        world.messagesOf(thread).where((m) => m['kind'] == 'voice'),
        isEmpty,
      );
      await tester.tap(find.byKey(const ValueKey('voice.send')));
      await until(
        tester,
        'voice message stored',
        () => world.messagesOf(thread).any((m) => m['kind'] == 'voice'),
      );
      final voice = world
          .messagesOf(thread)
          .firstWhere((m) => m['kind'] == 'voice');
      if (real) {
        final d = await uploadedDuration(app, fileOf(voice)['id']!);
        expect(d, isNotNull);
        expect(d!, lessThanOrEqualTo(const Duration(milliseconds: limitMs)));
      }
      expect(app.fake.server.violations, isEmpty);
    } finally {
      await app.stop();
    }
  });

  testWidgets('a photo of the library is sent as JPEG', (tester) async {
    final (app, thread, media, _) = await open(tester);
    final world = app.fake.server.world;
    try {
      final png = File('${temporary.path}/IMG_0042.png')
        ..writeAsBytesSync(
          img.encodePng(
            img.Image(width: 16, height: 12)..clear(img.ColorRgb8(20, 90, 160)),
          ),
        );
      media.next = RawMedia(path: png.path, name: 'IMG_0042.png');
      final attach = find.byWidgetPredicate(
        (w) => w is DsTappable && w.label == en.composerAttach,
      );
      await untilStill(tester, attach);
      await tester.tap(attach);
      final library = find.byKey(const ValueKey('composer.attach.library'));
      await app.waitFor(library);
      // The sheet slides in: tap once it stands still, where the person sees it.
      await untilStill(tester, library);
      await tester.tap(library);
      await app.waitFor(find.byType(UploadChip));
      await tester.tap(
        find.byWidgetPredicate(
          (w) => w is DsTappable && w.label == en.composerSend,
        ),
      );
      await until(
        tester,
        'photo message stored',
        () => world
            .messagesOf(thread)
            .any((m) => (m['attachments'] as List?)?.isNotEmpty ?? false),
      );
      final message = world
          .messagesOf(thread)
          .firstWhere((m) => (m['attachments'] as List?)?.isNotEmpty ?? false);
      final file = fileOf(message);
      final stored = world.files['${file['id']}']!;
      expect(stored.json['original_name'], 'IMG_0042.jpg');
      final bytes = stored.bytes!;
      expect(isJpeg(bytes), isTrue);
      expect(media.asked, [MediaSource.library]);
      expect(app.fake.server.violations, isEmpty);
    } finally {
      await app.stop();
    }
  });

  testWidgets('three photos of the library arrive in one message', (
    tester,
  ) async {
    final (app, thread, media, _) = await open(tester);
    final world = app.fake.server.world;
    try {
      media.many = [
        for (final (i, color) in [
          img.ColorRgb8(200, 30, 30),
          img.ColorRgb8(30, 160, 60),
          img.ColorRgb8(30, 60, 200),
        ].indexed)
          () {
            final png = File('${temporary.path}/IMG_000$i.png')
              ..writeAsBytesSync(
                img.encodePng(img.Image(width: 16, height: 12)..clear(color)),
              );
            return RawMedia(path: png.path, name: 'IMG_000$i.png');
          }(),
      ];
      final attach = find.byWidgetPredicate(
        (w) => w is DsTappable && w.label == en.composerAttach,
      );
      await untilStill(tester, attach);
      await tester.tap(attach);
      final library = find.byKey(const ValueKey('composer.attach.library'));
      await app.waitFor(library);
      await untilStill(tester, library);
      await tester.tap(library);
      await until(
        tester,
        'three chips',
        () => find.byType(UploadChip).evaluate().length == 3,
      );
      await tester.tap(
        find.byWidgetPredicate(
          (w) => w is DsTappable && w.label == en.composerSend,
        ),
      );
      bool withPhotos(Map<String, Object?> m) =>
          ((m['attachments'] as List?)?.length ?? 0) == 3;
      await until(
        tester,
        'message with three photos stored',
        () => world.messagesOf(thread).any(withPhotos),
      );
      final message = world.messagesOf(thread).singleWhere(withPhotos);
      final names = <Object?>[];
      for (final a in message['attachments']! as List) {
        final id = ((a as Map)['file']! as Map)['id'];
        final stored = world.files['$id']!;
        expect(isJpeg(stored.bytes!), isTrue);
        names.add(stored.json['original_name']);
      }
      expect(names, ['IMG_0000.jpg', 'IMG_0001.jpg', 'IMG_0002.jpg']);
      expect(media.limits, [10]);
      expect(app.fake.server.violations, isEmpty);
    } finally {
      await app.stop();
    }
  });
}
