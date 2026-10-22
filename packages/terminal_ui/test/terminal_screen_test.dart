// SPDX-License-Identifier: Apache-2.0

// The screen model: dropped link records release their anchors (red team M4-35: xterm walks
// every anchor of a line on each edit, so a program printing links forever froze the view),
// and link targets must be printable ASCII to open.

import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';

void main() {
  test('link records beyond the cap release their anchors', () {
    final screen = TerminalScreen(scrollbackLines: 100, onInput: (_) {});
    final chunk = StringBuffer();
    for (var i = 0; i < maxLinkRecords * 3; i++) {
      chunk.write('\x1b]8;;https://example.com/$i\x07x\x1b]8;;\x07');
    }
    screen.write(utf8.encode(chunk.toString()));
    var anchors = 0;
    for (final line in screen.terminal.buffer.lines.toList()) {
      anchors += line.anchors.length;
    }
    expect(screen.links, hasLength(maxLinkRecords));
    expect(anchors, lessThanOrEqualTo(2 * maxLinkRecords));
  });

  test('only printable ASCII https and http targets open', () {
    expect(isOpenableLink('https://example.com/a'), isTrue);
    expect(isOpenableLink('https://example.com @evil.example/'), isFalse);
    expect(isOpenableLink('https://exa mple.com/'), isFalse);
    expect(isOpenableLink('ftp://example.com/'), isFalse);
  });
}
