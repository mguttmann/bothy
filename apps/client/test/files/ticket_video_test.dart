// SPDX-License-Identifier: Apache-2.0

// The video dialog (12 9.2; WCAG 1.4.2; AUD-PKG-Q03, #701 and #809): the video does not start
// by itself, labelled controls play, pause, move and mute it, and a failure after the start
// fetches a new ticket and goes on at the position; a failure without progress since the last
// renewal shows the error text.

import 'package:client/files/ticket_player.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:video_player/video_player.dart';

import '../support/widgets.dart';

/// A controller without a platform player: it records what the view asks for.
class _FakeVideo extends ValueNotifier<VideoPlayerValue>
    implements VideoPlayerController {
  _FakeVideo(this.url) : super(const VideoPlayerValue(duration: Duration.zero));

  final Uri url;
  final List<Duration> seeks = [];
  bool disposed = false;

  @override
  int get playerId => VideoPlayerController.kUninitializedPlayerId;

  @override
  Future<void> initialize() async {
    value = value.copyWith(
      isInitialized: true,
      duration: const Duration(seconds: 90),
      size: const Size(640, 360),
    );
  }

  @override
  Future<void> play() async => value = value.copyWith(isPlaying: true);

  @override
  Future<void> pause() async => value = value.copyWith(isPlaying: false);

  @override
  Future<void> seekTo(Duration position) async {
    seeks.add(position);
    value = value.copyWith(position: position);
  }

  @override
  Future<void> setVolume(double volume) async =>
      value = value.copyWith(volume: volume);

  /// Plays on to [position].
  void advance(Duration position) => value = value.copyWith(position: position);

  /// The player fails, as on a range request with an expired ticket.
  void fail() => value = value.copyWith(errorDescription: 'ticket expired');

  @override
  Future<void> dispose() async {
    disposed = true;
    super.dispose();
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  late List<_FakeVideo> opened;
  late int renewals;

  Future<void> pumpView(WidgetTester tester, {bool canRenew = true}) async {
    opened = [];
    renewals = 0;
    await tester.pumpWidget(
      hostScreen(
        Scaffold(
          body: TicketVideoView(
            url: Uri.parse('https://chat.example.com/content?ticket=a'),
            renew: canRenew
                ? () async {
                    renewals++;
                    return Uri.parse(
                      'https://chat.example.com/content?ticket=r$renewals',
                    );
                  }
                : null,
            open: (url) => _FakeVideo(url).also(opened.add),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  DsButton buttonOf(WidgetTester tester, String key) =>
      tester.widget<DsButton>(find.byKey(ValueKey('file.video.$key')));

  testWidgets('the video waits for Play and has labelled controls', (
    tester,
  ) async {
    await pumpView(tester);
    final video = opened.single;
    expect(video.value.isPlaying, isFalse);
    expect(buttonOf(tester, 'play').label, en.fileVideoPlay);
    expect(buttonOf(tester, 'mute').label, en.fileVideoMute);
    expect(find.text(en.fileVideoTime('0:00', '1:30')), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('file.video.play')));
    await tester.pump();
    expect(video.value.isPlaying, isTrue);
    expect(buttonOf(tester, 'play').label, en.fileMediaPause);

    await tester.tap(find.byKey(const ValueKey('file.video.mute')));
    await tester.pump();
    expect(video.value.volume, 0);
    expect(buttonOf(tester, 'mute').label, en.fileVideoUnmute);

    final slider = tester.widget<Slider>(
      find.byKey(const ValueKey('file.video.position')),
    );
    expect(slider.label, en.fileVideoPosition);
    slider.onChanged!(30000);
    await tester.pump();
    expect(video.seeks.last, const Duration(seconds: 30));
    expect(find.text(en.fileVideoTime('0:30', '1:30')), findsOneWidget);
  });

  testWidgets('a failure after the start renews the ticket and goes on at the '
      'position', (tester) async {
    await pumpView(tester);
    final first = opened.single;
    await tester.tap(find.byKey(const ValueKey('file.video.play')));
    await tester.pump();
    first.advance(const Duration(seconds: 42));
    await tester.pump();
    first.fail();
    await tester.pumpAndSettle();

    expect(renewals, 1);
    expect(opened, hasLength(2));
    final second = opened.last;
    expect(second.url.queryParameters['ticket'], 'r1');
    expect(second.seeks, [const Duration(seconds: 42)]);
    expect(second.value.isPlaying, isTrue);
    expect(first.disposed, isTrue);
    expect(find.byKey(const ValueKey('file.video.failed')), findsNothing);

    // A failure without progress since the renewal is final.
    second.fail();
    await tester.pumpAndSettle();
    expect(renewals, 1);
    expect(find.text(en.fileMediaFailed), findsOneWidget);
  });

  testWidgets('without a way to renew, a failure shows the error text', (
    tester,
  ) async {
    await pumpView(tester, canRenew: false);
    opened.single
      ..advance(const Duration(seconds: 5))
      ..fail();
    await tester.pumpAndSettle();
    expect(find.text(en.fileMediaFailed), findsOneWidget);
  });
}

extension<T> on T {
  T also(void Function(T) f) {
    f(this);
    return this;
  }
}
