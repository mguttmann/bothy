// SPDX-License-Identifier: Apache-2.0

/// "Create team channel..." of the space menu (01 6.23 item 6; design cross-project-handoff.md
/// section 2 item 1): for an owner with `manage` on the space (`channel.create`), a short dialog
/// asks for a name, creates a text channel (`createChannel`, kind `text`, with an
/// `Idempotency-Key`), switches it to team mode (`enableChannelTeam`, 09 3.9.1) and then opens
/// the team dialog of the new channel, where the owner adds members. A refusal stays in the
/// dialog with the server's reason; a channel created before a refused switch is kept, so the
/// next press only switches it on. The entry stays hidden until the server reported the
/// person's access to the space.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../notifications_settings/providers.dart' show isOwnerProvider;
import '../thread/providers.dart' show spaceAccessProvider;
import 'providers.dart';
import 'team_dialog.dart' show showTeamDialogOf;
import 'team_port.dart';
import 'team_text.dart';

/// The entry "Create team channel..." of the menu of [space] for an owner with `manage` on the
/// space, or none.
List<Widget> teamCreateMenuItems(
  BuildContext context,
  WidgetRef ref,
  Space space,
) {
  final owner = ref.watch(isOwnerProvider).value ?? false;
  final access = ref.watch(spaceAccessProvider(space.id.value)).value;
  final manage = access?.capabilities.contains(Capability.manage) ?? false;
  if (!owner || !manage) return const [];
  // Taken now: the entry runs after the menu closed and its ref is gone (D-TEAM-APP-6).
  final container = ProviderScope.containerOf(context, listen: false);
  return [
    DsMenuItem(
      key: const ValueKey('menu.createTeamChannel'),
      onPressed: () =>
          unawaited(showTeamCreateDialog(context, container, space)),
      child: Text(AppLocalizations.of(context).teamCreateMenu),
    ),
  ];
}

/// Opens the dialog for [space] and, once the team channel exists with team mode on, the team
/// dialog of the new channel. Does nothing without a session.
Future<void> showTeamCreateDialog(
  BuildContext context,
  ProviderContainer container,
  Space space,
) async {
  final port = container.read(teamPortProvider);
  if (port == null) return;
  final created = await showDialog<ChannelId>(
    context: context,
    builder: (_) => TeamCreateDialog(
      space: space,
      port: port,
      newKey: container.read(teamKeysProvider),
    ),
  );
  if (created == null || !context.mounted) return;
  await showTeamDialogOf(context, container, created);
}

/// The dialog over [port]; it pops the ID of the new team channel.
class TeamCreateDialog extends StatefulWidget {
  /// Creates the dialog for [space]; [newKey] makes the `Idempotency-Key` of the create.
  const TeamCreateDialog({
    super.key,
    required this.space,
    required this.port,
    required this.newKey,
  });

  /// The space the channel is created in.
  final Space space;

  /// The server side.
  final TeamPort port;

  /// Makes an idempotency key.
  final String Function() newKey;

  @override
  State<TeamCreateDialog> createState() => _TeamCreateDialogState();
}

class _TeamCreateDialogState extends State<TeamCreateDialog> {
  final TextEditingController _name = TextEditingController();

  /// The channel made by an earlier press whose switch failed; the next press only switches.
  ChannelId? _created;
  String? _key;
  String? _keyFor;
  bool _busy = false;
  TeamFailure? _failure;

  String get _trimmed => _name.text.trim();

  bool get _canCreate => !_busy && (_created != null || _trimmed.isNotEmpty);

  Future<void> _create() async {
    final name = _trimmed;
    setState(() {
      _busy = true;
      _failure = null;
    });
    try {
      var channel = _created;
      if (channel == null) {
        // One key per name: a repeated press after a lost answer is the same request.
        if (_key == null || _keyFor != name) {
          _key = widget.newKey();
          _keyFor = name;
        }
        try {
          channel = await widget.port.createTextChannel(
            widget.space.id,
            name,
            idempotencyKey: _key!,
          );
        } on TeamFailure catch (e) {
          // A refusal is kept under its key (04 5.2): the next press is a new request.
          if (!e.offline) _key = null;
          rethrow;
        }
        _created = channel;
      }
      await widget.port.enable(channel);
      if (mounted) Navigator.of(context).pop(channel);
    } on TeamFailure catch (e) {
      if (mounted) setState(() => _failure = e);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  void dispose() {
    _name.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final failure = _failure;
    // No way out while a call runs: a closed dialog would still switch the channel on.
    return PopScope(canPop: !_busy, child: _dialog(l, colors, failure));
  }

  Widget _dialog(AppLocalizations l, DsColors colors, TeamFailure? failure) {
    return AlertDialog(
      key: const ValueKey('teamCreate.dialog'),
      title: Text(l.teamCreateTitle(dsIsolate(widget.space.name))),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 420),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
              key: const ValueKey('teamCreate.name'),
              controller: _name,
              autofocus: true,
              // The channel exists after a refused switch; its name no longer changes here.
              enabled: !_busy && _created == null,
              onChanged: (_) => setState(() {}),
              onSubmitted: (_) {
                if (_canCreate) unawaited(_create());
              },
              decoration: InputDecoration(
                labelText: l.teamCreateName,
                helperText: l.teamCreateHint,
                helperMaxLines: 3,
              ),
            ),
            if (failure != null) ...[
              const SizedBox(height: DsSpace.s8),
              Semantics(
                liveRegion: true,
                child: Text(
                  teamFailureText(l, failure),
                  key: const ValueKey('teamCreate.failure'),
                  style: dsTextStyle(DsType.bodySm)
                      .copyWith(color: colors.dangerFg),
                ),
              ),
            ],
          ],
        ),
      ),
      actions: [
        DsButton(
          key: const ValueKey('teamCreate.cancel'),
          label: l.teamCreateCancel,
          onPressed: _busy ? null : () => Navigator.of(context).pop(),
        ),
        DsButton(
          key: const ValueKey('teamCreate.submit'),
          label: l.teamCreateSubmit,
          style: DsButtonStyle.primary,
          onPressed: _canCreate ? () => unawaited(_create()) : null,
        ),
      ],
    );
  }
}
