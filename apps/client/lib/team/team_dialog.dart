// SPDX-License-Identifier: Apache-2.0

/// The team dialog of a text channel (09 3.9.1, 3.9.7; design cross-project-handoff.md MVP item
/// 6): the team mode switch, the member list with "Remove", the form that adds an agent with its
/// home channel and a handle, and "Stop all". Owners only: the channel menu and `/team` offer it
/// to an owner alone, and the server refuses everyone else (`team.update`, `team.stop`). Every
/// operation answers at once; a refusal stays in the dialog with the server's reason.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../notifications_settings/providers.dart' show isOwnerProvider;
import '../thread/providers.dart'
    show principalsByIdProvider, visibleChannelsProvider;
import 'providers.dart';
import 'team_controller.dart';
import 'team_model.dart';
import 'team_text.dart';

/// Opens the team dialog of [channel] for the signed-in owner, with [handle] filled into the
/// add form (`/team add <handle>`). Does nothing without a session.
///
/// Takes the [container] instead of a `WidgetRef`: a menu entry runs after its menu closed,
/// when the `WidgetRef` of the menu is already unmounted, so callers take the container while
/// they build ([ProviderScope.containerOf]).
Future<void> showTeamDialog(
  BuildContext context,
  ProviderContainer container,
  Channel channel, {
  String? handle,
}) async {
  final port = container.read(teamPortProvider);
  if (port == null) return;
  final controller = TeamController(
    channel: channel.id,
    port: port,
    newKey: container.read(teamKeysProvider),
    onChanged: () =>
        container.invalidate(channelTeamProvider(channel.id.value)),
  );
  unawaited(controller.load());
  try {
    await showDialog<void>(
      context: context,
      builder: (_) => Consumer(
        builder: (context, ref, _) {
          final principals =
              ref.watch(principalsByIdProvider).value ?? const {};
          return TeamDialog(
            controller: controller,
            principals: principals,
            channels: ref.watch(visibleChannelsProvider),
            initialHandle: handle,
          );
        },
      ),
    );
  } finally {
    controller.dispose();
  }
}

/// The dialog over [controller].
class TeamDialog extends StatefulWidget {
  /// Creates the dialog.
  const TeamDialog({
    super.key,
    required this.controller,
    required this.principals,
    required this.channels,
    this.initialHandle,
  });

  /// The team's state and operations.
  final TeamController controller;

  /// The stored principals: the agents to choose from and the names of the members.
  final Map<PrincipalId, Principal> principals;

  /// The channels the person sees: the home channels to choose from and their names.
  final List<Channel> channels;

  /// The handle the add form starts with, or null.
  final String? initialHandle;

  @override
  State<TeamDialog> createState() => _TeamDialogState();
}

class _TeamDialogState extends State<TeamDialog> {
  late final TextEditingController _handle = TextEditingController(
    text: widget.initialHandle ?? '',
  );
  PrincipalId? _agent;
  ChannelId? _home;

  /// Counts the members added here; a new value gives the agent field a fresh start.
  int _added = 0;

  TeamController get _c => widget.controller;

  @override
  void initState() {
    super.initState();
    _c.addListener(_changed);
    _handle.addListener(_changed);
  }

  @override
  void dispose() {
    _c.removeListener(_changed);
    _handle.dispose();
    super.dispose();
  }

  void _changed() {
    if (mounted) setState(() {});
  }

  List<Principal> get _agents {
    final agents =
        [
          for (final p in widget.principals.values)
            if (p.kind == PrincipalKind.agent) p,
        ]..sort(
          (a, b) => a.displayName.toLowerCase().compareTo(
            b.displayName.toLowerCase(),
          ),
        );
    return agents;
  }

  String _channelName(ChannelId id) {
    for (final c in widget.channels) {
      if (c.id == id) return c.name;
    }
    return id.value;
  }

  String _agentName(PrincipalId id) =>
      widget.principals[id]?.displayName ?? id.value;

  bool get _handleValid => teamHandlePattern.hasMatch(_handle.text.trim());

  bool get _canAdd =>
      !_c.busy && _agent != null && _home != null && _handleValid;

  Future<void> _add() async {
    final ok = await _c.add(
      agent: _agent!,
      home: _home!,
      handle: _handle.text.trim(),
    );
    if (ok && mounted) {
      setState(() {
        _agent = null;
        _home = null;
        _added++;
        _handle.clear();
      });
    }
  }

  Future<void> _stopAll(AppLocalizations l) async {
    final view = View.of(context);
    final direction = Directionality.of(context);
    if (await _c.stopAll()) {
      unawaited(
        SemanticsService.sendAnnouncement(view, l.teamStopped, direction),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final team = _c.team;
    final failure = _c.failure;
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final agent = _agent;
    final homes = agent == null
        ? const <Channel>[]
        : teamHomeCandidates(widget.channels, agent, team: _c.channel);
    final handleText = _handle.text.trim();
    return AlertDialog(
      key: const ValueKey('team.dialog'),
      title: Text(l.teamTitle),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 480),
        child: SingleChildScrollView(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            mainAxisSize: MainAxisSize.min,
            children: [
              SwitchListTile(
                key: const ValueKey('team.switch'),
                contentPadding: EdgeInsets.zero,
                title: Text(l.teamSwitch),
                value: _c.enabled,
                onChanged: !_c.loaded || _c.busy
                    ? null
                    : (on) => unawaited(_c.setEnabled(on: on)),
              ),
              Text(l.teamExplanation, style: secondary),
              if (failure != null) ...[
                const SizedBox(height: DsSpace.s8),
                Semantics(
                  liveRegion: true,
                  child: Text(
                    teamFailureText(l, failure),
                    key: const ValueKey('team.failure'),
                    style: dsTextStyle(DsType.bodySm)
                        .copyWith(color: colors.dangerFg),
                  ),
                ),
              ],
              if (team != null) ...[
                const SizedBox(height: DsSpace.s16),
                Text(l.teamMembers, style: dsTextStyle(DsType.label)),
                const SizedBox(height: DsSpace.s4),
                if (team.members.isEmpty)
                  Text(
                    l.teamNoMembers,
                    key: const ValueKey('team.empty'),
                    style: secondary,
                  ),
                for (final m in team.members)
                  ListTile(
                    key: ValueKey('team.member.${m.handle}'),
                    contentPadding: EdgeInsets.zero,
                    dense: true,
                    title: Text('@${m.handle}'),
                    subtitle: Text(
                      l.teamMemberLine(
                        dsIsolate(_agentName(m.agentId)),
                        dsIsolate(_channelName(m.homeChannelId)),
                      ),
                    ),
                    trailing: IconButton(
                      key: ValueKey('team.remove.${m.handle}'),
                      tooltip: l.teamRemove(m.handle),
                      onPressed: _c.busy
                          ? null
                          : () => unawaited(_c.remove(m.id)),
                      icon: DsIcon(
                        DsIcons.actionDelete,
                        size: DsIconSizeRole.sm,
                        decorative: true,
                      ),
                    ),
                  ),
                const SizedBox(height: DsSpace.s16),
                Text(l.teamAddTitle, style: dsTextStyle(DsType.label)),
                const SizedBox(height: DsSpace.s8),
                DsDropdownField<PrincipalId>(
                  // A new key after an add: the field keeps its text otherwise.
                  key: ValueKey('team.add.agent.$_added'),
                  label: l.teamAddAgent,
                  enableFilter: true,
                  entries: [
                    for (final a in _agents)
                      DropdownMenuEntry(
                        value: a.id,
                        label: dsIsolate(a.displayName),
                      ),
                  ],
                  initialSelection: _agent,
                  onSelected: (id) => setState(() {
                    _agent = id;
                    final candidates = id == null
                        ? const <Channel>[]
                        : teamHomeCandidates(
                            widget.channels,
                            id,
                            team: _c.channel,
                          );
                    // One candidate is chosen at once; several wait for the person.
                    _home = candidates.length == 1
                        ? candidates.single.id
                        : null;
                  }),
                ),
                const SizedBox(height: DsSpace.s8),
                if (agent != null && homes.isEmpty)
                  Text(
                    l.teamAddHomeNone,
                    key: const ValueKey('team.add.noHome'),
                    style: secondary,
                  )
                else
                  DsDropdownField<ChannelId>(
                    // A new agent gives the field new choices and a new start value.
                    key: ValueKey('team.add.home.${agent?.value}'),
                    label: l.teamAddHome,
                    enabled: agent != null,
                    entries: [
                      for (final c in homes)
                        DropdownMenuEntry(
                          value: c.id,
                          label: dsIsolate(c.name),
                        ),
                    ],
                    initialSelection: _home,
                    onSelected: (id) => setState(() => _home = id),
                  ),
                const SizedBox(height: DsSpace.s8),
                TextField(
                  key: const ValueKey('team.add.handle'),
                  controller: _handle,
                  autocorrect: false,
                  decoration: InputDecoration(
                    labelText: l.teamAddHandle,
                    helperText: l.teamAddHandleHint,
                    errorText: handleText.isEmpty || _handleValid
                        ? null
                        : l.teamAddHandleInvalid,
                    prefixText: '@',
                  ),
                  onSubmitted: (_) {
                    if (_canAdd) unawaited(_add());
                  },
                ),
                const SizedBox(height: DsSpace.s8),
                Align(
                  alignment: AlignmentDirectional.centerEnd,
                  child: DsButton(
                    key: const ValueKey('team.add.submit'),
                    label: l.teamAddSubmit,
                    style: DsButtonStyle.primary,
                    onPressed: _canAdd ? () => unawaited(_add()) : null,
                  ),
                ),
              ],
            ],
          ),
        ),
      ),
      actions: [
        if (team != null)
          Tooltip(
            message: l.teamStopAllSemantics,
            child: DsButton(
              key: const ValueKey('team.stopAll'),
              label: l.teamStopAll,
              style: DsButtonStyle.danger,
              onPressed: _c.busy ? null : () => unawaited(_stopAll(l)),
            ),
          ),
        DsButton(
          key: const ValueKey('team.close'),
          label: l.teamClose,
          onPressed: () => Navigator.of(context).pop(),
        ),
      ],
    );
  }
}

/// The entry "Team channel..." of the menu of [channel] for an owner in a channel that can be
/// a team channel ([teamCapable]), or none.
List<Widget> teamMenuItems(
  BuildContext context,
  WidgetRef ref,
  Channel channel,
) {
  if (!teamCapable(channel) || !(ref.watch(isOwnerProvider).value ?? false)) {
    return const [];
  }
  // Taken now: the entry runs after the menu closed and its ref is gone.
  final container = ProviderScope.containerOf(context, listen: false);
  return [
    DsMenuItem(
      key: const ValueKey('menu.team'),
      onPressed: () => unawaited(showTeamDialog(context, container, channel)),
      child: Text(AppLocalizations.of(context).teamMenu),
    ),
  ];
}
