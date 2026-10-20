// SPDX-License-Identifier: Apache-2.0

/// The port `VoiceRecorder` and its implementation over package `record` (15 13, 12 8.1;
/// brief M4-22 Design note 4): AAC-LC in an MP4 container (`.m4a`), mono, at the bitrate
/// `client.voice.bitrate`, into a file.
library;

import 'dart:async';
import 'dart:io';

import 'package:record/record.dart';

/// A finished recording.
final class RecordedFile {
  /// Creates the recording at [path].
  const RecordedFile({
    required this.path,
    required this.duration,
    required this.sizeBytes,
  });

  /// The `.m4a` file.
  final String path;

  /// How long it is.
  final Duration duration;

  /// Its size on disk.
  final int sizeBytes;
}

/// Records one voice message.
///
/// The caller asks [hasPermission] before the first [start] (the system asks the person at
/// the first use, 15 13), then calls [stop] or [cancel] exactly once. [amplitude] reports the
/// input level from 0 to 1 while recording.
abstract interface class VoiceRecorder {
  /// Whether the microphone may be used; asks the person when the system has not asked yet.
  Future<bool> hasPermission();

  /// Starts recording AAC-LC mono at [bitRate] bits per second into [path].
  ///
  /// Throws when the platform cannot record.
  Future<void> start({required String path, required int bitRate});

  /// Stops and returns the recording, or null when nothing was recorded.
  Future<RecordedFile?> stop();

  /// Stops and deletes what was recorded.
  Future<void> cancel();

  /// The input level from 0 (silence) to 1.
  Stream<double> get amplitude;

  /// Releases the recorder.
  Future<void> dispose();
}

/// [VoiceRecorder] over package `record` (7.1.2, BSD-3-Clause).
final class RecordVoiceRecorder implements VoiceRecorder {
  /// Creates the recorder.
  RecordVoiceRecorder() : _recorder = AudioRecorder();

  final AudioRecorder _recorder;
  final Stopwatch _clock = Stopwatch();

  /// How often the input level is read.
  static const Duration _levelEvery = Duration(milliseconds: 100);

  @override
  Future<bool> hasPermission() => _recorder.hasPermission();

  @override
  Future<void> start({required String path, required int bitRate}) async {
    _clock
      ..reset()
      ..start();
    await _recorder.start(
      // 15 13: AAC-LC is the one compressed format `record` writes on every target.
      RecordConfig(numChannels: 1, bitRate: bitRate),
      path: path,
    );
  }

  @override
  Future<RecordedFile?> stop() async {
    final path = await _recorder.stop();
    _clock.stop();
    if (path == null) return null;
    final file = File(path);
    if (!file.existsSync()) return null;
    return RecordedFile(
      path: path,
      duration: _clock.elapsed,
      sizeBytes: file.lengthSync(),
    );
  }

  @override
  Future<void> cancel() async {
    _clock.stop();
    await _recorder.cancel();
  }

  @override
  Stream<double> get amplitude => _recorder
      .onAmplitudeChanged(_levelEvery)
      // dBFS from about -60 (silence) to 0 (full scale).
      .map((a) => ((a.current + 60) / 60).clamp(0, 1).toDouble());

  @override
  Future<void> dispose() => _recorder.dispose();
}
