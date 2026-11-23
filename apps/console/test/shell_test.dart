// SPDX-License-Identifier: Apache-2.0

import 'dart:async';

import 'package:console/console.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import 'admin/admin_fakes.dart';
import 'login_test.dart' show pumpConsole;
import 'support/fakes.dart';

/// A browser whose console is signed in with a step-up.
TestBrowser _signedIn() {
  final b = TestBrowser();
  adminRoutes(b.server);
  b.server.routes['GET /api/v1/console/session'] = (_) =>
      Answer(200, adminSession());
  b.server.routes['GET /api/v1/groups'] = (_) =>
      const Answer(200, {'items': <Object>[]});
  return b;
}

/// A window of [width] by 900 logical pixels.
void _window(WidgetTester tester, double width) {
  tester.view.physicalSize = Size(width, 900);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
}

/// The tab's Page Visibility, changed by the test.
final class _Visibility implements PageVisibility {
  final StreamController<bool> _changes = StreamController.broadcast();

  @override
  bool hidden = false;

  @override
  Stream<bool> get changes => _changes.stream;

  void set(bool value) {
    hidden = value;
    _changes.add(value);
  }
}

void main() {
  testWidgets('a logout without an answer says so and keeps the console', (
    tester,
  ) async {
    // console-terminal-ui-3: the notice of the signed-in state was never shown.
    _window(tester, 1600);
    final b = _signedIn();
    b.server.routes['POST /api/v1/auth/logout'] = (_) =>
        const Answer(503, {'code': 'unavailable', 'status': 503});
    final c = await pumpConsole(tester, b, location: '/people');
    await tester.tap(find.text('Log out'));
    await tester.pumpAndSettle();
    expect(c.state, isA<ConsoleSignedIn>());
    expect(find.byKey(const ValueKey('console.shell.notice')), findsOneWidget);
    expect(
      find.text(
        'The server cannot be reached. Check the connection and try again.',
      ),
      findsOneWidget,
    );
  });

  testWidgets('the browser tab names the page', (tester) async {
    // a11y-6, WCAG 2.4.2.
    _window(tester, 1600);
    final c = await pumpConsole(tester, _signedIn(), location: '/people');
    Iterable<String> titles() =>
        tester.widgetList<Title>(find.byType(Title)).map((t) => t.title);
    expect(titles(), contains('People - Team Chat console'));
    c.open(ConsolePage.audit);
    await tester.pumpAndSettle();
    expect(titles(), contains('Audit - Team Chat console'));
  });

  testWidgets('a page chosen with the keyboard keeps the focus on its entry', (
    tester,
  ) async {
    // a11y-6, WCAG 2.4.3: the shell stays when the page changes.
    _window(tester, 1600);
    final c = await pumpConsole(tester, _signedIn(), location: '/people');
    final entry = find.widgetWithText(ListTile, 'Audit');
    final node = Focus.of(tester.element(find.text('Audit').first));
    node.requestFocus();
    await tester.pump();
    await tester.sendKeyEvent(LogicalKeyboardKey.enter);
    await tester.pumpAndSettle();
    expect(c.page, ConsolePage.audit);
    final focused = FocusManager.instance.primaryFocus!.context!;
    expect(
      find.descendant(of: entry, matching: find.byWidget(focused.widget)),
      findsOneWidget,
    );
  });

  testWidgets('a narrow window folds the navigation into a drawer', (
    tester,
  ) async {
    // a11y-16, WCAG 1.4.10: 320 CSS pixels, a 1280 pixel window at 400 percent zoom.
    // The page of the console's own Grafana entry holds a short note, so an overflow could
    // only come from the shell; the screens of admin_ui answer for their own width.
    _window(tester, 320);
    final c = await pumpConsole(tester, _signedIn(), location: '/grafana');
    expect(c.page, ConsolePage.grafana);
    expect(tester.takeException(), isNull);
    expect(find.text('Audit'), findsNothing);
    await tester.tap(find.byKey(const ValueKey('console.shell.menu')));
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
    await tester.tap(find.text('Audit'));
    await tester.pumpAndSettle();
    expect(c.page, ConsolePage.audit);
    expect(find.byType(Drawer), findsNothing);
  });

  testWidgets('a profile read that failed is tried again', (tester) async {
    // console-terminal-ui-15: one failed getMe left the time zone at UTC for the whole session.
    _window(tester, 1600);
    final b = _signedIn();
    var reads = 0;
    b.server.routes['GET /api/v1/me'] = (_) {
      reads++;
      if (reads == 1) {
        return const Answer(503, {'code': 'unavailable', 'status': 503});
      }
      return const Answer(200, {
        'principal': {
          'id': alexId,
          'kind': 'user',
          'handle': 'alex',
          'display_name': 'Alex',
        },
        'is_owner': true,
        'time_zone': 'Europe/Berlin',
        'permission_version': 1,
      });
    };
    final c = await pumpConsole(tester, b, location: '/people');
    final admin = ConsoleApp.adminOf(
      tester.element(find.byType(Navigator).first),
    );
    expect(admin.timeZone, 'UTC');
    c.open(ConsolePage.audit);
    await tester.pumpAndSettle();
    expect(reads, 2);
    expect(admin.timeZone, 'Europe/Berlin');
  });

  testWidgets('a tab that becomes visible reads its session again', (
    tester,
  ) async {
    // console-terminal-ui-15: another person's login in another tab changed the cookie.
    _window(tester, 1600);
    final b = _signedIn();
    final visibility = _Visibility();
    var reads = 0;
    final session = b.server.routes['GET /api/v1/console/session']!;
    b.server.routes['GET /api/v1/console/session'] = (r) {
      reads++;
      return session(r);
    };
    final env = b.environment;
    await tester.pumpWidget(
      ConsoleApp(
        environment: ConsoleEnvironment(
          origin: env.origin,
          flowStore: env.flowStore,
          navigation: env.navigation,
          passkeys: env.passkeys,
          visibility: visibility,
          adapter: b.server,
        ),
        initialLocation: Uri.parse('/people'),
        locale: const Locale('en'),
      ),
    );
    await tester.pumpAndSettle();
    final before = reads;
    visibility.set(true);
    await tester.pumpAndSettle();
    expect(reads, before);
    visibility.set(false);
    await tester.pumpAndSettle();
    expect(reads, before + 1);
  });
}
