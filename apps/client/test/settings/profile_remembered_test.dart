// SPDX-License-Identifier: Apache-2.0

// The profile of the signed-in person (language and time zone) on a device without an answer
// of the instance (docs/decisions.d/AUD-client-settings-01.md; 01 3.6, 01 9; R-CL-03): the
// app remembers what the instance last said and shows it at the next start before the
// network is asked, and it asks again when the connection is live again. The real
// controller, auth session, local database and engine run against the fake server and
// against a listener at its address that never answers.

import 'dart:io';

import 'package:client/app/account_wiper.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:client/settings/settings_providers.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';
import '../support/silent_listener.dart';

void main() {
  // flutter_test answers every HttpClient request with 400; these tests talk to the fake.
  HttpOverrides? saved;
  late FakeInstance f;
  late ProviderContainer c;
  SilentListener? silent;

  /// A container of the app whose providers are not built again after a failure: the state
  /// of a provider that used up its retries.
  ProviderContainer container() => ProviderContainer(
    overrides: [appPortsProvider.overrideWithValue(f.ports)],
    retry: (_, _) => null,
  );

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    f = await FakeInstance.start();
    f.seed.alex
      ..locale = 'de'
      ..timeZone = 'Europe/Berlin';
    c = container();
  });

  tearDown(() async {
    c.dispose();
    await silent?.close();
    silent = null;
    await f.stop();
    HttpOverrides.global = saved;
  });

  OnboardingState state() => c.read(onboardingControllerProvider);

  Future<void> live() => eventually(
    'live',
    () => state().services?.engine.state.phase == ConnectionPhase.live,
  );

  /// Signs Alex in and waits until the profile of the instance is shown.
  Future<void> signIn() async {
    final control = await connected(c, f);
    await control.loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
    await live();
    c.listen(profileControllerProvider, (_, _) {});
    await eventually(
      'the profile of the instance',
      () => c.read(profileControllerProvider).value?.locale == 'de',
    );
  }

  /// Quits the app; its session, its keys and its local data stay.
  Future<void> quit() async {
    final closing = state().services!.close();
    c.dispose();
    await closing;
  }

  /// Starts the app again and waits for the main view.
  Future<void> startAgain() async {
    c = container();
    c.read(onboardingControllerProvider);
    await eventually(
      'the main view',
      () => state().step == OnboardingStep.signedIn,
      timeout: const Duration(seconds: 10),
    );
    c.listen(profileControllerProvider, (_, _) {});
  }

  test('a start without an answer of the instance shows the language and the '
      'time zone of the last start', () async {
    await signIn();
    await quit();
    silent = await SilentListener.replace(f.server);

    await startAgain();

    await eventually(
      'the remembered profile',
      () => c.read(profileControllerProvider).value?.locale == 'de',
      timeout: const Duration(seconds: 5),
    ).catchError((Object _) {
      fail(
        'the profile waits for the network: '
        '${c.read(profileControllerProvider)}',
      );
    });
    expect(c.read(personTimeZoneProvider), 'Europe/Berlin');
  });

  test('a change of the profile is remembered for the next start', () async {
    await signIn();
    await c
        .read(profileControllerProvider.notifier)
        .change(locale: 'en', timeZone: 'Europe/Vienna');
    await quit();
    silent = await SilentListener.replace(f.server);

    await startAgain();

    await eventually(
      'the remembered profile',
      () => c.read(profileControllerProvider).value?.locale == 'en',
      timeout: const Duration(seconds: 5),
    );
    expect(c.read(personTimeZoneProvider), 'Europe/Vienna');
  });

  test(
    'a change is not replaced by the answer of a read that was under way',
    () async {
      await signIn();
      await quit();
      // The first read of the next start is answered late, after the person changed the
      // profile: its answer names the profile from before.
      f.server.faults.delay(
        const Duration(milliseconds: 400),
        operationId: 'getMe',
      );
      await startAgain();
      await eventually(
        'the remembered profile',
        () => c.read(profileControllerProvider).value?.locale == 'de',
      );

      await c
          .read(profileControllerProvider.notifier)
          .change(locale: 'en', timeZone: 'Europe/Vienna');
      expect(c.read(profileControllerProvider).value?.locale, 'en');
      await eventually(
        'the late answer',
        () => f.server.faults.pending.isEmpty,
      );
      await Future<void>.delayed(const Duration(milliseconds: 600));

      expect(c.read(profileControllerProvider).value?.locale, 'en');
      expect(c.read(personTimeZoneProvider), 'Europe/Vienna');
      final remembered = Profile.fromRemembered(
        await state().services!.secrets.read(ProfileController.rememberedKey),
      );
      expect(remembered?.locale, 'en');
      expect(remembered?.timeZone, 'Europe/Vienna');
    },
  );

  test(
    'the profile is asked again when the connection is live again',
    () async {
      await signIn();
      await quit();
      // The instance refuses the profile while the app starts, and another device changes it
      // meanwhile.
      f.server.faults.onRequest(
        'getMe',
        status: 503,
        code: 'unavailable',
        times: 100000,
      );
      f.seed.alex
        ..locale = 'en'
        ..timeZone = 'Europe/Vienna';
      await startAgain();
      await live();
      expect(c.read(profileControllerProvider).value?.locale, 'de');

      while (f.server.faults.pending.isNotEmpty) {
        f.server.faults.take('getMe');
      }
      f.server.faults.closeGateway(1001);

      await eventually(
        'the profile of the instance after the reconnect',
        () => c.read(profileControllerProvider).value?.locale == 'en',
        timeout: const Duration(seconds: 10),
      ).catchError((Object _) {
        fail(
          'the profile was not asked again: '
          '${c.read(profileControllerProvider)}',
        );
      });
      expect(c.read(personTimeZoneProvider), 'Europe/Vienna');
    },
  );

  test(
    'a first start without a remembered profile asks the instance again when '
    'the connection is live again',
    () async {
      final control = await connected(c, f);
      f.server.faults.onRequest(
        'getMe',
        status: 503,
        code: 'unavailable',
        times: 100000,
      );
      await control.loginWithPassword(
        WorldSeed.alexLogin,
        WorldSeed.password,
        WorldSeed.totp,
      );
      await live();
      c.listen(profileControllerProvider, (_, _) {});
      await eventually(
        'the refused profile',
        () => c.read(profileControllerProvider).hasError,
      );
      expect(c.read(personTimeZoneProvider), 'UTC');

      while (f.server.faults.pending.isNotEmpty) {
        f.server.faults.take('getMe');
      }
      f.server.faults.closeGateway(1001);

      await eventually(
        'the profile of the instance after the reconnect',
        () => c.read(profileControllerProvider).value?.locale == 'de',
        timeout: const Duration(seconds: 10),
      );
      expect(c.read(personTimeZoneProvider), 'Europe/Berlin');
    },
  );

  test('the remembered profile goes with the account', () async {
    await signIn();
    await c
        .read(onboardingControllerProvider.notifier)
        .endLocally(SignOutCause.loggedOut);
    await eventually('E7', () => state().step == OnboardingStep.signedOut);
    expect(await f.wiped(), isTrue);
  });
}
