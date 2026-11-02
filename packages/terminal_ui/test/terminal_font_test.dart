// SPDX-License-Identifier: Apache-2.0

// The terminal font (FIX-terminal-font): the bundled JetBrains Mono files match their manifest,
// the grid measures its cell from that font (one glyph advance, no extra spacing), and the
// columns the PTY receives follow from the view width and that advance.

import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:bothy_noise/bothy_noise.dart' show ResizeFrame;
import 'package:crypto/crypto.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';
import 'package:xterm/xterm.dart' as xterm;

import 'support/fakes.dart';
import 'support/widgets.dart';

// flutter test runs with the package folder as the working directory.
final _fonts = Directory('fonts');
const _manifestName = 'MANIFEST.json';

Map<String, Map<String, dynamic>> _files() =>
    ((jsonDecode(File('${_fonts.path}/$_manifestName').readAsStringSync())
                as Map<String, dynamic>)['files']
            as Map<String, dynamic>)
        .map((k, v) => MapEntry(k, v as Map<String, dynamic>));

/// The offset of the table [tag] in the TrueType file [font].
int _table(ByteData font, String tag) {
  final count = font.getUint16(4);
  for (var i = 0; i < count; i++) {
    final record = 12 + 16 * i;
    final name = String.fromCharCodes([
      for (var j = 0; j < 4; j++) font.getUint8(record + j),
    ]);
    if (name == tag) return font.getUint32(record + 8);
  }
  throw StateError('no $tag table');
}

/// The advance of one cell of the bundled regular face at [fontSize], read from the font file
/// itself: `hhea.advanceWidthMax` over `head.unitsPerEm` (every glyph of a monospace font has
/// this advance).
double _advance(double fontSize) {
  final bytes = File('${_fonts.path}/JetBrainsMono-Regular.ttf')
      .readAsBytesSync();
  final font = ByteData.sublistView(bytes);
  final unitsPerEm = font.getUint16(_table(font, 'head') + 18);
  final advanceMax = font.getUint16(_table(font, 'hhea') + 10);
  return advanceMax * fontSize / unitsPerEm;
}

void main() {
  test('every bundled terminal font file matches the manifest', () {
    final files = _files();
    final present = {
      for (final f in _fonts.listSync().whereType<File>())
        if (!f.path.endsWith(_manifestName)) f.uri.pathSegments.last,
    };
    expect(present, files.keys.toSet());
    for (final MapEntry(key: name, value: entry) in files.entries) {
      final digest = sha256
          .convert(File('${_fonts.path}/$name').readAsBytesSync())
          .toString();
      expect(digest, entry['sha256'], reason: name);
      expect(entry['license'], 'OFL-1.1', reason: name);
      if (name.endsWith('.ttf')) {
        expect(present, contains(entry['license_file']), reason: name);
      }
    }
    expect(
      files.keys,
      containsAll(<String>[
        'JetBrainsMono-Regular.ttf',
        'JetBrainsMono-Bold.ttf',
        'JetBrainsMono-Italic.ttf',
        'JetBrainsMono-BoldItalic.ttf',
        'JetBrainsMono.OFL.txt',
      ]),
    );
  });

  test('the terminal style names the bundled family and no other first', () {
    final style = terminalTextStyle(14);
    expect(style.fontFamily, terminalFontFamily);
    expect(terminalFontFamily, 'packages/terminal_ui/JetBrains Mono');
    expect(style.fontSize, 14);
    expect(style.toTextStyle().letterSpacing, isNull);
  });

  for (final fontSize in [12.0, 14.0, 17.0]) {
    testWidgets(
      'at font size $fontSize the cell is one glyph advance of the bundled font and the PTY '
      'gets the columns of the view width',
      (tester) async {
        const width = 701.0;
        final h = (await tester.runAsync(() async {
          final h = await Harness.create();
          await h.open();
          await h.connected();
          return h;
        }))!;
        await pumpApp(
          tester,
          Align(
            alignment: Alignment.topLeft,
            child: SizedBox(
              width: width,
              height: 400,
              child: TerminalScreenView(
                controller: h.controller,
                fontSize: fontSize,
              ),
            ),
          ),
        );
        await tester.pump();

        final advance = _advance(fontSize);
        final view = tester.state<xterm.TerminalViewState>(
          find.byType(xterm.TerminalView),
        );
        expect(view.renderTerminal.cellSize.width, closeTo(advance, 0.01));

        final cols = width ~/ advance;
        expect(h.controller.screen.terminal.viewWidth, cols);

        await tester.runAsync(() async {
          h.tick();
          await h.settle();
        });
        final resize = h.host.frames.whereType<ResizeFrame>().last;
        expect(resize.cols, cols);
        h.controller.dispose();
      },
    );
  }
}
