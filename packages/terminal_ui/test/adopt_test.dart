// SPDX-License-Identifier: Apache-2.0

// A tab for a session the server already lists (brief M4-36: one tab per own session on the
// host, 01 D11): `adopt` shows it detached with "Reattach", without a request, and `attach`
// then runs the flow of 15 23.2 item 6 with the replay; a closed session is not adopted. The
// pane passes the app's terminal shortcuts to the renderer, so they win over the shell keys
// while Ctrl+C still reaches the shell (R-PR-10).

import 'dart:convert';

import 'package:api_client/api_client.dart'
    show Host, TerminalSession, TerminalSessionState;
import 'package:flutter/services.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';

import 'support/fakes.dart';
import 'support/widgets.dart';

TerminalSession listed(Harness h, {TerminalSessionState? state}) =>
    h.api.grant().session.rebuild(
      (b) => b
        ..state = state ?? TerminalSessionState.detached
        ..openedAt = DateTime.utc(2026, 10, 20, 8),
    );

class _Marker extends Intent {
  const _Marker();
}

void main() {
  test('adopt shows a listed session detached with its profile and start, without a request', () async {
    final h = await Harness.create();
    final Host host = h.api.currentHost();
    h.controller.adopt(host: host, session: listed(h));
    expect(h.controller.state.phase, TerminalPhase.detached);
    expect(h.controller.state.detachCause, DetachCause.person);
    expect(h.controller.state.sessionId, sessionIdText);
    expect(h.controller.state.profile, 'shell');
    expect(h.controller.state.openedAt, DateTime.utc(2026, 10, 20, 8));
    expect(h.controller.host, host);
    expect(h.log, isEmpty);
  });

  test(
    'attach after adopt checks the pin, attaches and replays the scrollback',
    () async {
      final h = await Harness.create();
      h.controller.adopt(host: h.api.currentHost(), session: listed(h));
      await h.controller.attach();
      await h.connected();
      expect(h.log, ['getHost', 'attach']);
      await h.host.sendReplay(utf8.encode('replayed line\r\n'));
      await h.settle();
      expect(h.controller.screen.visibleText(), contains('replayed line'));
    },
  );

  test('a closed session, a busy or an open tab is not adopted', () async {
    final h = await Harness.create();
    h.controller.adopt(
      host: h.api.currentHost(),
      session: listed(h, state: TerminalSessionState.closed),
    );
    expect(h.controller.state.phase, TerminalPhase.idle);

    final g = await Harness.create();
    await g.open();
    await g.connected();
    g.controller.adopt(host: g.api.currentHost(), session: listed(g));
    expect(g.controller.state.phase, TerminalPhase.connected);
  });

  testWidgets(
    'the pane hands the app shortcuts to the renderer before the shell',
    (tester) async {
      final h = (await tester.runAsync(Harness.create))!;
      var hits = 0;
      await pumpApp(
        tester,
        Actions(
          actions: {_Marker: CallbackAction<_Marker>(onInvoke: (_) => hits++)},
          child: TerminalPane(
            controller: h.controller,
            fontSize: 14,
            shortcuts: const {
              SingleActivator(
                LogicalKeyboardKey.keyN,
                control: true,
                shift: true,
              ): _Marker(),
            },
          ),
        ),
      );
      await tester.tap(find.byType(TerminalScreenView));
      await tester.pumpAndSettle();
      await tester.sendKeyDownEvent(LogicalKeyboardKey.controlLeft);
      await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
      await tester.sendKeyEvent(LogicalKeyboardKey.keyN);
      await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
      await tester.sendKeyUpEvent(LogicalKeyboardKey.controlLeft);
      expect(hits, 1);
      // The tap timer of the renderer runs out.
      await tester.pump(const Duration(seconds: 1));
    },
  );
}
