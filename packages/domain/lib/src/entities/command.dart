// SPDX-License-Identifier: Apache-2.0

/// Slash commands for suggestions offline (03 `commands`; 15 6.1).
library;

import '../coded.dart';
import '../ids.dart';
import '../seq.dart';

/// Kind of a scope target (API `TargetKind`).
enum TargetKind implements Coded {
  /// The instance.
  instance('instance'),

  /// A space.
  space('space'),

  /// A channel.
  channel('channel'),

  /// A host.
  host('host'),

  /// A value of a newer server.
  unknown('');

  const TargetKind(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static TargetKind fromCode(String code) => decodeCode(values, code, unknown);
}

/// Level needed to use a command (03 `commands.required_level`, API `CommandLevel`).
enum CommandLevel implements Coded {
  /// Write.
  write('write'),

  /// Trigger.
  trigger('trigger'),

  /// A value of a newer server.
  unknown('');

  const CommandLevel(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static CommandLevel fromCode(String code) =>
      decodeCode(values, code, unknown);
}

/// Type of a command option (schema `command_options.v1`).
enum CommandOptionType implements Coded {
  /// Text.
  string('string'),

  /// Whole number.
  integer('integer'),

  /// Yes or no.
  boolean('boolean'),

  /// One of the choices.
  choice('choice'),

  /// A person.
  user('user'),

  /// A file.
  file('file'),

  /// A value of a newer server.
  unknown('');

  const CommandOptionType(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static CommandOptionType fromCode(String code) =>
      decodeCode(values, code, unknown);
}

/// A choice of a command option (API `CommandChoice`).
final class CommandChoice {
  /// Creates a [CommandChoice].
  const CommandChoice({required this.name, required this.value});

  /// Display name.
  final String name;

  /// Value.
  final String value;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  CommandChoice copyWith({String? name, String? value}) =>
      CommandChoice(name: name ?? this.name, value: value ?? this.value);

  @override
  bool operator ==(Object other) =>
      other is CommandChoice && other.name == name && other.value == value;

  @override
  int get hashCode => Object.hash(name, value);
}

/// An option of a command (API `CommandOption`).
final class CommandOption {
  /// Creates a [CommandOption].
  const CommandOption({
    required this.name,
    required this.description,
    required this.type,
    required this.required,
    required this.choices,
    required this.autocomplete,
  });

  /// Name.
  final String name;

  /// Description.
  final String description;

  /// Type.
  final CommandOptionType type;

  /// Whether it is required.
  final bool required;

  /// Choices of a `choice` option.
  final List<CommandChoice> choices;

  /// Whether the agent completes values.
  final bool autocomplete;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  CommandOption copyWith({
    String? name,
    String? description,
    CommandOptionType? type,
    bool? required,
    List<CommandChoice>? choices,
    bool? autocomplete,
  }) => CommandOption(
    name: name ?? this.name,
    description: description ?? this.description,
    type: type ?? this.type,
    required: required ?? this.required,
    choices: choices ?? this.choices,
    autocomplete: autocomplete ?? this.autocomplete,
  );

  @override
  bool operator ==(Object other) =>
      other is CommandOption &&
      other.name == name &&
      other.description == description &&
      other.type == type &&
      other.required == required &&
      listEquals(other.choices, choices) &&
      other.autocomplete == autocomplete;

  @override
  int get hashCode => Object.hash(
    name,
    description,
    type,
    required,
    Object.hashAll(choices),
    autocomplete,
  );
}

/// A slash command (03 `commands`, API `Command`).
final class Command {
  /// Creates a [Command].
  const Command({
    required this.id,
    required this.ownerPrincipalId,
    required this.name,
    required this.description,
    required this.scopeKind,
    required this.options,
    required this.requiredLevel,
    required this.control,
    required this.version,
    this.scopeId,
  });

  /// ID.
  final CommandId id;

  /// Agent or app that registered it.
  final PrincipalId ownerPrincipalId;

  /// Name.
  final String name;

  /// Description.
  final String description;

  /// Scope kind.
  final TargetKind scopeKind;

  /// Scope target, absent for the instance.
  final String? scopeId;

  /// Options.
  final List<CommandOption> options;

  /// Level needed.
  final CommandLevel requiredLevel;

  /// Control command; never goes to the model as a prompt.
  final bool control;

  /// Version (03 `version`; 04 9 item 3).
  final ResourceVersion version;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  Command copyWith({
    PrincipalId? ownerPrincipalId,
    String? name,
    String? description,
    TargetKind? scopeKind,
    String? scopeId,
    List<CommandOption>? options,
    CommandLevel? requiredLevel,
    bool? control,
    ResourceVersion? version,
  }) => Command(
    id: id,
    ownerPrincipalId: ownerPrincipalId ?? this.ownerPrincipalId,
    name: name ?? this.name,
    description: description ?? this.description,
    scopeKind: scopeKind ?? this.scopeKind,
    scopeId: scopeId ?? this.scopeId,
    options: options ?? this.options,
    requiredLevel: requiredLevel ?? this.requiredLevel,
    control: control ?? this.control,
    version: version ?? this.version,
  );

  @override
  bool operator ==(Object other) =>
      other is Command &&
      other.id == id &&
      other.ownerPrincipalId == ownerPrincipalId &&
      other.name == name &&
      other.description == description &&
      other.scopeKind == scopeKind &&
      other.scopeId == scopeId &&
      listEquals(other.options, options) &&
      other.requiredLevel == requiredLevel &&
      other.control == control &&
      other.version == version;

  @override
  int get hashCode => Object.hash(
    id,
    ownerPrincipalId,
    name,
    description,
    scopeKind,
    scopeId,
    Object.hashAll(options),
    requiredLevel,
    control,
    version,
  );
}
