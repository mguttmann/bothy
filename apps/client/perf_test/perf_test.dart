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
// - local echo: Enter in the composer until the waiting message is on screen;
// - patch to frame: a status line patch of stream-agent from its arrival in a `run.updated`
//   frame until the end of the first frame that shows it;
// - jank ratio: frames over one refresh interval while scrolling a thread of thousands of
//   messages;
// - catch-up rate: events applied per second after a reconnect with a backlog;
// - memory: the resident size with twenty threads in focus.

import 'dart:developer' show Timeline;
import 'dart:io';

import 'package:client/platform/frame_ticker.dart';
import 'package:domain/domain.dart' show AppConfig, ThreadId;
import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:integration_test/integration_test.dart';
import 'package:sync_engine/sync_engine.dart' show ConnectionPhase;

import '../e2e_test/flows/messaging_flows.dart' show composerText, sendText;
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
  double ms(Duration d) =>
      d.inMicroseconds / Duration.microsecondsPerMillisecond;

  testWidgets('desktop performance targets of 15 19', (tester) async {
    final app = await E2eApp.start(
      tester,
      world,
      ticker: SchedulerFrameTicker(),
    );
    final nonce = randomNonce().substring(0, 8);
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
        final watch = Stopwatch()..start();
        go('/t/$post');
        await app.until(
          'the messages of $post',
          () => find
              .descendant(
                of: find.byKey(ValueKey('history.$post')),
                matching: _message,
              )
              .evaluate()
              .isNotEmpty,
        );
        add('client.perf.thread_open_p95', ms(watch.elapsed));
      }

      // Local echo, twenty sends into the first post.
      go('/t/${posts.first}');
      await app.waitFor(find.byKey(const ValueKey('composer')));
      for (var i = 0; i < 20; i++) {
        final waitingBefore = _waiting.evaluate().length;
        await tester.enterText(composerText, 'Echo $nonce $i');
        await tester.pump();
        final watch = Stopwatch()..start();
        await tester.sendKeyEvent(
          LogicalKeyboardKey.enter,
          // A profile build has no debug names to look the physical key up (the performance level).
          physicalKey: PhysicalKeyboardKey.enter,
        );
        await app.until(
          'the waiting message $i',
          () => _waiting.evaluate().length > waitingBefore,
        );
        add('client.perf.send_local_echo_p95', ms(watch.elapsed));
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
              .firstMatch((e.widget as Text).data ?? '');
          if (m != null) {
            shown.putIfAbsent(int.parse(m.group(1)!), () => Timeline.now);
          }
        }
      });
      await sendText(tester, '@${world.streamAgent.handle} stream please');
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
      final before = app.device.transport.dispatched;
      c.network.restore();
      await app.until(
        'the catch-up starts',
        () =>
            app.state.services!.engine.state.phase ==
            ConnectionPhase.catchingUp,
      );
      final catchup = Stopwatch()..start();
      await app.waitLive();
      final seconds = catchup.elapsed.inMicroseconds / 1e6;
      final events = app.device.transport.dispatched - before;
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

      // Cold start from the filled data folder, twenty times (last: a restart ends the views).
      for (var i = 0; i < 20; i++) {
        final watch = Stopwatch()..start();
        await app.restart();
        await app.until(
          'the sidebar from the cache',
          () => find
              .byKey(ValueKey('sidebar.space.${world.spaceId}'))
              .evaluate()
              .isNotEmpty,
        );
        add('client.perf.cold_start_p95', ms(watch.elapsed));
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

/// The status line of stream-agent in the live box.
final Finder _streamLine = find.byWidgetPredicate(
  (w) => w is Text && (w.data ?? '').startsWith('Streaming '),
);
