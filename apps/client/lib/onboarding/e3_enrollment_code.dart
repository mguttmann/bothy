// SPDX-License-Identifier: Apache-2.0

/// E3 Redeem enrollment code (docs/spec/01-produkt.md 5.6; 08 3.2).
library;

import 'package:auth/auth.dart' show normalizeCode;
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../app/discovery.dart';
import 'notice_text.dart';
import 'onboarding_card.dart';
import 'onboarding_controller.dart';

/// The code field first, then the methods the instance offers for it: this device with its
/// keys, a new passkey where the build has passkeys, a link with each login service.
///
/// The code is normalized as 08 3.2 says before it is sent (case, spaces and hyphens do not
/// count); a pasted code works the same. Enter continues, Esc goes back to E2. A
/// "password and one-time code" method is not offered: the API has no operation that sets a
/// password while redeeming a code (docs/DECISIONS.md D-M4-18-9).
class EnrollmentCodeScreen extends StatefulWidget {
  /// Creates the screen.
  const EnrollmentCodeScreen({
    super.key,
    required this.instance,
    required this.passkeysAvailable,
    required this.onRedeem,
    required this.onBack,
    this.notice,
    this.busy = false,
  });

  /// The instance and its login services.
  final InstanceInfo instance;

  /// Whether this build can create passkeys.
  final bool passkeysAvailable;

  /// Redeems the normalized code with the chosen method.
  final void Function(String code, EnrollMethod method) onRedeem;

  /// Back to E2.
  final VoidCallback onBack;

  /// The error of the last attempt.
  final LoginNotice? notice;

  /// Whether a request runs.
  final bool busy;

  @override
  State<EnrollmentCodeScreen> createState() => _EnrollmentCodeScreenState();
}

class _EnrollmentCodeScreenState extends State<EnrollmentCodeScreen> {
  final _code = TextEditingController();
  late EnrollMethod _method = const EnrollThisDevice();

  @override
  void dispose() {
    _code.dispose();
    super.dispose();
  }

  void _continue() {
    if (widget.busy || _code.text.trim().isEmpty) return;
    widget.onRedeem(normalizeCode(_code.text), _method);
  }

  List<(String, EnrollMethod)> _methods(AppLocalizations l) => [
    ('device', const EnrollThisDevice()),
    if (widget.passkeysAvailable && widget.instance.localEnabled)
      ('passkey', const EnrollWithPasskey()),
    for (final p in widget.instance.providers)
      ('provider.${p.id}', EnrollWithProvider(p)),
  ];

  String _label(AppLocalizations l, EnrollMethod method) => switch (method) {
    EnrollThisDevice() => l.onboardingEnrollThisDevice,
    EnrollWithPasskey() => l.onboardingEnrollCreatePasskey,
    EnrollWithProvider(:final provider) => l.onboardingEnrollLink(
      provider.name,
    ),
  };

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final enabled = !widget.busy;
    return OnboardingCard(
      onEnter: _continue,
      onEscape: widget.onBack,
      children: [
        Align(
          alignment: Alignment.centerLeft,
          child: TextButton(
            key: const ValueKey('e3.back'),
            onPressed: widget.onBack,
            child: Text(l.onboardingBack),
          ),
        ),
        CardTitle(l.onboardingEnrollTitle),
        cardGap,
        if (widget.notice != null) ...[NoticeBanner(widget.notice!), cardGap],
        TextField(
          key: const ValueKey('e3.code'),
          controller: _code,
          enabled: enabled,
          autocorrect: false,
          decoration: InputDecoration(labelText: l.onboardingEnrollCode),
          onSubmitted: (_) => _continue(),
        ),
        cardGap,
        CardText(l.onboardingEnrollHow),
        RadioGroup<EnrollMethod>(
          groupValue: _method,
          onChanged: (m) {
            if (m != null && enabled) setState(() => _method = m);
          },
          child: Column(
            children: [
              for (final (key, method) in _methods(l))
                RadioListTile<EnrollMethod>(
                  key: ValueKey('e3.method.$key'),
                  value: method,
                  title: Text(_label(l, method)),
                ),
            ],
          ),
        ),
        cardGap,
        FilledButton(
          key: const ValueKey('e3.continue'),
          onPressed: enabled ? _continue : null,
          child: Text(l.onboardingContinue),
        ),
        controlGap,
        CardText(l.onboardingEnrollOnce, center: true),
      ],
    );
  }
}
