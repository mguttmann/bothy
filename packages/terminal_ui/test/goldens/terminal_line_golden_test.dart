// SPDX-License-Identifier: Apache-2.0

// Golden of one terminal line in the bundled font (FIX-terminal-font): the characters sit in
// cells of one glyph advance, without the wide spacing of a grid measured from another font;
// box drawing and powerline symbols come from the same font. Light and dark.

import 'dart:convert';
import 'dart:typed_data';

import 'package:alchemist/alchemist.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';

import '../support/fakes.dart';

/// A line as the owner saw it, a box and a powerline prompt: text, `U+2500` block and
/// `U+E0B0`.
const String _lines =
    'gates.passed 12/12  make check\r\n'
    '┌──┬──┐ │ok│ █▓▒░\r\n'
    ' ~/src  main  ';

const Size _size = Size(360, 72);

/// One harness per theme: a view owns the input transform of its controller.
final List<Harness> _harnesses = [];
int _next = 0;

Widget _line() => SizedBox.fromSize(
  size: _size,
  child: TerminalScreenView(
    controller: _harnesses[_next++ % _harnesses.length].controller,
    fontSize: 14,
  ),
);

void main() {
  setUpAll(() async {
    for (var i = 0; i < DsThemeMode.values.length; i++) {
      final h = await Harness.create();
      h.controller.screen.write(Uint8List.fromList(utf8.encode(_lines)));
      _harnesses.add(h);
    }
  });

  tearDownAll(() {
    for (final h in _harnesses) {
      h.controller.dispose();
    }
  });

  goldenTest(
    'a terminal line in the bundled monospace font, light and dark',
    fileName: 'terminal_line',
    builder: () => dsGoldenMatrix(name: 'terminal line', builder: _line),
  );
}
