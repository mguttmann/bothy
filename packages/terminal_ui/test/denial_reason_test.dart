// SPDX-License-Identifier: Apache-2.0

// A refused opening keeps the server's reason next to the problem code, and the banner shows
// the text of that reason (brief M4-36 fixed requirement 10: denials show the server's
// `reason`, SR10 to SR12, never a reason of the client's own).

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';
import 'package:terminal_ui/terminal_ui.dart';

import 'support/fakes.dart';
import 'support/widgets.dart';

void main() {
  test(
    'a refused opening keeps code and reason; a later phase drops them',
    () async {
      final h = await Harness.create();
      h.api.openError = const ApiProblem(
        status: 403,
        code: 'terminal_denied',
        reason: 'no_session_user',
      );
      await h.open();
      expect(h.controller.state.phase, TerminalPhase.failed);
      expect(h.controller.state.problemCode, 'terminal_denied');
      expect(h.controller.state.problemReason, 'no_session_user');
      expect(
        h.controller.state.copyWith(clearCauses: true).problemReason,
        isNull,
      );
    },
  );

  testWidgets('the banner shows the text of the reason', (tester) async {
    await pumpApp(
      tester,
      const TerminalStateBanner(
        state: TerminalTabState(
          phase: TerminalPhase.failed,
          problemCode: 'terminal_denied',
          problemReason: 'no_session_user',
        ),
      ),
    );
    final l = lookupAppLocalizations(const Locale('en'));
    expect(
      find.text(
        l.terminalStateFailed(
          problemText(l, 'terminal_denied', reason: 'no_session_user'),
        ),
      ),
      findsOneWidget,
    );
    expect(
      problemText(l, 'terminal_denied', reason: 'no_session_user'),
      isNot(l.problemTerminalDenied),
    );
  });
}
