// SPDX-License-Identifier: Apache-2.0

// Input keeps its order across frames: a long input is split into several `data` frames, and a
// key or a renderer reply sent while those frames are on their way follows the whole input, not
// its first chunk (docs/spec/24-terminal.md 5.4, the frames of one stream in order).

import 'dart:convert';

import 'package:bothy_noise/bothy_noise.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';

import 'support/fakes.dart';

void main() {
  test('a key during a long input follows the whole input', () async {
    final h = await Harness.create();
    await h.open();
    await h.connected();
    await h.frames(1);
    // Three chunks: two full `data` frames and a rest.
    final long = List<int>.generate(
      2 * (maxFrameLength - 1) + 10,
      (i) => 0x61 + i % 26,
    );
    h.controller.input(long);
    h.controller.input(utf8.encode('x'));
    await h.frames(1 + 4);
    expect(h.host.frames.skip(1).whereType<DataFrame>(), hasLength(4));
    expect(h.host.input, [...long, ...utf8.encode('x')]);
    expect(h.controller.state.phase, TerminalPhase.connected);
  });
}
