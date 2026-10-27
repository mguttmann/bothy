// SPDX-License-Identifier: Apache-2.0

// Accessibility of M9 Hosts and M8 Terminal on iOS and Android (brief M5-13 Design note 3;
// 01 5.3 M8, M9, 8): the host with its connection and LLM login in words, the own sessions with
// their state, "New session"; the terminal with the banner and "Reattach", the session switcher,
// the screen with the line at the cursor and "Read screen", and the extra keys bar with every
// key named. What the screen says while a shell runs is in ../terminal_a11y_test.dart.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart' show TerminalScreenView;

import '../../phone/support/terminal_scenes.dart';
import '../../terminal/support.dart' show FakeHostsApi, hostIdText;
import 'support.dart';

void main() {
  setUpAll(prepareTerminalScenes);

  for (final android in [false, true]) {
    final on = android ? ' on Android' : '';
    final platform = android ? TargetPlatform.android : TargetPlatform.iOS;

    phoneScreenTests(
      'M9 Hosts$on',
      build: () => hostsScene(android: android),
      platform: platform,
      reachable: (l) => [
        find.byKey(const ValueKey('phone.hosts.open.$hostIdText')),
        find.byKey(
          ValueKey('phone.hosts.session.${FakeHostsApi.sessionId(1)}'),
        ),
        find.byKey(const ValueKey('phone.hosts.new.$hostIdText')),
      ],
    );

    phoneScreenTests(
      'M8 Terminal$on',
      build: () => terminalScene(android: android),
      platform: platform,
      semantics: (tester, l) {
        // The screen is one node with the line at the cursor and "Read screen"; the bar names
        // Ctrl with its state (off here).
        final screen = tester.getSemantics(
          find.descendant(
            of: find.byType(TerminalScreenView),
            matching: find.bySemanticsLabel(
              RegExp('^${RegExp.escape(l.terminalScreenLabel(''))}'),
            ),
          ),
        );
        expect(
          screen.getSemanticsData().customSemanticsActionIds,
          hasLength(1),
        );
        expect(find.bySemanticsLabel(l.terminalKeyCtrlName), findsOneWidget);
      },
      reachable: (l) => [
        find.text(l.terminalReattach),
        find.text(l.terminalKeyCtrl),
        find.text(l.terminalKeyEsc),
      ],
    );
  }
}
