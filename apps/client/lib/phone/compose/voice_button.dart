// SPDX-License-Identifier: Apache-2.0

/// The microphone of the phone's input and the panel of a recording (01 5.3 M5, 8; brief M5-07
/// Design notes 2 and 3).
///
/// [PhoneVoiceButton] is the microphone: in the gesture mode the finger holds it to record,
/// swipes left to discard, swipes up to lock and releases to send; in the button mode (an
/// accessibility service is on, `MediaQuery.accessibleNavigation`) a tap records and a second
/// tap stops, because a press-and-hold gesture is not usable with every assistive technology.
/// [PhoneVoicePanel] above the input shows the running recording with its remaining time, the
/// notice before the limit, the locked recording with Stop, Send and Discard, the finished
/// recording with play, Send and Discard, and the notices of a denied microphone or a failed
/// recording.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../../files/ticket_player.dart';
import 'voice_recorder.dart';

/// How far, in tap targets (`density.target-min`), the finger moves from where it pressed
/// before a swipe left discards or a swipe up locks: far enough that a trembling finger never
/// does either by accident.
const double _swipeTargets = 2;

/// Whether the microphone shows the button mode (01 8): an accessibility service such as
/// VoiceOver, TalkBack or Switch Control is on.
bool voiceButtonMode(BuildContext context) =>
    MediaQuery.maybeAccessibleNavigationOf(context) ?? false;

/// The microphone of the phone's input for [controller].
class PhoneVoiceButton extends StatefulWidget {
  /// Creates the microphone; [buttonMode] null follows [voiceButtonMode].
  const PhoneVoiceButton({
    super.key,
    required this.controller,
    this.buttonMode,
  });

  /// The recording.
  final PhoneVoiceRecorder controller;

  /// Whether the button mode shows instead of the gestures; null follows the system.
  final bool? buttonMode;

  @override
  State<PhoneVoiceButton> createState() => _PhoneVoiceButtonState();
}

class _PhoneVoiceButtonState extends State<PhoneVoiceButton> {
  int? _pointer;
  Offset _origin = Offset.zero;

  PhoneVoiceRecorder get _voice => widget.controller;

  void _down(PointerDownEvent event) {
    if (_pointer != null) return;
    if (_voice.phase == PhoneVoicePhase.locked) {
      // The microphone shows Stop while a locked recording runs.
      unawaited(_voice.stop());
      return;
    }
    _pointer = event.pointer;
    _origin = event.position;
    unawaited(_voice.press());
  }

  void _move(PointerMoveEvent event, double swipe) {
    if (event.pointer != _pointer) return;
    if (_voice.phase != PhoneVoicePhase.recording) return;
    final delta = event.position - _origin;
    if (delta.dx <= -swipe) {
      _pointer = null;
      unawaited(_voice.discard());
    } else if (delta.dy <= -swipe) {
      _pointer = null;
      _voice.lock();
    }
  }

  void _up(PointerUpEvent event) {
    if (event.pointer != _pointer) return;
    _pointer = null;
    unawaited(_voice.release());
  }

  void _cancel(PointerCancelEvent event) {
    if (event.pointer != _pointer) return;
    _pointer = null;
    // The system took the touch (a call, a system gesture): keep the recording for the
    // person, never send it.
    unawaited(_voice.interrupt());
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final target = context.dsDensity.targetMin;
    final buttonMode = widget.buttonMode ?? voiceButtonMode(context);
    return ListenableBuilder(
      listenable: _voice,
      builder: (context, _) {
        final running = _voice.running;
        final icon = DsIcon(
          running ? DsIcons.actionStop : DsIcons.actionVoice,
          size: DsIconSizeRole.md,
          color: running ? colors.dangerFg : colors.textSecondary,
          decorative: true,
        );
        if (buttonMode) {
          return DsTappable(
            key: const ValueKey('voice.mic'),
            label: running ? l.voiceStop : l.composerVoice,
            excludeChildSemantics: true,
            onTap: () =>
                unawaited(running ? _voice.stop() : _voice.startLocked()),
            child: Center(widthFactor: 1, heightFactor: 1, child: icon),
          );
        }
        return Semantics(
          key: const ValueKey('voice.mic'),
          container: true,
          button: true,
          label: l.composerVoice,
          hint: l.voicePhoneHoldHint,
          // A screen reader's double tap, or Switch Control without the system's
          // accessibleNavigation, records in the locked form: no gesture needed.
          onTap: running ? null : () => unawaited(_voice.startLocked()),
          excludeSemantics: true,
          child: Listener(
            behavior: HitTestBehavior.opaque,
            onPointerDown: _down,
            onPointerMove: (e) => _move(e, target * _swipeTargets),
            onPointerUp: _up,
            onPointerCancel: _cancel,
            child: ConstrainedBox(
              constraints: BoxConstraints(minWidth: target, minHeight: target),
              child: Center(widthFactor: 1, heightFactor: 1, child: icon),
            ),
          ),
        );
      },
    );
  }
}

/// The panel of [controller] above the phone's input.
///
/// [platform] (`ios` or `android`) chooses the path to the system settings in the notice of a
/// denied microphone; [announceEvery] is `a11y.live_announce_min_interval`, the shortest time
/// between two announcements of the remaining time; [player] makes the player of the preview.
class PhoneVoicePanel extends StatefulWidget {
  /// Creates the panel.
  const PhoneVoicePanel({
    super.key,
    required this.controller,
    required this.platform,
    required this.announceEvery,
    required this.player,
  });

  /// The recording.
  final PhoneVoiceRecorder controller;

  /// The platform of the build.
  final String platform;

  /// `a11y.live_announce_min_interval`.
  final Duration announceEvery;

  /// Makes the player of the preview.
  final TicketPlayer Function() player;

  @override
  State<PhoneVoicePanel> createState() => _PhoneVoicePanelState();
}

class _PhoneVoicePanelState extends State<PhoneVoicePanel> {
  /// The remaining time the live region announced last, and when (in recording time).
  String? _announced;
  Duration? _announcedAt;

  TicketPlayer? _player;
  StreamSubscription<PlayerPhase>? _phases;
  bool _playing = false;

  PhoneVoiceRecorder get _voice => widget.controller;

  @override
  void dispose() {
    unawaited(_phases?.cancel());
    unawaited(_player?.dispose());
    super.dispose();
  }

  Future<void> _togglePlay() async {
    final file = _voice.recorded;
    if (file == null) return;
    final player = _player ??= widget.player();
    _phases ??= player.phases.listen((p) {
      if (mounted) setState(() => _playing = p == PlayerPhase.playing);
    });
    if (_playing) {
      await player.pause();
    } else {
      // The local recording: the preview before sending plays the temporary file (15 14).
      await player.play(Uri.file(file.path));
    }
  }

  Future<void> _stopPreview() async {
    final player = _player;
    _player = null;
    await _phases?.cancel();
    _phases = null;
    _playing = false;
    await player?.dispose();
  }

  /// The text of the live region: it changes at most once per [PhoneVoicePanel.announceEvery]
  /// of recording time, so a screen reader is not interrupted every second (01 8).
  String _live(String now) {
    final at = _announcedAt;
    final elapsed = _voice.elapsed;
    if (_announced == null ||
        at == null ||
        elapsed < at ||
        elapsed - at >= widget.announceEvery) {
      _announced = now;
      _announcedAt = elapsed;
    }
    return _announced!;
  }

  @override
  Widget build(BuildContext context) => ListenableBuilder(
    listenable: _voice,
    builder: (context, _) {
      final l = AppLocalizations.of(context);
      final format = L10nFormat(Localizations.localeOf(context));
      final phase = _voice.phase;
      if (phase != PhoneVoicePhase.ready && _player != null) {
        unawaited(_stopPreview());
      }
      if (!_voice.running) {
        _announced = null;
        _announcedAt = null;
      }
      Widget banner(
        String key,
        DsTone tone,
        String text, {
        VoidCallback? close,
      }) => Padding(
        padding: const EdgeInsets.only(bottom: DsSpace.s4),
        child: DsBanner(
          key: ValueKey('voice.$key'),
          tone: tone,
          text: text,
          liveRegion: true,
          actionLabel: close == null ? null : l.threadCancel,
          onAction: close,
        ),
      );
      Widget button(
        String key,
        String label,
        DsButtonStyle style,
        VoidCallback? onTap,
      ) => DsButton(
        key: ValueKey('voice.$key'),
        label: label,
        style: style,
        onPressed: onTap,
      );
      // The buttons of a recording, at the end of the line, wrapping when the text is large.
      Widget actions(List<Widget> buttons) => Padding(
        padding: const EdgeInsets.only(bottom: DsSpace.s4),
        child: Wrap(
          alignment: WrapAlignment.end,
          spacing: DsSpace.s8,
          runSpacing: DsSpace.s4,
          children: buttons,
        ),
      );
      final limit = _voice.limit;
      final remaining = _voice.remaining;
      switch (phase) {
        case PhoneVoicePhase.idle || PhoneVoicePhase.discarded:
          return const SizedBox.shrink();
        case PhoneVoicePhase.denied:
          return banner(
            'notice',
            DsTone.warning,
            widget.platform == 'android'
                ? l.voicePhoneDeniedAndroid
                : l.voicePhoneDeniedIos,
            close: () => unawaited(_voice.discard()),
          );
        case PhoneVoicePhase.failed:
          return banner(
            'notice',
            DsTone.warning,
            l.voiceFailed,
            close: () => unawaited(_voice.discard()),
          );
        case PhoneVoicePhase.recording ||
            PhoneVoicePhase.locked ||
            PhoneVoicePhase.stopping:
          final shown = remaining == null
              ? l.voicePhoneElapsed(format.elapsed(_voice.elapsed))
              : l.voicePhoneRemaining(format.elapsed(remaining));
          final locked = phase != PhoneVoicePhase.recording;
          return Column(
            key: const ValueKey('voice.panel'),
            crossAxisAlignment: CrossAxisAlignment.stretch,
            mainAxisSize: MainAxisSize.min,
            children: [
              if (_voice.noticeShown && remaining != null)
                // A fixed text, announced once when it appears: the remaining time beside it
                // follows the interval rule of the live region.
                banner(
                  'limitNotice',
                  DsTone.info,
                  l.voicePhoneLimitNotice(format.elapsed(_voice.noticeAt)),
                ),
              Padding(
                padding: const EdgeInsets.only(bottom: DsSpace.s4),
                child: _Time(text: shown, live: _live(shown), recording: true),
              ),
              if (locked)
                actions([
                  button(
                    'cancel',
                    l.voiceCancel,
                    DsButtonStyle.ghost,
                    () => unawaited(_voice.discard()),
                  ),
                  button(
                    'stop',
                    l.voiceStop,
                    DsButtonStyle.secondary,
                    phase == PhoneVoicePhase.locked
                        ? () => unawaited(_voice.stop())
                        : null,
                  ),
                  button(
                    'send',
                    l.voiceSend,
                    DsButtonStyle.primary,
                    phase == PhoneVoicePhase.locked
                        ? () => unawaited(_voice.send())
                        : null,
                  ),
                ]),
              if (!locked)
                Padding(
                  padding: const EdgeInsets.only(bottom: DsSpace.s4),
                  child: Text(
                    l.voicePhoneSwipeHint,
                    key: const ValueKey('voice.swipeHint'),
                    style: dsTextStyle(DsType.caption)
                        .copyWith(color: context.dsColors.textSecondary),
                  ),
                ),
            ],
          );
        case PhoneVoicePhase.ready:
          final duration = _voice.recorded?.duration ?? _voice.elapsed;
          final tooLong = _voice.overLimit;
          return Column(
            key: const ValueKey('voice.panel'),
            crossAxisAlignment: CrossAxisAlignment.stretch,
            mainAxisSize: MainAxisSize.min,
            children: [
              if (tooLong && limit != null)
                banner(
                  'tooLong',
                  DsTone.danger,
                  l.voicePhoneTooLong(format.elapsed(limit)),
                )
              else if (_voice.limitReached && limit != null)
                banner(
                  'limitReached',
                  DsTone.info,
                  l.voiceLimitReached(format.elapsed(limit)),
                ),
              Row(
                children: [
                  DsTappable(
                    key: const ValueKey('voice.preview'),
                    label: _playing ? l.voicePhonePause : l.voicePhonePlay,
                    excludeChildSemantics: true,
                    onTap: () => unawaited(_togglePlay()),
                    child: Center(
                      widthFactor: 1,
                      heightFactor: 1,
                      child: DsIcon(
                        _playing ? DsIcons.actionStop : DsIcons.actionVoice,
                        size: DsIconSizeRole.md,
                        color: context.dsColors.textPrimary,
                        decorative: true,
                      ),
                    ),
                  ),
                  const SizedBox(width: DsSpace.s6),
                  Expanded(
                    child: _Time(
                      text: l.voiceRecorded(format.elapsed(duration)),
                      live: null,
                      recording: false,
                    ),
                  ),
                ],
              ),
              actions([
                button(
                  'cancel',
                  l.voiceCancel,
                  DsButtonStyle.ghost,
                  () => unawaited(_voice.discard()),
                ),
                button(
                  'send',
                  l.voiceSend,
                  DsButtonStyle.primary,
                  tooLong ? null : () => unawaited(_voice.send()),
                ),
              ]),
            ],
          );
      }
    },
  );
}

/// The time of a recording: [text] shown, [live] (when set) the throttled text of the live
/// region that a screen reader announces.
class _Time extends StatelessWidget {
  const _Time({
    required this.text,
    required this.live,
    required this.recording,
  });

  final String text;
  final String? live;
  final bool recording;

  @override
  Widget build(BuildContext context) {
    final colors = context.dsColors;
    final row = Row(
      children: [
        if (recording) ...[
          // The red microphone marks a running recording.
          DsIcon(
            DsIcons.actionVoice,
            size: DsIconSizeRole.sm,
            color: colors.dangerFg,
            decorative: true,
          ),
          const SizedBox(width: DsSpace.s6),
        ],
        Flexible(
          child: Text(
            text,
            key: const ValueKey('voice.time'),
            style: dsTextStyle(DsType.bodySm)
                .copyWith(color: colors.textPrimary),
          ),
        ),
      ],
    );
    final announced = live;
    if (announced == null) return row;
    return Semantics(
      key: const ValueKey('voice.live'),
      container: true,
      liveRegion: true,
      label: announced,
      child: ExcludeSemantics(child: row),
    );
  }
}
