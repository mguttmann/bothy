// SPDX-License-Identifier: Apache-2.0

import 'package:alchemist/alchemist.dart';
import 'package:console/console.dart';
import 'package:design_system/testing.dart';
import 'package:flutter/material.dart';

/// The matrix of the brief: light and dark, text scaling 1 and 2, German and English.
GoldenTestGroup _matrix(String name, Widget Function() builder) =>
    dsGoldenMatrix(
      name: name,
      builder: () => SizedBox(width: 480, height: 720, child: builder()),
      textScales: const [1.0, 2.0],
      locales: const [Locale('de'), Locale('en')],
    );

const _providers = [
  ConsoleProvider(
    id: 'p1',
    kind: ConsoleProviderKind.oidc,
    name: 'Company SSO',
  ),
];

void main() {
  goldenTest(
    'E2 login of the console',
    fileName: 'console_e2_login',
    builder: () => _matrix(
      'E2',
      () => LoginView(
        productName: 'Team Chat',
        providers: _providers,
        localEnabled: true,
        passwordEnabled: true,
        notice: ConsoleNotice.loginFailed,
        onProvider: (_) {},
        onPasskey: (_) {},
        onPassword: (_, _, _) {},
      ),
    ),
  );

  goldenTest(
    'step-up of the console',
    fileName: 'console_step_up',
    builder: () => _matrix(
      'step-up',
      () => StepUpView(
        methods: const [
          StepUpMethod.passkey,
          StepUpMethod.oidcReauth,
          StepUpMethod.passwordTotp,
        ],
        oidcProviders: _providers,
        onPasskey: () {},
        onOidc: (_) {},
        onPassword: (_, _) {},
      ),
    ),
  );

  goldenTest(
    'E7 of the console',
    fileName: 'console_e7_logged_out',
    builder: () => _matrix(
      'E7',
      () => LoggedOutView(reasonCode: 'session_revoked', onLogInAgain: () {}),
    ),
  );
}
