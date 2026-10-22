// SPDX-License-Identifier: Apache-2.0

/// The player of a voice message (15 13; 12 8, 12 9.2; brief M4-22 Design note 4): play and
/// pause through a file ticket, the speeds 1, 1.5 and 2, the position over the duration, and
/// the transcript below.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../files/providers.dart';
import '../files/ticket_player.dart';
import 'transcript_view.dart';

/// The speeds of a voice message, in the order the speed button steps through them (15 13).
const List<double> voiceSpeeds = [1, 1.5, 2];

/// The player of the voice file [file] with its transcript.
class VoicePlayer extends ConsumerStatefulWidget {
  /// Creates the player.
  const VoicePlayer({super.key, required this.file});

  /// The voice file (`audio/mp4`).
  final FileMeta file;

  @override
  ConsumerState<VoicePlayer> createState() => _VoicePlayerState();
}

class _VoicePlayerState extends ConsumerState<VoicePlayer> {
  TicketPlayer? _player;
  final List<StreamSubscription<Object>> _subs = [];
  PlayerPhase _phase = PlayerPhase.idle;
  Duration _position = Duration.zero;
  int _speed = 0;

  @override
  void dispose() {
    for (final s in _subs) {
      unawaited(s.cancel());
    }
    unawaited(_player?.dispose());
    super.dispose();
  }

  TicketPlayer _ensurePlayer() {
    final existing = _player;
    if (existing != null) return existing;
    final player = ref.read(filePortsProvider).player();
    _subs
      ..add(
        player.phases.listen((p) {
          if (!mounted) return;
          setState(() {
            _phase = p;
            if (p == PlayerPhase.completed) _position = Duration.zero;
          });
        }),
      )
      ..add(
        player.position.listen((p) {
          if (mounted) setState(() => _position = p);
        }),
      );
    return _player = player;
  }

  Future<void> _toggle() async {
    final player = _ensurePlayer();
    switch (_phase) {
      case PlayerPhase.playing:
        await player.pause();
      case PlayerPhase.paused:
        await player.resume();
      case PlayerPhase.loading:
        return;
      case PlayerPhase.idle || PlayerPhase.completed || PlayerPhase.failed:
        final tickets = ref.read(fileTicketsProvider);
        if (tickets == null) return;
        setState(() => _phase = PlayerPhase.loading);
        try {
          // 12 9.2: a new ticket per playback, handed to the player and kept nowhere else.
          final url = await tickets.url(widget.file.id);
          await player.play(url);
        } on Object {
          if (mounted) setState(() => _phase = PlayerPhase.failed);
        }
    }
  }

  Future<void> _nextSpeed() async {
    setState(() => _speed = (_speed + 1) % voiceSpeeds.length);
    // The player keeps the speed for the next playback too.
    await _ensurePlayer().setSpeed(voiceSpeeds[_speed]);
  }

  String _speedText(BuildContext context) {
    final speed = voiceSpeeds[_speed];
    final text = speed == speed.roundToDouble()
        ? speed.toStringAsFixed(0)
        : speed.toString();
    return Localizations.localeOf(context).languageCode == 'de'
        ? text.replaceAll('.', ',')
        : text;
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final format = L10nFormat(Localizations.localeOf(context));
    final total = Duration(milliseconds: widget.file.durationMs ?? 0);
    final playing = _phase == PlayerPhase.playing;
    final progress = total.inMilliseconds == 0
        ? 0.0
        : (_position.inMilliseconds / total.inMilliseconds)
              .clamp(0, 1)
              .toDouble();
    final speed = _speedText(context);
    return Padding(
      key: ValueKey('voice.${widget.file.id.value}'),
      padding: const EdgeInsets.only(top: DsSpace.s4),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              DsTappable(
                key: const ValueKey('voice.play'),
                onTap: () => unawaited(_toggle()),
                label: playing ? l.messageVoicePause : l.messageVoicePlay,
                radius: DsRadius.pill,
                excludeChildSemantics: true,
                child: Center(
                  widthFactor: 1,
                  heightFactor: 1,
                  child: DsIcon(
                    playing ? DsIcons.actionStop : DsIcons.actionVoice,
                    color: context.dsAccent.accentFg,
                    decorative: true,
                  ),
                ),
              ),
              const SizedBox(width: DsSpace.s8),
              Expanded(
                child: LinearProgressIndicator(
                  value: progress,
                  color: context.dsAccent.accentFg,
                  backgroundColor: colors.surfaceSunken,
                ),
              ),
              const SizedBox(width: DsSpace.s8),
              Text(
                format.elapsed(playing ? _position : total),
                style: dsTextStyle(DsType.codeSm)
                    .copyWith(color: colors.textSecondary),
              ),
              const SizedBox(width: DsSpace.s4),
              DsTappable(
                key: const ValueKey('voice.speed'),
                onTap: () => unawaited(_nextSpeed()),
                label: l.voiceSpeed(l.voiceSpeedValue(speed)),
                excludeChildSemantics: true,
                child: Padding(
                  padding: const EdgeInsets.symmetric(horizontal: DsSpace.s6),
                  child: Center(
                    widthFactor: 1,
                    heightFactor: 1,
                    child: Text(
                      l.voiceSpeedValue(speed),
                      style: dsTextStyle(DsType.codeSm)
                          .copyWith(color: colors.textPrimary),
                    ),
                  ),
                ),
              ),
            ],
          ),
          if (_phase == PlayerPhase.failed)
            Text(
              l.fileMediaFailed,
              style: dsTextStyle(DsType.caption)
                  .copyWith(color: colors.textSecondary),
            ),
          TranscriptView(file: widget.file),
        ],
      ),
    );
  }
}
