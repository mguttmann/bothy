// SPDX-License-Identifier: Apache-2.0

// The settings and device flows of the app against the fake server with two simulated devices
// of Alex, headless (brief M4-25 Tests; the same flows as integration_test/settings_sync_test
// and devices_test): settings changed on one device appear on the other, a reset too; the
// second device waits, the first confirms it with the code and step-up, then revokes it and
// the second ends in E7 with a wiped store; ending the own session logs out and wipes.

import 'dart:io';

import 'package:admin_ui/admin_ui.dart';
import 'package:auth/auth.dart';
import 'package:client/app/account_wiper.dart';
import 'package:client/app/providers/settings.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:client/settings/settings_providers.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';

/// Answers every step-up with yes and records its purpose, as a person who confirms would.
class RecordingPrompt implements StepUpPrompt {
  /// The purposes asked.
  final List<String> asked = [];

  @override
  Future<bool> ask(String purposeText) async {
    asked.add(purposeText);
    return true;
  }

  /// These tests confirm with the step-up key; a password prompt cancels.
  @override
  Future<StepUpCredentials?> askPassword(
    String purposeText, {
    bool rejected = false,
  }) async => null;
}

void main() {
  // flutter_test answers every HttpClient request with 400; these tests talk to the fake.
  HttpOverrides? saved;
  late FakeInstance first;
  late FakeInstance second;
  late ProviderContainer a;
  late ProviderContainer b;
  late RecordingPrompt prompt;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    prompt = RecordingPrompt();
    first = await FakeInstance.start(stepUpPrompt: prompt);
    second = await first.sibling();
    a = appContainer(first);
    b = appContainer(second);
    a.listen(userSettingsProvider, (_, _) {});
    b.listen(userSettingsProvider, (_, _) {});
  });

  tearDown(() async {
    a.dispose();
    b.dispose();
    await second.stopSibling();
    await first.stop();
    HttpOverrides.global = saved;
  });

  OnboardingState stateOf(ProviderContainer c) =>
      c.read(onboardingControllerProvider);

  Future<void> signIn(ProviderContainer c, FakeInstance f) async {
    final control = await connected(c, f);
    await control.loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
  }

  Future<void> live(ProviderContainer c) => eventually(
    'live',
    () => stateOf(c).services?.engine.state.phase == ConnectionPhase.live,
  );

  String deviceOf(ProviderContainer c) =>
      stateOf(c).services!.auth.session!.deviceId.value;

  test('settings stay in sync across two devices, a reset too', () async {
    await signIn(a, first);
    await signIn(b, second);
    await live(a);
    await live(b);

    final writer = a.read(settingsRepositoryProvider)!;
    await writer.set(SettingKeys.theme, 'dark');
    await writer.set(SettingKeys.enterSends, false);
    await eventually('theme and Enter sends on the second device', () {
      final s = b.read(personalSettingsProvider);
      return s.theme == 'dark' && !s.enterSends;
    });

    await writer.reset(SettingKeys.theme);
    await eventually(
      'reset on the second device',
      () => b.read(personalSettingsProvider).theme == 'system',
    );
    expect(a.read(personalSettingsProvider).theme, 'system');
    expect(first.server.requestsOf('putMySetting'), hasLength(2));
    expect(first.server.requestsOf('deleteMySetting'), hasLength(1));
    expect(first.server.violations, isEmpty);
  });

  test('confirm a waiting device with its code, then revoke it', () async {
    await signIn(a, first);
    await live(a);
    first.seed.alex.newDevicesPending = true;
    await signIn(b, second);
    expect(stateOf(b).step, OnboardingStep.pending);
    final waiting = deviceOf(b);
    final code = stateOf(b).code!;

    final self = a.read(selfRepositoryProvider)!;
    final devices = await self.devices();
    expect(devices.singleWhere((d) => d.isThisDevice).id, deviceOf(a));
    expect(
      devices.singleWhere((d) => d.id == waiting).state,
      SelfDeviceState.pending,
    );

    final confirmed = await self.approveDevice(
      waiting,
      code,
      purpose: 'Confirm the waiting device',
    );
    expect(confirmed.state, SelfDeviceState.active);
    expect(prompt.asked, ['Confirm the waiting device']);
    await eventually(
      'second device signed in',
      () => stateOf(b).step == OnboardingStep.signedIn,
    );
    await live(b);

    final revoked = await self.revokeDevice(
      waiting,
      reason: 'Lost',
      purpose: 'Revoke the device',
    );
    expect(revoked.state, SelfDeviceState.revoked);
    // The confirmation of the first action is still fresh: no second prompt.
    expect(prompt.asked, ['Confirm the waiting device']);
    await eventually(
      'second device in E7',
      () => stateOf(b).step == OnboardingStep.signedOut,
    );
    expect(await second.wiped(), isTrue);
    expect(first.server.violations, isEmpty);
  });

  test('a wrong code rejects the waiting device', () async {
    await signIn(a, first);
    first.seed.alex.newDevicesPending = true;
    await signIn(b, second);
    final result = await a
        .read(selfRepositoryProvider)!
        .approveDevice(deviceOf(b), '000000', purpose: 'Confirm');
    expect(result.state, SelfDeviceState.revoked);
    await eventually(
      'second device in E7',
      () => stateOf(b).step == OnboardingStep.signedOut,
    );
    expect(stateOf(b).cause, SignOutCause.rejected);
  });

  test('ending the own session logs out and wipes', () async {
    await signIn(a, first);
    await live(a);
    final self = a.read(selfRepositoryProvider)!;
    final current = (await self.sessions()).singleWhere((s) => s.current);

    await self.revokeSession(current.id);
    expect(prompt.asked, isEmpty);
    await a
        .read(onboardingControllerProvider.notifier)
        .endLocally(SignOutCause.loggedOut);

    // The gateway's 4012 for the ended session may have started the same end first.
    await eventually('E7', () => stateOf(a).step == OnboardingStep.signedOut);
    expect(await first.wiped(), isTrue);
  });

  test('the profile sets the language and the time zone', () async {
    await signIn(a, first);
    a.listen(profileControllerProvider, (_, _) {});
    await eventually(
      'profile',
      () => a.read(profileControllerProvider).hasValue,
    );
    await a
        .read(profileControllerProvider.notifier)
        .change(locale: 'en', timeZone: 'Europe/Berlin');
    expect(a.read(profileControllerProvider).value!.locale, 'en');
    expect(a.read(personTimeZoneProvider), 'Europe/Berlin');
    expect(first.seed.alex.locale, 'en');
  });
}
