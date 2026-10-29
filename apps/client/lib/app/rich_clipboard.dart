// SPDX-License-Identifier: Apache-2.0

/// The clipboard of the block copies of messages with rich text (docs/spec/01-product.md 6.12
/// item 4, R-PR-25; brief M4-48).
///
/// Flutter's [Clipboard] holds plain text only. A quote block also carries an HTML form, so a
/// paste into a rich editor keeps its formatting; the runners of macOS, Windows, iOS and
/// Android put both forms on the system clipboard through the method channel `bothy/clipboard`
/// (method `write`, arguments `text`, `html` and on Windows `cf_html`).
library;

import 'dart:convert';

import 'package:design_system/design_system.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

/// The [DsClipboard] of the app: plain text and HTML through the runner where a copy has an
/// HTML form, plain text through [Clipboard] otherwise.
///
/// A missing or failing channel (a build without the runner code, a clipboard another app
/// holds open) never loses the copy: the plain text goes through [Clipboard] then.
///
/// ```dart
/// DsClipboardScope(clipboard: const PlatformRichClipboard(), child: app);
/// ```
final class PlatformRichClipboard implements DsClipboard {
  /// Creates the clipboard.
  const PlatformRichClipboard();

  /// The channel of the runners.
  static const MethodChannel channel = MethodChannel('bothy/clipboard');

  @override
  Future<void> write(DsCopyData data) async {
    final html = data.html;
    if (html == null) {
      await Clipboard.setData(ClipboardData(text: data.text));
      return;
    }
    try {
      await channel.invokeMethod<void>('write', {
        'text': data.text,
        // Without the charset some macOS editors read the bytes as Latin-1.
        'html': '<meta charset="utf-8">$html',
        if (defaultTargetPlatform == TargetPlatform.windows)
          'cf_html': windowsClipboardHtml(html),
      });
    } on MissingPluginException {
      await Clipboard.setData(ClipboardData(text: data.text));
    } on PlatformException {
      await Clipboard.setData(ClipboardData(text: data.text));
    }
  }
}

/// The Windows clipboard format "HTML Format" (CF_HTML) of [fragment]: the header with the
/// byte offsets of the document and the fragment in its UTF-8 encoding, then the document.
///
/// The runner stores the UTF-8 bytes of this string as they are.
///
/// Example: `windowsClipboardHtml('<p>x</p>')` starts with `Version:0.9\r\nStartHTML:`.
String windowsClipboardHtml(String fragment) {
  const head = '<html><body>\r\n<!--StartFragment-->';
  const tail = '<!--EndFragment-->\r\n</body>\r\n</html>';
  String header(
    int startHtml,
    int endHtml,
    int startFragment,
    int endFragment,
  ) {
    String n(int v) => v.toString().padLeft(10, '0');
    return 'Version:0.9\r\n'
        'StartHTML:${n(startHtml)}\r\n'
        'EndHTML:${n(endHtml)}\r\n'
        'StartFragment:${n(startFragment)}\r\n'
        'EndFragment:${n(endFragment)}\r\n';
  }

  // The header has a fixed length, because every offset has ten digits.
  final start = utf8.encode(header(0, 0, 0, 0)).length;
  final fragmentStart = start + utf8.encode(head).length;
  final fragmentEnd = fragmentStart + utf8.encode(fragment).length;
  final end = fragmentEnd + utf8.encode(tail).length;
  return '${header(start, end, fragmentStart, fragmentEnd)}$head$fragment$tail';
}
