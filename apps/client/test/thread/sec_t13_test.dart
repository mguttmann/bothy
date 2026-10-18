// SPDX-License-Identifier: Apache-2.0

// SEC-T13 in the thread view (brief M4-20 Tests; SEC-OUT-1, SEC-OUT-2): raw HTML, a
// `javascript:` link, an image URL in Markdown and a bare URL render as text or inert links,
// no network request is attempted and there is no link preview.

import 'dart:io';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/column_host.dart';
import '../support/thread_fixtures.dart';
import '../support/thread_host.dart';

/// Fails on any HTTP request of the app.
class _NoNetwork extends HttpOverrides {
  int requests = 0;

  @override
  HttpClient createHttpClient(SecurityContext? context) {
    requests++;
    throw StateError('network request from the thread view');
  }
}

/// The inputs of SEC-T13 (named in the report of M4-20).
const List<String> sect13Inputs = [
  '<script>alert(1)</script>',
  '<img src=https://example.com/track.png onerror=alert(1)>',
  '[click](javascript:alert(1))',
  '![logo](https://example.com/logo.png)',
  'see https://example.com/page for details',
];

void main() {
  testWidgets('HTML, javascript links, image URLs and bare URLs load and run '
      'nothing', (tester) async {
    final net = _NoNetwork();
    await HttpOverrides.runZoned(() async {
      final host = await ColumnHost.open(tester);
      await host.run(
        tester,
        () => seedThread(
          host.db,
          messages: [
            for (var i = 0; i < sect13Inputs.length; i++)
              msg(i + 1, text: sect13Inputs[i], minutes: i * 30),
          ],
        ),
      );
      await pumpThread(tester, host, FakeThreadReads(host.db));

      final text = StringBuffer();
      for (final r in tester.widgetList<RichText>(find.byType(RichText))) {
        text.write(r.text.toPlainText());
      }
      final all = text.toString();
      expect(all, contains('<script>alert(1)</script>'));
      expect(all, contains('<img src='));
      expect(all, contains('click'), reason: 'the javascript link as text');
      expect(find.byType(Image), findsNothing);
      // Only the bare https URL is an active link; javascript: is not.
      final links = tester.widgetList<DsMarkdownLink>(
        find.byType(DsMarkdownLink),
      );
      for (final link in links) {
        expect(link.uri.scheme, isNot('javascript'));
      }
      expect(links.where((l) => l.uri.scheme == 'https'), hasLength(1));
      // No preview card: nothing but the message content is drawn per message.
      expect(find.byKey(const ValueKey('linkPreview')), findsNothing);
      await host.dispose(tester);
    }, createHttpClient: net.createHttpClient);
    expect(net.requests, 0);
  });
}
