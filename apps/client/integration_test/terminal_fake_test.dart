// SPDX-License-Identifier: Apache-2.0

// The terminal of the desktop app against the fake server with an in-process host (brief
// M4-36 Tests): opening from the group "Hosts" with the step-up, detaching, reattaching with
// the scrollback replayed, an attach from another device (`4103` here), a revoked grant
// (`4100` with `access_revoked`), a changed host key with the warning (TM-T18 through the app),
// and a canary of terminal output that reaches neither the local database nor the logs, the
// requests of the app (client metrics included) or the gateway.

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:api_client/api_client.dart'
    show ReasonBody, TerminalSessionAttach;
import 'package:client/terminal/host_terminal_page.dart' show scrollbackMatches;
import 'package:client/terminal/providers.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:l10n/l10n.dart';
import 'package:sync_engine/testing.dart';
import 'package:terminal_ui/terminal_ui.dart';
import 'package:terminal_ui/testing.dart' as ui;

import 'support/terminal_host.dart';
import 'support/test_app.dart';

final AppLocalizations en = lookupAppLocalizations(const Locale('en'));

/// The app signed in as Alex with the host "Build host" behind [bridge].
Future<(TestApp, FakeHost, HostBridge)> _signedIn(WidgetTester tester) async {
  final app = await TestApp.start(tester);
  final world = app.fake.server.world;
  // The in-process host and the fake's host share the ID: it is in the Noise prologue.
  final host = await ui.FakeHost.create(hostId: world.nextId());
  final bridge = HostBridge(host);
  world.terminal.hostLink = bridge;
  final h = world.addTerminalHost(
    id: host.hostId,
    name: 'Build host',
    noisePublicKey: host.noisePublicKey,
    sessionUsers: {app.fake.seed.alex.id: 'term-alex'},
    grantees: [app.fake.seed.alex.id],
  );
  await app.connect();
  await app.loginWithPassword();
  await app.waitFor(find.byKey(const ValueKey('column.sidebar')));
  await app.waitLive();
  return (app, h, bridge);
}

/// Pumps until [condition] holds, at most [patience].
Future<void> _until(
  WidgetTester tester,
  bool Function() condition, {
  Duration patience = const Duration(seconds: 20),
}) async {
  final end = DateTime.now().add(patience);
  while (!condition()) {
    if (DateTime.now().isAfter(end)) fail('condition never held');
    await tester.pump(const Duration(milliseconds: 20));
  }
}

/// Opens a shell on the host from the sidebar: "New session", the dialog, the step-up.
Future<TerminalTabs> _openShell(TestApp app, String hostId) async {
  final tester = app.tester;
  await app.waitFor(find.byKey(ValueKey('hosts.row.$hostId')));
  await tester.tap(find.byKey(ValueKey('hosts.row.$hostId')));
  await app.waitFor(find.byKey(const ValueKey('terminal.new')));
  await tester.tap(find.byKey(const ValueKey('terminal.new')));
  await tester.pumpAndSettle();
  await tester.tap(find.byKey(const ValueKey('terminal.new.shell')).last);
  await tester.pumpAndSettle();
  await tester.tap(find.text(en.terminalOpenConfirm));
  await app.waitFor(find.byKey(const ValueKey('stepUp.dialog')));
  await tester.tap(find.byKey(const ValueKey('stepUp.continue')));
  final tabs = app.container.read(terminalTabsProvider(hostId))!;
  await _until(
    tester,
    () => tabs.current?.state.phase == TerminalPhase.connected,
  );
  return tabs;
}

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets(
    'open, detach, reattach with the scrollback, another device, revocation',
    (tester) async {
      final (app, h, bridge) = await _signedIn(tester);
      try {
        final server = app.fake.server;
        final tabs = await _openShell(app, h.id);
        final tab = tabs.current!;
        await _until(tester, () => bridge.host.handshakeDone);
        await bridge.host.sendData(utf8.encode('first line\r\n'));
        await _until(tester, () => tab.screen.visibleText().contains('first'));
        expect(server.requestsOf('openTerminalSession').last.status, 201);
        expect(
          server.requestsOf('createStepUpChallenge'),
          hasLength(1),
          reason: 'one step-up for the opening',
        );

        // Detach keeps the session; Reattach within the quick window needs no step-up and
        // the replay replaces the screen.
        await tester.tap(find.text(en.terminalDetach));
        await _until(tester, () => tab.state.phase == TerminalPhase.detached);
        final session = server.world.terminal.sessions[tab.state.sessionId]!;
        await _until(tester, () => session.state == 'detached');
        await tester.tap(find.text(en.terminalReattach));
        await _until(tester, () => tab.state.phase == TerminalPhase.connected);
        await _until(tester, () => bridge.host.handshakeDone);
        await bridge.host.sendReplay(utf8.encode('replayed line\r\n'));
        await _until(
          tester,
          () => tab.screen.visibleText().contains('replayed line'),
        );
        expect(tab.screen.visibleText(), isNot(contains('first line')));
        expect(server.requestsOf('createStepUpChallenge'), hasLength(1));

        // Another device of Alex attaches: this stream ends with 4103.
        final other = await app.otherDevice();
        await other.auth.api.getTerminalApi().attachTerminalSession(
          sessionId: session.id,
          terminalSessionAttach: TerminalSessionAttach(
            (b) => b
              ..cols = 80
              ..rows = 24
              ..clientEphemeralKeyHash = 'A' * 43,
          ),
        );
        await _until(
          tester,
          () => tab.state.detachCause == DetachCause.replaced,
        );
        expect(server.world.terminal.closes.last.$2, 4103);
        await tester.pump();
        expect(find.textContaining(en.terminalStateDetachedReplaced), findsOne);

        // Reattach here (another device: a fresh step-up), then the grant is revoked: 4100
        // with access_revoked blocks the tab.
        await tester.tap(find.text(en.terminalReattach));
        await app.waitFor(find.byKey(const ValueKey('stepUp.dialog')));
        await tester.tap(find.byKey(const ValueKey('stepUp.continue')));
        await _until(tester, () => tab.state.phase == TerminalPhase.connected);
        final grant = server.world.terminalGrantOf(
          app.fake.seed.alex.id,
          h.id,
        )!;
        await other.auth.api.getAccessApi().revokeGrant(
          grantId: grant,
          reasonBody: ReasonBody((b) => b.reason = 'Leaves the project'),
        );
        await _until(tester, () => tab.state.phase == TerminalPhase.blocked);
        expect(tab.state.blockCause, BlockCause.revoked);
        expect(tab.state.closeReason, 'access_revoked');
        expect(server.world.terminal.closes.last, (
          session.id,
          4100,
          'access_revoked',
        ));
        expect(server.violations, isEmpty);
        await other.auth.dispose();
      } finally {
        await app.stop();
      }
    },
  );

  testWidgets(
    'a changed host key shows the warning and opens only after it (TM-T18)',
    (tester) async {
      final (app, h, bridge) = await _signedIn(tester);
      try {
        final server = app.fake.server;
        final tabs = await _openShell(app, h.id);
        final first = bridge.host.noisePublicKey;
        final pins = app.state.services!.database.hostKeyPinsDao;
        expect((await pins.all()).single.noisePublicKey, first);
        await tabs.current!.end();

        // The host is paired again: a new key behind the same ID.
        final next = await ui.FakeHost.create(
          hostId: h.id,
          seed: List<int>.generate(32, (i) => 200 - i),
        );
        bridge.host = next;
        h
          ..noisePublicKey = next.noisePublicKey
          ..fingerprint =
              'Q0ZJ-55RW-2HNE-K7TS-0000-0000-0000-0000-0000-0000-0000-0000-0000'
          ..keyGeneration += 1;
        server.world.hostUpdated(h, keyChanged: true);

        await tester.tap(find.byKey(const ValueKey('terminal.new')));
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const ValueKey('terminal.new.shell')).last);
        await app.waitFor(find.text(en.terminalKeyChangedTitle));
        await tester.tap(find.text(en.terminalKeyChangedAccept));
        await tester.pumpAndSettle();
        await tester.tap(find.text(en.terminalOpenConfirm));
        await app.waitFor(find.byKey(const ValueKey('stepUp.dialog')));
        await tester.tap(find.byKey(const ValueKey('stepUp.continue')));
        await _until(
          tester,
          () => tabs.current?.state.phase == TerminalPhase.connected,
        );
        final open = server.requestsOf('openTerminalSession').last;
        expect((open.body! as Map)['pin_changed'], isTrue);
        final pin = (await pins.all()).single;
        expect(pin.noisePublicKey, next.noisePublicKey);
        expect(pin.keyGeneration, h.keyGeneration);
        await tabs.current!.detach();
      } finally {
        await app.stop();
      }
    },
  );

  testWidgets(
    'terminal output reaches neither the database, the logs nor any request',
    (tester) async {
      const canary = 'CANARY-M436-7f3a91c2';
      final logs = <String>[];
      final previous = debugPrint;
      debugPrint = (message, {wrapWidth}) => logs.add(message ?? '');
      addTearDown(() => debugPrint = previous);
      await runZoned(
        () async {
          final (app, h, bridge) = await _signedIn(tester);
          try {
            final server = app.fake.server;
            final tabs = await _openShell(app, h.id);
            final tab = tabs.current!;
            await _until(tester, () => bridge.host.handshakeDone);
            // A plain line, a title (OSC 0), a link (OSC 8) and a clipboard write (OSC 52)
            // carry the canary.
            final b64 = base64.encode(utf8.encode(canary));
            await bridge.host.sendData(
              utf8.encode(
                '$canary plain\r\n'
                '\x1b]0;$canary title\x07'
                '\x1b]8;;https://example.com/$canary\x07link\x1b]8;;\x07\r\n'
                '\x1b]52;c;$b64\x07',
              ),
            );
            await _until(
              tester,
              () => tab.screen.visibleText().contains('$canary plain'),
            );
            // The search in the scrollback finds it in memory only.
            await tester.tap(find.byType(TerminalScreenView));
            await tester.pump();
            expect(scrollbackMatches(tab, canary), isNotEmpty);
            // Detach waits until the stream is closed, so nothing of it is in flight below.
            await tab.detach();

            final db = app.state.services!.database;
            final tables = await db
                .customSelect(
                  "SELECT name FROM sqlite_master WHERE type = 'table'",
                )
                .get();
            for (final t in tables) {
              final name = t.read<String>('name');
              for (final row
                  in await db.customSelect('SELECT * FROM "$name"').get()) {
                for (final value in row.data.values) {
                  expect('$value', isNot(contains(canary)), reason: name);
                }
              }
            }
            final file = File('${app.fake.instanceFolder.path}/local.db');
            expect(
              latin1.decode(await file.readAsBytes(), allowInvalid: true),
              isNot(contains(canary)),
            );
            for (final r in server.requests) {
              expect(
                jsonEncode(r.body),
                isNot(contains(canary)),
                reason: r.operationId,
              );
              expect(r.path, isNot(contains(canary)));
            }
            for (final e in server.world.log) {
              expect(jsonEncode(e.envelope), isNot(contains(canary)));
            }
            // The collector of the client metrics (M4-31): what it still holds; what it sent
            // is among the requests above.
            final metrics = app.fake.ports.measurements.metrics.take();
            expect(
              jsonEncode([for (final m in metrics) m.toJson()]),
              isNot(contains(canary)),
            );
          } finally {
            await app.stop();
          }
        },
        zoneSpecification: ZoneSpecification(
          print: (self, parent, zone, line) => logs.add(line),
        ),
      );
      expect(logs.where((l) => l.contains(canary)), isEmpty);
    },
  );
}
