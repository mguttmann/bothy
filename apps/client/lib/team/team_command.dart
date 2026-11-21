// SPDX-License-Identifier: Apache-2.0

/// The client command `/team` (09 3.9.1; the name is reserved, 10.1): `list` and `add` open the
/// team dialog (`add <handle>` with the handle filled in, since the agent and its home channel
/// are chosen there), `remove @handle` removes the member, `stop` stops every member run. It
/// never goes to a model and never through `invokeCommand`.
library;

import 'package:domain/domain.dart';
import 'package:l10n/l10n.dart';

import 'team_controller.dart';
import 'team_model.dart';
import 'team_port.dart';
import 'team_text.dart';

/// Runs [command] in [channel] over [port]: [openDialog] shows the team dialog, [say] shows a
/// short message, [onChanged] is called after an accepted change.
Future<void> runTeamCommand(
  TeamCommand command, {
  required ChannelId channel,
  required TeamPort port,
  required String Function() newKey,
  required AppLocalizations l,
  required Future<void> Function({String? handle}) openDialog,
  required void Function(String text) say,
  void Function()? onChanged,
}) async {
  switch (command) {
    case TeamList():
      return openDialog();
    case TeamAdd(:final handle):
      return openDialog(handle: handle);
    case TeamUsage():
      return say(l.teamUsage);
    case TeamRemove() || TeamStop():
      break;
  }
  final controller = TeamController(
    channel: channel,
    port: port,
    newKey: newKey,
    onChanged: onChanged,
  );
  try {
    await controller.load();
    final team = controller.team;
    final failure = controller.failure;
    if (failure != null) return say(teamFailureText(l, failure));
    if (team == null) return say(l.teamNotEnabled);
    switch (command) {
      case TeamRemove(:final handle):
        final member = team.byHandle(handle);
        if (member == null) return say(l.teamUnknownHandle(handle));
        final ok = await controller.remove(member.id);
        say(
          ok
              ? l.teamRemoved(member.handle)
              : teamFailureText(l, controller.failure!),
        );
      case TeamStop():
        final ok = await controller.stopAll();
        say(ok ? l.teamStopped : teamFailureText(l, controller.failure!));
      case TeamList() || TeamAdd() || TeamUsage():
        break;
    }
  } finally {
    controller.dispose();
  }
}
