// SPDX-License-Identifier: Apache-2.0

// Red team round 3: the marks painter paints only the visible part of the selection, as
// xterm does (render.dart effectFirstLine/effectLastLine).

import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/src/render/terminal_marks.dart';
import 'package:terminal_ui/terminal_ui.dart';
import 'package:xterm/xterm.dart' as xterm;

import 'support/fakes.dart';
import 'support/widgets.dart';

void main() {
  testWidgets(
    'a selection across the scrollback paints no more cells than are visible',
    (tester) async {
      final h = (await tester.runAsync(Harness.create))!;
      await pumpApp(
        tester,
        TerminalScreenView(controller: h.controller, fontSize: 14),
      );
      await tester.pump();

      final t = h.controller.screen.terminal;
      final text = List.generate(
        300,
        (i) => 'line $i '.padRight(t.viewWidth - 1, 'x'),
      ).join('\r\n');
      h.controller.screen.write(Uint8List.fromList(utf8.encode(text)));
      await tester.pump();
      expect(t.buffer.height, greaterThan(t.viewHeight * 3));

      final view = tester.widget<xterm.TerminalView>(
        find.byType(xterm.TerminalView),
      );
      view.controller!.setSelection(
        t.buffer.createAnchor(0, 0),
        t.buffer.createAnchor(t.viewWidth, t.buffer.height - 1),
      );
      await tester.pump();

      final marks = find.byWidgetPredicate(
        (w) => w is CustomPaint && w.painter is TerminalMarksPainter,
      );
      expect(marks, findsOneWidget);
      final render = tester.renderObject<RenderCustomPaint>(marks);

      var rects = 0;
      var paragraphs = 0;
      expect(
        render,
        paints..everything((Symbol method, List<dynamic> args) {
          if (method == #drawRect) rects++;
          if (method == #drawParagraph) paragraphs++;
          return true;
        }),
      );

      // xterm paints the rows the view shows, partial rows at both edges included.
      final visible = t.viewWidth * (t.viewHeight + 2);
      expect(rects, lessThanOrEqualTo(visible + 4));
      expect(paragraphs, lessThanOrEqualTo(visible));
    },
  );
}
