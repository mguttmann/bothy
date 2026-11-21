// SPDX-License-Identifier: Apache-2.0

// A local database the device can no longer open (docs/decisions.d/AUD-client-onboarding-01.md;
// 15 6.3, 9; docs/decisions.d/FIX-client-db-key.md): the secret store lost the database key,
// which the app never replaces while the file exists, or the file does not belong to the stored
// key, or the device cannot read the file at the moment. The real controller, auth session, encrypted local database
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

  // FIX-client-db-key: the key of an existing database is never replaced, and a database the
  // device cannot read at the moment is neither "unreadable" nor deleted.
  String keyName() =>
      'instance/${f.server.world.instanceId}/${SecureDatabaseKeySource.keyName}';

  Future<void> signedInAndQuit() async {
    final control = await connected(c, f);
    await login(control);
    await eventually(
      'live',
      () => state().services?.engine.state.phase == ConnectionPhase.live,
    );
    await quit();
  }

  Future<void> restart() async {
    c = appContainer(f);
    c.read(onboardingControllerProvider);
    await eventually(
      'the start settles',
      () => !state().starting && !state().busy,
    );
  }

  /// Sets [mode] on the -wal and -shm files (created empty when missing): the database file
  /// opens and the key check fails with SQLITE_CANTOPEN, as the files of a locked iPhone do.
  void chmod(String mode) {
    for (final suffix in ['-wal', '-shm']) {
      final file = File('${database().path}$suffix');
      if (!file.existsSync()) file.writeAsBytesSync([]);
      Process.runSync('chmod', [mode, file.path]);
    }
  }

  test('a stored session whose database key is gone gets no new key, only the reset', () async {
    await signedInAndQuit();
    f.secure.values.remove(keyName());
    final stored = database().lengthSync();
    await restart();
    expect(
      (state().step, state().serverStatus),
      (OnboardingStep.server, ServerStatus.localDataUnreadable),
    );
    expect(
      f.secure.values,
      isNot(contains(keyName())),
      reason: 'a new key never opens the stored file',
    );
    expect(database().lengthSync(), stored);
    await c.read(onboardingControllerProvider.notifier).resetLocalData();
    expect(state().step, OnboardingStep.signedIn);
    expect(f.secure.values, contains(keyName()));
  });

  test('a database key the locked keychain refuses keeps the key and the file, '
      'and "Connect" opens them once the device is unlocked', () async {
    await signedInAndQuit();
    final key = f.secure.values[keyName()];
    final stored = database().lengthSync();
    f.secure.lockedKeys.add(keyName());
    await restart();
    expect(
      (state().step, state().serverStatus),
      (OnboardingStep.server, ServerStatus.keychainUnavailable),
    );
    expect(f.secure.values[keyName()], key);
    expect(database().lengthSync(), stored);
    f.secure.lockedKeys.clear();
    await c
        .read(onboardingControllerProvider.notifier)
        .connect(f.server.baseUri.toString());
    expect(state().step, OnboardingStep.signedIn);
    expect(f.secure.values[keyName()], key);
  });

  for (final session in [true, false]) {
    test('a database file the device cannot read at the moment is no key error '
        'and is never deleted (session: $session)', () async {
      if (session) {
        await signedInAndQuit();
      } else {
        await connected(c, f);
        expect(state().step, OnboardingStep.login);
        await quit();
      }
      final key = f.secure.values[keyName()];
      final stored = database().lengthSync();
      chmod('000');
      addTearDown(() => chmod('600'));
      if (session) {
        await restart();
      } else {
        c = appContainer(f);
        await connected(c, f);
      }
      expect(
        (state().step, state().serverStatus),
        (OnboardingStep.server, ServerStatus.openFailed),
      );
      expect(f.secure.values[keyName()], key);
      chmod('600');
      expect(database().lengthSync(), stored, reason: 'nothing is deleted');
    }, testOn: 'mac-os || linux');
  }
}
