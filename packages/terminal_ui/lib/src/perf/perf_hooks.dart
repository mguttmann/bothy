// SPDX-License-Identifier: Apache-2.0

/// The test-only timing hooks of the terminal view (brief M4-38 Design note 2; docs/spec/
/// 15-client-architecture.md 19 and 23.5 row "Performance"): the controller reports each
/// decrypted `data` frame and the screen reports each drawn frame that follows new output, so the
/// performance level can measure `client.perf.terminal_frame_to_paint_p95` and
/// `client.perf.terminal_throughput_min` on the real path (relay, host service, PTY, Noise,
/// renderer).
///
/// The hooks exist only in a build with `--dart-define=TERMINAL_PERF_HOOKS=true`: every call site
/// is guarded by the constant [terminalPerfHooksEnabled], so a release build and a normal profile
/// build compile the calls out, and [TerminalPerfHooks.probe] refuses a probe without the define.
/// A probe sees decrypted output: it is meant for the synthetic output of the generator of the
/// performance level and keeps it in memory only (R-CL-17).
library;

import 'dart:typed_data';

import 'package:terminal_ui/src/controller/terminal_controller.dart';

/// Whether this build carries the timing hooks (`--dart-define=TERMINAL_PERF_HOOKS=true`). A
/// constant, so the compiler drops every guarded call when it is false.
const bool terminalPerfHooksEnabled = bool.fromEnvironment(
  'TERMINAL_PERF_HOOKS',
);

/// Receives the timing events of every terminal tab of the process. Times are microseconds of
/// `Timeline.now` (dart:developer), the clock of the engine's `FrameTiming` as well, so a probe
/// can pair a drawn frame with the frame timing of its rasterization.
abstract interface class TerminalPerfProbe {
  /// [bytes] of a `data` frame of [tab] were decrypted at [micros], right before the screen (or
  /// the replay buffer) takes them.
  void dataDecrypted(TerminalController tab, Uint8List bytes, int micros);

  /// A frame that shows the output [tab] took so far was built, laid out and painted and handed
  /// to the rasterizer at [micros] (a post-frame callback of the frame after new output).
  void frameDrawn(TerminalController tab, int micros);
}

/// The one probe of the process, set by the performance level.
///
/// ```dart
/// // flutter drive --profile --dart-define=TERMINAL_PERF_HOOKS=true ...
/// TerminalPerfHooks.probe = recorder;
/// ```
abstract final class TerminalPerfHooks {
  static TerminalPerfProbe? _probe;

  /// The probe, or null.
  static TerminalPerfProbe? get probe => _probe;

  /// Sets or clears the probe. Throws a [StateError] in a build without the define: such a build
  /// has no hook that could ever call it.
  static set probe(TerminalPerfProbe? value) {
    if (!terminalPerfHooksEnabled) {
      throw StateError(
        'the terminal timing hooks need --dart-define=TERMINAL_PERF_HOOKS=true',
      );
    }
    _probe = value;
  }
}
