// SPDX-License-Identifier: Apache-2.0

// The goldens of the recording states on the phone (brief M5-07 Tests; 01 5.3 M5): recording
// with the finger on the microphone, locked, the notice before the limit, the finished
// recording stopped at the limit, and the denied microphone, in the density `touch`, light and
// dark, text scaling 1 and 2, German and English. One file per state and theme keeps each file
// below repo.max_file_kb.

import 'dart:io';

import 'package:alchemist/alchemist.dart';
import 'package:client/phone/compose/voice_button.dart';
import 'package:client/phone/compose/voice_recorder.dart';
import 'package:client/voice/recorder.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import '../../phone/support/phone_app.dart' show phoneSize;
import '../../support/file_fakes.dart';
import '../../support/recording_clock.dart';

/// A recording shown in [phase] after [elapsed] of a limit of 15 minutes; the fields are set
/// directly, so no recorder runs while the golden renders.
PhoneVoiceRecorder _state(
  PhoneVoicePhase phase, {
  Duration elapsed = const Duration(seconds: 12),
  bool notice = false,
  bool limitReached = false,
}) {
  const limit = Duration(minutes: 15);
  return PhoneVoiceRecorder(
      recorder: FakeRecorder.new,
      bitRate: 64000,
      noticeLead: const Duration(seconds: 30),
      limit: limit,
      scratch: () async => Directory.systemTemp,
      clock: FakeRecordingClock(),
      onSend: (_) async {},
    )
    ..phase = phase
    ..elapsed = elapsed
    ..noticeShown = notice
    ..limitReached = limitReached
    ..recorded = phase == PhoneVoicePhase.ready
        ? RecordedFile(path: 'voice.m4a', duration: elapsed, sizeBytes: 1)
        : null;
}

/// The bottom of a thread on the phone: the panel above the input with the microphone.
Widget _scene(PhoneVoiceRecorder voice, {String platform = 'ios'}) => SizedBox(
  width: phoneSize.width,
  child: Padding(
    padding: const EdgeInsets.all(DsSpace.s16),
    child: Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        PhoneVoicePanel(
          controller: voice,
          platform: platform,
          announceEvery: const Duration(seconds: 10),
          player: FakePlayer.new,
        ),
        Row(
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            Expanded(
              child: DsComposer(
                targetName: 'gateway',
                maxLines: 4,
                enterSends: false,
                onSend: (_) {},
                onAttach: () {},
              ),
            ),
            PhoneVoiceButton(controller: voice, buttonMode: false),
          ],
        ),
      ],
    ),
  ),
);

/// The recording states by name, for the goldens and the accessibility tests (brief M5-10).
final Map<String, Widget Function()> voiceScenes = {
  'recording': () => _scene(_state(PhoneVoicePhase.recording)),
  'locked': () => _scene(_state(PhoneVoicePhase.locked)),
  'notice': () => _scene(
    _state(
      PhoneVoicePhase.locked,
      elapsed: const Duration(minutes: 14, seconds: 35),
      notice: true,
    ),
  ),
  'limit_reached': () => _scene(
    _state(
      PhoneVoicePhase.ready,
      elapsed: const Duration(minutes: 14, seconds: 59),
      limitReached: true,
    ),
  ),
  'denied': () => _scene(_state(PhoneVoicePhase.denied)),
};

void main() {
  for (final MapEntry(key: name, value: scene) in voiceScenes.entries) {
    for (final theme in DsThemeMode.values) {
      goldenTest(
        'voice $name in ${theme.name}, text scale 1 and 2, de and en',
        fileName: 'phone_voice_${name}_${theme.name}',
        pumpBeforeTest: dsPumpIcons,
        builder: () => dsGoldenMatrix(
          name: name,
          themes: [theme],
          densities: const [DsDensityMode.touch],
          textScales: const [1.0, 2.0],
          locales: const [Locale('de'), Locale('en')],
          builder: scene,
        ),
      );
    }
  }
}
