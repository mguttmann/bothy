// SPDX-License-Identifier: Apache-2.0

// The terminal view of a host (01 D11; brief M4-36 Tests): "New session" offers the host's
// profiles; the profile `claude` offers only the working folders of the person's own entry in
// `reported_session_users` (24 2.2, ruling T13); a refused opening shows the server's reason
// (SR10 to SR12); opening runs the step-up and connects; a session the server lists becomes a
// tab with "Reattach".

import 'package:client/terminal/providers.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:terminal_ui/terminal_ui.dart';

import 'support.dart';

void main() {
  testWidgets('the profile claude offers only the own working folders', (
    tester,
  ) async {
    final w = await TerminalWorld.create(tester);
    await w.pump(tester);
    await tester.tap(find.byKey(const ValueKey('terminal.new')));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('terminal.new.shell')), findsWidgets);
    await tester.tap(find.byKey(const ValueKey('terminal.new.claude')).last);
    await tester.pumpAndSettle();
    expect(find.text(en.terminalOpenWorkdir), findsOneWidget);
    await tester.tap(find.byType(DropdownButton<int>));
    await tester.pumpAndSettle();
    expect(find.text('Project A'), findsWidgets);
    expect(find.text('Project B'), findsWidgets);
    expect(find.text('Sam only'), findsNothing);
    await tester.tap(find.text('Project B').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text(en.terminalOpenConfirm));
    await tester.pump();
    await pumpUntil(tester, () => w.api.opens.isNotEmpty);
    expect(w.api.opens.single.profile, 'claude');
    expect(w.api.opens.single.workdirIndex, 1);
    expect(w.stepUps, hasLength(1));
  });

  testWidgets('a refused opening shows the reason the server gave', (
    tester,
  ) async {
    final w = await TerminalWorld.create(tester);
    w.api.openError = denied('no_session_user');
    final c = await w.pump(tester);
    await tester.tap(find.byKey(const ValueKey('terminal.new')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('terminal.new.shell')).last);
    await tester.pumpAndSettle();
    await tester.tap(find.text(en.terminalOpenConfirm));
    final tabs = c.read(terminalTabsProvider(hostIdText))!;
    await pumpUntil(
      tester,
      () => tabs.current?.state.phase == TerminalPhase.failed,
    );
    await tester.pump();
    expect(
      find.text(
        en.terminalStateFailed(
          problemText(en, 'terminal_denied', reason: 'no_session_user'),
        ),
      ),
      findsOneWidget,
    );
    expect(
      en.problemTerminalDeniedNoSessionUser,
      isNot(en.problemTerminalDenied),
    );
  });

  testWidgets(
    'opening runs the step-up and connects; the tab shows the profile',
    (tester) async {
      final w = await TerminalWorld.create(tester);
      final c = await w.pump(tester);
      final tabs = await openShell(tester, c);
      expect(w.stepUps, hasLength(1));
      expect(tabs.tabs, hasLength(1));
      expect(find.text(en.terminalTabLabel('shell', 1)), findsOneWidget);
      expect(w.pins.pins[hostIdText]?.noisePublicKey, w.host.noisePublicKey);
      // End the stream so no timer of the renderer outlives the test.
      await tabs.current!.detach();
      await tester.pump(const Duration(seconds: 1));
    },
  );

  testWidgets('a session the server lists becomes a tab with Reattach', (
    tester,
  ) async {
    final w = await TerminalWorld.create(tester);
    w.api.listed.add(FakeHostsApi.session(FakeHostsApi.sessionId(40)));
    w.api.listed.add(
      FakeHostsApi.session(FakeHostsApi.sessionId(41), userId: samId),
    );
    final c = await w.pump(tester);
    final tabs = c.read(terminalTabsProvider(hostIdText))!;
    await pumpUntil(tester, () => tabs.tabs.isNotEmpty);
    await tester.pump();
    expect(tabs.tabs, hasLength(1), reason: 'only the own session');
    expect(tabs.current!.state.phase, TerminalPhase.detached);
    expect(find.text(en.terminalReattach), findsOneWidget);
  });
}
