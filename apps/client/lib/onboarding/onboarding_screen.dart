// SPDX-License-Identifier: Apache-2.0

/// The onboarding screen of a route: reads the controller and the ports and shows E1 to E7
/// (docs/spec/01-product.md 5.6).
library;

import 'dart:async';

import 'package:domain/domain.dart' show AppConfigValues;
import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../app/account_wiper.dart';
import '../app/providers/ports.dart';
import '../app/update/update_notice.dart' show openUpdatePath;
import '../lock/lock_screen.dart';
import '../lock/providers.dart';
import 'e1_server_address.dart';
import 'e2_login.dart';
import 'e3_enrollment_code.dart';
import 'e4_pending_device.dart';
import 'e5_pairing.dart';
import 'e6_app_lock.dart';
import 'e7_signed_out.dart';
import 'onboarding_controller.dart';

/// Shows the screen of [step] with the state of the [OnboardingController]. The router keeps
/// [step] equal to the controller's step (`redirectFor`), except for E6, which the app lock
/// enters (brief M5-08).
class OnboardingScreen extends ConsumerWidget {
  /// Creates the screen.
  const OnboardingScreen({super.key, required this.step});

  /// The screen of the route.
  final OnboardingStep step;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(onboardingControllerProvider);
    final control = ref.read(onboardingControllerProvider.notifier);
    final ports = ref.watch(appPortsProvider);
    final config = ref.watch(appConfigProvider);
    final instance = state.instance;
    DateTime now() => ports.clock.now();
    switch (step) {
      case OnboardingStep.server || OnboardingStep.signedIn:
        return ServerAddressScreen(
          key: const ValueKey('screen.e1'),
          address: state.address,
          status: state.serverStatus,
          minVersion: state.minVersion,
          onConnect: control.connect,
          // 18 9 item 2: below the minimum version E1 offers the update path of the build.
          onUpdate: ports.updatePath.available
              ? () => unawaited(openUpdatePath(ref))
              : null,
        );
      case OnboardingStep.login:
        if (instance == null) return const SizedBox.shrink();
        return LoginScreen(
          key: const ValueKey('screen.e2'),
          instance: instance,
          passkeysAvailable: ports.passkeysFor(instance) != null,
          notice: state.notice,
          busy: state.busy,
          onProvider: control.loginWithProvider,
          onPasskey: control.loginWithPasskey,
          onPassword: control.loginWithPassword,
          onEnroll: control.openEnroll,
          onPair: control.openPairing,
          onChangeServer: control.changeServer,
        );
      case OnboardingStep.enroll:
        if (instance == null) return const SizedBox.shrink();
        return EnrollmentCodeScreen(
          key: const ValueKey('screen.e3'),
          instance: instance,
          passkeysAvailable: ports.passkeysFor(instance) != null,
          notice: state.notice,
          busy: state.busy,
          onRedeem: control.redeem,
          onBack: control.backToLogin,
        );
      case OnboardingStep.pending:
        final deadline = state.deadline;
        return PendingDeviceScreen(
          key: const ValueKey('screen.e4'),
          code: state.code ?? '',
          deadline: deadline ?? now(),
          clock: now,
          announceInterval: config.liveAnnounceMinInterval,
          onCancel: control.cancelPending,
        );
      case OnboardingStep.pair:
        return PairingScreen(
          key: const ValueKey('screen.e5'),
          code: state.code,
          deadline: state.deadline,
          expired: state.pairingExpired,
          notice: state.notice,
          busy: state.busy,
          clock: now,
          announceInterval: config.liveAnnounceMinInterval,
          onNewCode: control.openPairing,
          onBack: control.backToLogin,
        );
      case OnboardingStep.locked:
        final productName = instance?.productName ?? ports.branding.appName;
        final lock = ref.watch(appLockProvider);
        if (lock == null) {
          // No lock of a signed-in phone: the router never shows E6 then; nothing to unlock.
          return AppLockScreen(
            key: const ValueKey('screen.e6'),
            productName: productName,
            logo: instance?.logo,
            onUnlock: () {},
            onLogout: control.logout,
          );
        }
        return AppLockGate(
          lock: lock,
          productName: productName,
          logo: instance?.logo,
          onLogout: control.logout,
        );
      case OnboardingStep.signedOut:
        return SignedOutScreen(
          key: const ValueKey('screen.e7'),
          cause: state.cause ?? SignOutCause.loggedOut,
          busy: state.busy,
          onLoginAgain: control.loginAgain,
        );
    }
  }
}
