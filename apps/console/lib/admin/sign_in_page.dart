// SPDX-License-Identifier: Apache-2.0

/// The page "Sign-in" of the console: the own sign-in methods (docs/spec/01-product.md 5.5;
/// 08 2.5, 3.3; design note docs/design/sign-in-methods.md).
library;

import 'package:admin_ui/admin_ui.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../platform/browser.dart';
import '../session/console_controller.dart';
import 'console_admin.dart';

/// [MySignInView] over the console's client: passkeys through the browser
/// (`navigator.credentials.create`), linking through a redirect that comes back on
/// `/console/sign-in/callback`, and the result of such a link as the first notice.
///
/// Example: `ConsoleSignInPage(controller: c, admin: admin)`.
class ConsoleSignInPage extends StatefulWidget {
  /// Creates the page.
  const ConsoleSignInPage({
    super.key,
    required this.controller,
    required this.admin,
  });

  /// The console's session and its client.
  final ConsoleController controller;

  /// The time zone and the clock of the lists.
  final ConsoleAdmin admin;

  @override
  State<ConsoleSignInPage> createState() => _ConsoleSignInPageState();
}

class _ConsoleSignInPageState extends State<ConsoleSignInPage> {
  late final ({IdentityLinkOutcome outcome, String provider})? _linked = widget
      .controller
      .takeLinkResult();

  /// A cancelled browser prompt, or an authenticator that already holds a passkey of the
  /// person, reads as "no passkey created".
  Future<Map<String, Object?>?> _createPasskey(
    Map<String, Object?> options,
  ) async {
    try {
      return await widget.controller.environment.passkeys.create(options);
    } on PasskeyCancelled {
      return null;
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final c = widget.controller;
    final linked = _linked;
    return SingleChildScrollView(
      key: const ValueKey('console.signIn'),
      padding: const EdgeInsets.all(DsSpace.s24),
      child: Align(
        alignment: Alignment.topLeft,
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 760),
          child: MySignInView(
            repository: ApiSignInRepository(c.api),
            timeZone: widget.admin.timeZone,
            clock: widget.admin.clock,
            passwordEnabled: c.passwordEnabled,
            createPasskey: _createPasskey,
            providers: [
              for (final p in c.providers)
                if (p.kind == ConsoleProviderKind.oidc)
                  SignInProvider(id: p.id, name: p.name),
            ],
            linkIdentity: c.linkIdentity,
            initialNotice: linked == null
                ? null
                : switch (linked.outcome) {
                    IdentityLinkOutcome.linked => SelfNotice(
                      l.signInLinked(linked.provider),
                    ),
                    IdentityLinkOutcome.cancelled => SelfNotice(
                      l.signInLinkCancelled,
                      tone: DsTone.warning,
                    ),
                    _ => SelfNotice(l.signInLinkFailed, tone: DsTone.danger),
                  },
          ),
        ),
      ),
    );
  }
}
