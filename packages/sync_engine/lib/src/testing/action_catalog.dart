// SPDX-License-Identifier: Apache-2.0

/// The action catalogue of the fake server (brief M4-26 Design note 3): the `step_up` column of
/// `docs/spec/actions.yaml`, so the fake asks for a step-up exactly where 05 marks an action,
/// without a copy of the table.
library;

import 'dart:io';

import 'package:yaml/yaml.dart';

import 'schema_validator.dart' show embeddedActionsYaml;

/// The `step_up` value of every action of `docs/spec/actions.yaml` (05 2.4).
///
/// Values are the strings `true`, `false`, `fresh` (the terminal's fresh step-up, 08 6.3) and
/// `conditional` (05 2.4 defines when it applies).
///
/// Example:
/// ```dart
/// final catalog = ActionCatalog.load();
/// catalog.stepUp('user.disable'); // 'true'
/// ```
final class ActionCatalog {
  ActionCatalog._(this._stepUp);

  static ActionCatalog? _loaded;

  final Map<String, String> _stepUp;

  /// Loads `docs/spec/actions.yaml` of the repository that holds [from] (default: the working
  /// directory), see [read]; read once per isolate.
  static ActionCatalog load({Directory? from}) => _loaded ??= read(from: from);

  /// Reads `docs/spec/actions.yaml` of the repository that holds [from] (default: the working
  /// directory), walking up to the folder that has it. Without such a folder (an app under
  /// test in the macOS app sandbox, which cannot read the repository) it uses [embedded].
  static ActionCatalog read({Directory? from}) {
    var dir = (from ?? Directory.current).absolute;
    while (true) {
      final file = File('${dir.path}/docs/spec/actions.yaml');
      if (file.existsSync()) return _parse(file.readAsStringSync());
      final parent = dir.parent;
      if (parent.path == dir.path) return embedded();
      dir = parent;
    }
  }

  /// The copy of `docs/spec/actions.yaml` compiled into the fake (`contract_documents.g.dart`,
  /// written by `make contract-documents`; `make codegen-check` keeps it equal to the file,
  /// docs/DECISIONS.md D-M4-26-9).
  static ActionCatalog embedded() => _parse(embeddedActionsYaml());

  static ActionCatalog _parse(String yaml) {
    final doc = loadYaml(yaml) as YamlMap;
    return ActionCatalog._({
      for (final action in doc['actions'] as YamlList)
        (action as YamlMap)['id'] as String: '${action['step_up']}',
    });
  }

  /// The `step_up` value of [action]; throws an [ArgumentError] for an unknown action.
  String stepUp(String action) =>
      _stepUp[action] ??
      (throw ArgumentError.value(action, 'action', 'not in actions.yaml'));
}
