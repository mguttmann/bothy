// SPDX-License-Identifier: Apache-2.0

/// The changed parts of an edit of a built-in agent (docs/spec/26-builtin-agents.md 4.1 item 2,
/// 8.4; brief A1-09 Design note 2): before "Save" the editor names what differs from the
/// current version, by part and name, never by text, the way the server's audit entry names
/// the changed parts. A definition change takes effect at the agent's next turn, so the person
/// sees the list before the step-up.
library;

import 'dart:convert';

/// The kind of a changed part.
enum DefinitionChangeKind {
  /// The instructions (`CLAUDE.md`).
  instructions,

  /// A skill that is new; [DefinitionChange.name] is its name.
  skillAdded,

  /// A skill that is gone.
  skillRemoved,

  /// A skill whose description, body or files differ.
  skillChanged,

  /// An MCP server that is new.
  serverAdded,

  /// An MCP server that is gone.
  serverRemoved,

  /// An MCP server whose transport, address, command or entries differ.
  serverChanged,

  /// A member of `settings`; [DefinitionChange.name] is its key, for example `model`.
  setting,

  /// The origin of the definition (display only, 26 4.2).
  origin,

  /// The agent's tool class (`agent.tool_class.set`), not part of the definition;
  /// [DefinitionChange.name] is the new class and [DefinitionChange.from] the old one.
  toolClass,
}

/// One changed part.
final class DefinitionChange {
  /// Creates the change.
  const DefinitionChange(this.kind, [this.name = '', this.from = '']);

  /// What changed.
  final DefinitionChangeKind kind;

  /// The name of the skill, the server or the setting, the new class, or empty.
  final String name;

  /// The old class of a [DefinitionChangeKind.toolClass] change, else empty.
  final String from;

  @override
  bool operator ==(Object other) =>
      other is DefinitionChange &&
      other.kind == kind &&
      other.name == name &&
      other.from == from;

  @override
  int get hashCode => Object.hash(kind, name, from);

  @override
  String toString() => '${kind.name}($name)';
}

/// The settings keys in the order the editor shows them.
const List<String> _settingKeys = [
  'model',
  'tool_class_wish',
  'allowed_tools',
  'max_cost_usd_per_run',
];

/// The changed parts from [before] to [after], two `agent_definition.v1` bodies: instructions,
/// then skills and MCP servers by name (changed in the order of [after], added, removed in the
/// order of [before]), then each settings key, then the origin.
///
/// Example: `definitionChanges(stored.json, draft.definitionJson())` is
/// `[DefinitionChange(DefinitionChangeKind.instructions)]` after an edit of the instructions.
List<DefinitionChange> definitionChanges(
  Map<String, Object?> before,
  Map<String, Object?> after,
) {
  final out = <DefinitionChange>[];
  if (!_same(before['instructions'], after['instructions'])) {
    out.add(const DefinitionChange(DefinitionChangeKind.instructions));
  }
  _named(
    out,
    _byName(before['skills']),
    _byName(after['skills']),
    added: DefinitionChangeKind.skillAdded,
    removed: DefinitionChangeKind.skillRemoved,
    changed: DefinitionChangeKind.skillChanged,
  );
  _named(
    out,
    _byName(before['mcp_servers']),
    _byName(after['mcp_servers']),
    added: DefinitionChangeKind.serverAdded,
    removed: DefinitionChangeKind.serverRemoved,
    changed: DefinitionChangeKind.serverChanged,
  );
  final s0 = _map(before['settings']);
  final s1 = _map(after['settings']);
  for (final key in _settingKeys) {
    if (!_same(s0[key], s1[key])) {
      out.add(DefinitionChange(DefinitionChangeKind.setting, key));
    }
  }
  if (!_same(before['origin'], after['origin'])) {
    out.add(const DefinitionChange(DefinitionChangeKind.origin));
  }
  return out;
}

void _named(
  List<DefinitionChange> out,
  Map<String, Object?> before,
  Map<String, Object?> after, {
  required DefinitionChangeKind added,
  required DefinitionChangeKind removed,
  required DefinitionChangeKind changed,
}) {
  final changedOrAdded = <DefinitionChange>[];
  final addedOnly = <DefinitionChange>[];
  for (final MapEntry(:key, :value) in after.entries) {
    if (!before.containsKey(key)) {
      addedOnly.add(DefinitionChange(added, key));
    } else if (!_same(before[key], value)) {
      changedOrAdded.add(DefinitionChange(changed, key));
    }
  }
  out
    ..addAll(changedOrAdded)
    ..addAll(addedOnly)
    ..addAll([
      for (final key in before.keys)
        if (!after.containsKey(key)) DefinitionChange(removed, key),
    ]);
}

/// The entries of a list of named objects by `name`; a second entry of the same name keeps
/// the first (the server refuses duplicates with `duplicate_name`).
Map<String, Object?> _byName(Object? list) {
  final out = <String, Object?>{};
  if (list is List) {
    for (final item in list) {
      if (item is Map) out.putIfAbsent('${item['name'] ?? ''}', () => item);
    }
  }
  return out;
}

Map<String, Object?> _map(Object? value) =>
    value is Map ? value.cast<String, Object?>() : const {};

bool _same(Object? a, Object? b) => _canonical(a) == _canonical(b);

String _canonical(Object? value) {
  if (value is Map) {
    final keys = [for (final k in value.keys) '$k']..sort();
    return '{${[for (final k in keys) '${jsonEncode(k)}:${_canonical(value[k])}'].join(',')}}';
  }
  if (value is List) return '[${value.map(_canonical).join(',')}]';
  if (value is num) return jsonEncode(value.toDouble());
  return jsonEncode(value);
}
