// SPDX-License-Identifier: Apache-2.0

/// Recording and sending a voice message (15 13; 12 8.1; brief M4-22 Design note 4):
/// [VoiceController] asks for the microphone at the first use, records into a temporary
/// `.m4a` file at `client.voice.bitrate`, stops by itself at `voice_max_duration_ms` and
/// hands the recording to its sender; [VoiceRecordingBar] is its bar above the composer, and
/// [sendVoice] writes the upload entry and the message with `kind = voice` and exactly one
/// attachment.
library;

import 'dart:async';
import 'dart:io';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';
import 'package:local_store/local_store.dart' show OutboxEntry;
import 'package:sync_engine/sync_engine.dart';

import '../files/attach_controller.dart';
import '../thread/thread_actions.dart';
import 'recorder.dart';

/// What the voice controller does now.
enum VoicePhase {
  /// Nothing recorded.
  idle,

  /// Recording.
  recording,

  /// A recording waits to be sent or discarded.
  recorded,

  /// The microphone permission is denied: the bar explains the system settings.
  denied,

  /// The recorder failed.
  failed,
}

/// One voice recording of the composer.
///
/// Example:
/// ```dart
/// final voice = VoiceController(recorder: ports.recorder, bitRate: config.voiceBitrate,
///     limit: Duration(milliseconds: limits.voiceMaxDurationMs), scratch: tempDir,
///     onSend: (file) => sendVoice(outbox, attach, thread, file));
/// await voice.start();
/// ```
final class VoiceController extends ChangeNotifier {
  /// Creates the controller; [recorder] makes a recorder per recording, [bitRate] is
  /// `client.voice.bitrate`, [limit] `voice_max_duration_ms` (null while unknown: the server
  /// checks then), [scratch] the folder of the recordings and [onSend] sends one.
  VoiceController({
    required VoiceRecorder Function() recorder,
    required this.bitRate,
    required this._scratch,
    required this._onSend,
    this.limit,
  }) : _newRecorder = recorder;

  final VoiceRecorder Function() _newRecorder;
  final Future<Directory> Function() _scratch;
  final Future<void> Function(RecordedFile file) _onSend;

  /// `client.voice.bitrate` in bits per second.
  final int bitRate;

  /// `voice_max_duration_ms` of the instance, or null.
  Duration? limit;

  VoiceRecorder? _recorder;
  Timer? _ticker;
  Timer? _stopper;
  bool _disposed = false;

  /// The phase.
  VoicePhase phase = VoicePhase.idle;

  /// How long the recording runs or ran.
  Duration elapsed = Duration.zero;

  /// Whether the recording stopped at [limit].
  bool limitReached = false;

  /// The recording that waits in [VoicePhase.recorded].
  RecordedFile? recorded;

  void _set(VoicePhase next) {
    phase = next;
    if (!_disposed) notifyListeners();
  }

  /// Starts a recording; asks for the microphone first (the system asks the person once).
  Future<void> start() async {
    if (phase == VoicePhase.recording) return;
    final recorder = _recorder ??= _newRecorder();
    limitReached = false;
    recorded = null;
    elapsed = Duration.zero;
    if (!await recorder.hasPermission()) {
      _set(VoicePhase.denied);
      return;
    }
    final dir = await _scratch();
    await dir.create(recursive: true);
    final path =
        '${dir.path}${Platform.pathSeparator}'
        'voice-${DateTime.now().toUtc().millisecondsSinceEpoch}.m4a';
    try {
      await recorder.start(path: path, bitRate: bitRate);
    } on Object {
      _set(VoicePhase.failed);
      return;
    }
    _set(VoicePhase.recording);
    final started = DateTime.now();
    _ticker = Timer.periodic(const Duration(seconds: 1), (_) {
      elapsed = DateTime.now().difference(started);
      if (!_disposed) notifyListeners();
    });
    final max = limit;
    if (max != null) {
      // 15 13: never longer than `voice_max_duration_ms`.
      _stopper = Timer(max, () {
        limitReached = true;
        unawaited(stop());
      });
    }
  }

  /// Stops the recording; it then waits to be sent.
  Future<void> stop() async {
    if (phase != VoicePhase.recording) return;
    _ticker?.cancel();
    _stopper?.cancel();
    final file = await _recorder?.stop();
    if (file == null) {
      _set(VoicePhase.failed);
      return;
    }
    recorded = file;
    final max = limit;
    elapsed = limitReached && max != null ? max : file.duration;
    _set(VoicePhase.recorded);
  }

  /// Discards the recording or the notice.
  Future<void> cancel() async {
    _ticker?.cancel();
    _stopper?.cancel();
    if (phase == VoicePhase.recording) await _recorder?.cancel();
    final file = recorded;
    recorded = null;
    if (file != null) {
      final f = File(file.path);
      if (f.existsSync()) await f.delete();
    }
    _set(VoicePhase.idle);
  }

  /// Sends the stopped recording (stopping a running one first).
  Future<void> send() async {
    if (phase == VoicePhase.recording) await stop();
    final file = recorded;
    if (file == null) return;
    recorded = null;
    _set(VoicePhase.idle);
    await _onSend(file);
  }

  @override
  void dispose() {
    _disposed = true;
    _ticker?.cancel();
    _stopper?.cancel();
    unawaited(_recorder?.dispose());
    super.dispose();
  }
}

/// Writes the voice message of [file] into [thread]: an upload entry through [attach] and a
/// message with `kind = voice` and exactly that one attachment, which waits for the upload
/// (12 8.1; 15 7.5).
Future<OutboxEntry> sendVoice(
  Outbox outbox,
  AttachController attach,
  ThreadId thread,
  RecordedFile file,
) async {
  final attachments = await attach.voice(file);
  return outbox.enqueue(
    ThreadActions.messageRequest(
      thread,
      '',
      attachments: attachments,
      kind: MessageKind.voice,
    ),
  );
}

/// The bar of [controller] above the composer: the running time with Stop and Discard, the
/// stopped recording with Send and Discard (and the limit notice), or the notice of a denied
/// microphone.
class VoiceRecordingBar extends StatelessWidget {
  /// Creates the bar.
  const VoiceRecordingBar({super.key, required this.controller});

  /// The recording.
  final VoiceController controller;

  @override
  Widget build(BuildContext context) => ListenableBuilder(
    listenable: controller,
    builder: (context, _) {
      final l = AppLocalizations.of(context);
      final colors = context.dsColors;
      final format = L10nFormat(Localizations.localeOf(context));
      Widget button(
        String key,
        String label,
        DsButtonStyle style,
        VoidCallback onTap,
      ) => DsButton(
        key: ValueKey('voice.$key'),
        label: label,
        style: style,
        onPressed: onTap,
      );
      switch (controller.phase) {
        case VoicePhase.idle:
          return const SizedBox.shrink();
        case VoicePhase.denied || VoicePhase.failed:
          return Padding(
            padding: const EdgeInsets.only(bottom: DsSpace.s4),
            child: DsBanner(
              key: const ValueKey('voice.notice'),
              tone: DsTone.warning,
              text: controller.phase == VoicePhase.denied
                  ? l.voiceMicrophoneDenied
                  : l.voiceFailed,
              liveRegion: true,
              actionLabel: l.threadCancel,
              onAction: () => unawaited(controller.cancel()),
            ),
          );
        case VoicePhase.recording || VoicePhase.recorded:
          final recording = controller.phase == VoicePhase.recording;
          final time = format.elapsed(controller.elapsed);
          final max = controller.limit;
          return Padding(
            key: const ValueKey('voice.bar'),
            padding: const EdgeInsets.only(bottom: DsSpace.s4),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              mainAxisSize: MainAxisSize.min,
              children: [
                if (controller.limitReached && max != null)
                  Text(
                    l.voiceLimitReached(format.elapsed(max)),
                    style: dsTextStyle(DsType.caption)
                        .copyWith(color: colors.textSecondary),
                  ),
                Row(
                  children: [
                    DsIcon(
                      DsIcons.actionVoice,
                      size: DsIconSizeRole.sm,
                      color: recording ? colors.dangerFg : colors.textSecondary,
                      decorative: true,
                    ),
                    const SizedBox(width: DsSpace.s6),
                    Expanded(
                      child: Semantics(
                        liveRegion: !recording,
                        child: Text(
                          recording
                              ? l.voiceRecording(time)
                              : l.voiceRecorded(time),
                          style: dsTextStyle(DsType.bodySm)
                              .copyWith(color: colors.textPrimary),
                        ),
                      ),
                    ),
                    button(
                      'cancel',
                      l.voiceCancel,
                      DsButtonStyle.ghost,
                      () => unawaited(controller.cancel()),
                    ),
                    if (recording)
                      button(
                        'stop',
                        l.voiceStop,
                        DsButtonStyle.secondary,
                        () => unawaited(controller.stop()),
                      ),
                    button(
                      'send',
                      l.voiceSend,
                      DsButtonStyle.primary,
                      () => unawaited(controller.send()),
                    ),
                  ],
                ),
              ],
            ),
          );
      }
    },
  );
}
