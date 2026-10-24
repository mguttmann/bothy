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

/// Not a command: the text goes out as a message.
final class SendMessage extends CommandSend {
  /// The outcome.
  const SendMessage();
}

/// What sending [text] does in a channel offering [builtins] and the registered [commands]
/// (already filtered for the channel, `commandsForChannel`), with [operatorId] and
/// [principals] to resolve qualified names (`/stats@robin`).
///
/// A name of a built-in is always the built-in (the names are reserved, 09 10.1); a built-in
/// the server's answers do not allow here is not in [builtins], so its text goes out as a
/// message like any unknown command. Between registered commands an unqualified name means
/// the operator's command, or the only one of that name.
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
        : const SendMessage();
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

/// The registered command [token] names (`name` or `name@handle`), or null.
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
  for (final item in items) {
    if (item is RegisteredChoice && item.token.toLowerCase() == lower) {
      return item.command;
    }
  }
  // An unqualified name of a command that is shown qualified: the operator's, else none.
  return null;
}
