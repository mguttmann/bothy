// SPDX-License-Identifier: Apache-2.0

// The life cycle of the terminal tabs of the app (FIX-terminal-session-lifecycle): "End" of a
// live session ends it on the host and closes its tab; an ended tab closes with one "Close" and
// leaves no ghost tab; a session the server ended while the app slept ends its tab on refresh;
// a tab whose stream dropped attaches again once the app is back online or in front, a tab the
// person detached does not; the session list shows every own session with "Reattach".

import 'dart:async';

import 'package:api_client/api_client.dart' as api show TerminalSessionState;
import 'package:api_client/api_client.dart' show TerminalCloseReason;
import 'package:client/terminal/providers.dart';
import 'package:client/terminal/reconnect.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart' show AppActivity, AppLifecycle;
import 'package:terminal_ui/terminal_ui.dart';
import 'package:terminal_ui/testing.dart';

import 'support.dart';

/// Counts the reconnect calls.
final class _Target implements TerminalReconnectTarget {
  int calls = 0;

  @override
  void reconnectLost() => calls++;
}

/// A life cycle the test drives.
final class _Lifecycle implements AppLifecycle {
  final StreamController<AppActivity> _changes = StreamController.broadcast(
    sync: true,
  );

  @override
  AppActivity current = AppActivity.foreground;

  void set(AppActivity a) {
    current = a;
    _changes.add(a);
  }

  @override
  Stream<AppActivity> get changes => _changes.stream;
}

void main() {
  setUpAll(TerminalWorld.prepareIdleHost);

  testWidgets('End of a live session ends it on the host and closes the tab', (
    tester,
  ) async {
    final w = await TerminalWorld.create(tester);
    final c = await w.pump(tester);
    final tabs = await openShell(tester, c);
    final id = tabs.current!.state.sessionId!;
    await tester.tap(find.widgetWithText(OutlinedButton, en.terminalEnd));
    await pumpUntil(tester, () => tabs.tabs.isEmpty);
    expect(w.api.closes, [id]);
    await tester.pump();
    expect(find.byKey(const ValueKey('terminal.empty')), findsOneWidget);
  });

  testWidgets('an ended tab closes with Close and leaves no ghost tab', (
    tester,
  ) async {
    final w = await TerminalWorld.create(tester);
    final c = await w.pump(tester);
    final tabs = await openShell(tester, c);
    final id = tabs.current!.state.sessionId!;
    w.events.changes.add(
      TerminalSessionUpdated(id, closed: true, closeReason: 'detach_expired'),
    );
    await pumpUntil(
      tester,
      () => tabs.current?.state.phase == TerminalPhase.ended,
    );
    await tester.pump();
    expect(
      find.widgetWithText(OutlinedButton, en.terminalDetach),
      findsNothing,
    );
    await tester.tap(find.widgetWithText(OutlinedButton, en.terminalCloseTab));
    await tester.pump();
    expect(tabs.tabs, isEmpty);
    // The server lists the closed session; a refresh brings no tab back.
    w.api.listed.add(
      FakeHostsApi.session(id, state: api.TerminalSessionState.closed),
    );
    await tester.runAsync(tabs.refresh);
    expect(tabs.tabs, isEmpty);
  });

  test(
    'refresh ends a tab whose session the server closed meanwhile',
    () async {
      final fake = FakeHostsApi(await FakeHost.create(hostId: hostIdText));
      final id = FakeHostsApi.sessionId(1);
      fake.listed.add(
        FakeHostsApi.session(id, state: api.TerminalSessionState.detached),
      );
      final tabs = TerminalTabs(
        hostId: hostIdText,
        services: TerminalWorld.servicesFor(fake, const Stream.empty()),
        selfUserId: alexId,
      );
      addTearDown(tabs.dispose);
      await tabs.refresh();
      expect(tabs.current!.state.phase, TerminalPhase.detached);
      fake.listed
        ..clear()
        ..add(
          FakeHostsApi.session(
            id,
            state: api.TerminalSessionState.closed,
          ).rebuild((b) => b.closeReason = TerminalCloseReason.detachExpired),
        );
      await tabs.refresh();
      expect(tabs.current!.state.phase, TerminalPhase.ended);
      expect(tabs.current!.state.closeReason, 'detach_expired');
      tabs.close(tabs.current!);
      expect(tabs.tabs, isEmpty);
    },
  );

  testWidgets('a dropped stream attaches again on reconnect; a detach by the '
      'person stays', (tester) async {
    final w = await TerminalWorld.create(tester);
    final c = await w.pump(tester);
    final tabs = await openShell(tester, c);
    await tester.runAsync(() => w.host.close(1006));
    await pumpUntil(
      tester,
      () =>
          tabs.current?.state.phase == TerminalPhase.detached &&
          tabs.current?.state.detachCause == DetachCause.connectionLost,
    );
    tabs.reconnectLost();
    await pumpUntil(
      tester,
      () => tabs.current?.state.phase == TerminalPhase.connected,
    );
    await tester.runAsync(tabs.current!.detach);
    await tester.pump();
    tabs.reconnectLost();
    await tester.pump();
    expect(tabs.current!.state.phase, TerminalPhase.detached);
    expect(tabs.current!.state.detachCause, DetachCause.person);
  });

  test(
    'the reconnector asks the tabs to reattach when the app is back online or '
    'in front',
    () async {
      final online = StreamController<bool>.broadcast(sync: true);
      addTearDown(online.close);
      final lifecycle = _Lifecycle();
      final r = TerminalReconnector(
        online: online.stream,
        lifecycle: lifecycle,
      );
      addTearDown(r.dispose);
      final t = _Target();
      r.register(t);
      online.add(true);
      expect(t.calls, 0, reason: 'the first state is no change');
      online
        ..add(false)
        ..add(true);
      expect(t.calls, 1);
      online.add(true);
      expect(t.calls, 1, reason: 'online stays online');
      lifecycle
        ..set(AppActivity.background)
        ..set(AppActivity.foreground);
      expect(t.calls, 2);
      r.unregister(t);
      online
        ..add(false)
        ..add(true);
      expect(t.calls, 2);
    },
  );

  testWidgets('Sessions lists the own sessions with start, detach time and '
      'Reattach', (tester) async {
    final w = await TerminalWorld.create(tester);
    w.api.listed.add(
      FakeHostsApi.session(
        FakeHostsApi.sessionId(7),
        state: api.TerminalSessionState.detached,
      ).rebuild((b) => b.detachedAt = DateTime.utc(2026, 10, 20, 9, 40)),
    );
    final c = await w.pump(tester);
    final tabs = c.read(terminalTabsProvider(hostIdText))!;
    await pumpUntil(tester, () => tabs.tabs.isNotEmpty);
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('terminal.sessions')));
    await tester.pumpAndSettle();
    expect(find.text('shell 1'), findsWidgets);
    expect(find.textContaining('Detached since'), findsOneWidget);
    await tester.tap(
      find.byKey(const ValueKey('terminal.sessions.0.reattach')),
    );
    await pumpUntil(
      tester,
      () => tabs.current?.state.phase == TerminalPhase.connected,
    );
  });
}
