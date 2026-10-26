// SPDX-License-Identifier: Apache-2.0

/// The port `VoiceRecorder` and its implementation over package `record` (15 13, 12 8.1;
/// brief M4-22 Design note 4): AAC-LC in an MP4 container (`.m4a`), mono, at the bitrate
/// `client.voice.bitrate`, into a file.
library;

import 'dart:async';
import 'dart:io';

import 'package:record/record.dart';

import 'mp4_duration.dart';

/// The sample rate of a recording: the default of package `record` for AAC-LC.
final int voiceSampleRate = const RecordConfig().sampleRate;

/// Samples in one AAC-LC frame (ISO/IEC 14496-3).
const int _aacFrameSamples = 1024;

/// The time a recording allows for its stop timer to fire late and for the stop call, while
/// the microphone still records (D-M5-07-1): a busy or loaded phone runs a timer late by tens
/// to hundreds of milliseconds, and the duration the server reads must stay within the limit
/// anyway. Half a second is nothing against a limit of minutes.
const Duration voiceStopAllowance = Duration(milliseconds: 500);

/// How much earlier than `voice.max_duration` a recording stops (brief M5-07 Design note 1;
/// docs/decisions.d/M5-07.md D-M5-07-1): the `mvhd` duration the server reads (12 8.1) counts
/// whole AAC frames and can include the encoder's priming frames, so three frames at
/// [sampleRate] (the frame in progress and two priming frames) plus [voiceStopAllowance] for a
/// late timer and the stop call.
Duration voiceStopMargin({int? sampleRate}) =>
    Duration(
      microseconds:
          3 *
          _aacFrameSamples *
          Duration.microsecondsPerSecond ~/
          (sampleRate ?? voiceSampleRate),
    ) +
    voiceStopAllowance;

/// When a recording limited to [limit] (`voice_max_duration_ms`) stops: [voiceStopMargin]
/// before it, never below zero.
Duration voiceStopAt(Duration limit, {int? sampleRate}) {
  final at = limit - voiceStopMargin(sampleRate: sampleRate);
  return at.isNegative ? Duration.zero : at;
}

/// The time and the timers of a recording; the tests replace it with a fake clock, so they
/// never wait for real time.
abstract interface class RecordingClock {
  /// The current time.
  DateTime now();

  /// Calls [callback] once after [delay].
  Timer after(Duration delay, void Function() callback);

  /// Calls [callback] every [period] until the timer is cancelled.
  Timer every(Duration period, void Function() callback);
}

/// The [RecordingClock] of the running app: [now] from the app clock, real timers.
final class SystemRecordingClock implements RecordingClock {
  /// Creates the clock over [now] (the app's `Clock.now`).
  const SystemRecordingClock(this._now);

  final DateTime Function() _now;

  @override
  DateTime now() => _now();

  @override
  Timer after(Duration delay, void Function() callback) =>
      Timer(delay, callback);

  @override
  Timer every(Duration period, void Function() callback) =>
      Timer.periodic(period, (_) => callback());
}

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

  /// How long it is: the `mvhd` duration of the file where the recorder wrote one (12 8.1).
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
      RecordConfig(
        numChannels: 1,
        bitRate: bitRate,
        sampleRate: voiceSampleRate,
      ),
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
    // The duration the server will read (12 8.1); the stopwatch only when the file has none.
    Duration? measured;
    try {
      measured = await mp4Duration(path);
    } on FileSystemException {
      measured = null;
    }
    return RecordedFile(
      path: path,
      duration: measured ?? _clock.elapsed,
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
