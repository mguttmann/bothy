// SPDX-License-Identifier: Apache-2.0

// The cold start of a signed-in person (docs/decisions.d/AUD-client-onboarding-01.md; 01 3.6;
// D-M4-18-15): the main view opens from the local data before the network is asked, so an
// instance that never answers holds nothing back, and what the network answers afterwards is
// applied. The real controller, auth session, local database, engine and discovery client run
// against the fake server and against a listener at its address that never answers.

import 'dart:convert';
import 'dart:io';

import 'package:client/app/discovery.dart';
import 'package:client/onboarding/onboarding_controller.dart';
import 'package:client/platform/secure_store.dart';
import 'package:domain/domain.dart' show ClientVersionDecision;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart';
import 'package:sync_engine/testing.dart';

import '../support/fake_instance.dart';
import '../support/fakes.dart' show testBranding;
import '../support/silent_listener.dart';

/// The real discovery client; once [instanceId] is set, the instance it finds at the address
/// is another one, as after a new installation of the server behind the same name.
final class _OtherInstance implements DiscoveryClient {
  late DiscoveryClient real;
  String? instanceId;

  @override
  Future<DiscoveryOutcome> discover(Uri address) async {
    final outcome = await real.discover(address);
    final id = instanceId;
    if (outcome is! DiscoveryFound || id == null) return outcome;
    final found = outcome.instance;
    return DiscoveryFound(
      InstanceInfo(
        address: found.address,
        instanceId: id,
        productName: found.productName,
        providers: found.providers,
        localEnabled: found.localEnabled,
        passwordEnabled: found.passwordEnabled,
      ),
      version: outcome.version,
    );
  }

  @override
  Future<ClientVersionDecision?> checkVersion(Uri address) =>
      real.checkVersion(address);
}

void main() {
  // flutter_test answers every HttpClient request with 400; these tests talk to the fake.
  HttpOverrides? saved;
  late FakeInstance f;
  late ProviderContainer c;
  late _OtherInstance discovery;
  SilentListener? silent;

  setUp(() async {
    saved = HttpOverrides.current;
    HttpOverrides.global = null;
    discovery = _OtherInstance();
    f = await FakeInstance.start(discovery: (real) => discovery..real = real);
    c = appContainer(f);
  });

  tearDown(() async {
    c.dispose();
    await silent?.close();
    silent = null;
    await f.stop();
    HttpOverrides.global = saved;
  });

  OnboardingState state() => c.read(onboardingControllerProvider);

  /// Signs Alex in, waits until the engine is live and quits the app; its session, its keys
  /// and its local data stay. Returns the ID of the device.
  Future<String> signInAndQuit() async {
    final control = await connected(c, f);
    await control.loginWithPassword(
      WorldSeed.alexLogin,
      WorldSeed.password,
      WorldSeed.totp,
    );
    expect(state().step, OnboardingStep.signedIn);
    await eventually(
      'live',
      () => state().services?.engine.state.phase == ConnectionPhase.live,
    );
    final id = state().services!.auth.session!.deviceId.value;
    final closing = state().services!.close();
    c.dispose();
    await closing;
    return id;
  }

  /// Starts the app again and waits for the main view.
  Future<void> startAgain() async {
    c = appContainer(f);
    c.read(onboardingControllerProvider);
    await eventually(
      'the main view',
      () => state().step == OnboardingStep.signedIn,
      timeout: const Duration(seconds: 10),
    ).catchError((Object _) {
      fail(
        'the start waits for the network: step ${state().step}, '
        'status ${state().serverStatus}',
      );
    });
  }

  test('the start screen shows until the first screen is decided', () async {
    expect(c.read(onboardingControllerProvider).starting, isTrue);
    await eventually('E1', () => !state().starting);
    expect(state().step, OnboardingStep.server);
    expect(state().address, testBranding.defaultServer);
    expect(state().serverStatus, ServerStatus.idle);
  });

  test('a stored session shows the main view from the local data while the '
      'instance never answers', () async {
    await signInAndQuit();
    silent = await SilentListener.replace(f.server);
    await startAgain();
    expect(state().restored, isTrue);
    expect(state().starting, isFalse);
    expect(state().instance?.instanceId, f.server.world.instanceId);
    // The local data of the session before: the spaces the snapshot stored.
    final spaces = await state().services!.database
        .customSelect('SELECT count(*) AS n FROM spaces')
        .getSingle();
    expect(spaces.read<int>('n'), greaterThan(0));
    await eventually('the network is asked', () => silent!.connections > 0);
    expect(state().step, OnboardingStep.signedIn);
  });

  test(
    'what discovery answers after the start replaces the remembered instance',
    () async {
      await signInAndQuit();
      final before = f.server.world.productName;
      f.server.world.productName = 'Renamed Chat';
      c = appContainer(f);
      // The first screen comes from what the app remembers.
      c.read(onboardingControllerProvider);
      await eventually(
        'the main view',
        () => state().step == OnboardingStep.signedIn,
      );
      await eventually(
        'the answer of discovery',
        () => state().instance?.productName == 'Renamed Chat',
      );
      expect(before, isNot('Renamed Chat'));
      expect(state().step, OnboardingStep.signedIn);
      expect(state().restored, isTrue);
      final remembered = InstanceInfo.fromJson(
        jsonDecode((await AppPreferences(f.secure).instanceJson())!),
      );
      expect(remembered?.productName, 'Renamed Chat');
      await eventually(
        'live again',
        () => state().services?.engine.state.phase == ConnectionPhase.live,
      );
      expect(f.server.violations, isEmpty);
    },
  );

  test('another instance at the address shows its login after the start', () async {
    await signInAndQuit();
    const other = '0193f2a7-0000-7000-8000-0000000000ee';
    discovery.instanceId = other;
    c = appContainer(f);
    c.read(onboardingControllerProvider);
    await eventually(
      'E2 of the other instance',
      () =>
          state().step == OnboardingStep.login &&
          state().instance?.instanceId == other,
      timeout: const Duration(seconds: 10),
    ).catchError((Object _) {
      fail('expected E2, got ${state().step} (${state().serverStatus})');
    });
    expect(state().services?.instance.instanceId, other);
    // Nothing of the first instance is deleted: its data and its session stay.
    expect(f.instanceFolder.existsSync(), isTrue);
    expect(
      f.secure.values.keys.where(
        (k) =>
            k.contains(f.server.world.instanceId) &&
            k.endsWith('/refresh_token'),
      ),
      isNotEmpty,
    );
  });

  test('a device the server still holds for confirmation shows E4 after the start', () async {
    final id = await signInAndQuit();
    // The app stored the session of an active device; the server says it waits.
    f.server.world.devices[id]!.state = 'pending';
    c = appContainer(f);
    c.read(onboardingControllerProvider);
    await eventually(
      'E4',
      () => state().step == OnboardingStep.pending,
      timeout: const Duration(seconds: 10),
    ).catchError((Object _) {
      fail('expected E4, got ${state().step} (${state().cause})');
    });
    expect(state().code, f.server.world.devices[id]!.verificationCode);
    expect(state().deadline, isNotNull);
  });
}
