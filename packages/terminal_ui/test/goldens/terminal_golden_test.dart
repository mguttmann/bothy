// SPDX-License-Identifier: Apache-2.0

// Goldens of the terminal view (docs/design/components.md 12, rule "Golden tests in light and
// dark with warning, detached and extra keys bar"; brief M4-35): the pane connected with
// output, detached, ended and blocked, the key change warning, the pane with the extra keys bar
// and the bar with latched modifiers; light and dark, text scaling 1 and 2, German and English.

import 'dart:convert';

import 'package:alchemist/alchemist.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';

import '../support/fakes.dart';

/// The size of every pane, so the xterm view lays out with a fixed grid; at text scale 2 the
/// header, tabs and banner scroll within their share of the height (chromeHeightShare).
const Size _paneSize = Size(720, 360);

/// The clock of the header: the grant opened the session at 09:30, so 12 hours leave 11:30.
DateTime _now() => DateTime.utc(2026, 10, 20, 10);

const Duration _maxSessionDuration = Duration(hours: 12);

/// Two full fingerprints of 13 groups of four (components.md 12 "Key change warning").
const String _storedFingerprint =
    '7KQ2-9XTA-M4PB-1C8D-H3WN-6RFJ-2LQX-8ZTB-5MVE-0KPA-4DRS-9YHC-1NGU';
const String _currentFingerprint =
    'Q0ZJ-55RW-2HNE-K7TS-B8LM-3XQD-7PWA-6CVJ-0RTN-9KEF-2HUY-5SGZ-4MDB';

/// The output of the connected scenarios: a prompt, `ls` and two file names.
const String _output = '~/src \$ ls\r\nnotes.txt  report.md\r\n~/src \$ ';

/// The cells of one matrix: two themes, two locales and two text scales.
const int _cells = 8;

/// One controller per cell: a pane owns the input transform and lays the xterm grid out, so
/// two panes must never share a controller. The two runs of a golden (CI and platform) pump
/// one after the other and reuse the same controllers; tearDownAll disposes them.
final class _Pool {
  _Pool(this.prepare);

  /// Brings a fresh harness into the state of the scenario, with real async (no tester).
  final Future<void> Function(Harness h) prepare;
  final List<Harness> harnesses = [];
  int _next = 0;

  Future<void> fill() async {
    for (var i = 0; i < _cells; i++) {
      final h = await Harness.create();
      await h.open();
      await h.connected();
      await h.host.sendData(utf8.encode(_output));
      for (var j = 0; j < 2000; j++) {
        if (h.controller.screen.visibleText().contains('report.md')) break;
        await Future<void>.delayed(Duration.zero);
      }
      await prepare(h);
      await h.settle();
      harnesses.add(h);
    }
  }

  Harness take() => harnesses[_next++ % _cells];
}

final Map<String, _Pool> _panes = {
  'connected': _Pool((h) async {}),
  'detached': _Pool((h) => h.controller.detach()),
  'ended': _Pool((h) async {
    await h.host.close(4100, 'process_exited');
    await h.until((s) => s.phase == TerminalPhase.ended);
  }),
  'blocked': _Pool((h) async {
    await h.host.close(4104);
    await h.until((s) => s.phase == TerminalPhase.blocked);
  }),
  'extra_keys': _Pool((h) async {}),
};

Widget _pane(String name, Harness h) => Material(
  type: MaterialType.transparency,
  child: SizedBox.fromSize(
    size: _paneSize,
    child: TerminalPane(
      controller: h.controller,
      fontSize: 14,
      maxSessionDuration: _maxSessionDuration,
      now: _now,
      showExtraKeys: name == 'extra_keys',
    ),
  ),
);

Widget _warning() => const Material(
  type: MaterialType.transparency,
  child: SizedBox(
    width: 640,
    child: KeyChangedDialog(
      stored: _storedFingerprint,
      current: _currentFingerprint,
    ),
  ),
);

/// Ctrl latched for one key, Alt held (components.md 12 "Extra keys bar").
Widget _latchedBar() {
  final keys = ExtraKeysController()
    ..tapCtrl()
    ..holdAlt();
  return Material(
    type: MaterialType.transparency,
    child: SizedBox(
      width: _paneSize.width,
      child: ExtraKeysBar(
        keys: keys,
        terminal: () => throw StateError('no key is pressed in a golden'),
      ),
    ),
  );
}

GoldenTestGroup _matrix(String name, Widget Function() builder) =>
    dsGoldenMatrix(
      name: name,
      builder: builder,
      textScales: const [1.0, 2.0],
      locales: const [Locale('de'), Locale('en')],
    );

void main() {
  setUpAll(() async {
    for (final pool in _panes.values) {
      await pool.fill();
    }
  });

  tearDownAll(() {
    for (final pool in _panes.values) {
      for (final h in pool.harnesses) {
        h.controller.dispose();
      }
    }
  });

  for (final MapEntry(key: name, value: pool) in _panes.entries) {
    goldenTest(
      'pane $name in light and dark, text scale 1 and 2, de and en',
      fileName: 'terminal_$name',
      pumpBeforeTest: dsPumpIcons,
      builder: () => _matrix(name, () => _pane(name, pool.take())),
    );
  }

  goldenTest(
    'key change warning in light and dark, text scale 1 and 2, de and en',
    fileName: 'terminal_key_changed',
    pumpBeforeTest: dsPumpIcons,
    builder: () => _matrix('key_changed', _warning),
  );

  goldenTest(
    'extra keys bar with Ctrl latched and Alt held in light and dark, text scale 1 and 2, '
    'de and en',
    fileName: 'terminal_extra_keys_latched',
    pumpBeforeTest: dsPumpIcons,
    builder: () => _matrix('extra_keys_latched', _latchedBar),
  );
}
