// SPDX-License-Identifier: Apache-2.0

// The terminal in the browser (brief M6-18 Design notes 5 and 6): the console's own UI opens a
// terminal on the seeded host "Build host" with a passkey step-up, the test measures the
// frame-to-paint time and the throughput of the view, detaches, lets the runner check the attach
// without a new passkey and the foreign origin (synchronisation point "detached"), reattaches
// with a new passkey, checks that the scrollback was replayed and logs out, after which the runner
// checks that the pin store is gone (tools/console-e2e headless/terminal_suites.py).
//
// The numbers go to the browser console in one line with the prefix E2E_TERMINAL_METRICS; it
// never carries terminal content, only marker counts and times. The finders by the keys
// console.terminal.open, .pane, .detach and .reattach are ConsoleTerminalKeys of the route
// "Hosts and terminal".

import 'dart:convert';
import 'dart:js_interop';
import 'dart:js_interop_unsafe';
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';
import 'package:terminal_ui/terminal_ui.dart';

import '../admin/support.dart';

@JS('__e2eSeed')
external JSString? get _seedJson;

/// The key of the passkey button of the console's step-up prompt, tapped whenever it shows
/// (the virtual authenticator of the runner answers with user verification).
const _passkey = ValueKey('consoleStepUp.passkey');

/// The number of frame-to-paint markers the host writes.
const _marks = 60;

/// The bytes of the burst of the throughput measurement and the width `fold` breaks them at.
const _burstBytes = 4 * 1024 * 1024;
const _burstLineChars = 128;

/// The bytes the program of the burst writes: the burst, a line feed after each full line but
/// the last, and the line feed before the end marker.
const _burstOutputBytes = _burstBytes + _burstBytes ~/ _burstLineChars;

// The commands typed into the shell. The markers are put together by the shell (`%s` with
// M=E2E), so the echo of a command never matches a marker line. The host's clock is the
// kernel clock the browser container shares.
const _readyCommand = r'''M=E2E; printf '%sREADY\n' "$M"''';
final _markCommand = r'''M=E2E; for i in $(seq 1 {N}); do printf '%sMARK %d %s\n' "$M" "$i" "$(date +%s%3N)"; sleep 0.05; done'''
    .replaceAll('{N}', '$_marks');
final _burstCommand = r'''M=E2E; S=$(date +%s%3N); head -c {B} /dev/zero | tr '\0' x | fold -w {W}; printf '\n%sTPEND %s\n' "$M" "$S"'''
    .replaceAll('{B}', '$_burstBytes')
    .replaceAll('{W}', '$_burstLineChars');
const _scrollbackCommand = r'''M=E2E; printf '%sSCROLLBACK\n' "$M"''';

final _readyLine = RegExp(r'^E2EREADY\s*$');
final _markLine = RegExp(r'^E2EMARK (\d+) (\d+)\s*$');
final _endLine = RegExp(r'^E2ETPEND (\d+)\s*$');
final _scrollbackLine = RegExp(r'^E2ESCROLLBACK\s*$');

void main() {
  patrolTest('terminal open, measure, detach, reattach with a passkey, log out', (
    $,
  ) async {
    final host = _seededHost();
    await openConsole($, '/');
    await tapOn($, find.text('Hosts and terminal'));
    await tapOn($, find.text(host.name));
    await tapKey($, 'console.terminal.open');
    // The dialog "Open terminal" of terminal_ui: the profile shell, then confirm.
    await tapOn(
      $,
      find.byWidgetPredicate(
        (w) => w is RadioListTile<String> && w.value == 'shell',
      ),
    );
    await tapOn(
      $,
      find.descendant(
        of: find.byType(OpenTerminalDialog),
        matching: find.byType(FilledButton),
      ),
    );
    await waitFor($, byKey('console.terminal.pane'));
    final c = _controller($);
    await _pumpUntil(
      $,
      () => c.state.phase == TerminalPhase.connected,
      'the terminal to connect',
    );
    _type(c, _readyCommand);
    await _pumpUntil(
      $,
      () => _hasLine(c, _readyLine),
      'the output of the shell',
    );

    final numbers = await _measure($, c);
    await _syncWithRunner($, 'open');

    _type(c, _scrollbackCommand);
    await _pumpUntil(
      $,
      () => _hasLine(c, _scrollbackLine),
      'the scrollback marker',
    );
    await tapKey($, 'console.terminal.detach');
    await _pumpUntil(
      $,
      () => c.state.phase == TerminalPhase.detached,
      'the detach',
    );
    await _syncWithRunner($, 'detached');

    final before = c.screen.terminal;
    var replayFrames = false;
    void watchReplay() {
      if (c.state.replaying) replayFrames = true;
    }

    c.addListener(watchReplay);
    await tapKey($, 'console.terminal.reattach');
    final after = await _reattached($);
    // A replay swaps the terminal model of the screen (TerminalScreen.replaceWith).
    await _pumpUntil(
      $,
      () => !identical(after.screen.terminal, before) || replayFrames,
      'the replay of the scrollback',
      timeout: const Duration(seconds: 20),
      failOnTimeout: false,
    );
    c.removeListener(watchReplay);
    final seenAgain =
        !identical(after.screen.terminal, before) &&
        _hasLine(after, _scrollbackLine);

    // ignore: avoid_print
    print(
      'E2E_TERMINAL_METRICS ${jsonEncode({...numbers, 'opened': true, 'replay_frames': replayFrames, 'text_seen_again': seenAgain})}',
    );

    await tapOn($, find.text('Log out'));
    await waitFor($, find.text('Log in'));
  });
}

/// The host the runner seeded (`window.__e2eSeed.host`).
({String id, String name}) _seededHost() {
  final raw = _seedJson?.toDart;
  if (raw == null) throw StateError('window.__e2eSeed is not set');
  final host =
      (jsonDecode(raw) as Map<String, Object?>)['host']!
          as Map<String, Object?>;
  return (id: host['id']! as String, name: host['name']! as String);
}

/// The controller of the terminal pane on screen.
TerminalController _controller(PatrolIntegrationTester $) =>
    $.tester.widget<TerminalPane>(find.byType(TerminalPane).first).controller;

/// Sends [command] and a carriage return to the shell, as the keys of the view would.
void _type(TerminalController c, String command) =>
    c.input(utf8.encode('$command\r'));

/// The text of the last [count] lines of the terminal model.
List<String> _tail(TerminalController c, int count) {
  final lines = c.screen.terminal.buffer.lines;
  final start = math.max(0, lines.length - count);
  return [for (var i = start; i < lines.length; i++) lines[i].getText()];
}

/// Whether a line near the end of the terminal model matches [pattern].
bool _hasLine(TerminalController c, RegExp pattern) => _tail(
  c,
  c.screen.terminal.viewHeight + 5,
).any((l) => pattern.hasMatch(l.trimRight()));

/// Pumps until [done], confirming every step-up prompt with the passkey on the way.
///
/// Throws a [TestFailure] after [timeout] unless [failOnTimeout] is false.
Future<void> _pumpUntil(
  PatrolIntegrationTester $,
  bool Function() done,
  String what, {
  Duration timeout = const Duration(seconds: 90),
  bool failOnTimeout = true,
}) async {
  final end = DateTime.now().add(timeout);
  while (!done()) {
    if (!DateTime.now().isBefore(end)) {
      if (!failOnTimeout) return;
      throw TestFailure('timed out waiting for $what; ${screenText()}');
    }
    await $.tester.pump(const Duration(milliseconds: 100));
    final passkey = find.byKey(_passkey);
    if (passkey.evaluate().isNotEmpty) await $.tester.tap(passkey);
  }
}

/// Waits until the reattached tab is connected and returns its controller (the pane may hold
/// a new one).
Future<TerminalController> _reattached(PatrolIntegrationTester $) async {
  await _pumpUntil(
    $,
    () =>
        find.byType(TerminalPane).evaluate().isNotEmpty &&
        _controller($).state.phase == TerminalPhase.connected &&
        !_controller($).state.replaying,
    'the reattach',
  );
  return _controller($);
}

/// Tells the runner that the test reached [phase] and waits for its acknowledgement
/// (`window.__e2ePhase`, `window.__e2eAck`).
Future<void> _syncWithRunner(PatrolIntegrationTester $, String phase) async {
  globalContext['__e2ePhase'] = phase.toJS;
  final end = DateTime.now().add(const Duration(minutes: 5));
  while (DateTime.now().isBefore(end)) {
    await $.tester.pump(const Duration(milliseconds: 200));
    if (globalContext['__e2eAck']?.dartify() == phase) return;
  }
  throw TestFailure('the runner did not acknowledge $phase');
}

/// Measures the frame-to-paint time of the markers and the throughput of the burst (Design
/// note 6) with every frame the engine asks for (frame policy fully live) and returns the
/// numbers: per marker the time from the write of its data frame into the model to the end of
/// the next frame, and from the host's clock value in the marker to that end; for the burst its
/// size, the host's time of its start and the end of the frame that painted its end marker.
Future<Map<String, Object?>> _measure(
  PatrolIntegrationTester $,
  TerminalController c,
) async {
  final binding = $.tester.binding;
  final live = binding is LiveTestWidgetsFlutterBinding ? binding : null;
  final policy = live?.framePolicy;
  live?.framePolicy = LiveTestWidgetsFlutterBindingFramePolicy.fullyLive;
  final writeToPaint = <double>[];
  final hostToPaint = <double>[];
  final seen = <int>{};
  int? burstHostStart;
  double? burstPaintEnd;

  void afterNextFrame(void Function(int paintedUs) record) {
    SchedulerBinding.instance.addPostFrameCallback(
      (_) => record(DateTime.now().microsecondsSinceEpoch),
    );
    SchedulerBinding.instance.scheduleFrame();
  }

  void onWrite() {
    final writtenUs = DateTime.now().microsecondsSinceEpoch;
    for (final line in _tail(c, c.screen.terminal.viewHeight + 1)) {
      final text = line.trimRight();
      final mark = _markLine.firstMatch(text);
      if (mark != null && seen.add(int.parse(mark.group(1)!))) {
        final hostMs = int.parse(mark.group(2)!);
        afterNextFrame((paintedUs) {
          writeToPaint.add((paintedUs - writtenUs) / 1000);
          hostToPaint.add(paintedUs / 1000 - hostMs);
        });
      }
      final end = _endLine.firstMatch(text);
      if (end != null && burstHostStart == null) {
        burstHostStart = int.parse(end.group(1)!);
        afterNextFrame((paintedUs) => burstPaintEnd = paintedUs / 1000);
      }
    }
  }

  c.screen.addListener(onWrite);
  try {
    _type(c, _markCommand);
    await _pumpUntil(
      $,
      () => writeToPaint.length >= _marks,
      'the frame-to-paint markers',
      timeout: const Duration(seconds: 60),
      failOnTimeout: false,
    );
    _type(c, _burstCommand);
    await _pumpUntil(
      $,
      () => burstPaintEnd != null,
      'the end of the burst',
      timeout: const Duration(minutes: 3),
    );
  } finally {
    c.screen.removeListener(onWrite);
    if (live != null && policy != null) live.framePolicy = policy;
  }
  return {
    'marks_expected': _marks,
    'write_to_paint_ms': writeToPaint,
    'host_to_paint_ms': hostToPaint,
    'throughput': {
      'bytes': _burstOutputBytes,
      'host_start_ms': burstHostStart,
      'paint_end_ms': burstPaintEnd,
    },
  };
}
