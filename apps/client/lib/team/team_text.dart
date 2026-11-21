// SPDX-License-Identifier: Apache-2.0

/// The texts of team states and refusals (09 3.9.1, 3.9.6, 3.9.7).
library;

import 'package:design_system/design_system.dart' show dsReasonText;
import 'package:l10n/l10n.dart';

import 'team_model.dart';
import 'team_port.dart';

/// The text of a refused or failed team operation: "Refused: `reason`", the reason
/// translated where this app knows it, else the problem's text.
String teamFailureText(AppLocalizations l, TeamFailure failure) {
  if (failure.offline) return l.teamOffline;
  final reason = failure.reason;
  final text =
      teamReasonText(l, reason) ??
      (reason == null ? null : dsReasonText(l, reason)) ??
      problemText(l, failure.code!, reason: reason);
  return l.teamRefused(text);
}

/// The translated team [reason] of 09 3.9.1, or null for another one.
String? teamReasonText(AppLocalizations l, String? reason) => switch (reason) {
  'team_handle_taken' => l.teamReasonHandleTaken,
  'team_member_exists' => l.teamReasonMemberExists,
  'team_max_members' => l.teamReasonMaxMembers,
  'team_home_not_operated' => l.teamReasonHomeNotOperated,
  'team_home_is_team' => l.teamReasonHomeIsTeam,
  'team_readers_not_owners' => l.teamReasonReadersNotOwners,
  'team_not_enabled' => l.teamReasonNotEnabled,
  'dm_channel' => l.teamReasonDm,
  'channel_kind' => l.teamReasonChannelKind,
  'channel_archived' => l.teamReasonArchived,
  'channel_read_only' => l.teamReasonReadOnly,
  _ => null,
};

/// The word of a member [state] in the header.
String teamStateText(AppLocalizations l, TeamMemberState state) =>
    switch (state) {
      TeamMemberState.idle => l.teamStateIdle,
      TeamMemberState.running => l.teamStateRunning,
      TeamMemberState.paused => l.teamStatePaused,
    };

/// The pause line of the header for [pause].
String teamPauseText(AppLocalizations l, TeamPause pause) => switch (pause) {
  TeamPause.turnBudget => l.teamPausedTurnBudget,
  TeamPause.pingpong => l.teamPausedPingpong,
  TeamPause.stopped => l.teamPausedStopped,
  TeamPause.unknown => l.teamPausedOther,
};
