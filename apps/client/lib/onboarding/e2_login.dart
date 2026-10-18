// SPDX-License-Identifier: Apache-2.0

/// E2 Login (docs/spec/01-product.md 5.6; 08 2.2, 2.4, 2.5).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../app/discovery.dart';
import 'notice_text.dart';
import 'onboarding_card.dart';
import 'onboarding_controller.dart';

/// Exactly the methods discovery names (fixed requirement 13): a button per OIDC or proxy
/// provider; with a `local` driver the login name, "Log in with passkey" where the build has
/// passkeys ([passkeysAvailable], D-M4-18-9) and, with `local_password_enabled`, "Password and
/// one-time code" with its second step; always "I have an enrollment code" and "Pair with
/// another device". Without a method: "No login is set up for this instance".
///
/// Enter triggers the main action (the open password step, else the first provider); Esc goes
/// back to E1.
class LoginScreen extends StatefulWidget {
  /// Creates the screen.
  const LoginScreen({
    super.key,
    required this.instance,
    required this.passkeysAvailable,
    required this.onProvider,
    required this.onPasskey,
    required this.onPassword,
    required this.onEnroll,
    required this.onPair,
    required this.onChangeServer,
    this.notice,
    this.busy = false,
  });

  /// The instance: name, logo, address and methods.
  final InstanceInfo instance;

  /// Whether this build can use passkeys.
  final bool passkeysAvailable;

  /// The error of the last attempt.
  final LoginNotice? notice;

  /// Whether a login runs.
  final bool busy;

  /// Login with a provider.
  final ValueChanged<LoginProvider> onProvider;

  /// Passkey login with the login name (may be empty).
  final ValueChanged<String> onPasskey;

  /// Password login with login name, password and one-time code.
  final void Function(String loginName, String password, String code)
  onPassword;

  /// "I have an enrollment code" (E3).
  final VoidCallback onEnroll;

  /// "Pair with another device" (E5).
  final VoidCallback onPair;

  /// Back to E1.
  final VoidCallback onChangeServer;

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
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

  void _submitPassword() {
    if (widget.busy) return;
    widget.onPassword(_name.text, _password.text, _code.text);
  }

  VoidCallback? get _mainAction {
    final w = widget;
    if (w.busy) return null;
    if (_passwordOpen) return _submitPassword;
    if (w.instance.providers.isNotEmpty) {
      return () => w.onProvider(w.instance.providers.first);
    }
    return null;
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final w = widget;
    final instance = w.instance;
    final enabled = !w.busy;
    final passkey = instance.localEnabled && w.passkeysAvailable;
    return OnboardingCard(
      onEnter: _mainAction,
      onEscape: w.onChangeServer,
      children: [
        BrandMark(logo: instance.logo),
        cardGap,
        CardTitle(l.onboardingLoginTitle(instance.productName)),
        cardGap,
        if (w.notice != null) ...[NoticeBanner(w.notice!), cardGap],
        if (!instance.hasLoginMethod) ...[
          DsBanner(
            key: const ValueKey('e2.noMethod'),
            tone: DsTone.info,
            text: l.onboardingNoMethod,
          ),
          cardGap,
        ],
        for (final p in instance.providers) ...[
          FilledButton(
            key: ValueKey('e2.provider.${p.id}'),
            onPressed: enabled ? () => w.onProvider(p) : null,
            child: Text(l.onboardingLoginWithProvider(p.name)),
          ),
          controlGap,
        ],
        if (instance.providers.isNotEmpty && instance.localEnabled) ...[
          CardText(l.onboardingOr, center: true),
          controlGap,
        ],
        if (instance.localEnabled) ...[
          TextField(
            key: const ValueKey('e2.loginName'),
            controller: _name,
            enabled: enabled,
            autocorrect: false,
            autofillHints: const [AutofillHints.username],
            decoration: InputDecoration(labelText: l.onboardingLoginName),
          ),
          controlGap,
          if (passkey) ...[
            OutlinedButton(
              key: const ValueKey('e2.passkey'),
              onPressed: enabled ? () => w.onPasskey(_name.text) : null,
              child: Text(l.onboardingLoginWithPasskey),
            ),
            controlGap,
          ],
          if (instance.passwordEnabled && !_passwordOpen)
            OutlinedButton(
              key: const ValueKey('e2.password.open'),
              onPressed: enabled
                  ? () => setState(() => _passwordOpen = true)
                  : null,
              child: Text(l.onboardingPasswordAndCode),
            ),
          if (instance.passwordEnabled && _passwordOpen) ...[
            TextField(
              key: const ValueKey('e2.password'),
              controller: _password,
              enabled: enabled,
              obscureText: true,
              autofillHints: const [AutofillHints.password],
              decoration: InputDecoration(labelText: l.onboardingPassword),
            ),
            controlGap,
            TextField(
              key: const ValueKey('e2.code'),
              controller: _code,
              enabled: enabled,
              autofillHints: const [AutofillHints.oneTimeCode],
              keyboardType: TextInputType.number,
              decoration: InputDecoration(labelText: l.onboardingOneTimeCode),
            ),
            controlGap,
            FilledButton(
              key: const ValueKey('e2.submit'),
              onPressed: enabled ? _submitPassword : null,
              child: Text(l.onboardingLogIn),
            ),
          ],
        ],
        cardGap,
        TextButton(
          key: const ValueKey('e2.enroll'),
          onPressed: enabled ? w.onEnroll : null,
          child: Text(l.onboardingEnrollmentCodeLink),
        ),
        TextButton(
          key: const ValueKey('e2.pair'),
          onPressed: enabled ? w.onPair : null,
          child: Text(l.onboardingPairLink),
        ),
        cardGap,
        Row(
          children: [
            Expanded(
              child: CardText(
                l.onboardingServerHost(instance.address.authority),
              ),
            ),
            TextButton(
              key: const ValueKey('e2.change'),
              onPressed: w.onChangeServer,
              child: Text(l.onboardingChangeServer),
            ),
          ],
        ),
      ],
    );
  }
}
