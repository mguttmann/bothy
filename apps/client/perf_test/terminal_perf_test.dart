// SPDX-License-Identifier: Apache-2.0

// The terminal targets of the performance level of the desktop app (brief M4-38 Design note 2;
// docs/spec/15-client-architecture.md 19 and 23.5 row "Performance"): a profile build with the
// timing hooks of terminal_ui (`--dart-define=TERMINAL_PERF_HOOKS=true`) against the client stack
// of the terminal (the real server, relay and host service in a container with the generator of
// clientstack/termgen) measures `client.perf.terminal_frame_to_paint_p95` and
// `client.perf.terminal_throughput_min` (terminal_perf.dart) and reports the summary to the
// driver test_driver/perf_driver.dart, which writes build/perf/summary.json (merged with the
// chat targets in make perf-desktop) and fails when a target is missed.
//
// Alex opens the host from the group "Hosts" twice: a session of the generator's rate profile
// (one numbered line per interval, frame to paint per line) and one of its burst profile (chunks
// at full speed, each after the line feed the test types once the end line of the chunk before
// was drawn, so at most one chunk is in flight). The burst must run without a detach: the host's
// send queue and the relay buffer disconnect a reader that falls behind (24 6.2), which is the
// backlog the target excludes.

import 'dart:convert';
import 'dart:developer' show Timeline;
import 'dart:typed_data';
import 'dart:ui' show FramePhase, FrameTiming;

import 'package:client/platform/frame_ticker.dart';
import 'package:client/terminal/providers.dart';
import 'package:domain/domain.dart' show AppConfig;
import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:l10n/l10n.dart';
import 'package:terminal_ui/perf_hooks.dart';
import 'package:terminal_ui/terminal_ui.dart';

import '../e2e_test/support/e2e_app.dart';
import '../e2e_test/support/e2e_network.dart';
import '../e2e_test/support/terminal_stack.dart';
import 'perf_summary.dart';
import 'terminal_perf.dart';

/// The device the numbers come from and whether it is a reference device of 15 19
/// (tool/e2e_suite.sh, brief M4-34 Design note 4).
const String deviceLabel = String.fromEnvironment(
  'PERF_DEVICE_LABEL',
  defaultValue: 'unnamed device',
);

/// Whether [deviceLabel] is a reference device.
const bool referenceDevice =
    String.fromEnvironment('PERF_REFERENCE', defaultValue: '0') == '1';

/// The texts of the app in the locale it runs in (the system's: a German desktop shows German).
AppLocalizations _l(E2eApp app) =>
    AppLocalizations.of(app.tester.element(find.byType(Navigator).first));

/// Routes the hooks of the tab under measurement to its recording.
final class _Probe implements TerminalPerfProbe {
  TerminalController? tab;
  RatePerf? rate;
  BurstPerf? burst;

  @override
  void dataDecrypted(TerminalController tab, Uint8List bytes, int micros) {
    if (!identical(tab, this.tab)) return;
    // Synthetic output of the generator only; it stays in this process's memory.
    final text = utf8.decode(bytes, allowMalformed: true);
    rate?.decrypted(text, micros);
    burst?.decrypted(text, bytes.length, micros);
  }

  @override
  void frameDrawn(TerminalController tab, int micros) {
    if (!identical(tab, this.tab)) return;
    final visible = tab.screen.visibleText();
    rate?.drawn(visible, micros);
    burst?.drawn(visible, micros);
  }
}

void main() {
  final binding = IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  // The engine draws frames on its own schedule, so frame timings and waits are real.
  binding.framePolicy = LiveTestWidgetsFlutterBindingFramePolicy.fullyLive;
  final stack = TerminalStack.fromEnvironment();
  final generator = stack.generator!;
  late E2eNetwork network;
  setUpAll(() async {
    network = await E2eNetwork.start(
      stack.server,
      stack.caPem,
      upstreamPort: stack.publishedPort,
    );
    network.install();
  });
  tearDownAll(() => network.stop());

  testWidgets('terminal performance targets of 15 19', (tester) async {
    expect(
      terminalPerfHooksEnabled,
      isTrue,
      reason: 'build with --dart-define=TERMINAL_PERF_HOOKS=true',
    );
    final probe = _Probe();
    TerminalPerfHooks.probe = probe;
    addTearDown(() => TerminalPerfHooks.probe = null);
    final frames = <RasterFrame>[];
    // The start of the build of each frame by the end of its build, for the side value "wait
    // for the frame" of the summary.
    final buildStarts = <int, int>{};
    final rasterStarts = <int, int>{};
    void onTimings(List<FrameTiming> timings) {
      for (final t in timings) {
        final finish = t.timestampInMicroseconds(FramePhase.buildFinish);
        buildStarts[finish] = t.timestampInMicroseconds(FramePhase.buildStart);
        rasterStarts[finish] = t.timestampInMicroseconds(
          FramePhase.rasterStart,
        );
        frames.add((
          buildFinish: finish,
          rasterFinish: t.timestampInMicroseconds(FramePhase.rasterFinish),
        ));
      }
    }

    SchedulerBinding.instance.addTimingsCallback(onTimings);
    addTearDown(
      () => SchedulerBinding.instance.removeTimingsCallback(onTimings),
    );

    final app = await E2eApp.start(
      tester,
      stack,
      ticker: SchedulerFrameTicker(),
    );
    final hostId = stack.hostId!;
    final rate = RatePerf();
    final burst = BurstPerf();
    var detachedDuringBurst = false;
    try {
      await app.signIn((await tester.runAsync(stack.enrollmentCode))!);
      final tabs = await _tabs(app, hostId);

      // Frame to paint: the rate profile, until its done line is drawn.
      probe
        ..rate = rate
        ..tab = await _open(app, tabs, generator.rateProfile);
      await app.until(
        'the done line of the rate profile',
        () => rate.done,
        detail: () => '${rate.decryptedLines} lines decrypted',
      );
      await _waitTimings(app, frames, rate.lastDrawn);
      await probe.tab!.end();
      probe.rate = null;

      // Throughput: the burst profile, one chunk per typed line feed.
      final tab = await _open(app, tabs, generator.burstProfile);
      probe
        ..burst = burst
        ..tab = tab;
      for (var i = 0; i < generator.burstChunks; i++) {
        burst.beginChunk(i);
        tab.input(utf8.encode('\r'));
        await app.until(
          'the end line of chunk $i drawn',
          () =>
              burst.chunkDrawn(i) || tab.state.phase != TerminalPhase.connected,
          detail: () => '${tab.state}',
        );
        if (tab.state.phase != TerminalPhase.connected) {
          detachedDuringBurst = true;
          break;
        }
      }
      await _waitTimings(app, frames, burst.lastDrawn);
      if (!detachedDuringBurst) {
        // The last line feed ends the generator.
        tab.input(utf8.encode('\r'));
        await app.until(
          'the generator ended',
          () => tab.state.phase == TerminalPhase.exited,
          detail: () => '${tab.state}',
        );
      }
      await tab.end();
    } finally {
      await app.stop();
    }

    final f2p = rate.frameToPaintMs(frames);
    final parts = rate.partsMs(frames);
    // From the decrypt to the start of the build of the frame that shows the line: the wait for
    // the next frame of the display.
    final waits = <double>[];
    final builds = <double>[];
    final rasters = <double>[];
    for (final p in rate.pairs) {
      RasterFrame? match;
      for (final f in frames) {
        if (f.buildFinish <= p.drawn &&
            p.drawn - f.buildFinish <= maxPaintGapMicros &&
            (match == null || f.buildFinish > match.buildFinish)) {
          match = f;
        }
      }
      final start = match == null ? null : buildStarts[match.buildFinish];
      if (match == null || start == null) continue;
      waits.add((start - p.decrypted) / 1000);
      builds.add((match.buildFinish - start) / 1000);
      rasters.add(
        (match.rasterFinish - rasterStarts[match.buildFinish]!) / 1000,
      );
    }
    final displays = binding.platformDispatcher.displays;
    double? pct(List<double> v, double q) {
      if (v.isEmpty) return null;
      final sorted = [...v]..sort();
      return sorted[((q * sorted.length).ceil() - 1).clamp(
        0,
        sorted.length - 1,
      )];
    }

    final t = burst.throughput(frames);
    final summary = PerfSummary.evaluate(
      targets: terminalTargets(AppConfig.defaults()),
      device: deviceLabel,
      reference: referenceDevice,
      samples: {
        'client.perf.terminal_frame_to_paint_p95': f2p,
        if (t != null && !detachedDuringBurst)
          'client.perf.terminal_throughput_min': [t.bytesPerSecond],
      },
      details: {
        'terminal_generator': {
          'rate_lines': generator.rateLines,
          'rate_interval_ms': generator.rateInterval.inMilliseconds,
          'burst_chunk_bytes': generator.burstChunkBytes,
          'burst_chunks': generator.burstChunks,
        },
        'terminal_frames_paired': f2p.length,
        'terminal_frame_to_paint_p50_ms': pct(f2p, 0.5),
        'terminal_decrypt_to_frame_end_p50_ms': pct(parts.toFrame, 0.5),
        'terminal_decrypt_to_frame_end_p95_ms': pct(parts.toFrame, 0.95),
        'terminal_frame_end_to_raster_p50_ms': pct(parts.toRaster, 0.5),
        'terminal_frame_end_to_raster_p95_ms': pct(parts.toRaster, 0.95),
        'terminal_decrypt_to_build_start_p50_ms': pct(waits, 0.5),
        'terminal_decrypt_to_build_start_p95_ms': pct(waits, 0.95),
        'terminal_build_p50_ms': pct(builds, 0.5),
        'terminal_build_p95_ms': pct(builds, 0.95),
        'terminal_raster_p50_ms': pct(rasters, 0.5),
        'terminal_raster_p95_ms': pct(rasters, 0.95),
        'display_refresh_hz': displays.isEmpty
            ? null
            : displays.first.refreshRate,
        'terminal_lines_decrypted': rate.decryptedLines,
        if (t != null) ...{
          'terminal_burst_bytes': t.bytes,
          'terminal_burst_seconds': t.seconds,
          'terminal_burst_chunks_drawn': t.chunks,
          'terminal_backlog_max_bytes': t.backlogMaxBytes,
        },
        'terminal_detached_during_burst': detachedDuringBurst,
      },
    );
    // The log names only numbers, never terminal content.
    debugPrint(
      'perf: terminal frame to paint over ${f2p.length} lines, burst '
      '${t?.bytes ?? 0} bytes in ${t?.seconds.toStringAsFixed(3) ?? '-'} s, '
      'largest backlog ${t?.backlogMaxBytes ?? 0} bytes',
    );
    // The driver writes the summary and fails the run on a missed target (Design note 2).
    binding.reportData = summary.toJson();
  }, timeout: Timeout.none);
}

/// The tabs of [hostId] after the host was opened from the group "Hosts".
Future<TerminalTabs> _tabs(E2eApp app, String hostId) async {
  await app.tap('hosts.row.$hostId');
  await app.waitFor(find.byKey(const ValueKey('terminal.new')));
  // Read into a local first: a provider read in the return of an async function throws.
  final tabs = app.container.read(terminalTabsProvider(hostId))!;
  return tabs;
}

/// Opens a session of [profile] with "New session" and the step-up; returns its tab once
/// connected.
Future<TerminalController> _open(
  E2eApp app,
  TerminalTabs tabs,
  String profile,
) async {
  final before = tabs.tabs.length;
  await app.tap('terminal.new');
  await app.tapSettled(find.byKey(ValueKey('terminal.new.$profile')));
  await app.tapSettled(find.text(_l(app).terminalOpenConfirm));
  final dialog = find.byKey(const ValueKey('stepUp.continue'));
  bool connected() =>
      tabs.tabs.length > before &&
      tabs.current?.state.phase == TerminalPhase.connected;
  await app.until(
    'the $profile session',
    () => connected() || dialog.evaluate().isNotEmpty,
  );
  if (!connected()) {
    await app.tapSettled(dialog);
    await app.until('the $profile session connected', connected);
  }
  return tabs.current!;
}

/// Pumps until the engine reported the frame timings up to the frame drawn at [lastDrawn]
/// (they come in batches).
Future<void> _waitTimings(
  E2eApp app,
  List<RasterFrame> frames,
  int? lastDrawn,
) async {
  if (lastDrawn == null) return;
  await app.until(
    'the frame timings of the measurement',
    () => frames.any((f) => f.buildFinish >= lastDrawn),
    detail: () =>
        '${frames.length} timings, now ${Timeline.now}, last drawn $lastDrawn',
  );
}
