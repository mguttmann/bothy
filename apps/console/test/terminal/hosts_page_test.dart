// SPDX-License-Identifier: Apache-2.0

import 'package:admin_ui/admin_ui.dart';
import 'package:console/console.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';

import '../admin/admin_fakes.dart';
import '../support/fakes.dart';
import 'terminal_fakes.dart';

const _openKey = ValueKey('console.terminal.open');
const _paneKey = ValueKey('console.terminal.pane');

/// A signed-in console with "Build host" and the passkey step-up.
TestBrowser _signedIn() {
  final b = TestBrowser();
  adminRoutes(b.server);
  terminalRoutes(b.server);
  b.server.routes['GET /api/v1/console/session'] = (_) =>
      Answer(200, adminSession());
  b.server.routes['POST /api/v1/auth/step-up/challenge'] = (_) => Answer(200, {
    'challenge_id': '0192f4a0-7b1c-747c-835d-13eca158d3c0',
    'challenge': 'Zm9vYmFy',
    'passkey_options': {'challenge': 'Zm9vYmFy', 'rpId': 'chat.example.com'},
    'expires_at': '2026-10-10T09:35:00.000Z',
  });
  b.server.routes['POST /api/v1/auth/step-up/passkey'] = (_) =>
      const Answer(200, stepUpResult);
  return b;
}

Future<ConsoleController> _pump(
  WidgetTester tester,
  TestBrowser b,
  FakeSockets sockets,
  FakePinDatabase pins, {
  required String location,
}) async {
  tester.view.physicalSize = const Size(1600, 1200);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(
    ConsoleApp(
      environment: b.environment,
      initialLocation: Uri.parse(location),
      locale: const Locale('en'),
      terminalPorts: ConsoleTerminalPorts(
        connect: sockets.connect,
        pins: pins,
        openLink: (_) {},
      ),
    ),
  );
  await tester.pumpAndSettle();
  return ConsoleApp.controllerOf(tester.element(find.byType(Navigator).first));
}

void main() {
  group('Hosts and terminal', () {
    test('the hosts page starts at the list of A8 and a host has a path', () {
      expect(
        ConsoleRoute.parse('/hosts')!.admin,
        const AdminLocation(AdminSection.hosts),
      );
      final detail = ConsoleRoute.parse('/hosts/$buildHostId')!;
      expect(detail.page, ConsolePage.hosts);
      expect(
        detail.admin,
        const AdminLocation(AdminSection.hosts, id: buildHostId),
      );
      expect(detail.path, '/hosts/$buildHostId');
    });

    testWidgets('the page shows A8 from admin_ui', (tester) async {
      final b = _signedIn();
      await _pump(
        tester,
        b,
        FakeSockets(),
        FakePinDatabase(),
        location: '/hosts',
      );
      expect(find.byType(HostsList), findsOneWidget);
      expect(find.text('Build host'), findsWidgets);
      // The list has no open action; a host's page has it.
      expect(find.byKey(_openKey), findsNothing);
    });

    testWidgets(
      'a host offers "Open terminal" and opens the pane over the console ports',
      (tester) async {
        final b = _signedIn();
        final sockets = FakeSockets();
        final c = await _pump(
          tester,
          b,
          sockets,
          FakePinDatabase(),
          location: '/hosts/$buildHostId',
        );
        expect(c.path, '/hosts/$buildHostId');
        expect(find.byType(HostDetail), findsOneWidget);
        expect(find.byKey(_openKey), findsOneWidget);
        await tester.tap(find.byKey(_openKey));
        await tester.pumpAndSettle();
        // The open dialog of terminal_ui with the console's pinning statement.
        expect(find.byType(OpenTerminalDialog), findsOneWidget);
        expect(
          find.byKey(const ValueKey('console.terminal.pinningNotice')),
          findsOneWidget,
        );
        await tester.tap(find.text('Confirm with passkey or device key'));
        await tester.pumpAndSettle();
        // The passkey step-up: only the passkey, then the browser asks.
        expect(
          find.byKey(const ValueKey('consoleStepUp.passkey')),
          findsOneWidget,
        );
        expect(
          find.byKey(const ValueKey('consoleStepUp.passwordField')),
          findsNothing,
        );
        await tester.tap(find.byKey(const ValueKey('consoleStepUp.passkey')));
        await tester.pumpAndSettle();
        expect(b.passkeys.calls.single['userVerification'], 'required');
        expect(
          b.server.to('POST', '/api/v1/auth/step-up/passkey'),
          hasLength(1),
        );
        expect(
          b.server.to('POST', '/api/v1/hosts/$buildHostId/terminal/sessions'),
          hasLength(1),
        );
        // The stream goes to the same origin; the pane and its controls are there.
        expect(
          sockets.opened.single.url,
          Uri.parse('wss://chat.example.com$streamPath'),
        );
        expect(find.byKey(_paneKey), findsOneWidget);
        expect(find.byType(TerminalPane), findsOneWidget);
        for (final key in [
          'console.terminal.detach',
          'console.terminal.reattach',
          'console.terminal.end',
          'console.terminal.close',
        ]) {
          expect(find.byKey(ValueKey(key)), findsOneWidget, reason: key);
        }
        // End closes the session on the server; Close returns to the host.
        await tester.tap(find.byKey(const ValueKey('console.terminal.end')));
        await tester.pumpAndSettle();
        expect(
          b.server.to(
            'POST',
            '/api/v1/terminal/sessions/$terminalSessionId/close',
          ),
          hasLength(1),
        );
        await tester.tap(find.byKey(const ValueKey('console.terminal.close')));
        await tester.pumpAndSettle();
        expect(find.byKey(_paneKey), findsNothing);
        expect(find.byType(HostDetail), findsOneWidget);
      },
    );

    testWidgets('logout ends the terminal view and deletes the pins', (
      tester,
    ) async {
      final b = _signedIn();
      b.server.routes['POST /api/v1/auth/logout'] = (_) => const Answer(204);
      final pins = FakePinDatabase();
      final sockets = FakeSockets();
      final c = await _pump(
        tester,
        b,
        sockets,
        pins,
        location: '/hosts/$buildHostId',
      );
      await tester.tap(find.byKey(_openKey));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Confirm with passkey or device key'));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('consoleStepUp.passkey')));
      await tester.pumpAndSettle();
      expect(find.byKey(_paneKey), findsOneWidget);
      await tester.tap(find.text('Log out'));
      await tester.pumpAndSettle();
      expect(pins.deletions, 1);
      expect(c.state, isA<ConsoleLoggedOut>());
      // The view is gone and its stream closed with it.
      expect(find.byKey(_paneKey), findsNothing);
      expect(sockets.opened.single.closedWith, isNotNull);
    });
  });
}
