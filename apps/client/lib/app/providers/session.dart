// SPDX-License-Identifier: Apache-2.0

/// Login, store, engine and connection state as providers (docs/spec/15-client-architektur.md
/// 4: "connection, sync state, login" are `Notifier`s over `sync_engine` and `auth`).
library;

import 'package:local_store/local_store.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:sync_engine/sync_engine.dart';

import '../../onboarding/onboarding_controller.dart';
import '../bootstrap.dart';
import '../discovery.dart';

part 'session.g.dart';

/// The services of the signed-in instance, or null before the login and after a wipe.
@Riverpod(keepAlive: true)
InstanceServices? signedInServices(Ref ref) {
  final state = ref.watch(onboardingControllerProvider);
  return state.step == OnboardingStep.signedIn ? state.services : null;
}

/// The instance whose branding the app shows (01 5.6: before the login only name, logo and
/// accent), or null before E1 succeeded.
@Riverpod(keepAlive: true)
InstanceInfo? currentInstance(Ref ref) =>
    ref.watch(onboardingControllerProvider).instance;

/// The local database of the signed-in instance.
@Riverpod(keepAlive: true)
LocalDatabase? localDatabase(Ref ref) =>
    ref.watch(signedInServicesProvider)?.database;

/// The sync engine of the signed-in instance.
@Riverpod(keepAlive: true)
SyncEngine? syncEngine(Ref ref) => ref.watch(signedInServicesProvider)?.engine;

/// The connection state of the engine (15 7.2); offline while no engine runs.
@Riverpod(keepAlive: true)
Stream<ConnectionState> connectionState(Ref ref) async* {
  final engine = ref.watch(syncEngineProvider);
  if (engine == null) {
    yield const ConnectionState(ConnectionPhase.offline);
    return;
  }
  yield engine.state;
  yield* engine.states;
}
