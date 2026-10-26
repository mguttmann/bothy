// SPDX-License-Identifier: Apache-2.0

/// The test-only timing hooks of the terminal view (brief M4-38): [TerminalPerfHooks] and
/// [TerminalPerfProbe] exist only in a build with `--dart-define=TERMINAL_PERF_HOOKS=true`
/// ([terminalPerfHooksEnabled]); import this library only from the performance level.
library;

export 'src/perf/perf_hooks.dart';
