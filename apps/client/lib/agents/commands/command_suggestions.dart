// SPDX-License-Identifier: Apache-2.0

/// The slash suggestions of the composer (01 D5; components.md 9; 09 10.3, 10.5; brief M4-21
/// Design note 4): on "/" at the start of a line the built-ins of 09 10.1 and the registered
/// commands of the channel, the operator's first, equal names of other agents qualified with
/// their handle, control commands marked.
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:l10n/l10n.dart';

import 'builtin_commands.dart';

/// A command the person can choose: a built-in or a registered one.
sealed class CommandChoiceItem {
  const CommandChoiceItem();

  /// What the composer inserts after the slash, for example `model` or `stats@robin`.
  String get token;
}

/// A built-in command (09 10.1).
final class BuiltinChoice extends CommandChoiceItem {
  /// Creates the choice of [command].
  const BuiltinChoice(this.command);

  /// The built-in.
  final BuiltinCommand command;

  @override
  String get token => command.name;
}

/// A registered command of an agent or app (03 `commands`).
final class RegisteredChoice extends CommandChoiceItem {
  /// Creates the choice of [command]; [qualifier] is the owner's handle when another
  /// command of the channel has the same name.
  const RegisteredChoice(this.command, {this.qualifier});

  /// The command.
  final Command command;

  /// The handle that tells equal names apart, or null.
  final String? qualifier;

  @override
  String get token =>
      qualifier == null ? command.name : '${command.name}@$qualifier';
}

/// The commands of [all] that apply in [channel] of [spaceId] (09 10.3): scope `instance`,
/// the channel's space or the channel itself; per owner and name only the most specific scope
/// (`channel` before `space` before `instance`), like `listChannelCommands` answers.
List<Command> commandsForChannel(
  Iterable<Command> all, {
  required ChannelId channel,
  required SpaceId? spaceId,
}) {
  int rank(Command c) => switch (c.scopeKind) {
    TargetKind.channel when c.scopeId == channel.value => 3,
    TargetKind.space when spaceId != null && c.scopeId == spaceId.value => 2,
    TargetKind.instance => 1,
    _ => 0,
  };
  final best = <(PrincipalId, String), Command>{};
  for (final c in all) {
    final r = rank(c);
    if (r == 0) continue;
    final key = (c.ownerPrincipalId, c.name.toLowerCase());
    final held = best[key];
    if (held == null || rank(held) < r) best[key] = c;
  }
  return best.values.toList();
}

/// The suggestions for the slash [query] (the text after "/", lower-cased prefix match):
/// first the built-ins [builtins] (control commands of every channel), then the commands of
/// the [operatorId], then the others; each group by name, owner handle and ID. A name that
/// more than one owner registered is qualified with the handle for every owner but the
/// operator (09 10.3).
List<CommandChoiceItem> commandSuggestions(
  String query, {
  required List<BuiltinCommand> builtins,
  required List<Command> commands,
  required PrincipalId? operatorId,
  required Map<PrincipalId, Principal> principals,
}) {
  final q = query.toLowerCase();
  String handle(PrincipalId id) => principals[id]?.handle ?? id.value;
  final byName = <String, int>{};
  for (final c in commands) {
    byName.update(c.name.toLowerCase(), (n) => n + 1, ifAbsent: () => 1);
  }
  int order(Command a, Command b) {
    final groupA = a.ownerPrincipalId == operatorId ? 0 : 1;
    final groupB = b.ownerPrincipalId == operatorId ? 0 : 1;
    var c = groupA.compareTo(groupB);
    if (c == 0) c = a.name.toLowerCase().compareTo(b.name.toLowerCase());
    if (c == 0) {
      c = handle(a.ownerPrincipalId).compareTo(handle(b.ownerPrincipalId));
    }
    if (c == 0) c = a.id.value.compareTo(b.id.value);
    return c;
  }

  final registered = [...commands]..sort(order);
  return [
    for (final b in builtins)
      if (b.name.startsWith(q)) BuiltinChoice(b),
    for (final c in registered)
      if (c.name.toLowerCase().startsWith(q) ||
          '${c.name}@${handle(c.ownerPrincipalId)}'.toLowerCase().startsWith(q))
        RegisteredChoice(
          c,
          qualifier:
              (byName[c.name.toLowerCase()] ?? 0) > 1 &&
                  c.ownerPrincipalId != operatorId
              ? handle(c.ownerPrincipalId)
              : null,
        ),
  ];
}

/// [item] as a row of `DsSuggestionList`: built-ins with their translated description and
/// marked as control commands, registered ones with the owner's name.
DsSuggestion suggestionOf(
  AppLocalizations l,
  CommandChoiceItem item,
  Map<PrincipalId, Principal> principals,
) => switch (item) {
  BuiltinChoice(:final command) => DsSuggestion(
    id: 'builtin.${command.name}',
    command: '/${command.name}',
    description: command.description(l),
    control: true,
  ),
  RegisteredChoice(:final command) => DsSuggestion(
    id: 'command.${command.id.value}',
    command: '/${item.token}',
    description: command.description,
    originName:
        principals[command.ownerPrincipalId]?.displayName ??
        command.ownerPrincipalId.value,
    control: command.control,
  ),
};
