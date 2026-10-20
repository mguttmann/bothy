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
  /// Creates the player.
  AudioTicketPlayer() : _player = ap.AudioPlayer();

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
class TicketVideoView extends StatefulWidget {
  /// Creates the view of [url].
  const TicketVideoView({super.key, required this.url});

  /// The ticket URL.
  final Uri url;

  @override
  State<TicketVideoView> createState() => _TicketVideoViewState();
}

class _TicketVideoViewState extends State<TicketVideoView> {
  late final VideoPlayerController _controller =
      VideoPlayerController.networkUrl(widget.url);
  bool _ready = false;
  bool _failed = false;

  @override
  void initState() {
    super.initState();
    unawaited(
      _controller
          .initialize()
          .then((_) async {
            if (!mounted) return;
            setState(() => _ready = true);
            await _controller.play();
          })
          .catchError((Object _) {
            if (mounted) setState(() => _failed = true);
          }),
    );
  }

  @override
  void dispose() {
    unawaited(_controller.dispose());
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    if (_failed) {
      return Center(child: Text(l.fileMediaFailed));
    }
    if (!_ready) {
      return const Center(child: CircularProgressIndicator());
    }
    return AspectRatio(
      aspectRatio: _controller.value.aspectRatio,
      child: VideoPlayer(_controller),
    );
  }
}

/// Opens the video [name] of [ticket] in a dialog built by [surface].
Future<void> showVideoDialog(
  BuildContext context, {
  required String name,
  required Uri ticket,
  required Widget Function(Uri) surface,
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
              child: surface(ticket),
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
