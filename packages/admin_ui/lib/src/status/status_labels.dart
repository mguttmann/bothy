// SPDX-License-Identifier: Apache-2.0

/// Words, icons and state roles of the status page (brief M7-21 requirements 9 and 16): every
/// state has a word and an icon besides its colour, and colours come only from the state roles
/// of the design tokens. Maintenance (a window or the maintenance mode) uses the role `info`,
/// so it never looks like an outage (`warning`, `danger`).
library;

import 'package:api_client/api_client.dart' as api;
import 'package:design_system/design_system.dart';
import 'package:flutter/widgets.dart';
import 'package:l10n/l10n.dart';

/// The look of one state: its [label], its [icon] and its [color] from a state role.
typedef StatusLook = ({String label, DsGlyph icon, Color color});

/// The look of the component state [state] in [context].
///
/// Example: `statusStateLook(context, StatusComponentState.degraded).label` is "Degraded".
StatusLook statusStateLook(
  BuildContext context,
  api.StatusComponentState state,
) {
  final l = AppLocalizations.of(context);
  final c = context.dsColors;
  return switch (state) {
    api.StatusComponentState.degraded => (
      label: l.statusStateDegraded,
      icon: DsIcons.stateWarning,
      color: c.warningFg,
    ),
    api.StatusComponentState.partialOutage => (
      label: l.statusStatePartialOutage,
      icon: DsIcons.stateWarning,
      color: c.attentionFg,
    ),
    api.StatusComponentState.majorOutage => (
      label: l.statusStateMajorOutage,
      icon: DsIcons.stateError,
      color: c.dangerFg,
    ),
    api.StatusComponentState.maintenance => (
      label: l.statusStateMaintenance,
      icon: DsIcons.stateInfo,
      color: c.infoFg,
    ),
    // operational and a value of a newer server.
    _ => (
      label: l.statusStateOperational,
      icon: DsIcons.runCompleted,
      color: c.successFg,
    ),
  };
}

/// The word of the incident state [state].
String statusIncidentStateLabel(
  AppLocalizations l,
  api.StatusIncidentState state,
) => switch (state) {
  api.StatusIncidentState.scheduled => l.statusIncidentScheduled,
  api.StatusIncidentState.identified => l.statusIncidentIdentified,
  api.StatusIncidentState.monitoring => l.statusIncidentMonitoring,
  api.StatusIncidentState.resolved => l.statusIncidentResolved,
  _ => l.statusIncidentInvestigating,
};

/// The word of the impact [impact].
String statusImpactLabel(AppLocalizations l, api.StatusIncidentImpact impact) =>
    switch (impact) {
      api.StatusIncidentImpact.minor => l.statusImpactMinor,
      api.StatusIncidentImpact.major => l.statusImpactMajor,
      api.StatusIncidentImpact.critical => l.statusImpactCritical,
      _ => l.statusImpactNone,
    };

/// The word of the kind [kind]: an incident or a maintenance window.
String statusKindLabel(AppLocalizations l, api.StatusIncidentKind kind) =>
    kind == api.StatusIncidentKind.maintenance
    ? l.statusKindMaintenance
    : l.statusKindIncident;

/// [instant] as the day and clock time in [timeZone] seen from [now] ("Today 14:05"); a time
/// zone the database does not know falls back to UTC.
String statusTime(
  Locale locale,
  DateTime instant, {
  required DateTime now,
  required String timeZone,
}) {
  final format = L10nFormat(locale);
  String at(String zone) =>
      '${format.relativeDay(instant, now: now, timeZone: zone)} '
      '${format.time(instant, timeZone: zone)}';
  try {
    return at(timeZone);
  } on ArgumentError {
    return at('UTC');
  }
}

/// A state with its icon and word, in the colour of its role; the icon is decorative because
/// the word says the same.
class StatusStateChip extends StatelessWidget {
  /// Creates the chip for [look].
  const StatusStateChip({super.key, required this.look, this.large = false});

  /// The state's look.
  final StatusLook look;

  /// Whether it is the overall state at the top of the page.
  final bool large;

  @override
  Widget build(BuildContext context) => Row(
    mainAxisSize: MainAxisSize.min,
    children: [
      DsIcon(
        look.icon,
        size: large ? DsIconSizeRole.md : DsIconSizeRole.sm,
        color: look.color,
        decorative: true,
      ),
      const SizedBox(width: DsSpace.s4),
      Flexible(
        child: Text(
          look.label,
          style: dsTextStyle(large ? DsType.title : DsType.bodySm)
              .copyWith(color: look.color),
        ),
      ),
    ],
  );
}
