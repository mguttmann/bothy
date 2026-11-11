// SPDX-License-Identifier: Apache-2.0

// A local database the device can no longer open (docs/decisions.d/AUD-client-onboarding-01.md;
// 15 6.3, 9): the secret store lost the database key and created a new one, or the file does
// not belong to the stored key. The real controller, auth session, encrypted local database
// and engine run against the fake server.

import 'dart:convert';
import 'dart:io';

import 'package:client/onboarding/onboarding_controller.dart';
import 'package:client/platform/secure_store.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';

void main() {
  // flutter_test answers every HttpClient request with 400; these tests talk to the fake.
  HttpOverrides? saved;
  late FakeInstance f;
  late ProviderContainer c;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    f = await FakeInstance.start();
    c = appContainer(f);
  });

  tearDown(() async {
    c.dispose();
    await f.stop();
    HttpOverrides.global = saved;
  });

  OnboardingState state() => c.read(onboardingControllerProvider);
  File database() => File('${f.instanceFolder.path}/local.db');

  Future<void> login(OnboardingController control) => control.loginWithPassword(
    WorldSeed.alexLogin,
    WorldSeed.password,
    WorldSeed.totp,
  );

  /// Quits the app: the services close, the files, keys and secrets stay.
  Future<void> quit() async {
    final closing = state().services?.close();
    c.dispose();
    await closing;
  }

  test(
    'a database whose key went with the lost secrets does not block the login',
    () async {
      // The app connected before: the database of the instance exists, encrypted with a key
      // of the secret store.
      await connected(c, f);
      expect(state().step, OnboardingStep.login);
      expect(database().existsSync(), isTrue);
      await quit();
      // The secret store lost everything (a deleted secrets file, another keychain): the next
      // start creates a new database key, which does not open the stored file.
      f.secure.values.clear();
      c = appContainer(f);
      final control = await connected(c, f);
      expect(
        (state().step, state().serverStatus),
        (OnboardingStep.login, ServerStatus.idle),
        reason: 'E1 must lead on to E2',
      );
      await login(control);
      expect(state().step, OnboardingStep.signedIn);
      await eventually(
        'live',
        () => state().services?.engine.state.phase == ConnectionPhase.live,
      );
      expect(f.server.violations, isEmpty);
    },
  );

  test(
    'a stored session whose database the key does not open offers the reset, '
    'which keeps the session and loads everything again',
    () async {
      final control = await connected(c, f);
      await login(control);
      await eventually(
        'live',
        () => state().services?.engine.state.phase == ConnectionPhase.live,
      );
      final device = state().services!.auth.session!.deviceId.value;
      await quit();
      // The session stays, the database key is another one: the file is unreadable.
      final keyName =
          'instance/${f.server.world.instanceId}/'
          '${SecureDatabaseKeySource.keyName}';
      expect(f.secure.values, contains(keyName));
      f.secure.values[keyName] = base64Encode(List<int>.filled(32, 7));
      final stored = database().lengthSync();

      c = appContainer(f);
      c.read(onboardingControllerProvider);
      await eventually(
        'E1 with its result',
        () => !state().starting && !state().busy,
      );
      expect(
        (state().step, state().serverStatus),
        (OnboardingStep.server, ServerStatus.localDataUnreadable),
      );
      expect(state().address, f.server.baseUri.toString());
      expect(state().services, isNull);
      // Nothing is deleted without the person's word: the file holds the unsent work.
      expect(database().lengthSync(), stored);

      await c.read(onboardingControllerProvider.notifier).resetLocalData();
      expect(state().step, OnboardingStep.signedIn);
      expect(state().services!.auth.session!.deviceId.value, device);
      expect(f.server.world.devices.keys, [device], reason: 'no new login');
      await eventually(
        'live again',
        () => state().services?.engine.state.phase == ConnectionPhase.live,
      );
      final spaces = await state().services!.database
          .customSelect('SELECT count(*) AS n FROM spaces')
          .getSingle();
      expect(spaces.read<int>('n'), greaterThan(0));
      expect(f.server.violations, isEmpty);
    },
  );
}
