// SPDX-License-Identifier: Apache-2.0

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../session/console_controller.dart';
import 'card.dart';
import 'notice_text.dart';

/// The step-up directly after the login (08 7.5): the methods of the session in the order
/// passkey, new OIDC login, password with one-time code. Nothing of the administration is
/// reachable before it succeeds.
class StepUpView extends StatefulWidget {
  /// Creates the view.
  const StepUpView({
    super.key,
    required this.methods,
    required this.oidcProviders,
    required this.onPasskey,
    required this.onOidc,
    required this.onPassword,
    this.notice,
    this.busy = false,
  });

  /// `step_up_methods_available` of the session.
  final List<StepUpMethod> methods;

  /// The OIDC providers for the re-login.
  final List<ConsoleProvider> oidcProviders;

  /// The message of the last attempt, or null.
  final ConsoleNotice? notice;

  /// Whether a request runs.
  final bool busy;

  /// Step-up with a passkey.
  final VoidCallback onPasskey;

  /// Step-up with a new OIDC login at a provider.
  final void Function(ConsoleProvider provider) onOidc;

  /// Step-up with password and one-time code.
  final void Function(String password, String code) onPassword;

  @override
  State<StepUpView> createState() => _StepUpViewState();
}

class _StepUpViewState extends State<StepUpView> {
  final _password = TextEditingController();
  final _code = TextEditingController();
  bool _passwordOpen = false;

  @override
  void dispose() {
    _password.dispose();
    _code.dispose();
    super.dispose();
  }

  void _submit() => widget.onPassword(_password.text, _code.text);

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final w = widget;
    final enabled = !w.busy;
    final oidc = w.methods.contains(StepUpMethod.oidcReauth)
        ? w.oidcProviders
        : const <ConsoleProvider>[];
    final hasMethod =
        w.methods.contains(StepUpMethod.passkey) ||
        oidc.isNotEmpty ||
        w.methods.contains(StepUpMethod.passwordTotp);
    return ConsoleCard(
      children: [
        CardTitle(l.consoleStepUpTitle),
        cardGap,
        Text(l.consoleStepUpExplanation, textAlign: TextAlign.center),
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
          DsBanner(tone: DsTone.warning, text: l.consoleStepUpNoMethod),
        if (w.methods.contains(StepUpMethod.passkey)) ...[
          FilledButton(
            onPressed: enabled ? w.onPasskey : null,
            child: Text(l.consoleStepUpPasskey),
          ),
          const SizedBox(height: DsSpace.s8),
        ],
        for (final p in oidc) ...[
          OutlinedButton(
            onPressed: enabled ? () => w.onOidc(p) : null,
            child: Text(l.consoleStepUpOidc(p.name)),
          ),
          const SizedBox(height: DsSpace.s8),
        ],
        if (w.methods.contains(StepUpMethod.passwordTotp) && !_passwordOpen)
          OutlinedButton(
            onPressed: enabled
                ? () => setState(() => _passwordOpen = true)
                : null,
            child: Text(l.consoleStepUpPassword),
          ),
        if (w.methods.contains(StepUpMethod.passwordTotp) && _passwordOpen) ...[
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
            onSubmitted: enabled ? (_) => _submit() : null,
          ),
          const SizedBox(height: DsSpace.s8),
          FilledButton(
            onPressed: enabled ? _submit : null,
            child: Text(l.consoleStepUpConfirm),
          ),
        ],
      ],
    );
  }
}
