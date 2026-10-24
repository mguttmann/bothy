// SPDX-License-Identifier: Apache-2.0

/// Loads the commands of a channel when it is opened (brief M4-21 Scope; 15 6.1; 09 10.3):
/// `listChannelCommands` into the local `commands` table by the version rule, so the slash
/// suggestions work offline. Later changes arrive as `command.changed`, which the applier of
/// M4-14 stores; the built-ins are not part of the list (the client shows its own).
library;

import 'package:api_client/api_client.dart' as wire;
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

/// Reads the commands of a channel from the server.
typedef ChannelCommandsFetch = Future<List<Command>> Function(
  ChannelId channel,
);

/// The fetch of `listChannelCommands` through [api].
ChannelCommandsFetch channelCommandsFetch(wire.ApiClient api) =>
    (channel) async {
      final response = await api.getCommandsApi().listChannelCommands(
        channelId: channel.value,
      );
      return [
        for (final c in response.data?.items ?? const <wire.Command>[])
          commandOfWire(c),
      ];
    };

/// The API `Command` [c] as the domain entity of the local table.
Command commandOfWire(wire.Command c) => Command(
  id: CommandId.parse(c.id),
  ownerPrincipalId: PrincipalId.parse(c.ownerPrincipalId),
  name: c.name,
  description: c.description,
  scopeKind: TargetKind.fromCode(c.scopeKind.name),
  scopeId: c.scopeId,
  options: [
    for (final o in c.options)
      CommandOption(
        name: o.name,
        description: o.description,
        type: CommandOptionType.fromCode(o.type.name),
        required: o.required_,
        choices: [
          for (final ch in o.choices ?? const <wire.CommandChoice>[])
            CommandChoice(name: ch.name, value: ch.value),
        ],
        autocomplete: o.autocomplete ?? false,
      ),
  ],
  requiredLevel: CommandLevel.fromCode(c.requiredLevel.name),
  control: c.control,
  version: ResourceVersion(c.version),
);

/// Loads the commands of [channel] through [fetch] into [db]; offline or refused, the stored
/// ones stay. Returns how many were stored.
Future<int> loadChannelCommands(
  LocalDatabase db,
  ChannelCommandsFetch fetch,
  ChannelId channel,
) async {
  final List<Command> commands;
  try {
    commands = await fetch(channel);
  } on Object {
    return 0;
  }
  for (final c in commands) {
    await db.commandsDao.upsertIfNewer(c);
  }
  return commands.length;
}
