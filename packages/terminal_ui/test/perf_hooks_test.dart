// SPDX-License-Identifier: Apache-2.0

// The test-only timing hooks of the terminal view (brief M4-38 Design note 2): without the define
// TERMINAL_PERF_HOOKS the build has none (the probe is refused and every call site in lib/ is
// guarded by the constant, which the compiler drops), and with the define the controller reports
// each decrypted `data` frame and the screen each drawn frame after new output.
//
// `flutter test` runs the first group; the second group runs with
// `flutter test --dart-define=TERMINAL_PERF_HOOKS=true test/perf_hooks_test.dart` (the build of
// make perf-desktop).

import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/perf_hooks.dart';
import 'package:terminal_ui/terminal_ui.dart';

import 'support/fakes.dart';
import 'support/widgets.dart';

final class _Recorder implements TerminalPerfProbe {
  final List<(TerminalController, String, int)> decrypted = [];
  final List<(TerminalController, int)> drawn = [];

  @override
  void dataDecrypted(TerminalController tab, Uint8List bytes, int micros) =>
      decrypted.add((tab, utf8.decode(bytes), micros));

  @override
  void frameDrawn(TerminalController tab, int micros) =>
      drawn.add((tab, micros));
}

/// Every place in lib/ (outside the hook's own libraries) that names the probe, with the
/// statement around it: the line before and the line itself, joined.
List<(String, String)> _probeCalls() => [
  for (final f
      in Directory('lib')
          .listSync(recursive: true)
          .whereType<File>()
          .where((f) => f.path.endsWith('.dart')))
    if (!f.path.endsWith('perf_hooks.dart'))
      for (final (i, line) in f.readAsLinesSync().indexed)
        if (line.contains('TerminalPerfHooks'))
          (
            f.path,
            [
              if (i > 0) f.readAsLinesSync()[i - 1].trim(),
              line.trim(),
            ].join(' '),
          ),
];

void main() {
  group('without the define', () {
    test('the build has no hooks and refuses a probe', () {
      expect(terminalPerfHooksEnabled, isFalse);
      expect(() => TerminalPerfHooks.probe = _Recorder(), throwsStateError);
      expect(TerminalPerfHooks.probe, isNull);
    }, skip: terminalPerfHooksEnabled ? 'built with the hooks' : false);
  });

  test('every call site names the probe behind the constant guard', () {
    final calls = _probeCalls();
    // The controller (decrypted data) and the screen (drawn frames).
    expect(calls.map((c) => c.$1).toSet(), hasLength(2));
    for (final (file, line) in calls) {
      expect(
        line,
        matches(
          RegExp(
            r'if \(terminalPerfHooksEnabled\)\s*\{\s*TerminalPerfHooks\.probe\?\.',
          ),
        ),
        reason: '$file: a call outside the guard stays in a release build',
      );
    }
  });

  group('with the define', () {
    testWidgets(
      'the controller reports decrypted data and the screen the drawn frame after it',
      (tester) async {
        final probe = _Recorder();
        TerminalPerfHooks.probe = probe;
        addTearDown(() => TerminalPerfHooks.probe = null);
        final h = (await tester.runAsync(Harness.create))!;
        await pumpApp(
          tester,
          TerminalPane(controller: h.controller, fontSize: 14),
        );
        final opening = h.open();
        await pumpUntil(
          tester,
          () => h.controller.state.phase == TerminalPhase.connected,
        );
        await opening;
        await tester.runAsync(() => h.host.sendData(utf8.encode('seq 1\r\n')));
        await pumpUntil(tester, () => probe.decrypted.isNotEmpty);
        await tester.pump();
        final (tab, bytes, decryptedAt) = probe.decrypted.single;
        expect(tab, same(h.controller));
        expect(bytes, 'seq 1\r\n');
        // The frame after the output reported itself, later than the decrypt.
        expect(
          probe.drawn.where((d) => identical(d.$1, tab) && d.$2 >= decryptedAt),
          isNotEmpty,
        );
        expect(h.controller.screen.visibleText(), contains('seq 1'));
        await h.controller.detach();
        await tester.pump(const Duration(seconds: 1));
      },
      skip: !terminalPerfHooksEnabled,
    );
  });
}
