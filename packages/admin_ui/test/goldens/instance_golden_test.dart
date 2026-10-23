// SPDX-License-Identifier: Apache-2.0

// Goldens of the instance administration of M6-09 (brief M6-09 Tests): the settings list, the
// branding with the accent picker and its preview, the login providers, the OIDC form and the
// feature flags; light and dark, text scaling 1 and 2, German and English. One file per view
// keeps each file below repo.max_file_kb.

import 'package:admin_ui/admin_ui.dart';
import 'package:alchemist/alchemist.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

import '../admin/support.dart';

Widget _frame(Widget child) => Material(
  type: MaterialType.transparency,
  child: SizedBox(
    width: 760,
    child: Padding(padding: const EdgeInsets.all(16), child: child),
  ),
);

FakeAdminRepository _repository() {
  final r = FakeAdminRepository(prompt: StepUpDialogPrompt(() => null))
    ..stepUpFresh = true;
  r.otherOwnerSets('locale.default', 'en');
  r.seedProvider(ProviderKind.oidc, 'Company login', {
    'issuer': 'https://login.example.com',
    'client_id': 'bothy-example',
    'client_auth': 'private_key_jwt',
    'client_key_ref': 'file:/run/bothy/secrets/idp/EXAMPLE-key',
    'client_key_alg': 'PS256',
  });
  r.seedProvider(ProviderKind.trustedHeader, 'Tailnet', {
    'login_header': 'Tailscale-User-Login',
  }, enabled: false);
  return r;
}

final Map<String, Widget Function()> _views = {
  'settings': () => _frame(
    InstanceSettingsSection(
      repository: _repository(),
      timeZone: 'Europe/Berlin',
      clock: testClock,
    ),
  ),
  'branding': () => _frame(BrandingSection(repository: _repository())),
  'providers': () => _frame(ProvidersSection(repository: _repository())),
  'provider_oidc': () {
    final r = _repository();
    return ProviderEditorDialog(
      repository: r,
      kind: ProviderKind.oidc,
      provider: r.providersById.values.first,
    );
  },
  'flags': () => _frame(
    FlagsSection(
      repository: _repository(),
      timeZone: 'Europe/Berlin',
      clock: testClock,
    ),
  ),
};

void main() {
  for (final MapEntry(key: name, value: view) in _views.entries) {
    goldenTest(
      '$name in light and dark, text scale 1 and 2, de and en',
      fileName: 'instance_$name',
      pumpBeforeTest: dsPumpIcons,
      builder: () => dsGoldenMatrix(
        name: name,
        builder: view,
        textScales: const [1.0, 2.0],
        locales: const [Locale('de'), Locale('en')],
      ),
    );
  }
}
