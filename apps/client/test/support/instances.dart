// SPDX-License-Identifier: Apache-2.0

// The discovery fixtures of E2 (brief M4-18 Design note 8): OIDC only; local with passkey and
// password; nothing.

import 'package:client/app/discovery.dart';

/// The address of the fixtures.
final Uri fixtureAddress = Uri.parse('https://chat.example.com');

/// Two OIDC providers, no local driver.
final InstanceInfo oidcOnly = InstanceInfo(
  address: fixtureAddress,
  instanceId: '0193f2a7-0000-7000-8000-0000000000a1',
  productName: 'Team Chat',
  accentColor: 'loch',
  providers: const [
    LoginProvider(id: '0193f2a7-0000-7000-8000-0000000000b1', kind: 'oidc', name: 'Company SSO'),
    LoginProvider(id: '0193f2a7-0000-7000-8000-0000000000b2', kind: 'trusted_header', name: 'Office proxy'),
  ],
);

/// A local driver with passwords.
final InstanceInfo localWithPassword = InstanceInfo(
  address: fixtureAddress,
  instanceId: '0193f2a7-0000-7000-8000-0000000000a2',
  productName: 'Team Chat',
  localEnabled: true,
  passwordEnabled: true,
);

/// No login method.
final InstanceInfo nothing = InstanceInfo(
  address: fixtureAddress,
  instanceId: '0193f2a7-0000-7000-8000-0000000000a3',
  productName: 'Team Chat',
);
