// SPDX-License-Identifier: Apache-2.0

// The golden of a long single-line code block (01 6.14, R-PR-28; brief M4-49): the live case of
// one long sentence sent as a fenced block "to copy". By default it wraps, so the whole sentence
// is visible; after the toggle "Wrap lines" it stays one line with the horizontal scrollbar.
// Light and dark, on the desktop (comfortable) and on the phone (touch), in German.

import 'package:alchemist/alchemist.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import 'sheets.dart';

const _sentence =
    'Please copy this long sentence into the ticket, so that everyone involved '
    'knows exactly which steps are still open tonight and who takes them over '
    'before the release on Monday.';

Widget _codeSheet() => Sheet([
  DsMarkdown('```\n$_sentence\n```', onOpenLink: (_) {}),
  DsCodeBlockScope(
    wrap: false,
    child: DsMarkdown('```text\n$_sentence\n```', onOpenLink: (_) {}),
  ),
]);

void main() {
  goldenTest(
    'a long single-line code block wraps by default and scrolls without wrap',
    fileName: 'code_block_long_line',
    pumpBeforeTest: dsPumpIcons,
    builder: () => dsGoldenMatrix(
      name: 'code block long line',
      builder: _codeSheet,
      densities: const [DsDensityMode.comfortable, DsDensityMode.touch],
      locales: const [Locale('de')],
    ),
  );
}
