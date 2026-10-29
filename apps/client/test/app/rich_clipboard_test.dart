// SPDX-License-Identifier: Apache-2.0

// The clipboard of the block copies (01 6.12 item 4, R-PR-25; brief M4-48): a quote goes to the
// runner's channel `bothy/clipboard` as plain text and HTML (on Windows also as the clipboard
// format "HTML Format"), a code block as plain text only; without the channel the plain text
// still arrives.

import 'dart:convert';

import 'package:client/app/rich_clipboard.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  final messenger =
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
  late List<MethodCall> channel;
  late List<String> plain;

  setUp(() {
    channel = [];
    plain = [];
    messenger.setMockMethodCallHandler(PlatformRichClipboard.channel, (
      call,
    ) async {
      channel.add(call);
      return null;
    });
    messenger.setMockMethodCallHandler(SystemChannels.platform, (call) async {
      if (call.method == 'Clipboard.setData') {
        plain.add((call.arguments as Map)['text'] as String);
      }
      return null;
    });
  });

  tearDown(() {
    messenger.setMockMethodCallHandler(PlatformRichClipboard.channel, null);
    messenger.setMockMethodCallHandler(SystemChannels.platform, null);
    debugDefaultTargetPlatformOverride = null;
  });

  test('a copy with HTML goes to the channel with both forms', () async {
    debugDefaultTargetPlatformOverride = TargetPlatform.macOS;
    await const PlatformRichClipboard().write(
      const DsCopyData('a b', html: '<p>a <strong>b</strong></p>'),
    );
    expect(plain, isEmpty);
    expect(channel.single.method, 'write');
    final args = channel.single.arguments as Map;
    expect(args['text'], 'a b');
    expect(args['html'], '<meta charset="utf-8"><p>a <strong>b</strong></p>');
    expect(args.containsKey('cf_html'), isFalse);
  });

  test('on Windows the channel also gets the HTML clipboard format', () async {
    debugDefaultTargetPlatformOverride = TargetPlatform.windows;
    await const PlatformRichClipboard().write(
      const DsCopyData('x', html: '<p>x</p>'),
    );
    final args = channel.single.arguments as Map;
    expect(args['cf_html'], windowsClipboardHtml('<p>x</p>'));
  });

  test(
    'a copy without HTML is plain text through the system clipboard',
    () async {
      const text = '\tif x:\n\t\treturn "\u00fc \u{1F600}"';
      await const PlatformRichClipboard().write(const DsCopyData(text));
      expect(channel, isEmpty);
      expect(plain, [text]);
    },
  );

  test('without the channel the plain text still arrives', () async {
    messenger.setMockMethodCallHandler(PlatformRichClipboard.channel, null);
    await const PlatformRichClipboard().write(
      const DsCopyData('a b', html: '<p>a b</p>'),
    );
    expect(plain, ['a b']);
  });

  test('a failing channel falls back to the plain text', () async {
    messenger.setMockMethodCallHandler(PlatformRichClipboard.channel, (
      call,
    ) async {
      throw PlatformException(code: 'clipboard', message: 'busy');
    });
    await const PlatformRichClipboard().write(
      const DsCopyData('a b', html: '<p>a b</p>'),
    );
    expect(plain, ['a b']);
  });

  group('windowsClipboardHtml', () {
    String slice(List<int> bytes, int from, int to) =>
        utf8.decode(bytes.sublist(from, to));

    int offset(String doc, String name) =>
        int.parse(RegExp('$name:(\\d{10})').firstMatch(doc)![1]!);

    test('the offsets point at the document and the fragment in bytes', () {
      const fragment = '<p>\u00fc \u{1F600} \u05D0 &amp;</p>';
      final doc = windowsClipboardHtml(fragment);
      final bytes = utf8.encode(doc);
      expect(doc, startsWith('Version:0.9\r\n'));
      expect(
        slice(bytes, offset(doc, 'StartFragment'), offset(doc, 'EndFragment')),
        fragment,
      );
      expect(
        slice(bytes, offset(doc, 'StartHTML'), offset(doc, 'EndHTML')),
        startsWith('<html>'),
      );
      expect(offset(doc, 'EndHTML'), bytes.length);
      expect(
        slice(bytes, offset(doc, 'StartHTML'), offset(doc, 'EndHTML')),
        endsWith('</html>'),
      );
    });
  });
}
