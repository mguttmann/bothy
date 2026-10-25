// SPDX-License-Identifier: Apache-2.0

// The performance level of the desktop app (brief M4-34 Design note 3; docs/spec/
// 15-client-architecture.md 19 and 20 row "Performance"): a profile build against the real server
// of the Compose stack measures the targets `client.perf.*` of the desktop and reports the
// summary of perf_summary.dart to the driver test_driver/perf_driver.dart, which writes
// build/perf/summary.json and fails when a target is missed. `make perf-desktop` runs it through
// tool/e2e_suite.sh perf (flutter drive --profile); it needs the defines of e2e_suite.sh.
//
// What is measured (each from the app's own frames, with the frames of the engine running live):
// - cold start: a restart of the app on a filled data folder until the sidebar shows the space
//   from the local database;
// - thread open: a thread opened before, from its route until its first message is on screen;
// - local echo: a send from the composer until the row of the message is on screen;
// - patch to frame: a status line patch of stream-agent from its arrival in a `run.updated`
//   frame until the end of the first frame that shows it;
// - jank ratio: frames over one refresh interval while scrolling a thread of thousands of
//   messages;
// - catch-up rate: events applied per second after a reconnect with a backlog;
// - memory: the resident size with twenty threads in focus.

import 'dart:developer' show Timeline;
import 'dart:io';

import 'package:client/platform/frame_ticker.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart' show AppConfig, ThreadId;
import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:integration_test/integration_test.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart' show ConnectionPhase;

import '../e2e_test/flows/messaging_flows.dart' show composerText;
import '../e2e_test/support/e2e_app.dart';
import '../e2e_test/support/e2e_context.dart';
import '../e2e_test/support/e2e_keys.dart';
import '../e2e_test/support/e2e_world.dart';
import 'perf_summary.dart';

/// The device the numbers come from and whether it is a reference device of 15 19
/// (tool/e2e_suite.sh, brief M4-34 Design note 4).
const String deviceLabel = String.fromEnvironment(
  'PERF_DEVICE_LABEL',
  defaultValue: 'unnamed device',
);

/// Whether [deviceLabel] is a reference device.
const bool referenceDevice =
    String.fromEnvironment('PERF_REFERENCE', defaultValue: '0') == '1';

/// The threads in focus of the memory target (15 19: "twenty threads in focus").
const int focusedThreads = 20;

/// The backlog of the catch-up target and the length of the scrolled thread: "several
/// thousand messages" (Design note 3).
const int backlog = 2000;

/// Messages Sam sends at once while the backlog fills.
const int parallelPosts = 8;

void main() {
  final binding = IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  // The engine draws frames on its own schedule, so frame timings and waits are real.
  binding.framePolicy = LiveTestWidgetsFlutterBindingFramePolicy.fullyLive;
  final c = E2eContext(E2eWorld.fromEnvironment())..register();
  final world = c.world;
  final samples = <String, List<double>>{};
  void add(String name, double value) =>
      (samples[name] ??= <double>[]).add(value);

  testWidgets('desktop performance targets of 15 19', (tester) async {
    final app = await E2eApp.start(
      tester,
      world,
      ticker: SchedulerFrameTicker(),
    );
    final nonce = randomNonce().substring(0, 8);
    final clock = _FrameClock(app);
    void go(String path) => GoRouter.of(
      tester.element(find.byKey(const ValueKey('column.sidebar'))),
    ).go(path);

    try {
      await app.signIn(await c.alexCode());

      // Twenty posts with a few messages each, opened once from the network.
      final posts = <String>[];
      for (var i = 0; i < focusedThreads; i++) {
        final post = (await tester.runAsync(
          () => c.owner.createPost(world.forumId, 'Perf $nonce $i', 'Start $i'),
        ))!;
        for (var m = 0; m < 3; m++) {
          await tester.runAsync(() => c.sam.post(post, 'Message $m of $i'));
        }
        posts.add(post);
      }
      for (final post in posts) {
        go('/t/$post');
        await app.waitFor(find.byKey(ValueKey('history.$post')));
        await app.waitFor(_message);
      }

      // Memory with twenty threads in focus (the gateway focus of 04 6.2 holds them all).
      app.state.services!.engine.focus.setVisible({
        for (final p in posts) ThreadId.parse(p),
      }, idle: false);
      await tester.pump(const Duration(milliseconds: 500));
      add('client.perf.memory_desktop_max', ProcessInfo.currentRss.toDouble());

      // Thread open from the cache, twenty times.
      for (final post in posts) {
        go('/c/${world.generalId}');
        await app.until(
          'general',
          () => find.byKey(ValueKey('history.$post')).evaluate().isEmpty,
        );
        final opened = await clock.measure(
          'the messages of $post',
          () => find
              .descendant(
                of: find.byKey(ValueKey('history.$post')),
                matching: _message,
              )
              .evaluate()
              .isNotEmpty,
          () async => go('/t/$post'),
        );
        add('client.perf.thread_open_p95', opened);
      }

      // Local echo, twenty sends into the first post.
      // The composer of the thread before stays on screen until the route has switched, so the
      // wait is for the history of this post with its messages, then the composer.
      go('/t/${posts.first}');
      await app.waitFor(
        find.descendant(
          of: find.byKey(ValueKey('history.${posts.first}')),
          matching: _message,
        ),
      );
      await app.waitFor(find.byKey(const ValueKey('composer')));
      for (var i = 0; i < 20; i++) {
        final text = 'Echo $nonce $i';
        await _type(tester, text);
        // The echo is the row of the text, waiting or already confirmed: on a fast stack the
        // confirmation can land before the frame that would show the waiting row. The send
        // button: a simulated Enter does not reach the composer in a profile build.
        final echo = await clock.measure(
          'the echo of message $i',
          () => find
              .descendant(
                of: find.byKey(ValueKey('history.${posts.first}')),
                matching: find.textContaining(text, findRichText: true),
              )
              .evaluate()
              .isNotEmpty,
          () => tester.tap(_sendButton),
          detail: () => 'composer "${_composerValue(tester)}"',
        );
        add('client.perf.send_local_echo_p95', echo);
        await app.until(
          'the confirmation of $i',
          () => _waiting.evaluate().isEmpty,
        );
      }

      // Patch to frame: stream-agent streams its status line into the open post.
      final shown = <int, int>{};
      var watching = true;
      SchedulerBinding.instance.addPersistentFrameCallback((_) {
        if (!watching) return;
        for (final e in _streamLine.evaluate()) {
          final m = RegExp(r'Streaming (\d+) of')
              .firstMatch(_plain(e.widget as Text));
          if (m != null) {
            shown.putIfAbsent(int.parse(m.group(1)!), () => Timeline.now);
          }
        }
      });
      await _type(tester, '@${world.streamAgent.handle} stream please');
      await tester.tap(_sendButton);
      final arrivals = app.device.transport.streamArrivals;
      await app.until(
        'the stream of stream-agent',
        () => shown.length >= 25 && arrivals.length >= 25,
        detail: () => 'shown ${shown.length}, arrived ${arrivals.length}',
      );
      watching = false;
      for (final MapEntry(:key, :value) in arrivals.entries) {
        final drawn = shown[key];
        if (drawn != null) {
          add('client.perf.patch_to_frame_p95', (drawn - value) / 1000);
        }
      }

      // Catch-up: a backlog of Sam's messages while the app is offline, then the reconnect.
      final big = posts[1];
      go('/t/$big');
      await app.waitFor(find.byKey(ValueKey('history.$big')));
      c.network.cut();
      await app.until(
        'offline',
        () => app.state.services!.engine.state.phase != ConnectionPhase.live,
      );
      await tester.runAsync(() async {
        for (var i = 0; i < backlog; i += parallelPosts) {
          await Future.wait([
            for (var j = i; j < i + parallelPosts && j < backlog; j++)
              c.sam.post(big, 'Backlog $nonce $j'),
          ]);
        }
      });
      // The phases come from the engine's state stream, so a catch-up shorter than one poll of
      // the wait is still seen; the events count from the start of the catch-up.
      final catchup = Stopwatch()..start();
      Duration? started;
      Duration? ended;
      var before = 0;
      var after = 0;
      final phases = app.state.services!.engine.states.listen((s) {
        if (started == null && s.phase == ConnectionPhase.catchingUp) {
          started = catchup.elapsed;
          before = app.device.transport.dispatched;
        } else if (started != null &&
            ended == null &&
            s.phase == ConnectionPhase.live) {
          ended = catchup.elapsed;
          after = app.device.transport.dispatched;
        }
      });
      c.network.restore();
      await app.until('the catch-up and live', () => ended != null);
      await phases.cancel();
      final events = after - before;
      final seconds = (ended! - started!).inMicroseconds / 1e6;
      add('client.perf.catchup_events_per_s_min', events / seconds);

      // Jank while scrolling the long thread up and down.
      final frames = <Duration>[];
      void onTimings(List<FrameTiming> timings) =>
          frames.addAll([for (final t in timings) t.totalSpan]);
      await app.waitFor(_message);
      final list = find.descendant(
        of: find.byKey(ValueKey('history.$big')),
        matching: find.byType(Scrollable),
      );
      SchedulerBinding.instance.addTimingsCallback(onTimings);
      for (var i = 0; i < 20; i++) {
        await tester.timedDrag(
          list.first,
          Offset(0, i.isEven ? 600 : -300),
          const Duration(milliseconds: 300),
        );
        await tester.pump(const Duration(milliseconds: 100));
      }
      SchedulerBinding.instance.removeTimingsCallback(onTimings);
      final displays = binding.platformDispatcher.displays;
      final rate = displays.isEmpty ? 60.0 : displays.first.refreshRate;
      final budget = Duration(
        microseconds: (Duration.microsecondsPerSecond / (rate > 0 ? rate : 60))
            .round(),
      );
      add('client.perf.jank_ratio_max', jankRatio(frames, budget));
      // The log names the refresh rate: the budget is one interval of the display, so a 120 Hz
      // panel measures against 8.3 ms where the reference Mac of 15 19 has 16.7 ms.
      debugPrint(
        'perf: ${frames.length} frames while scrolling at ${rate.toStringAsFixed(0)} Hz, '
        '${frames.where((f) => f > budget).length} over ${budget.inMicroseconds} us',
      );

      // Cold start from the filled data folder, twenty times (last: a restart ends the views).
      for (var i = 0; i < 20; i++) {
        // The end of the app before is not part of the start.
        await app.shutDown();
        final start = await clock.measure(
          'the sidebar from the cache',
          () => find
              .byKey(ValueKey('sidebar.space.${world.spaceId}'))
              .evaluate()
              .isNotEmpty,
          app.pumpApp,
        );
        add('client.perf.cold_start_p95', start);
      }
    } finally {
      await app.stop();
    }

    final summary = PerfSummary.evaluate(
      targets: desktopTargets(AppConfig.defaults()),
      device: deviceLabel,
      reference: referenceDevice,
      samples: samples,
    );
    // The driver writes the summary and fails the run on a missed target (Design note 3).
    binding.reportData = summary.toJson();
  });
}

/// A message row of a history.
final Finder _message = find.byWidgetPredicate(
  (w) =>
      w.key is ValueKey<String> &&
      (w.key! as ValueKey<String>).value.startsWith('message.'),
);

/// The rows of messages that wait in the outbox.
final Finder _waiting = find.byWidgetPredicate(
  (w) =>
      w.key is ValueKey<String> &&
      (w.key! as ValueKey<String>).value.startsWith('row.outbox.'),
);

/// The text of [t]: the live box draws the status line as `Text.rich` (DsStatusLine).
String _plain(Text t) => t.data ?? t.textSpan?.toPlainText() ?? '';

/// The status line of stream-agent in the live box.
final Finder _streamLine = find.byWidgetPredicate(
  (w) => w is Text && _plain(w).startsWith('Streaming '),
);

/// Types [text] into the composer and checks that the field holds it before the send.
Future<void> _type(WidgetTester tester, String text) async {
  await tester.enterText(composerText, text);
  await tester.pump();
  expect(_composerValue(tester), text, reason: 'the composer holds the text');
}

/// The text in the composer, for the message of a failed wait.
String _composerValue(WidgetTester tester) {
  final fields = composerText.evaluate();
  if (fields.isEmpty) return '<no composer>';
  return (fields.first.widget as TextField).controller?.text ??
      '<no controller>';
}

/// The send button of the composer.
final Finder _sendButton = find.byWidgetPredicate(
  (w) => w is DsTappable && w.label == _en.composerSend,
);

final AppLocalizations _en = lookupAppLocalizations(const Locale('en'));

/// Measures from the start of an action until the end of the first frame in which a condition
/// holds: a persistent frame callback runs after the frame is built, laid out and painted, so the
/// time is that of the drawn frame and not of the next poll of [E2eApp.until].
final class _FrameClock {
  _FrameClock(this.app) {
    SchedulerBinding.instance.addPersistentFrameCallback(_onFrame);
  }

  /// The app the waits pump.
  final E2eApp app;

  bool Function()? _condition;
  final Stopwatch _watch = Stopwatch();
  Duration? _hit;

  void _onFrame(Duration _) {
    final condition = _condition;
    if (condition == null || _hit != null) return;
    if (condition()) _hit = _watch.elapsed;
  }

  /// Runs [action] and returns the milliseconds until the first frame in which [condition]
  /// holds; fails like [E2eApp.until] naming [what].
  Future<double> measure(
    String what,
    bool Function() condition,
    Future<void> Function() action, {
    String Function()? detail,
  }) async {
    _hit = null;
    _watch
      ..reset()
      ..start();
    _condition = condition;
    try {
      await action();
      await app.until(what, () => _hit != null, detail: detail);
    } finally {
      _condition = null;
    }
    return _hit!.inMicroseconds / Duration.microsecondsPerMillisecond;
  }
}
