// SPDX-License-Identifier: Apache-2.0

/// The provider of the app lock (brief M5-08 Design note 1).
library;

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../onboarding/onboarding_controller.dart';
import 'app_lock_controller.dart';

part 'providers.g.dart';

/// The app lock of the signed-in instance on a phone, or null on the desktop (a later ticket
/// brings the lock there) and while nobody is signed in.
///
/// The policy is the cached `app_lock` of the instance row, so the lock works offline and at a
/// cold start before the network; a start that resumed a stored session locks at once when the
/// lock applies (`OnboardingState.restored`), a fresh login does not.
@Riverpod(keepAlive: true)
AppLockController? appLock(Ref ref) {
  final ports = ref.watch(appPortsProvider);
  final services = ref.watch(signedInServicesProvider);
  if (!ports.formFactor.isPhone || services == null) return null;
  final controller = AppLockController(
    port: ports.appLock,
    clock: ports.clock,
    lifecycle: ports.lifecycle,
    store: SecretAppLockSwitch(services.secrets),
    policy: services.database.instanceDao.watch().map((i) => i?.appLock),
    lockAtStart: ref.read(onboardingControllerProvider).restored,
    monotonic: SleepAwareClock(ports.sinceBoot),
  );
  ref.onDispose(controller.dispose);
  return controller;
}
