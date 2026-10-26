// SPDX-License-Identifier: Apache-2.0

// The accessible button mode of the phone's microphone (brief M5-07 Tests and Design note 3;
// 01 8): with an accessibility service on, a tap records and a second tap stops, every control
// has a label, the remaining time is a live region that changes at most once per
// `a11y.live_announce_min_interval`, the notice before the limit is announced, and the
// guidelines pass. Fake recorder, fake clock.

import 'dart:io';

import 'package:client/phone/compose/voice_button.dart';
import 'package:client/phone/compose/voice_recorder.dart';
import 'package:client/voice/recorder.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../../support/file_fakes.dart';
import '../../support/recording_clock.dart';
import '../../support/until.dart';
import '../../support/widgets.dart';
import '../support/phone_app.dart' show phoneWindow;
import '../support/phone_host.dart';

void main() {
  late Directory scratch;
  late FakeRecordingClock clock;
  late FakeRecorder recorder;
  late List<RecordedFile> sent;

  setUp(() {
    scratch = Directory.systemTemp.createTempSync('voice-a11y');
    clock = FakeRecordingClock();
    recorder = FakeRecorder();
    sent = [];
  });
  tearDown(() => scratch.deleteSync(recursive: true));

  const interval = Duration(seconds: 10);

  PhoneVoiceRecorder voice({Duration? limit}) => PhoneVoiceRecorder(
    recorder: () => recorder,
    bitRate: 64000,
    noticeLead: const Duration(seconds: 30),
    limit: limit,
    scratch: () async => scratch,
    clock: clock,
    onSend: (f) async => sent.add(f),
  );

  Future<void> pump(
    WidgetTester tester,
    PhoneVoiceRecorder v, {
    bool accessible = true,
  }) async {
    phoneWindow(tester);
    await tester.pumpWidget(
      hostPhone(
        Builder(
          builder: (context) => MediaQuery(
            data: MediaQuery.of(context)
                .copyWith(accessibleNavigation: accessible),
            child: Scaffold(
              body: Align(
                alignment: Alignment.bottomCenter,
                child: Padding(
                  padding: const EdgeInsets.all(DsSpace.s16),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      PhoneVoicePanel(
                        controller: v,
                        platform: 'ios',
                        announceEvery: interval,
                        player: FakePlayer.new,
                      ),
                      Align(
                        alignment: Alignment.centerRight,
                        child: PhoneVoiceButton(controller: v),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pump();
  }

  Finder labeled(String label) => find.bySemanticsLabel(label);

  String liveLabel(WidgetTester tester) =>
      tester.getSemantics(find.byKey(const ValueKey('voice.live'))).label;

  Future<void> waitFor(
    WidgetTester tester,
    bool Function() done, {
    required String what,
  }) async {
    await pumpUntil(tester, done, what: what);
    await tester.pump();
  }

  Future<void> act(WidgetTester tester, Future<void> Function() action) async {
    await tester.runAsync(action);
    await tester.pump();
  }

  testWidgets('a tap records, a second tap stops, then Send or Discard', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    final v = voice(limit: const Duration(minutes: 15));
    await pump(tester, v);
    expect(labeled(en.composerVoice), findsOneWidget);
    await tester.tap(labeled(en.composerVoice));
    await waitFor(
      tester,
      () => v.phase == PhoneVoicePhase.locked,
      what: 'v.phase == PhoneVoicePhase.locked',
    );
    expect(v.phase, PhoneVoicePhase.locked);
    for (final label in [en.voiceCancel, en.voiceStop, en.voiceSend]) {
      expect(labeled(label), findsWidgets, reason: label);
    }
    // The microphone itself stops in the button mode.
    expect(
      tester.widget<DsTappable>(find.byKey(const ValueKey('voice.mic'))).label,
      en.voiceStop,
    );
    await tester.tap(find.byKey(const ValueKey('voice.stop')));
    await waitFor(
      tester,
      () => v.phase == PhoneVoicePhase.ready,
      what: 'v.phase == PhoneVoicePhase.ready',
    );
    for (final label in [en.voicePhonePlay, en.voiceCancel, en.voiceSend]) {
      expect(labeled(label), findsWidgets, reason: label);
    }
    await tester.tap(find.byKey(const ValueKey('voice.send')));
    await waitFor(tester, () => sent.isNotEmpty, what: 'sent.isNotEmpty');
    expect(sent, hasLength(1));
    await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
    await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
    await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
    v.dispose();
    handle.dispose();
  });

  testWidgets('the gesture mode has the hint and a semantic tap records', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    final v = voice();
    await pump(tester, v, accessible: false);
    final mic = tester.getSemantics(find.byKey(const ValueKey('voice.mic')));
    expect(mic.label, en.composerVoice);
    expect(mic.hint, en.voicePhoneHoldHint);
    expect(mic.flagsCollection.isButton, isTrue);
    // A screen reader's activation records in the locked form, no gesture needed.
    tester.semantics.tap(find.semantics.byLabel(en.composerVoice));
    await waitFor(
      tester,
      () => v.phase == PhoneVoicePhase.locked,
      what: 'v.phase == PhoneVoicePhase.locked',
    );
    expect(find.byKey(const ValueKey('voice.send')), findsOneWidget);
    await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
    await act(tester, v.discard);
    v.dispose();
    handle.dispose();
  });

  testWidgets(
    'the remaining time changes its live region at most once per interval',
    (tester) async {
      final handle = tester.ensureSemantics();
      final v = voice(limit: const Duration(minutes: 2));
      await pump(tester, v);
      await act(tester, v.startLocked);
      final node = tester.getSemantics(
        find.byKey(const ValueKey('voice.live')),
      );
      expect(node.flagsCollection.isLiveRegion, isTrue);
      final labels = <String>[liveLabel(tester)];
      final changes = <Duration>[Duration.zero];
      for (var s = 1; s <= 45; s++) {
        clock.advance(const Duration(seconds: 1));
        await tester.pump();
        final now = liveLabel(tester);
        if (now != labels.last) {
          labels.add(now);
          changes.add(v.elapsed);
        }
      }
      // The visible time ticks every second; the live region only every interval.
      expect(labels.length, greaterThan(1));
      for (var i = 1; i < changes.length; i++) {
        expect(changes[i] - changes[i - 1], greaterThanOrEqualTo(interval));
      }
      expect(labels.length, lessThanOrEqualTo(45 ~/ interval.inSeconds + 1));
      expect(
        find.text(
          en.voicePhoneRemaining(
            L10nFormat(const Locale('en')).elapsed(v.remaining!),
          ),
        ),
        findsOneWidget,
      );
      await act(tester, v.discard);
      v.dispose();
      handle.dispose();
    },
  );

  testWidgets('the notice before the limit is an announced banner', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    final v = voice(limit: const Duration(seconds: 50));
    await pump(tester, v);
    await act(tester, v.startLocked);
    expect(find.byKey(const ValueKey('voice.limitNotice')), findsNothing);
    clock.advance(voiceStopAt(const Duration(seconds: 50)) - v.noticeAt);
    await tester.pump();
    final banner = find.byKey(const ValueKey('voice.limitNotice'));
    expect(banner, findsOneWidget);
    expect(find.text(en.voicePhoneLimitNotice('0:30')), findsOneWidget);
    expect(
      find.descendant(
        of: banner,
        matching: find.byWidgetPredicate(
          (w) => w is Semantics && (w.properties.liveRegion ?? false),
        ),
      ),
      findsWidgets,
    );
    // At the limit: stopped, ready, and the reason shown.
    clock.advance(v.noticeAt);
    await waitFor(
      tester,
      () => v.phase == PhoneVoicePhase.ready,
      what: 'v.phase == PhoneVoicePhase.ready',
    );
    expect(find.byKey(const ValueKey('voice.limitReached')), findsOneWidget);
    expect(find.text(en.voiceLimitReached('0:50')), findsOneWidget);
    await act(tester, v.discard);
    v.dispose();
    handle.dispose();
  });

  testWidgets('a denied microphone shows the way to the settings', (
    tester,
  ) async {
    recorder.permitted = false;
    final v = voice();
    await pump(tester, v);
    await act(tester, v.press);
    expect(find.text(en.voicePhoneDeniedIos), findsOneWidget);
    v.dispose();
  });
}
