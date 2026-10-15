// SPDX-License-Identifier: Apache-2.0

/// The app configuration: every value of place `App` of the configuration reference and the
/// server keys the apps read for their own behaviour (D-W-115, D-W-116), each with the default
/// of the reference (`docs/operations/configuration.md` section 4).
///
/// `app_config_keys.g.dart` is generated from the reference by `tools/configdoc/app_config.py`;
/// `make codegen-check` fails when it is stale. Limits that the server delivers in
/// `InstanceLimits` are never here; the apps read them from `getInstance`.
library;

import 'dart:convert';

part 'app_config_keys.g.dart';

/// Kind of a configuration value, derived from its default; each has exactly one parser
/// (configuration reference section 2).
enum AppConfigKind {
  /// `true` or `false`.
  boolean,

  /// A whole number with one unit `ms`, `s`, `m`, `h` or `d`, read as a [Duration].
  duration,

  /// A whole number with `KiB`, `MiB` or `GiB`, read as bytes.
  bytes,

  /// A whole number with `KiB/s`, `MiB/s` or `GiB/s`, read as bytes per second.
  byteRate,

  /// A whole number with `kbps`, read as bits per second.
  bitRate,

  /// A whole number with `px`, read as logical pixels.
  pixels,

  /// A whole number in base ten.
  integer,

  /// A finite decimal number.
  decimal,

  /// A list of texts, separated by commas or as a JSON array.
  list,
}

/// One key of the table: its name, place in the reference, kind and written default.
final class AppConfigKey {
  /// Creates an entry; only the generated table does.
  const AppConfigKey(this.key, this.place, this.kind, this.defaultText);

  /// The key, for example `client.outbox_stuck_after`.
  final String key;

  /// The place column of the reference: `App`, or `Server` for the keys of D-W-115.
  final String place;

  /// The kind, derived from the default.
  final AppConfigKind kind;

  /// The default exactly as the reference writes it, for example `2m`.
  final String defaultText;
}

final RegExp _duration = RegExp(r'^(0|[1-9][0-9]*)(ms|s|m|h|d)$');
final RegExp _integer = RegExp(r'^-?(0|[1-9][0-9]*)$');
final RegExp _decimal = RegExp(
  r'^-?([0-9]+(\.[0-9]*)?|\.[0-9]+)([eE][-+]?[0-9]+)?$',
);
final RegExp _bytes = RegExp(r'^(0|[1-9][0-9]*)(KiB|MiB|GiB)$');
final RegExp _byteRate = RegExp(r'^(0|[1-9][0-9]*)(KiB|MiB|GiB)/s$');
final RegExp _bitRate = RegExp(r'^(0|[1-9][0-9]*)kbps$');
final RegExp _pixels = RegExp(r'^(0|[1-9][0-9]*)px$');

const Map<String, int> _binary = {
  'KiB': 1024,
  'MiB': 1024 * 1024,
  'GiB': 1024 * 1024 * 1024,
};

/// Parses [text] as a value of [kind].
///
/// Returns a `bool`, `Duration`, `int`, `double` or `List<String>`. Throws a
/// [FormatException] that names [key] and the kind but never the text, because a value may
/// come from an untrusted override.
///
/// Example: `parseAppConfigValue('ui.autocomplete_debounce', AppConfigKind.duration, '150ms')`
/// is `Duration(milliseconds: 150)`.
Object parseAppConfigValue(String key, AppConfigKind kind, String text) {
  FormatException invalid() =>
      FormatException('$key: not a valid ${kind.name} value');
  int whole(String digits) => int.tryParse(digits) ?? (throw invalid());
  switch (kind) {
    case AppConfigKind.boolean:
      if (text == 'true') return true;
      if (text == 'false') return false;
      throw invalid();
    case AppConfigKind.duration:
      final m = _duration.firstMatch(text) ?? (throw invalid());
      final n = whole(m.group(1)!);
      return switch (m.group(2)!) {
        'ms' => Duration(milliseconds: n),
        's' => Duration(seconds: n),
        'm' => Duration(minutes: n),
        'h' => Duration(hours: n),
        _ => Duration(days: n),
      };
    case AppConfigKind.bytes:
      final m = _bytes.firstMatch(text) ?? (throw invalid());
      return whole(m.group(1)!) * _binary[m.group(2)!]!;
    case AppConfigKind.byteRate:
      final m = _byteRate.firstMatch(text) ?? (throw invalid());
      return whole(m.group(1)!) * _binary[m.group(2)!]!;
    case AppConfigKind.bitRate:
      final m = _bitRate.firstMatch(text) ?? (throw invalid());
      return whole(m.group(1)!) * 1000;
    case AppConfigKind.pixels:
      final m = _pixels.firstMatch(text) ?? (throw invalid());
      return whole(m.group(1)!);
    case AppConfigKind.integer:
      if (!_integer.hasMatch(text)) throw invalid();
      return whole(text);
    case AppConfigKind.decimal:
      if (!_decimal.hasMatch(text)) throw invalid();
      final value = double.parse(text);
      if (!value.isFinite) throw invalid();
      return value;
    case AppConfigKind.list:
      return _list(text, invalid);
  }
}

List<String> _list(String text, FormatException Function() invalid) {
  if (text.isEmpty) return const <String>[];
  if (text.startsWith('[')) {
    final Object? decoded;
    try {
      decoded = jsonDecode(text);
    } on FormatException {
      throw invalid();
    }
    if (decoded is! List || decoded.any((e) => e is! String || e.isEmpty)) {
      throw invalid();
    }
    return List.unmodifiable(decoded.cast<String>());
  }
  final items = text.split(',').map((e) => e.trim()).toList();
  if (items.any((e) => e.isEmpty)) throw invalid();
  return List.unmodifiable(items);
}

/// The configuration values of the apps, one typed getter per key ([AppConfigValues]).
///
/// Every package reads values from here by key; code comments name the key, never the number
/// (fixed requirement 9 of the M4 briefs).
///
/// Example:
/// ```dart
/// final config = AppConfig.defaults();
/// config.outboxStuckAfter; // Duration(minutes: 2)
/// final tuned = config.withOverrides({'ui.quick_switch_max_results': '30'});
/// ```
final class AppConfig {
  AppConfig._(this._values);

  /// Every key with the default of the reference.
  factory AppConfig.defaults() => AppConfig._({
    for (final k in appConfigKeys)
      k.key: parseAppConfigValue(k.key, k.kind, k.defaultText),
  });

  final Map<String, Object> _values;

  /// Returns a copy with the values of [raw] (key to written value) replacing the current
  /// ones; each is parsed with the parser of its kind.
  ///
  /// This is the override hook for the apps; they decide later where overrides come from.
  /// Throws a [FormatException] naming the key (never the value) for a key that is not in the
  /// table or a value that does not parse.
  AppConfig withOverrides(Map<String, String> raw) {
    final next = Map<String, Object>.of(_values);
    for (final entry in raw.entries) {
      final key = _byKey[entry.key];
      if (key == null) {
        throw FormatException('${entry.key}: not a key of AppConfig');
      }
      next[key.key] = parseAppConfigValue(key.key, key.kind, entry.value);
    }
    return AppConfig._(next);
  }

  /// The value of [key] as type [T]; the typed getters of [AppConfigValues] call this.
  ///
  /// Throws an [ArgumentError] for a key that is not in the table.
  T value<T extends Object>(String key) {
    final v = _values[key];
    if (v == null) {
      throw ArgumentError.value(key, 'key', 'not a key of AppConfig');
    }
    return v as T;
  }

  /// Every key of the table.
  static Iterable<String> get keys => _byKey.keys;

  static final Map<String, AppConfigKey> _byKey = {
    for (final k in appConfigKeys) k.key: k,
  };
}
