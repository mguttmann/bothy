// SPDX-License-Identifier: Apache-2.0

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'card.dart';

/// E7 in the console (01 5.6): the reason as text, without internal names, and "Log in again".
/// The console keeps no data in the browser, so the sentence about deleted device data of the
/// apps does not apply.
class LoggedOutView extends StatelessWidget {
  /// Creates the view; [reasonCode] is the problem code that ended the session, null after the
  /// person's own logout.
  const LoggedOutView({super.key, required this.onLogInAgain, this.reasonCode});

  /// The problem code, or null.
  final String? reasonCode;

  /// Back to E2.
  final VoidCallback onLogInAgain;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final code = reasonCode;
    return ConsoleCard(
      children: [
        CardTitle(l.consoleLoggedOutTitle),
        cardGap,
        Semantics(
          liveRegion: true,
          child: Text(
            code == null
                ? l.consoleLoggedOutByYou
                : l.consoleLoggedOutReason(problemText(l, code)),
            style: dsTextStyle(DsType.body),
            textAlign: TextAlign.center,
          ),
        ),
        cardGap,
        FilledButton(onPressed: onLogInAgain, child: Text(l.consoleLogInAgain)),
      ],
    );
  }
}
