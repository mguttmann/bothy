// SPDX-License-Identifier: Apache-2.0

// A10 in the browser (brief M6-09 Tests): the owner adds an OIDC provider and, after logging
// out, sees its button on E2. It logs the console out, so it runs last in the bundle. The
// provider points to an example issuer; adding it reads nothing from the issuer (08 2.6 checks
// the configuration only), so no identity provider has to run in the Compose network (Ruling
// D-M6-09-5).

import 'package:console/console.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';

import 'support.dart';

void main() {
  patrolTest('A10 add an OIDC provider and see its button after logout', (
    $,
  ) async {
    await openConsole($, '/instance');
    await tapKey($, 'instance.tab.providers');
    await waitFor($, byKey('providers.redirect.uri'));
    await tapKey($, 'providers.add');
    await tapKey($, 'providers.add.oidc');
    await typeKey($, 'provider.name', 'Example login');
    await typeKey($, 'provider.oidc.issuer', 'https://login.example.com');
    await typeKey($, 'provider.oidc.clientId', 'console-e2e');
    await tapKey($, 'provider.oidc.clientAuth');
    await tapOn($, find.text('client_secret_basic'));
    await typeKey($, 'provider.oidc.secretRef', 'env:BOTHY_IDP_EXAMPLE');
    await tapKey($, 'provider.save');
    await tapKey($, 'confirm.submit');
    await waitFor($, find.text('Example login is added.'));
    expectStepUp();

    await controllerOf($).logout();
    await $.tester.pumpAndSettle();
    if (!$.tester.testTextInput.isRegistered) $.tester.testTextInput.register();
    await $.tester.pumpWidget(
      ConsoleApp(
        key: UniqueKey(),
        environment: ConsoleEnvironment.browser(),
        initialLocation: Uri(path: '/'),
        locale: const Locale('en'),
      ),
    );
    await waitFor($, find.text('Log in with Example login'));
  });
}
