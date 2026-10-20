// SPDX-License-Identifier: Apache-2.0

// The settings in the app (brief M4-25 Scope): `/settings/<section>` from the shell, every
// section of D8, "Not found or no access" for an unknown one, the step-up dialog over the
// app's navigator, the device views in "Devices and sessions" with the end of the account
// after revoking this device, and the app language from the profile (01 9).

import 'dart:io';

import 'package:admin_ui/admin_ui.dart';
import 'package:client/app/account_wiper.dart';
import 'package:client/app/app.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/app/providers/settings.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:client/settings/settings_providers.dart';
import 'package:client/settings/settings_shell.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import '../support/fakes.dart';
import '../support/widgets.dart';

class _Onboarding extends OnboardingController {
  final List<SignOutCause> ended = [];

  @override
  OnboardingState build() =>
      const OnboardingState(step: OnboardingStep.signedIn);

  @override
  Future<void> endLocally(SignOutCause cause) async => ended.add(cause);
}

class _Profile extends ProfileController {
  _Profile(this.locale);

  final String? locale;

  @override
  Future<Profile> build() async =>
      Profile(locale: locale, timeZone: 'Europe/Berlin');
}

class _Self implements SelfRepository {
  final List<String> calls = [];

  @override
  Future<List<SelfDevice>> devices() async => [
    SelfDevice(
      id: 'laptop',
      name: 'Alex laptop',
      platform: 'macos',
      state: SelfDeviceState.active,
      createdAt: DateTime.utc(2026, 10),
      isThisDevice: true,
    ),
  ];

  @override
  Future<List<SelfSession>> sessions() async => [
    SelfSession(
      id: 's1',
      deviceId: 'laptop',
      deviceName: 'Alex laptop',
      authMethod: SelfAuthMethod.passwordTotp,
      lastUsedAt: DateTime.utc(2026, 10),
      current: true,
    ),
  ];

  @override
  Future<SelfDevice> approveDevice(
    String deviceId,
    String code, {
    required String purpose,
  }) => throw UnimplementedError();

  @override
  Future<SelfDevice> approvePairing(String code, {required String purpose}) =>
      throw UnimplementedError();

  @override
  Future<SelfDevice> revokeDevice(
    String deviceId, {
    required String reason,
    required String purpose,
  }) async {
    calls.add('revokeDevice $deviceId');
    return (await devices()).single;
  }

  @override
  Future<void> revokeSession(String sessionId) async =>
      calls.add('revokeSession $sessionId');
}

void main() {
  late Directory root;
  setUp(() async => root = Directory.systemTemp.createTempSync('settings'));
  tearDown(() => root.deleteSync(recursive: true));

  Future<(_Onboarding, _Self)> pumpApp(
    WidgetTester tester, {
    String? locale = 'en',
    List<Override> extra = const [],
  }) async {
    desktopWindow(tester);
    final control = _Onboarding();
    final self = _Self();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          appPortsProvider.overrideWithValue(testPorts(dataRoot: root)),
          onboardingControllerProvider.overrideWith(() => control),
          profileControllerProvider.overrideWith(() => _Profile(locale)),
          selfRepositoryProvider.overrideWithValue(self),
          userSettingsProvider.overrideWith((ref) => Stream.value(const {})),
          ...extra,
        ],
        child: const ClientApp(),
      ),
    );
    await tester.pumpAndSettle();
    return (control, self);
  }

  Future<void> tap(WidgetTester tester, String key) async {
    await tester.tap(find.byKey(ValueKey(key)));
    await tester.pumpAndSettle();
  }

  testWidgets('the shell opens the settings; every section of D8 opens', (
    tester,
  ) async {
    await pumpApp(tester);
    await tap(tester, 'sidebar.settings');
    expect(find.byKey(const ValueKey('screen.settings')), findsOneWidget);
    expect(find.byKey(const ValueKey('settings.theme')), findsOneWidget);

    for (final section in SettingsSection.values) {
      await tap(tester, 'settings.nav.${section.segment}');
      expect(
        find.descendant(
          of: find.byType(SingleChildScrollView),
          matching: find.text(section.label(en)),
        ),
        findsWidgets,
        reason: section.name,
      );
    }
    expect(find.byKey(const ValueKey('settings.appVersion')), findsOneWidget);

    await tap(tester, 'settings.back');
    expect(find.byKey(const ValueKey('column.sidebar')), findsOneWidget);
  });

  testWidgets('an unknown section is not found; /settings opens Appearance', (
    tester,
  ) async {
    await pumpApp(tester);
    await tap(tester, 'sidebar.settings');
    final router = GoRouter.of(tester.element(find.byType(SettingsShell)));
    router.go('/settings/nothing');
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('settings.notFound')), findsOneWidget);
    router.go('/settings');
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('settings.theme')), findsOneWidget);
  });

  testWidgets('the step-up dialog opens over the app', (tester) async {
    await pumpApp(tester);
    final ports = ProviderScope.containerOf(
      tester.element(find.byType(ClientApp)),
    ).read(appPortsProvider);
    final answer = ports.stepUpPrompt.ask('Revoke the device Alex phone');
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('stepUp.dialog')), findsOneWidget);
    expect(find.text('Revoke the device Alex phone'), findsOneWidget);
    await tap(tester, 'stepUp.continue');
    expect(await answer, isTrue);
  });

  testWidgets('revoking this device and ending this session end the account', (
    tester,
  ) async {
    final (control, self) = await pumpApp(tester);
    await tap(tester, 'sidebar.settings');
    await tap(tester, 'settings.nav.devices');
    expect(find.byKey(const ValueKey('device.thisDevice')), findsOneWidget);

    await tap(tester, 'device.laptop.revoke');
    await tap(tester, 'revoke.confirm');
    expect(self.calls, ['revokeDevice laptop']);
    expect(control.ended, [SignOutCause.revokedBySelf]);

    await tap(tester, 'session.s1.end');
    await tap(tester, 'revoke.confirm');
    expect(self.calls.last, 'revokeSession s1');
    expect(control.ended.last, SignOutCause.loggedOut);
  });

  testWidgets('the app language follows the profile', (tester) async {
    await pumpApp(tester, locale: 'de');
    await tap(tester, 'sidebar.settings');
    expect(find.text('Einstellungen'), findsWidgets);
  });
}
