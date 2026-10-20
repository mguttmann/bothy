// SPDX-License-Identifier: Apache-2.0

/// Devices and sessions: the views of `admin_ui` in the desktop settings
/// (docs/spec/01-product.md 5.1 D8; 08 5, 10; brief M4-25 Scope).
library;

import 'dart:async';

import 'package:admin_ui/admin_ui.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../app/account_wiper.dart';
import '../onboarding/onboarding_controller.dart';
import 'settings_providers.dart';
import 'settings_shell.dart';

/// The Devices and sessions section: [MyDevicesView] and [MySessionsView] over the REST source
/// of the signed-in instance. Revoking this device or ending the current session ends the
/// account here: the wiper deletes every local datum and E7 shows (08 10, R-SY-14). A change
/// in one list loads the other again.
class DevicesSection extends ConsumerStatefulWidget {
  /// Creates the section; [clock] is the time the lists show relative days for.
  const DevicesSection({super.key, this.clock = DateTime.now});

  /// The current time.
  final DateTime Function() clock;

  @override
  ConsumerState<DevicesSection> createState() => _DevicesSectionState();
}

class _DevicesSectionState extends ConsumerState<DevicesSection> {
  final _Pulse _devicesChanged = _Pulse();
  final _Pulse _sessionsChanged = _Pulse();

  @override
  void dispose() {
    _devicesChanged.dispose();
    _sessionsChanged.dispose();
    super.dispose();
  }

  void _end(SignOutCause cause) => unawaited(
    ref.read(onboardingControllerProvider.notifier).endLocally(cause),
  );

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final repository = ref.watch(selfRepositoryProvider);
    final zone = ref.watch(personTimeZoneProvider);
    return SectionFrame(
      title: l.settingsSectionDevices,
      children: [
        if (repository != null) ...[
          MyDevicesView(
            repository: repository,
            timeZone: zone,
            clock: widget.clock,
            reload: _devicesChanged,
            onChanged: _sessionsChanged.pulse,
            onThisDeviceRevoked: () => _end(SignOutCause.revokedBySelf),
          ),
          const SizedBox(height: DsSpace.s24),
          MySessionsView(
            repository: repository,
            timeZone: zone,
            clock: widget.clock,
            reload: _sessionsChanged,
            onChanged: _devicesChanged.pulse,
            onCurrentSessionEnded: () => _end(SignOutCause.loggedOut),
          ),
        ],
      ],
    );
  }
}

/// Tells one list that the other one changed something.
class _Pulse extends ChangeNotifier {
  void pulse() => notifyListeners();
}
