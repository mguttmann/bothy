// SPDX-License-Identifier: Apache-2.0

/// The member bar of a team channel under its thread header (design cross-project-handoff.md
/// MVP item 2 and 6): every member with its handle and state (idle, working, paused), the pause
/// line while member inputs are paused, "Stop all" and the entry to the team dialog. The state
/// of a member comes from the runs of its work thread in the local `runs` table
/// (`run.state_changed`, 04 11.3) and the channel's pause from `getChannelTeam`.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../agents/providers.dart' show threadRunsProvider;
import 'providers.dart';
import 'team_controller.dart';
import 'team_dialog.dart';
import 'team_model.dart';
import 'team_port.dart';
import 'team_text.dart';

/// The bar of [channel] with its [team], reading the runs of every work thread.
class TeamHeaderBar extends ConsumerWidget {
  /// Creates the bar.
  const TeamHeaderBar({super.key, required this.channel, required this.team});

  /// The team channel.
  final Channel channel;

  /// Its team.
  final TeamView team;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final states = {
      for (final m in team.members)
        m.id: teamMemberState(
          m,
          ref.watch(threadRunsProvider(m.workThreadId.value)).value ??
              const <RunSummary>[],
          team.pause,
        ),
    };
    return TeamHeaderView(
      team: team,
      states: states,
      onStopAll: () => unawaited(stopTeam(context, ref, channel.id)),
      onManage: () => unawaited(showTeamDialog(context, ref, channel)),
    );
  }
}

/// Stops every member run of [channel] (09 3.9.7) and says the outcome in a snack bar.
Future<void> stopTeam(
  BuildContext context,
  WidgetRef ref,
  ChannelId channel,
) async {
  final port = ref.read(teamPortProvider);
  if (port == null) return;
  final l = AppLocalizations.of(context);
  final messenger = ScaffoldMessenger.maybeOf(context);
  final controller = TeamController(
    channel: channel,
    port: port,
    newKey: ref.read(teamKeysProvider),
    onChanged: () => ref.invalidate(channelTeamProvider(channel.value)),
  );
  final ok = await controller.stopAll();
  final failure = controller.failure;
  controller.dispose();
  messenger?.showSnackBar(
    SnackBar(
      content: Text(
        ok || failure == null ? l.teamStopped : teamFailureText(l, failure),
      ),
    ),
  );
}

/// The bar of [team] with the member [states] by member ID.
class TeamHeaderView extends StatelessWidget {
  /// Creates the bar.
  const TeamHeaderView({
    super.key,
    required this.team,
    required this.states,
    required this.onStopAll,
    required this.onManage,
  });

  /// The team.
  final TeamView team;

  /// The state of every member by its ID.
  final Map<String, TeamMemberState> states;

  /// Stops every member run.
  final VoidCallback onStopAll;

  /// Opens the team dialog.
  final VoidCallback onManage;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final pause = team.pause;
    final anyRunning = states.values.contains(TeamMemberState.running);
    return Semantics(
      key: const ValueKey('team.header'),
      container: true,
      label: l.teamHeader,
      child: Container(
        padding: const EdgeInsets.symmetric(
          horizontal: DsSpace.s12,
          vertical: DsSpace.s6,
        ),
        decoration: BoxDecoration(
          border: Border(bottom: BorderSide(color: colors.borderSubtle)),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          mainAxisSize: MainAxisSize.min,
          children: [
            Row(
              children: [
                Expanded(
                  child: Wrap(
                    spacing: DsSpace.s6,
                    runSpacing: DsSpace.s4,
                    children: [
                      for (final m in team.members)
                        // One node per member, so a screen reader reads each on its own.
                        Semantics(
                          container: true,
                          child: _MemberChip(
                            handle: m.handle,
                            state: states[m.id] ?? TeamMemberState.idle,
                          ),
                        ),
                    ],
                  ),
                ),
                const SizedBox(width: DsSpace.s8),
                DsButton(
                  key: const ValueKey('team.header.stopAll'),
                  label: l.teamStopAll,
                  style: DsButtonStyle.danger,
                  // Nothing to stop while no member works.
                  onPressed: anyRunning ? onStopAll : null,
                ),
                const SizedBox(width: DsSpace.s4),
                IconButton(
                  key: const ValueKey('team.header.manage'),
                  tooltip: l.teamTitle,
                  onPressed: onManage,
                  icon: DsIcon(
                    DsIcons.actionSettings,
                    size: DsIconSizeRole.sm,
                    decorative: true,
                  ),
                ),
              ],
            ),
            if (pause != null) ...[
              const SizedBox(height: DsSpace.s4),
              Semantics(
                liveRegion: true,
                child: Text(
                  teamPauseText(l, pause),
                  key: const ValueKey('team.header.pause'),
                  style: dsTextStyle(
                    DsType.bodySm,
                  ).copyWith(color: dsToneColors(context, DsTone.warning).fg),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

class _MemberChip extends StatelessWidget {
  const _MemberChip({required this.handle, required this.state});

  final String handle;
  final TeamMemberState state;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final (glyph, background, foreground) = switch (state) {
      TeamMemberState.running => (
        DsIcons.runRunning,
        colors.infoSubtle,
        colors.infoFg,
      ),
      TeamMemberState.paused => (
        DsIcons.runWaiting,
        dsToneColors(context, DsTone.warning).subtle,
        dsToneColors(context, DsTone.warning).fg,
      ),
      TeamMemberState.idle => (
        DsIcons.agent,
        colors.surfaceRaised,
        colors.textSecondary,
      ),
    };
    final word = teamStateText(l, state);
    return DsBadge(
      key: ValueKey('team.header.member.$handle'),
      text: l.teamMemberSemantics(handle, word),
      glyph: glyph,
      background: background,
      foreground: foreground,
    );
  }
}
