// SPDX-License-Identifier: Apache-2.0

/// Recording a voice message on the phone (01 5.3 M5, 8; 15 13; 12 8.1; brief M5-07 Design
/// notes 1 and 2): [PhoneVoiceRecorder] runs one recording through the states of
/// [PhoneVoicePhase] over the [VoiceRecorder] port of M4-22, counts the time on an injected
/// [RecordingClock], shows the notice `client.voice.limit_notice_lead` before
/// `voice_max_duration_ms`, stops by itself a margin before that limit ([voiceStopAt]) and
/// hands a finished recording to its sender, which writes the upload entry and the message
/// with `kind = voice` into the outbox ([sendVoice]).
library;

import 'dart:async';
import 'dart:io';

import 'package:flutter/foundation.dart';

import '../../voice/recorder.dart';

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

/// The states of a recording on the phone (brief M5-07 Design note 2).
enum PhoneVoicePhase {
  /// Nothing recorded; the microphone waits for a press.
  idle,

  /// The finger holds the microphone and the recording runs: releasing sends, a swipe left
  /// discards, a swipe up locks.
  recording,

  /// The recording runs without the finger (after the swipe up or in the button mode): Stop,
  /// Send and Discard show.
  locked,

  /// The recorder finishes the file.
  stopping,

  /// The file exists and is within the limit: the preview with play, Send and Discard.
  ready,

  /// The recording was discarded and its file deleted.
  discarded,

  /// The microphone permission is denied: the notice explains the system settings.
  denied,

  /// The recorder failed.
  failed,
}

/// One voice recording of the phone's composer.
///
/// Gestures (01 5.3 M5): [press] starts, [release] sends a running recording, [discard] (the
/// swipe left) deletes it, [lock] (the swipe up) keeps it running without the finger. The
/// button mode of 01 8 calls [startLocked], then [stop] or [send] or [discard]. [interrupt]
/// (the app goes to the background, a call) stops a running recording and keeps it as
/// [PhoneVoicePhase.ready]; nothing is ever sent without the person.
///
/// Example:
/// ```dart
/// final voice = PhoneVoiceRecorder(recorder: ports.recorder, bitRate: config.voiceBitrate,
///     noticeLead: config.voiceLimitNoticeLead, scratch: temporaryFolder,
///     clock: SystemRecordingClock(ports.clock.now),
///     onSend: (file) => sendVoice(outbox, attach, thread, file));
/// await voice.press();
/// await voice.release();
/// ```
final class PhoneVoiceRecorder extends ChangeNotifier {
  /// Creates the controller.
  ///
  /// [recorder] makes the recorder of the port, [bitRate] is `client.voice.bitrate`,
  /// [noticeLead] `client.voice.limit_notice_lead`, [limit] `voice_max_duration_ms` (null while
  /// unknown: then only the server checks), [scratch] the temporary folder of the recordings,
  /// [clock] the time and [onSend] sends a finished recording (and throws when it could not).
  PhoneVoiceRecorder({
    required VoiceRecorder Function() recorder,
    required this.bitRate,
    required this.noticeLead,
    required this._scratch,
    required this._onSend,
    required this._clock,
    this.limit,
  }) : _newRecorder = recorder;

  final VoiceRecorder Function() _newRecorder;
  final Future<Directory> Function() _scratch;
  final Future<void> Function(RecordedFile file) _onSend;
  final RecordingClock _clock;

  /// `client.voice.bitrate` in bits per second.
  final int bitRate;

  /// `client.voice.limit_notice_lead`.
  final Duration noticeLead;

  /// `voice_max_duration_ms` of the instance, or null while unknown.
  Duration? limit;

  /// The tick of the shown time; the text shows whole seconds.
  static const Duration _tick = Duration(seconds: 1);

  VoiceRecorder? _recorder;
  Timer? _ticker;
  Timer? _notice;
  Timer? _stopper;
  DateTime? _startedAt;
  bool _disposed = false;

  /// Whether the finger left the microphone while the recorder started: the start then ends
  /// in a discard (a tap too short to record).
  bool _releasedWhileStarting = false;

  /// Whether Discard came while the recorder finished the file ([PhoneVoicePhase.stopping]).
  bool _discardWhenStopped = false;

  /// The state.
  PhoneVoicePhase phase = PhoneVoicePhase.idle;

  /// How long the recording runs or ran.
  Duration elapsed = Duration.zero;

  /// Whether the notice before the limit shows.
  bool noticeShown = false;

  /// Whether the recording stopped by itself at the limit.
  bool limitReached = false;

  /// The finished recording in [PhoneVoicePhase.ready].
  RecordedFile? recorded;

  /// When the recording stops by itself, measured from its start; null without a limit.
  Duration? get stopsAt {
    final max = limit;
    return max == null ? null : voiceStopAt(max);
  }

  /// The time left until [stopsAt], never below zero; null without a limit.
  Duration? get remaining {
    final end = stopsAt;
    if (end == null) return null;
    final left = end - elapsed;
    return left.isNegative ? Duration.zero : left;
  }

  /// The time the notice before the limit names: `client.voice.limit_notice_lead`, or the
  /// whole recording time when the limit is shorter than the lead.
  Duration get noticeAt {
    final end = stopsAt ?? Duration.zero;
    return noticeLead < end ? noticeLead : end;
  }

  /// Whether a recording runs ([PhoneVoicePhase.recording] or [PhoneVoicePhase.locked]).
  bool get running =>
      phase == PhoneVoicePhase.recording || phase == PhoneVoicePhase.locked;

  void _set(PhoneVoicePhase next) {
    phase = next;
    _changed();
  }

  void _changed() {
    if (!_disposed) notifyListeners();
  }

  /// The finger presses the microphone: asks for the permission at the first use (the system
  /// asks the person once, 15 13) and starts recording in [PhoneVoicePhase.recording].
  Future<void> press() => _begin(locked: false);

  /// The button mode (01 8): starts recording directly in [PhoneVoicePhase.locked].
  Future<void> startLocked() => _begin(locked: true);

  Future<void>? _starting;

  Future<void> _begin({required bool locked}) =>
      _starting ??= _start(locked: locked).whenComplete(() => _starting = null);

  Future<void> _start({required bool locked}) async {
    // A running or finished recording is never replaced by a second press.
    if (running ||
        phase == PhoneVoicePhase.stopping ||
        phase == PhoneVoicePhase.ready) {
      return;
    }
    _cancelTimers();
    _releasedWhileStarting = false;
    noticeShown = false;
    limitReached = false;
    recorded = null;
    elapsed = Duration.zero;
    final recorder = _recorder ??= _newRecorder();
    final permitted = await recorder.hasPermission();
    if (_disposed) return;
    if (!permitted) {
      _set(PhoneVoicePhase.denied);
      return;
    }
    // A press that ended while the system asked for the microphone records nothing: the
    // person lifted the finger to answer the dialog.
    if (_releasedWhileStarting && !locked) {
      _set(PhoneVoicePhase.idle);
      return;
    }
    final dir = await _scratch();
    await dir.create(recursive: true);
    if (_disposed) return;
    final path =
        '${dir.path}${Platform.pathSeparator}'
        'voice-${_clock.now().toUtc().millisecondsSinceEpoch}.m4a';
    // The recording time counts from before the start call, so a slow start is counted
    // against the limit too (D-M5-07-1).
    final startedAt = _clock.now();
    try {
      await recorder.start(path: path, bitRate: bitRate);
    } on Object {
      _set(PhoneVoicePhase.failed);
      return;
    }
    if (_disposed) {
      await recorder.cancel();
      return;
    }
    if (_releasedWhileStarting && !locked) {
      // Released before the recorder ran: a tap, not a message.
      await recorder.cancel();
      await _deleteFile(path);
      _set(PhoneVoicePhase.idle);
      return;
    }
    _startedAt = startedAt;
    final spent = _clock.now().difference(startedAt);
    _set(locked ? PhoneVoicePhase.locked : PhoneVoicePhase.recording);
    _ticker = _clock.every(_tick, _tickNow);
    _armLimit(spent);
  }

  /// The notice and the stop of the limit, counted from the start call; [spent] is the time
  /// the recorder took to start.
  void _armLimit(Duration spent) {
    final end = stopsAt;
    if (end == null) return;
    Duration fromNow(Duration at) {
      final left = at - spent;
      return left.isNegative ? Duration.zero : left;
    }

    // The notice `client.voice.limit_notice_lead` before the stop; at once when the limit
    // is shorter than the lead.
    final noticeAt = end - noticeLead;
    _notice = _clock.after(
      fromNow(noticeAt.isNegative ? Duration.zero : noticeAt),
      () {
        noticeShown = true;
        _changed();
      },
    );
    // 12 8.1: the stop comes [voiceStopMargin] before `voice_max_duration_ms`.
    _stopper = _clock.after(fromNow(end), () {
      limitReached = true;
      noticeShown = true;
      unawaited(stop());
    });
  }

  void _tickNow() {
    final started = _startedAt;
    if (started == null || !running) return;
    elapsed = _clock.now().difference(started);
    _changed();
  }

  /// The finger leaves the microphone: a running recording ([PhoneVoicePhase.recording]) is
  /// sent; a start still waiting for the recorder ends without a recording.
  Future<void> release() async {
    if (_starting != null && phase == PhoneVoicePhase.idle) {
      _releasedWhileStarting = true;
      return;
    }
    if (phase != PhoneVoicePhase.recording) return;
    await send();
  }

  /// The swipe up: the recording runs on without the finger.
  void lock() {
    if (phase != PhoneVoicePhase.recording) return;
    _set(PhoneVoicePhase.locked);
  }

  /// Stops a running recording; it then waits in [PhoneVoicePhase.ready]. A second call while
  /// one stops (Stop at the moment of the limit) waits for the first.
  Future<void> stop() => _stopping ??= _stop().whenComplete(() {
    _stopping = null;
  });

  Future<void>? _stopping;

  Future<void> _stop() async {
    if (!running) return;
    _cancelTimers();
    _tickNowFinal();
    _set(PhoneVoicePhase.stopping);
    RecordedFile? file;
    try {
      file = await _recorder?.stop();
    } on Object {
      file = null;
    }
    if (_disposed || _discardWhenStopped) {
      // Discarded while the recorder finished, or the composer went: the file goes too.
      _discardWhenStopped = false;
      if (file != null) await _deleteFile(file.path);
      return;
    }
    if (file == null) {
      _set(PhoneVoicePhase.failed);
      return;
    }
    recorded = file;
    elapsed = file.duration;
    _set(PhoneVoicePhase.ready);
  }

  void _tickNowFinal() {
    final started = _startedAt;
    if (started != null) elapsed = _clock.now().difference(started);
  }

  /// Whether [recorded] is longer than `voice_max_duration_ms` (the server would refuse it
  /// with `voice_too_long`, 12 8.1): then it cannot be sent.
  bool get overLimit {
    final file = recorded;
    final max = limit;
    return file != null && max != null && file.duration > max;
  }

  /// Sends the recording: stops a running one first, then hands the file to the sender and
  /// returns to [PhoneVoicePhase.idle]. A send that fails keeps the recording in
  /// [PhoneVoicePhase.ready] for another try; a recording over the limit is never sent.
  Future<void> send() async {
    if (running) await stop();
    final file = recorded;
    if (file == null || phase != PhoneVoicePhase.ready || overLimit) return;
    recorded = null;
    _set(PhoneVoicePhase.idle);
    try {
      await _onSend(file);
    } on Object {
      if (_disposed || phase != PhoneVoicePhase.idle) return;
      recorded = file;
      _set(PhoneVoicePhase.ready);
    }
  }

  /// The swipe left or the Discard button: drops the recording and deletes its file at once
  /// (requirement 16); also closes the notice of a denied or failed recording.
  Future<void> discard() async {
    _cancelTimers();
    if (phase == PhoneVoicePhase.stopping) {
      // The recorder is finishing the file: it is deleted as soon as it exists.
      _discardWhenStopped = true;
      _set(PhoneVoicePhase.discarded);
      return;
    }
    if (_starting != null && phase == PhoneVoicePhase.idle) {
      _releasedWhileStarting = true;
    }
    if (running) await _recorder?.cancel();
    final file = recorded;
    recorded = null;
    if (file != null) await _deleteFile(file.path);
    if (phase == PhoneVoicePhase.denied || phase == PhoneVoicePhase.failed) {
      _set(PhoneVoicePhase.idle);
      return;
    }
    _set(PhoneVoicePhase.discarded);
  }

  /// The app goes to the background or a call starts: a running recording stops and waits in
  /// [PhoneVoicePhase.ready] for the person; it is never sent by itself (Design note 2).
  Future<void> interrupt() async {
    if (_starting != null && phase == PhoneVoicePhase.idle) {
      _releasedWhileStarting = true;
    }
    if (running) await stop();
  }

  void _cancelTimers() {
    _ticker?.cancel();
    _notice?.cancel();
    _stopper?.cancel();
    _ticker = _notice = _stopper = null;
  }

  static Future<void> _deleteFile(String path) async {
    try {
      final f = File(path);
      if (f.existsSync()) await f.delete();
    } on FileSystemException {
      // Lies in the temporary folder, which the system empties.
    }
  }

  @override
  void dispose() {
    _disposed = true;
    _cancelTimers();
    final recorder = _recorder;
    final file = recorded;
    final wasRunning = running;
    recorded = null;
    if (recorder != null) {
      // A running recording goes with the composer; a pending start finishes (and cancels its
      // recording) before the recorder goes.
      final starting = _starting;
      unawaited(() async {
        if (starting != null) {
          await starting.then<void>((_) {}, onError: (Object _) {});
        }
        if (wasRunning) await recorder.cancel();
        await recorder.dispose();
      }());
    }
    if (file != null) unawaited(_deleteFile(file.path));
    super.dispose();
  }
}
