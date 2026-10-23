// SPDX-License-Identifier: Apache-2.0

// The login providers (brief M6-09 Design note 3, Tests): the OIDC form with the fields of
// 08 2.6 and the redirect URI to register, secret references written only (a stored reference
// is never shown and an empty field keeps it), the proxy header form with the loopback rule and
// the server's 409, and disable and delete dialogs that say the provider's sessions end
// (R-ID-12); every change goes through the step-up.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

const String _secretRef = 'file:/run/bothy/secrets/idp/EXAMPLE-client';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository r;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    r = instanceRepository(navigator);
  });

  Future<void> pump(WidgetTester tester) =>
      pumpInstance(tester, ProvidersSection(repository: r), navigator);

  Future<void> openAdd(WidgetTester tester, String kind) async {
    await tester.tap(find.byKey(const ValueKey('providers.add')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(ValueKey('providers.add.$kind')));
    await tester.pumpAndSettle();
  }

  Future<void> type(WidgetTester tester, String key, String text) async {
    final field = find.byKey(ValueKey(key));
    await tester.ensureVisible(field);
    await tester.enterText(field, text);
    await tester.pumpAndSettle();
  }

  Future<void> save(WidgetTester tester) async {
    final button = find.byKey(const ValueKey('provider.save'));
    await tester.ensureVisible(button);
    await tester.tap(button);
    await tester.pumpAndSettle();
  }

  /// Every text the screen shows: rendered, in fields and selectable.
  List<String> screenTexts(WidgetTester tester) => [
    for (final e in find.byType(RichText).evaluate())
      (e.widget as RichText).text.toPlainText(),
    for (final e in find.byType(EditableText).evaluate())
      (e.widget as EditableText).controller.text,
  ];

  testWidgets('the redirect URI is shown and copied', (tester) async {
    String? copied;
    tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
      SystemChannels.platform,
      (call) async {
        if (call.method == 'Clipboard.setData') {
          copied = (call.arguments as Map)['text'] as String;
        }
        return null;
      },
    );
    addTearDown(
      () => tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
        SystemChannels.platform,
        null,
      ),
    );
    await pump(tester);
    expect(find.byKey(const ValueKey('providers.empty')), findsOneWidget);
    expect(
      tester
          .widget<SelectableText>(
            find.byKey(const ValueKey('providers.redirect.uri')),
          )
          .data,
      fakeRedirectUri,
    );
    await tester.tap(find.byKey(const ValueKey('providers.redirect.copy')));
    await tester.pumpAndSettle();
    expect(copied, fakeRedirectUri);
    expect(find.text('Copied'), findsOneWidget);
  });

  testWidgets('add an OIDC provider with a write-only secret reference', (
    tester,
  ) async {
    await pump(tester);
    await openAdd(tester, 'oidc');
    expect(find.byKey(const ValueKey('provider.editor')), findsOneWidget);
    expect(find.byKey(const ValueKey('provider.redirect.uri')), findsOneWidget);
    await type(tester, 'provider.name', 'Company login');
    await type(tester, 'provider.oidc.issuer', 'https://login.example.com');
    await type(tester, 'provider.oidc.clientId', 'bothy-example');
    await pickOption(tester, 'provider.oidc.clientAuth', 'client_secret_basic');
    final secret = find.byKey(const ValueKey('provider.oidc.secretRef'));
    expect(
      tester
          .widget<EditableText>(
            find.descendant(of: secret, matching: find.byType(EditableText)),
          )
          .obscureText,
      isTrue,
    );
    expect(find.byKey(const ValueKey('provider.oidc.keyAlg')), findsNothing);
    await type(tester, 'provider.oidc.secretRef', _secretRef);
    await save(tester);

    final body = tester
        .widget<Text>(find.byKey(const ValueKey('confirm.body')))
        .data!;
    expect(body, contains('A new OpenID Connect provider named Company login'));
    expect(body, contains('Issuer: https://login.example.com'));
    expect(body, isNot(contains(_secretRef)));
    await confirmChange(tester);
    await confirmStepUp(tester, 'Add the login provider Company login');

    expect(r.providerBodies.single['config'], {
      'issuer': 'https://login.example.com',
      'client_id': 'bothy-example',
      'client_auth': 'client_secret_basic',
      'client_secret_ref': _secretRef,
    });
    expect(find.text('Company login is added.'), findsOneWidget);
    expect(textOf(tester, 'provider.idp-1.state'), 'OpenID Connect · Enabled');
    // The stored reference is shown nowhere.
    expect(screenTexts(tester).where((t) => t.contains('EXAMPLE')), isEmpty);
  });

  testWidgets('the server names the missing reference at its field', (
    tester,
  ) async {
    r.stepUpFresh = true;
    await pump(tester);
    await openAdd(tester, 'oidc');
    await type(tester, 'provider.name', 'Company login');
    await type(tester, 'provider.oidc.issuer', 'https://login.example.com');
    await type(tester, 'provider.oidc.clientId', 'bothy-example');
    await save(tester);
    await confirmChange(tester);

    final key = tester.widget<TextField>(
      find.byKey(const ValueKey('provider.oidc.keyRef')),
    );
    expect(key.decoration!.errorText, 'This field is required.');
    expect(find.byKey(const ValueKey('provider.editor')), findsOneWidget);
    expect(r.providersById, isEmpty);
  });

  testWidgets('editing keeps the stored reference without showing it', (
    tester,
  ) async {
    r.stepUpFresh = true;
    final p = r.seedProvider(ProviderKind.oidc, 'Company login', {
      'issuer': 'https://login.example.com',
      'client_id': 'bothy-example',
      'client_auth': 'client_secret_basic',
      'client_secret_ref': _secretRef,
      'groups_claim': 'groups',
    });
    await pump(tester);
    expect(screenTexts(tester).where((t) => t.contains('EXAMPLE')), isEmpty);
    await tester.tap(find.byKey(ValueKey('provider.${p.id}.edit')));
    await tester.pumpAndSettle();
    final secret = tester.widget<TextField>(
      find.byKey(const ValueKey('provider.oidc.secretRef')),
    );
    expect(secret.controller!.text, isEmpty);
    expect(
      secret.decoration!.helperText,
      contains('Leave empty to keep the stored reference.'),
    );
    expect(screenTexts(tester).where((t) => t.contains('EXAMPLE')), isEmpty);

    await type(tester, 'provider.oidc.clientId', 'bothy-chat');
    await save(tester);
    final body = tester
        .widget<Text>(find.byKey(const ValueKey('confirm.body')))
        .data!;
    expect(body, 'Changed settings: client_id');
    await confirmChange(tester);

    expect(r.providerBodies.single['config'], {
      'issuer': 'https://login.example.com',
      'client_id': 'bothy-chat',
      'client_auth': 'client_secret_basic',
      'client_secret_ref': _secretRef,
      'groups_claim': 'groups',
    });
    expect(r.providerBodies.single.containsKey('name'), isFalse);
  });

  testWidgets('a new reference is named in the summary, never shown', (
    tester,
  ) async {
    r.stepUpFresh = true;
    final p = r.seedProvider(ProviderKind.oidc, 'Company login', {
      'issuer': 'https://login.example.com',
      'client_id': 'bothy-example',
      'client_auth': 'client_secret_basic',
      'client_secret_ref': _secretRef,
    });
    await pump(tester);
    await tester.tap(find.byKey(ValueKey('provider.${p.id}.edit')));
    await tester.pumpAndSettle();
    await type(tester, 'provider.oidc.secretRef', 'env:BOTHY_IDP_ROTATED');
    await save(tester);
    expect(
      tester.widget<Text>(find.byKey(const ValueKey('confirm.body'))).data,
      'Changed settings: client_secret_ref',
    );
    await confirmChange(tester);
    expect(
      (r.providerBodies.single['config']! as Map)['client_secret_ref'],
      'env:BOTHY_IDP_ROTATED',
    );
  });

  testWidgets('the proxy header form shows the loopback rule and the 409', (
    tester,
  ) async {
    r
      ..stepUpFresh = true
      ..loopback = false;
    await pump(tester);
    await openAdd(tester, 'trusted_header');
    expect(
      find.byKey(const ValueKey('provider.header.loopback')),
      findsOneWidget,
    );
    expect(find.byKey(const ValueKey('provider.redirect.uri')), findsNothing);
    await type(tester, 'provider.name', 'Tailnet');
    await type(tester, 'provider.header.login', 'Tailscale-User-Login');
    await save(tester);
    await confirmChange(tester);

    final refused = find.byKey(const ValueKey('provider.loopbackRefused'));
    expect(refused, findsOneWidget);
    expect(
      find.descendant(
        of: refused,
        matching: find.textContaining(
          'Bind server.listen_addr to a loopback address.',
        ),
      ),
      findsOneWidget,
    );
    expect(r.providersById, isEmpty);
  });

  testWidgets('disabling warns that its sessions end, then step-up', (
    tester,
  ) async {
    final p = r.seedProvider(ProviderKind.oidc, 'Company login', {
      'issuer': 'https://login.example.com',
      'client_id': 'bothy-example',
      'client_auth': 'private_key_jwt',
      'client_key_ref': _secretRef,
    });
    await pump(tester);
    await tester.tap(find.byKey(ValueKey('provider.${p.id}.toggle')));
    await tester.pumpAndSettle();
    expect(find.text('Disable Company login?'), findsOneWidget);
    expect(
      find.textContaining(
        'All sessions that started with Company login end immediately.',
      ),
      findsOneWidget,
    );
    await confirmChange(tester);
    await confirmStepUp(tester, 'Disable the login provider Company login');

    expect(r.providersById[p.id]!.enabled, isFalse);
    expect(r.providerBodies.single, {'enabled': false});
    expect(
      find.text('Company login is disabled; its sessions ended.'),
      findsOneWidget,
    );
    expect(
      textOf(tester, 'provider.${p.id}.state'),
      'OpenID Connect · Disabled',
    );
  });

  testWidgets('enabling a proxy header provider meets the loopback rule', (
    tester,
  ) async {
    r
      ..stepUpFresh = true
      ..loopback = false;
    final p = r.seedProvider(ProviderKind.trustedHeader, 'Tailnet', {
      'login_header': 'Tailscale-User-Login',
    }, enabled: false);
    await pump(tester);
    await tester.tap(find.byKey(ValueKey('provider.${p.id}.toggle')));
    await tester.pumpAndSettle();
    expect(
      find.textContaining('listens on a loopback address'),
      findsOneWidget,
    );
    await confirmChange(tester);
    expect(
      find.textContaining(
        'The server refused: the proxy header provider needs a server',
      ),
      findsOneWidget,
    );
    expect(r.providersById[p.id]!.enabled, isFalse);
  });

  testWidgets('deleting warns and shows the hint of a provider in use', (
    tester,
  ) async {
    r.stepUpFresh = true;
    final p = r.seedProvider(ProviderKind.local, 'Local accounts', const {});
    r.providersInUse.add(p.id);
    await pump(tester);
    expect(find.byKey(ValueKey('provider.${p.id}.edit')), findsOneWidget);
    await tester.tap(find.byKey(ValueKey('provider.${p.id}.delete')));
    await tester.pumpAndSettle();
    expect(
      find.textContaining(
        'All sessions that started with Local accounts end immediately.',
      ),
      findsOneWidget,
    );
    await confirmChange(tester);
    expect(find.textContaining('disable the provider instead'), findsOneWidget);
    expect(r.providersById, contains(p.id));

    r.providersInUse.clear();
    await tester.tap(find.byKey(ValueKey('provider.${p.id}.delete')));
    await tester.pumpAndSettle();
    await confirmChange(tester);
    expect(r.providersById, isEmpty);
    expect(find.text('Local accounts is deleted.'), findsOneWidget);
  });

  testWidgets('a provider of an unknown driver cannot be edited', (
    tester,
  ) async {
    final p = r.seedProvider(ProviderKind.unknown, 'SAML', const {});
    await pump(tester);
    expect(find.byKey(ValueKey('provider.${p.id}.edit')), findsNothing);
    expect(find.byKey(ValueKey('provider.${p.id}.toggle')), findsOneWidget);
  });

  testWidgets('the form cannot be closed while its write runs', (tester) async {
    await pump(tester);
    await openAdd(tester, 'local');
    await type(tester, 'provider.name', 'Local accounts');
    await save(tester);
    await confirmChange(tester);
    // The step-up of the write is open: the form waits for its result.
    expect(find.byType(StepUpDialog), findsOneWidget);
    expect(
      tester
          .widget<TextButton>(find.byKey(const ValueKey('provider.cancel')))
          .onPressed,
      isNull,
    );
    expect(
      tester
          .widget<PopScope<dynamic>>(
            find
                .ancestor(
                  of: find.byKey(const ValueKey('provider.editor')),
                  matching: find.byWidgetPredicate((w) => w is PopScope),
                )
                .first,
          )
          .canPop,
      isFalse,
    );
    await tester.tap(find.byKey(const ValueKey('stepUp.continue')));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('provider.editor')), findsNothing);
    expect(find.text('Local accounts is added.'), findsOneWidget);
  });
}
