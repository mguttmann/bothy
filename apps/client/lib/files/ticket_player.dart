// SPDX-License-Identifier: Apache-2.0

/// Audio and video through file tickets (12 9.2; brief M4-22 Design note 3): a media player
/// sends its own requests, often with `Range`, and cannot sign a DPoP proof, so the app asks
/// `createFileTicket` for a short-lived URL and hands it to the player. A ticket URL is a
/// bearer secret: it is never logged, never stored and never shown.
library;

import 'dart:async';

import 'package:api_client/api_client.dart' as api;
import 'package:audioplayers/audioplayers.dart' as ap;
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';
import 'package:video_player/video_player.dart';

/// What a [TicketPlayer] does now.
enum PlayerPhase {
  /// Nothing loaded, or stopped.
  idle,

  /// Asking for the ticket or buffering.
  loading,

  /// Playing.
  playing,

  /// Paused.
  paused,

  /// Played to the end.
  completed,

  /// The ticket or the playback failed.
  failed,
}

/// Plays one audio source from a URL.
abstract interface class TicketPlayer {
  /// Plays [url] from the start at the current speed.
  Future<void> play(Uri url);

  /// Pauses.
  Future<void> pause();

  /// Continues after [pause].
  Future<void> resume();

  /// Sets the speed (1, 1.5 or 2 for voice messages, 15 13).
  Future<void> setSpeed(double speed);

  /// The position while playing.
  Stream<Duration> get position;

  /// The phases of the playback.
  Stream<PlayerPhase> get phases;

  /// Stops and releases the player.
  Future<void> dispose();
}

/// [TicketPlayer] over package `audioplayers` (MIT): AVPlayer on macOS, Media Foundation on
/// Windows.
final class AudioTicketPlayer implements TicketPlayer {
  /// Creates the player; the package logs nothing ([silenceLogs]).
  AudioTicketPlayer() : _player = _quiet();

  static ap.AudioPlayer _quiet() {
    silenceLogs();
    return ap.AudioPlayer();
  }

  /// Turns off the logger of `audioplayers`, which prints the source URL of a failed playback:
  /// a ticket URL must never reach a log (12 9.2 item 4).
  static void silenceLogs() => ap.AudioLogger.logLevel = ap.AudioLogLevel.none;

  final ap.AudioPlayer _player;
  double _speed = 1;

  @override
  Future<void> play(Uri url) async {
    await _player.play(ap.UrlSource(url.toString()));
    await _player.setPlaybackRate(_speed);
  }

  @override
  Future<void> pause() => _player.pause();

  @override
  Future<void> resume() => _player.resume();

  @override
  Future<void> setSpeed(double speed) async {
    _speed = speed;
    await _player.setPlaybackRate(speed);
  }

  @override
  Stream<Duration> get position => _player.onPositionChanged;

  @override
  Stream<PlayerPhase> get phases => _player.onPlayerStateChanged.map(
    (s) => switch (s) {
      ap.PlayerState.playing => PlayerPhase.playing,
      ap.PlayerState.paused => PlayerPhase.paused,
      ap.PlayerState.completed => PlayerPhase.completed,
      _ => PlayerPhase.idle,
    },
  );

  @override
  Future<void> dispose() => _player.dispose();
}

/// Mints file tickets (`createFileTicket`, 12 9.2) and turns them into absolute URLs.
///
/// Example: `player.play(await tickets.url(file.id));`
final class FileTickets {
  /// Creates the minter on [client] for the server at [baseUrl].
  FileTickets(this._client, this.baseUrl);

  final api.ApiClient _client;

  /// The origin of the server; the ticket's `url` is a path below it.
  final Uri baseUrl;

  /// A new ticket URL of [variant] (`original` or a thumbnail) of [fileId].
  ///
  /// Throws the `DioException` of the request (`409 file_not_ready`, `file_blocked`, `404`).
  Future<Uri> url(FileId fileId, {String variant = 'original'}) async {
    final response = await _client.getFilesApi().createFileTicket(
      fileId: fileId.value,
      fileTicketCreate: api.FileTicketCreate(
        (b) => b..variant = api.FileTicketCreateVariantEnum.valueOf(variant),
      ),
    );
    return baseUrl.resolve(response.data!.url);
  }
}

/// A video from a ticket URL through package `video_player` (BSD-3-Clause); the controller
/// holds the URL in memory only.
///
/// The video does not start by itself (WCAG 1.4.2): labelled controls play and pause it, move
/// the position (a slider the arrow keys move as well) and mute it, and show the position.
/// A ticket lives `file.ticket_ttl` and the server checks it on every range request (12 9.2):
/// when the player fails after the start, [renew] fetches a new ticket and the video goes on
/// at its position; only a failure without progress since the last renewal ends in the error
/// text.
class TicketVideoView extends StatefulWidget {
  /// Creates the view of [url]; [renew] mints a new ticket URL of the same file.
  const TicketVideoView({
    super.key,
    required this.url,
    this.renew,
    @visibleForTesting this.open,
  });

  /// The ticket URL.
  final Uri url;

  /// A new ticket URL of the same file, or null (then a failure after the start is final).
  final Future<Uri> Function()? renew;

  /// Creates the controller of a URL; the network player by default.
  final VideoPlayerController Function(Uri url)? open;

  @override
  State<TicketVideoView> createState() => _TicketVideoViewState();
}

class _TicketVideoViewState extends State<TicketVideoView> {
  VideoPlayerController? _controller;
  bool _ready = false;
  bool _failed = false;
  bool _renewing = false;

  /// The position and the playing state last seen without an error.
  Duration _position = Duration.zero;
  bool _wasPlaying = false;

  /// The position of the last renewal; a failure before the video got past it is final.
  Duration? _renewedAt;

  @override
  void initState() {
    super.initState();
    unawaited(_start(widget.url));
  }

  /// Opens [url] and, once ready, moves to [at] and plays when [play].
  Future<void> _start(
    Uri url, {
    Duration at = Duration.zero,
    bool play = false,
  }) async {
    final controller = (widget.open ?? VideoPlayerController.networkUrl)(url);
    final old = _controller;
    _controller = controller;
    old?.removeListener(_changed);
    if (old != null) unawaited(old.dispose());
    try {
      await controller.initialize();
      if (!mounted || controller != _controller) return;
      if (at > Duration.zero) await controller.seekTo(at);
      if (play) await controller.play();
    } on Object {
      if (mounted && controller == _controller) {
        setState(() => _failed = true);
      }
      return;
    }
    if (!mounted || controller != _controller) return;
    controller.addListener(_changed);
    setState(() {
      _ready = true;
      _failed = false;
    });
  }

  void _changed() {
    final controller = _controller;
    if (controller == null || !mounted) return;
    final value = controller.value;
    if (value.hasError) {
      if (!_renewing) unawaited(_recover());
      return;
    }
    _position = value.position;
    _wasPlaying = value.isPlaying;
    setState(() {});
  }

  /// A failure after the start: a new ticket, at the position and in the state of before.
  Future<void> _recover() async {
    final renew = widget.renew;
    final last = _renewedAt;
    if (renew == null || (last != null && _position <= last)) {
      setState(() => _failed = true);
      return;
    }
    _renewing = true;
    _renewedAt = _position;
    try {
      final url = await renew();
      if (!mounted) return;
      await _start(url, at: _position, play: _wasPlaying);
    } on Object {
      if (mounted) setState(() => _failed = true);
    } finally {
      _renewing = false;
    }
  }

  @override
  void dispose() {
    final controller = _controller;
    controller?.removeListener(_changed);
    if (controller != null) unawaited(controller.dispose());
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final controller = _controller;
    if (_failed) {
      return Center(
        child: Text(
          l.fileMediaFailed,
          key: const ValueKey('file.video.failed'),
        ),
      );
    }
    if (!_ready || controller == null) {
      return const Center(child: CircularProgressIndicator());
    }
    final value = controller.value;
    final duration = value.duration;
    final position = value.position > duration ? duration : value.position;
    final muted = value.volume == 0;
    final format = L10nFormat(Localizations.localeOf(context));
    final time = l.fileVideoTime(
      format.elapsed(position),
      format.elapsed(duration),
    );
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Flexible(
          child: AspectRatio(
            aspectRatio: value.aspectRatio,
            child: VideoPlayer(controller),
          ),
        ),
        const SizedBox(height: DsSpace.s8),
        Slider(
          key: const ValueKey('file.video.position'),
          value: position.inMilliseconds.toDouble(),
          max: duration.inMilliseconds <= 0
              ? 1
              : duration.inMilliseconds.toDouble(),
          semanticFormatterCallback: (_) => time,
          label: l.fileVideoPosition,
          onChanged: duration.inMilliseconds <= 0
              ? null
              : (ms) => unawaited(
                  controller.seekTo(Duration(milliseconds: ms.round())),
                ),
        ),
        Row(
          children: [
            DsButton(
              key: const ValueKey('file.video.play'),
              label: value.isPlaying ? l.fileMediaPause : l.fileVideoPlay,
              style: DsButtonStyle.ghost,
              onPressed: () => unawaited(
                value.isPlaying ? controller.pause() : controller.play(),
              ),
            ),
            DsButton(
              key: const ValueKey('file.video.mute'),
              label: muted ? l.fileVideoUnmute : l.fileVideoMute,
              style: DsButtonStyle.ghost,
              onPressed: () => unawaited(controller.setVolume(muted ? 1 : 0)),
            ),
            const SizedBox(width: DsSpace.s8),
            Expanded(
              child: Text(
                time,
                key: const ValueKey('file.video.time'),
                textAlign: TextAlign.end,
                style: dsTextStyle(DsType.caption),
              ),
            ),
          ],
        ),
      ],
    );
  }
}

/// Opens the video [name] of [ticket] in a dialog built by [surface]; [renew] mints a new
/// ticket when the first one expired during the playback.
Future<void> showVideoDialog(
  BuildContext context, {
  required String name,
  required Uri ticket,
  required Future<Uri> Function() renew,
  required Widget Function(Uri, Future<Uri> Function()) surface,
}) => showDialog<void>(
  context: context,
  builder: (context) {
    final l = AppLocalizations.of(context);
    return Dialog(
      key: const ValueKey('file.video'),
      child: Padding(
        padding: const EdgeInsets.all(DsSpace.s16),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              l.fileVideoTitle(dsIsolate(name)),
              style: dsTextStyle(DsType.heading),
            ),
            const SizedBox(height: DsSpace.s12),
            ConstrainedBox(
              constraints: const BoxConstraints(maxHeight: 480),
              child: surface(ticket, renew),
            ),
            const SizedBox(height: DsSpace.s12),
            Align(
              alignment: AlignmentDirectional.centerEnd,
              child: DsButton(
                key: const ValueKey('file.video.close'),
                label: l.fileImageClose,
                style: DsButtonStyle.ghost,
                onPressed: () => Navigator.of(context).pop(),
              ),
            ),
          ],
        ),
      ),
    );
  },
);
