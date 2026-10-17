// SPDX-License-Identifier: Apache-2.0

/// E1 Server address (docs/spec/01-produkt.md 5.6, R-PR-14).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'onboarding_card.dart';
import 'onboarding_controller.dart';

/// One field for the address of the instance and "Connect".
///
/// The result line follows [status]: no instance, an invalid certificate (no way on, no
/// exception button, R-PR-14), not set up yet with the pointer to the setup wizard, or "App too
/// old" with [minVersion]. Enter connects.
class ServerAddressScreen extends StatefulWidget {
  /// Creates the screen.
  const ServerAddressScreen({
    super.key,
    required this.address,
    required this.status,
    required this.onConnect,
    this.minVersion,
  });

  /// The address to show in the field.
  final String address;

  /// The result of the last attempt.
  final ServerStatus status;

  /// The version the instance requires, for [ServerStatus.appTooOld].
  final String? minVersion;

  /// Connects to the typed address.
  final ValueChanged<String> onConnect;

  @override
  State<ServerAddressScreen> createState() => _ServerAddressScreenState();
}

class _ServerAddressScreenState extends State<ServerAddressScreen> {
  late final TextEditingController _address = TextEditingController(
    text: widget.address,
  );

  @override
  void dispose() {
    _address.dispose();
    super.dispose();
  }

  bool get _busy => widget.status == ServerStatus.connecting;

  void _connect() {
    if (!_busy) widget.onConnect(_address.text);
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final (tone, text) = switch (widget.status) {
      ServerStatus.idle || ServerStatus.connecting => (null, null),
      ServerStatus.invalidAddress => (
        DsTone.danger,
        l.onboardingAddressInvalid,
      ),
      ServerStatus.noInstance => (DsTone.danger, l.onboardingNoInstance),
      ServerStatus.certificateInvalid => (
        DsTone.danger,
        l.onboardingCertificateInvalid,
      ),
      ServerStatus.notSetUp => (DsTone.info, l.onboardingNotSetUp),
      ServerStatus.appTooOld => (
        DsTone.warning,
        l.onboardingAppTooOld(widget.minVersion ?? ''),
      ),
    };
    return OnboardingCard(
      onEnter: _connect,
      children: [
        const BrandMark(),
        cardGap,
        CardTitle(l.onboardingConnectTitle),
        cardGap,
        TextField(
          key: const ValueKey('e1.address'),
          controller: _address,
          enabled: !_busy,
          keyboardType: TextInputType.url,
          autocorrect: false,
          decoration: InputDecoration(labelText: l.onboardingServerAddress),
          onSubmitted: (_) => _connect(),
        ),
        cardGap,
        FilledButton(
          key: const ValueKey('e1.connect'),
          onPressed: _busy ? null : _connect,
          child: Text(_busy ? l.onboardingConnecting : l.onboardingConnect),
        ),
        if (tone != null && text != null) ...[
          cardGap,
          DsBanner(
            key: ValueKey('e1.status.${widget.status.name}'),
            tone: tone,
            text: text,
            liveRegion: true,
          ),
        ],
      ],
    );
  }
}
