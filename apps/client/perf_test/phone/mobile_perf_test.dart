// SPDX-License-Identifier: Apache-2.0

// The performance level of the phone apps (brief M5-11 Design note 5; docs/spec/
// 15-client-architecture.md 19 and 20 row "Performance"): a profile build on a physical iPhone
// or Android phone measures the targets `client.perf.*` of the phones and reports the summary of
// mobile_perf_summary.dart to the driver test_driver/mobile_perf_driver.dart, which writes it to
// build/perf/mobile-summary.json. tool/mobile_perf.sh runs it (flutter drive --profile); a
// simulator or emulator runs no profile build of a phone, so it needs a device.
//
// The server is the fake of M4-17 inside the app, on its own isolate (perf_fake.dart): the
// numbers are those of the client, without a network, a server or its load in them, and the run
// needs nothing but the phone. The desktop level measures against the Compose stack instead; a
// profile build of a phone cannot reach it, because only debug builds of the phone apps trust the
// test CA of the stack.
//
// What is measured, each from the app's own frames with the engine drawing live, as on the
// desktop (perf_test/perf_test.dart, D-M4-34-5):
// - cold start: the start of the app on its filled data folder until the first drawn frame that
//   shows the space in the sidebar of the phone, twenty times (the end of the app before is not
//   counted), each plus the time from the start of the process to the entry of `main` once
//   (process, engine and Dart VM; mobile_device.dart reads it from the platform). What a process
//   does only once after `main` (the binding, fonts, loading the database library) is not in
//   the restarts, as on the desktop level;
// - thread open: a thread opened before, from its route until its first message is on screen;
// - local echo: a send from the composer until the row of the message is on screen;
// - patch to frame: a status line patch of a run from its arrival in a `run.updated` frame
//   until the end of the first frame that shows it;
// - jank ratio: frames over one refresh interval while scrolling a thread of thousands of
//   messages, from the timings of the engine;
// - catch-up rate: events applied per second after a reconnect with a backlog of several pages
//   of the fake's catch-up;
// - memory: the resident set of the process with five open threads, `ProcessInfo.currentRss`,
//   which the Dart runtime reads from the kernel: `resident_size` of the Mach task on iOS and the
//   resident pages of `/proc/self/statm` on Android. It is the resident set, not the footprint
//   the system limits an app by, and it includes the fake's isolate, so it is the larger number.

import 'dart:developer' show Timeline;
import 'dart:io';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart' show AppConfig, ThreadId;
import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:integration_test/integration_test.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/sync_engine.dart' show ConnectionPhase;

import '../perf_summary.dart';
import 'mobile_device.dart';
import 'mobile_perf_app.dart';
import 'mobile_perf_summary.dart';
import 'perf_fake.dart';

/// The backlog of the catch-up and the length of the scrolled thread: "several thousand
/// messages", several pages of the fake's catch-up (as the desktop level).
const int backlog = 2000;

/// The scroll gestures of the jank measurement, alternately back and forth.
const int scrollGestures = 20;

void main() {
  // The entry of `main`, before anything of the test runs: the part of the cold start the
  // restarts below cannot repeat (process, engine, Dart VM) ends here.
  final entry = MainEntry.record();
  final binding = IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  // The engine draws frames on its own schedule, so frame timings and waits are real.
  binding.framePolicy = LiveTestWidgetsFlutterBindingFramePolicy.fullyLive;
  final samples = <String, List<double>>{};
  final details = <String, Object?>{
    'server': 'fake server of M4-17 inside the app, on its own isolate',
    'memory_source': 'ProcessInfo.currentRss (resident set)',
  };
  void add(String name, double value) =>
      (samples[name] ??= <double>[]).add(value);

  testWidgets('phone performance targets of 15 19', (tester) async {
    final device = await readMobileDevice();
    final startup = await entry.processToMain();
    debugPrint('perf-mobile: measuring on ${device.label}');
    final fake = await PerfFake.spawn();
    MobilePerfApp? started;
    try {
      final app = started = await MobilePerfApp.start(tester, fake);
      final clock = _FrameClock(app);
      final threads = fake.threads;
      void go(String path) => GoRouter.of(
        tester.element(find.byKey(const ValueKey('phone.shell')).first),
      ).go(path);

      await app.signIn();

      // Every thread once from the network, so the cache holds them.
      for (final t in threads) {
        go('/t/$t');
        await app.waitFor(_messagesOf(t));
      }

      // Memory with five open threads: all of them in the gateway focus (04 6.2), the last one
      // on screen.
      app.state.services!.engine.focus.setVisible({
        for (final t in threads) ThreadId.parse(t),
      }, idle: false);
      await tester.pump(const Duration(milliseconds: 500));
      add('client.perf.memory_mobile_max', ProcessInfo.currentRss.toDouble());

      // Thread open from the cache, from the spaces page once the thread on screen has left
      // (so no transition of the page before runs into the measurement).
      var onScreen = threads.last;
      for (var i = 0; i < minSamples; i++) {
        final t = threads[i % threads.length];
        go('/');
        await app.until(
          'the spaces page',
          () => find.byKey(ValueKey('history.$onScreen')).evaluate().isEmpty,
        );
        final opened = await clock.measure(
          'the messages of $t',
          () => _messagesOf(t).evaluate().isNotEmpty,
          () async => go('/t/$t'),
        );
        add('client.perf.thread_open_p95', opened);
        onScreen = t;
      }

      // Local echo, twenty sends into the first thread once its page stands: the page of the
      // thread before has left and the new one ended its transition (a tap during the slide
      // lands beside the button).
      final first = threads.first;
      go('/t/$first');
      await app.waitFor(_messagesOf(first));
      final previous = onScreen;
      await app.until(
        'the page of the thread before left',
        () =>
            previous == first ||
            find.byKey(ValueKey('history.$previous')).evaluate().isEmpty,
      );
      await app.waitFor(_composerOf(first));
      final send = _sendButtonOf(tester, first);
      await app.until('the thread page ended its transition', () {
        final found = send.evaluate();
        if (found.isEmpty) return false;
        final animation = ModalRoute.of(found.first)?.animation;
        return animation == null ||
            animation.status == AnimationStatus.completed;
      });
      final field = find.descendant(
        of: _composerOf(first),
        matching: find.byType(TextField),
      );
      for (var i = 0; i < minSamples; i++) {
        final text = 'Echo $i done';
        await _type(tester, field, text);
        // The echo is the row of the text, waiting or already confirmed: the confirmation can
        // land before the frame that would show the waiting row.
        final echo = await clock.measure(
          'the echo of message $i',
          () => find
              .descendant(
                of: find.byKey(ValueKey('history.$first')),
                matching: find.textContaining(text, findRichText: true),
              )
              .evaluate()
              .isNotEmpty,
          () => tester.tap(send),
          detail: () => 'composer "${_composerValue(field)}"',
        );
        add('client.perf.send_local_echo_p95', echo);
        await app.until(
          'the confirmation of $i',
          () => _waiting.evaluate().isEmpty,
        );
      }

      // Patch to frame: the fake streams a run's status line into the open thread.
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
      await fake.stream(first);
      final arrivals = app.transport.record.streamArrivals;
      await app.until(
        'the stream of the run',
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
      final big = threads[1];
      go('/t/$big');
      await app.waitFor(_messagesOf(big));
      app.network.set(false);
      await fake.disconnect(app.deviceId);
      await app.until(
        'offline',
        () => app.state.services!.engine.state.phase != ConnectionPhase.live,
      );
      await fake.post(big, backlog);
      // The phases come from the engine's state stream, so a catch-up shorter than one poll of
      // the wait is still seen; the events count from the start of the catch-up. The arrival
      // of `catchup.complete` tells how much of the time the delivery took.
      int? catchupStart;
      int? catchupEnd;
      var before = 0;
      var after = 0;
      app.transport
        ..sessionAt = null
        ..completeAt = null;
      final phases = app.state.services!.engine.states.listen((s) {
        if (catchupStart == null && s.phase == ConnectionPhase.catchingUp) {
          catchupStart = Timeline.now;
          before = app.transport.record.dispatched;
        } else if (catchupStart != null &&
            catchupEnd == null &&
            s.phase == ConnectionPhase.live) {
          catchupEnd = Timeline.now;
          after = app.transport.record.dispatched;
        }
      });
      app.network.set(true);
      await app.until('the catch-up and live', () => catchupEnd != null);
      await phases.cancel();
      final events = after - before;
      final seconds = (catchupEnd! - catchupStart!) / 1e6;
      final session = app.transport.sessionAt;
      final complete = app.transport.completeAt;
      details
        ..['catchup_events'] = events
        ..['catchup_ms'] = seconds * 1000
        ..['catchup_delivery_ms'] = session == null || complete == null
            ? null
            : (complete - session) / 1000;
      add('client.perf.catchup_events_per_s_min', events / seconds);

      // Jank while scrolling the long thread back and forth, without pauses: only the frames
      // of the scroll count (with the engine drawing live, an idle frame between two gestures
      // would dilute the ratio).
      final list = find.descendant(
        of: find.byKey(ValueKey('history.$big')),
        matching: find.byKey(const ValueKey('history.list')),
      );
      await app.waitFor(list);
      final height = tester.getSize(list.first).height;
      final frames = <Duration>[];
      var batches = 0;
      void onTimings(List<FrameTiming> timings) {
        batches++;
        frames.addAll([for (final t in timings) t.totalSpan]);
      }

      SchedulerBinding.instance.addTimingsCallback(onTimings);
      for (var i = 0; i < scrollGestures; i++) {
        await tester.timedDrag(
          list.first,
          Offset(0, height * (i.isEven ? 0.6 : -0.3)),
          const Duration(milliseconds: 300),
        );
      }
      // The engine reports timings in batches; the batch of the last gesture comes after it.
      final seen = batches;
      await app.until(
        'the frame timings of the last gesture',
        () => batches > seen,
      );
      SchedulerBinding.instance.removeTimingsCallback(onTimings);
      final displays = binding.platformDispatcher.displays;
      final rate = displays.isEmpty ? 60.0 : displays.first.refreshRate;
      final budget = Duration(
        microseconds: (Duration.microsecondsPerSecond / (rate > 0 ? rate : 60))
            .round(),
      );
      add('client.perf.jank_ratio_max', jankRatio(frames, budget));
      details
        ..['refresh_rate_hz'] = rate
        ..['scroll_frames'] = frames.length
        ..['scroll_frames_over_budget'] = frames
            .where((f) => f > budget)
            .length;

      // Cold start from the filled data folder, twenty times (last: a restart ends the views).
      final restarts = <double>[];
      for (var i = 0; i < minSamples; i++) {
        // The end of the app before is not part of the start.
        await app.shutDown();
        restarts.add(
          await clock.measure(
            'the sidebar from the cache',
            () => find
                .byKey(ValueKey('sidebar.space.${fake.spaceId}'))
                .evaluate()
                .isNotEmpty,
            app.pumpApp,
          ),
        );
      }
      if (startup == null) {
        // Without the start of the process the samples would leave out part of the start, so
        // the target stays unmeasured and fails rather than pass on a part.
        details['cold_start_note'] =
            'the platform did not tell the start '
            'of the process';
      } else {
        final share =
            startup.inMicroseconds / Duration.microsecondsPerMillisecond;
        details['cold_start_process_to_main_ms'] = share;
        for (final r in restarts) {
          add('client.perf.cold_start_p95', share + r);
        }
      }
      details['contract_violations'] = (await fake.violations()).length;
    } finally {
      // The summary goes to the driver also after a failure, with what was measured; the
      // driver writes it and the failed test fails the run (Design note 5).
      final summary = MobilePerfSummary.evaluate(
        targets: mobileTargets(AppConfig.defaults()),
        device: device,
        samples: samples,
        details: details,
      );
      binding.reportData = summary.toJson();
      for (final r in summary.perf.results) {
        debugPrint(
          'perf-mobile: ${r.target.name} ${r.value} ${r.target.unit} '
          '(${r.samples} samples, ${r.pass ? 'met' : 'missed'})',
        );
      }
      debugPrint(
        'perf-mobile: result ${summary.result.json}, details $details',
      );
      try {
        await started?.stop();
      } finally {
        await fake.stop();
      }
    }
  });
}

/// A message row of a history.
final Finder _message = find.byWidgetPredicate(
  (w) =>
      w.key is ValueKey<String> &&
      (w.key! as ValueKey<String>).value.startsWith('message.'),
);

/// The message rows of the history of [thread].
Finder _messagesOf(String thread) => find.descendant(
  of: find.byKey(ValueKey('history.$thread')),
  matching: _message,
);

/// The rows of messages that wait in the outbox.
final Finder _waiting = find.byWidgetPredicate(
  (w) =>
      w.key is ValueKey<String> &&
      (w.key! as ValueKey<String>).value.startsWith('row.outbox.'),
);

/// The text of [t]: the live box draws the status line as `Text.rich` (DsStatusLine).
String _plain(Text t) => t.data ?? t.textSpan?.toPlainText() ?? '';

/// The status line of the run in the live box.
final Finder _streamLine = find.byWidgetPredicate(
  (w) => w is Text && _plain(w).startsWith('Streaming '),
);

/// The composer of the history of [thread].
Finder _composerOf(String thread) => find.descendant(
  of: find.byKey(ValueKey('history.$thread')),
  matching: find.byKey(const ValueKey('composer')),
);

/// Types [text] into the composer's text [field] and checks that the field holds it before the
/// send.
Future<void> _type(WidgetTester tester, Finder field, String text) async {
  await tester.enterText(field, text);
  await tester.pump();
  expect(_composerValue(field), text, reason: 'the composer holds the text');
}

/// The text in the composer's text [field], for the message of a failed wait.
String _composerValue(Finder field) {
  final fields = field.evaluate();
  if (fields.isEmpty) return '<no composer>';
  return (fields.first.widget as TextField).controller?.text ??
      '<no controller>';
}

/// The send button of the composer of [thread], by its label in the language the app shows
/// (the phone's).
Finder _sendButtonOf(WidgetTester tester, String thread) {
  final composer = _composerOf(thread);
  final texts = lookupAppLocalizations(
    Localizations.localeOf(tester.element(composer.first)),
  );
  return find.descendant(
    of: composer,
    matching: find.byWidgetPredicate(
      (w) => w is DsTappable && w.label == texts.composerSend,
    ),
  );
}

/// Measures from the start of an action until the end of the first frame in which a condition
/// holds: a persistent frame callback runs after the frame is built, laid out and painted, so the
/// time is that of the drawn frame and not of the next poll of [MobilePerfApp.until].
final class _FrameClock {
  _FrameClock(this.app) {
    SchedulerBinding.instance.addPersistentFrameCallback(_onFrame);
  }

  /// The app the waits pump.
  final MobilePerfApp app;

  bool Function()? _condition;
  final Stopwatch _watch = Stopwatch();
  Duration? _hit;

  void _onFrame(Duration _) {
    final condition = _condition;
    if (condition == null || _hit != null) return;
    if (condition()) _hit = _watch.elapsed;
  }

  /// Runs [action] and returns the milliseconds until the first frame in which [condition]
  /// holds; fails like [MobilePerfApp.until] naming [what].
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
