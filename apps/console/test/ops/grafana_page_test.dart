// SPDX-License-Identifier: Apache-2.0

// The page "Operations (Grafana)" of the console (brief M6-17 Design note 3; 25 9.2 items 5 and
// 6): it appears in the navigation only when getInstance names grafana_embed_url (owners, with
// console.grafana_embed_url set), frames exactly that URL with sandbox
// "allow-scripts allow-same-origin" and referrerpolicy "no-referrer", states that it follows the
// permissions of Grafana and opens the dashboard in a new tab on request.

import 'package:console/console.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/admin_pages_test.dart' show browserWindow, signedIn;
import '../login_test.dart' show pumpConsole;
import '../support/fakes.dart';

const String _url = 'https://grafana.example.com/d/ops/operations?orgId=1';

Map<String, Object?> _instance({String? url}) => {
  'instance_id': '0192f4a0-7b1c-7db7-89b9-2fe4c48dcb3e',
  'branding': {'product_name': 'Team Chat', 'version': 1},
  'locale_default': 'en',
  'spec_version': '0.2.0',
  'features': <Object>[],
  'limits': {
    'message_max_content_bytes': 40000,
    'message_max_attachments': 10,
    'file_max_upload_bytes': 104857600,
    'thread_title_max_chars': 120,
    'voice_max_duration_ms': 300000,
  },
  'grafana_embed_url': ?url,
};

TestBrowser _browser({String? url}) {
  final b = signedIn();
  b.server.routes['GET /api/v1/instance'] = (_) =>
      Answer(200, _instance(url: url));
  return b;
}

const String _label = 'Operations (Grafana)';

void main() {
  testWidgets('the navigation has the page only with grafana_embed_url', (
    tester,
  ) async {
    browserWindow(tester);
    await pumpConsole(tester, _browser(url: _url), location: '/people');
    expect(find.widgetWithText(ListTile, _label), findsOneWidget);
  });

  testWidgets('no page without the member (no owner, or the key empty)', (
    tester,
  ) async {
    browserWindow(tester);
    await pumpConsole(tester, _browser(), location: '/people');
    expect(find.widgetWithText(ListTile, _label), findsNothing);
    expect(find.byType(ConsoleGrafanaFrame), findsNothing);
  });

  testWidgets('the page frames exactly the URL, with the note and the link', (
    tester,
  ) async {
    browserWindow(tester);
    final b = _browser(url: _url);
    final c = await pumpConsole(tester, b, location: '/people');
    await tester.tap(find.widgetWithText(ListTile, _label));
    await tester.pumpAndSettle();
    expect(c.page, ConsolePage.grafana);
    final frame = tester.widget<ConsoleGrafanaFrame>(
      find.byType(ConsoleGrafanaFrame),
    );
    expect(frame.url, Uri.parse(_url));
    expect(
      find.text(
        'This page shows Grafana. It follows the permissions of Grafana, '
        'not those of this instance.',
      ),
      findsOneWidget,
    );
    // The frame fills the content area below the note.
    expect(
      tester.getSize(find.byType(ConsoleGrafanaFrame)).width,
      greaterThan(1000),
    );
    await tester.tap(find.text('Open in a new tab'));
    expect(b.links.opened, [Uri.parse(_url)]);
  });

  testWidgets('the path of the page without the member shows no frame', (
    tester,
  ) async {
    browserWindow(tester);
    await pumpConsole(tester, _browser(), location: '/grafana');
    expect(find.byType(ConsoleGrafanaFrame), findsNothing);
    expect(
      find.text('No Grafana dashboard is set up for this console.'),
      findsOneWidget,
    );
  });

  test('the iframe is sandboxed and sends no referrer (25 9.2 item 5)', () {
    expect(grafanaFrameSandbox, 'allow-scripts allow-same-origin');
    expect(grafanaFrameReferrerPolicy, 'no-referrer');
  });
}
