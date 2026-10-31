// SPDX-License-Identifier: Apache-2.0

// The media types the client plays (12 9.3 inline list, FIX-voice-send): the server stores an
// Apple recording (MP4 brand "M4A ") as `audio/x-m4a`, so a voice message from macOS or iOS
// must get the voice player like one stored as `audio/mp4`.

import 'package:client/files/attachment_mapping.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/file_widgets.dart';

void main() {
  test('MP4 audio of both detected names and MP3 are audio', () {
    for (final mime in ['audio/mp4', 'audio/x-m4a', 'audio/mpeg']) {
      expect(isAudio(fileOf(1, name: 'voice.m4a', mime: mime)), isTrue);
    }
  });

  test('video, WAV and other files are no audio for the player', () {
    for (final mime in ['video/mp4', 'audio/wav', 'application/pdf']) {
      expect(isAudio(fileOf(1, mime: mime)), isFalse, reason: mime);
    }
  });
}
