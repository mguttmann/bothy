// SPDX-License-Identifier: Apache-2.0

/// One row of the own device list (docs/spec/08-identity.md 5.4: "platform, name, last
/// activity"; 01 5.1 D8).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../data/self_repository.dart';

/// The day and clock time of [at] as the person reads them: "Today 14:05", "Yesterday 9:12
/// AM", a weekday or a date, in [timeZone] (`users.time_zone`; an unknown zone reads as UTC)
/// relative to [now].
String activityText(
  BuildContext context,
  DateTime at, {
  required DateTime now,
  required String timeZone,
}) {
  final format = L10nFormat(Localizations.localeOf(context));
  try {
    return '${format.relativeDay(at, now: now, timeZone: timeZone)} '
        '${format.time(at, timeZone: timeZone)}';
  } on ArgumentError {
    return '${format.relativeDay(at, now: now, timeZone: 'UTC')} '
        '${format.time(at, timeZone: 'UTC')}';
  }
}

/// The name of the platform code [platform] (`DevicePlatform`).
String platformText(AppLocalizations l, String platform) => switch (platform) {
  'ios' => l.devicesPlatformIos,
  'android' => l.devicesPlatformAndroid,
  'macos' => l.devicesPlatformMacos,
  'windows' => l.devicesPlatformWindows,
  'web' => l.devicesPlatformWeb,
  'cli' => l.devicesPlatformCli,
  _ => l.devicesPlatformOther,
};

/// One own device: name, "This device", platform, app version, state and last activity, and
/// the actions the device's state allows.
///
/// There is no hardware label: the API's `Device` has no hardware or attestation field (brief
/// M4-25 requirement 15). [onConfirm] and [onRevoke] are null where the action does not apply
/// or the server refused it; the tile then shows no button.
class DeviceTile extends StatelessWidget {
  /// Creates the tile.
  const DeviceTile({
    super.key,
    required this.device,
    required this.now,
    required this.timeZone,
    this.onConfirm,
    this.onRevoke,
  });

  /// The device.
  final SelfDevice device;

  /// The current time, for the relative day.
  final DateTime now;

  /// The time zone of the person.
  final String timeZone;

  /// "Enter code" of a waiting device, or null.
  final VoidCallback? onConfirm;

  /// "Revoke", or null.
  final VoidCallback? onRevoke;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final details = [
      platformText(l, device.platform),
      if (device.appVersion != null) l.devicesAppVersion(device.appVersion!),
      l.devicesLastActive(
        activityText(
          context,
          device.lastSeenAt ?? device.createdAt,
          now: now,
          timeZone: timeZone,
        ),
      ),
    ];
    final state = switch (device.state) {
      SelfDeviceState.pending => l.devicesStatePending,
      SelfDeviceState.revoked => l.devicesStateRevoked,
      SelfDeviceState.active => null,
    };
    return Padding(
      key: ValueKey('device.${device.id}'),
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
      child: Wrap(
        spacing: DsSpace.s8,
        runSpacing: DsSpace.s8,
        crossAxisAlignment: WrapCrossAlignment.center,
        alignment: WrapAlignment.spaceBetween,
        children: [
          ConstrainedBox(
            constraints: const BoxConstraints(minWidth: 240),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Wrap(
                  spacing: DsSpace.s8,
                  crossAxisAlignment: WrapCrossAlignment.center,
                  children: [
                    Text(
                      device.name,
                      style: dsTextStyle(DsType.bodyStrong)
                          .copyWith(color: colors.textPrimary),
                    ),
                    if (device.isThisDevice)
                      DsBadge(
                        key: const ValueKey('device.thisDevice'),
                        text: l.devicesThisDevice,
                        background: colors.accentSubtle,
                        foreground: colors.textPrimary,
                      ),
                    if (state != null)
                      DsBadge(
                        text: state,
                        background: device.state == SelfDeviceState.pending
                            ? colors.attentionSubtle
                            : colors.dangerSubtle,
                        foreground: colors.textPrimary,
                      ),
                  ],
                ),
                Wrap(
                  spacing: DsSpace.s12,
                  children: [
                    for (final detail in details)
                      Text(
                        detail,
                        style: dsTextStyle(DsType.bodySm)
                            .copyWith(color: colors.textSecondary),
                      ),
                  ],
                ),
              ],
            ),
          ),
          Wrap(
            spacing: DsSpace.s8,
            children: [
              if (onConfirm != null)
                FilledButton(
                  key: ValueKey('device.${device.id}.confirm'),
                  onPressed: onConfirm,
                  child: Text(l.devicesEnterCode),
                ),
              if (onRevoke != null)
                OutlinedButton(
                  key: ValueKey('device.${device.id}.revoke'),
                  onPressed: onRevoke,
                  child: Text(l.devicesRevoke),
                ),
            ],
          ),
        ],
      ),
    );
  }
}
