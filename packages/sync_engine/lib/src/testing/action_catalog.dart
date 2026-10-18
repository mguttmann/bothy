// SPDX-License-Identifier: Apache-2.0

/// The action catalogue of the fake server (brief M4-26 Design note 3): the `step_up` column of
/// `docs/spec/actions.yaml`, so the fake asks for a step-up exactly where 05 marks an action,
/// without a copy of the table.
library;

import 'dart:io';

import 'package:yaml/yaml.dart';

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
  /// directory), walking up to the folder that has it; read once per isolate.
  ///
  /// Throws a [StateError] when no repository root is found above [from].
  static ActionCatalog load({Directory? from}) {
    final cached = _loaded;
    if (cached != null) return cached;
    var dir = (from ?? Directory.current).absolute;
    while (true) {
      final file = File('${dir.path}/docs/spec/actions.yaml');
      if (file.existsSync()) {
        final doc = loadYaml(file.readAsStringSync()) as YamlMap;
        return _loaded = ActionCatalog._({
          for (final action in doc['actions'] as YamlList)
            (action as YamlMap)['id'] as String: '${action['step_up']}',
        });
      }
      final parent = dir.parent;
      if (parent.path == dir.path) {
        throw StateError(
          'no docs/spec/actions.yaml above the working directory',
        );
      }
      dir = parent;
    }
  }

  /// The `step_up` value of [action]; throws an [ArgumentError] for an unknown action.
  String stepUp(String action) =>
      _stepUp[action] ??
      (throw ArgumentError.value(action, 'action', 'not in actions.yaml'));
}
