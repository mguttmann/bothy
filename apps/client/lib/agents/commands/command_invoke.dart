// SPDX-License-Identifier: Apache-2.0

/// What sending a text that starts with "/" does (09 10.1, 10.4; brief M4-21 Design note 4):
/// a built-in runs its own operation, a complete registered command goes through
/// `invokeCommand` with an `Idempotency-Key` (the outbox entry's ID), anything else is a
/// normal message.
library;

import 'package:domain/domain.dart';

import 'builtin_commands.dart';
import 'command_options.dart';
import 'command_suggestions.dart';

/// The outcome of [resolveCommand].
sealed class CommandSend {
  const CommandSend();
}

/// A built-in with the text after its name.
final class SendBuiltin extends CommandSend {
  /// Creates the outcome.
  const SendBuiltin(this.command, this.argument);

  /// The built-in.
  final BuiltinCommand command;

  /// The text after the name (the title of `/title`).
  final String argument;
}

/// A registered command with its option values.
final class SendRegistered extends CommandSend {
  /// Creates the outcome.
  const SendRegistered(this.command, this.options);

  /// The command.
  final Command command;

  /// Name and value of every option that has one.
  final List<({String name, String value})> options;
}

/// A built-in the server's answers do not allow here: nothing goes out, the text stays and the
/// composer names the reason (09 10.1: control commands never go to a model).
final class BlockedBuiltin extends CommandSend {
  /// Creates the outcome.
  const BlockedBuiltin(this.command);

  /// The built-in.
  final BuiltinCommand command;
}

/// Not a command: the text goes out as a message.
final class SendMessage extends CommandSend {
  /// The outcome.
  const SendMessage();
}

/// What sending [text] does in a channel offering [builtins] and the registered [commands]
/// (already filtered for the channel, `commandsForChannel`), with [operatorId] and
/// [principals] to resolve qualified names (`/robin:stats`, 09 10.3).
///
/// A name of a built-in is always the built-in (the names are reserved, 09 10.1); a built-in
/// the server's answers do not allow here is not in [builtins] and is [BlockedBuiltin]: it is
/// never sent as a message, which would reach the model or a running turn. Between registered
/// commands an unqualified name means the operator's command, or the only one of that name.
CommandSend resolveCommand(
  String text, {
  required List<BuiltinCommand> builtins,
  required List<Command> commands,
  required PrincipalId? operatorId,
  required Map<PrincipalId, Principal> principals,
}) {
  final slash = slashCommandOf(text.trimRight());
  if (slash == null) return const SendMessage();
  final builtin = BuiltinCommand.byName(slash.token);
  if (builtin != null) {
    return builtins.contains(builtin)
        ? SendBuiltin(builtin, slash.rest)
        : BlockedBuiltin(builtin);
  }
  final command = commandOfToken(
    slash.token,
    commands: commands,
    operatorId: operatorId,
    principals: principals,
  );
  if (command == null) return const SendMessage();
  return SendRegistered(command, invokeOptions(command, slash.rest));
}

/// The registered command [token] names, or null: `handle:name` names the command of that
/// owner (also the operator's or one without a namesake), a bare `name` the operator's
/// command or the only one of that name.
Command? commandOfToken(
  String token, {
  required List<Command> commands,
  required PrincipalId? operatorId,
  required Map<PrincipalId, Principal> principals,
}) {
  final items = commandSuggestions(
    '',
    builtins: const [],
    commands: commands,
    operatorId: operatorId,
    principals: principals,
  );
  final lower = token.toLowerCase();
  final colon = lower.indexOf(':');
  if (colon > 0) {
    final handle = lower.substring(0, colon);
    final name = lower.substring(colon + 1);
    for (final c in commands) {
      final owner =
          principals[c.ownerPrincipalId]?.handle ?? c.ownerPrincipalId.value;
      if (owner.toLowerCase() == handle && c.name.toLowerCase() == name) {
        return c;
      }
    }
    return null;
  }
  for (final item in items) {
    if (item is RegisteredChoice && item.token.toLowerCase() == lower) {
      return item.command;
    }
  }
  return null;
}
