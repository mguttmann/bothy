// SPDX-License-Identifier: Apache-2.0

/// The providers of the phone terminal (brief M5-12): the coordinator of detaching and
/// reattaching with the life cycle and the app lock.
library;

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../app/providers/ports.dart';
import '../../app/providers/session.dart';
import '../../lock/providers.dart';
import 'terminal_lifecycle.dart';

part 'providers.g.dart';

/// The coordinator of the signed-in phone, or null on the desktop and before the login; the
/// terminal screen M8 registers its tabs with it.
@Riverpod(keepAlive: true)
PhoneTerminalCoordinator? phoneTerminalCoordinator(Ref ref) {
  final ports = ref.watch(appPortsProvider);
  if (!ports.formFactor.isPhone) return null;
  if (ref.watch(signedInServicesProvider) == null) return null;
  final lock = ref.watch(appLockProvider);
  final coordinator = PhoneTerminalCoordinator(
    lifecycle: ports.lifecycle,
    lock: lock == null ? null : TerminalAppLockGate(lock),
  );
  ref.onDispose(coordinator.dispose);
  return coordinator;
}
