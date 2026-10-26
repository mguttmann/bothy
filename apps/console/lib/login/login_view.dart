// SPDX-License-Identifier: Apache-2.0

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../session/console_controller.dart';
import 'card.dart';
import 'notice_text.dart';

/// E2 in the console form (01 5.6): the instance name, a button per provider of discovery and,
/// with a `local` driver, the login name with the passkey and, if passwords are on, the password
/// with the one-time code. No enrollment code and no pairing: the console has no device screens.
class LoginView extends StatefulWidget {
  /// Creates the view.
  const LoginView({
    super.key,
    required this.productName,
    required this.providers,
    required this.localEnabled,
    required this.passwordEnabled,
    required this.onProvider,
    required this.onPasskey,
    required this.onPassword,
    this.notice,
    this.busy = false,
    this.onPublicStatus,
  });

  /// `branding.product_name`.
  final String productName;

  /// The OIDC and proxy providers.
  final List<ConsoleProvider> providers;

  /// Whether a `local` driver is active.
  final bool localEnabled;

  /// Whether `local_password_enabled`.
  final bool passwordEnabled;

  /// The message of the last attempt, or null.
  final ConsoleNotice? notice;

  /// Whether a request runs; the buttons are off meanwhile.
  final bool busy;

  /// Opens the public status page (brief M7-21), or null for no link.
  final VoidCallback? onPublicStatus;

  /// Login with a provider.
  final void Function(ConsoleProvider provider) onProvider;

  /// Passkey login with the login name (may be empty).
  final void Function(String loginName) onPasskey;

  /// Password login with login name, password and one-time code.
  final void Function(String loginName, String password, String code)
  onPassword;

  @override
  State<LoginView> createState() => _LoginViewState();
}

class _LoginViewState extends State<LoginView> {
  final _name = TextEditingController();
  final _password = TextEditingController();
  final _code = TextEditingController();
  bool _passwordOpen = false;

  @override
  void dispose() {
    _name.dispose();
    _password.dispose();
    _code.dispose();
    super.dispose();
  }

  void _submitPassword() =>
      widget.onPassword(_name.text, _password.text, _code.text);

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final w = widget;
    final enabled = !w.busy;
    final hasMethod = w.providers.isNotEmpty || w.localEnabled;
    return ConsoleCard(
      children: [
        CardTitle(l.consoleLoginTitle(w.productName)),
        cardGap,
        if (w.notice != null) ...[
          DsBanner(
            tone: DsTone.danger,
            text: noticeText(l, w.notice!),
            liveRegion: true,
          ),
          cardGap,
        ],
        if (!hasMethod)
          DsBanner(tone: DsTone.info, text: l.consoleLoginNoMethod),
        for (final p in w.providers) ...[
          FilledButton(
            onPressed: enabled ? () => w.onProvider(p) : null,
            child: Text(l.consoleLoginWithProvider(p.name)),
          ),
          const SizedBox(height: DsSpace.s8),
        ],
        if (w.providers.isNotEmpty && w.localEnabled) ...[
          Text(l.consoleLoginOr, textAlign: TextAlign.center),
          const SizedBox(height: DsSpace.s8),
        ],
        if (w.localEnabled) ...[
          TextField(
            controller: _name,
            enabled: enabled,
            autofillHints: const [AutofillHints.username],
            decoration: InputDecoration(labelText: l.consoleLoginName),
          ),
          const SizedBox(height: DsSpace.s8),
          OutlinedButton(
            onPressed: enabled ? () => w.onPasskey(_name.text) : null,
            child: Text(l.consoleLoginWithPasskey),
          ),
          if (w.passwordEnabled && !_passwordOpen) ...[
            const SizedBox(height: DsSpace.s8),
            OutlinedButton(
              onPressed: enabled
                  ? () => setState(() => _passwordOpen = true)
                  : null,
              child: Text(l.consoleLoginPasswordAndCode),
            ),
          ],
          if (w.passwordEnabled && _passwordOpen) ...[
            const SizedBox(height: DsSpace.s8),
            TextField(
              controller: _password,
              enabled: enabled,
              obscureText: true,
              autofillHints: const [AutofillHints.password],
              decoration: InputDecoration(labelText: l.consoleLoginPassword),
            ),
            const SizedBox(height: DsSpace.s8),
            TextField(
              controller: _code,
              enabled: enabled,
              autofillHints: const [AutofillHints.oneTimeCode],
              keyboardType: TextInputType.number,
              decoration: InputDecoration(labelText: l.consoleLoginOneTimeCode),
              onSubmitted: enabled ? (_) => _submitPassword() : null,
            ),
            const SizedBox(height: DsSpace.s8),
            FilledButton(
              onPressed: enabled ? _submitPassword : null,
              child: Text(l.consoleLoginSubmit),
            ),
          ],
        ],
        if (w.onPublicStatus case final open?) ...[
          cardGap,
          TextButton(
            key: const ValueKey('console.login.publicStatus'),
            onPressed: open,
            child: Text(l.statusPublicLink),
          ),
        ],
      ],
    );
  }
}
