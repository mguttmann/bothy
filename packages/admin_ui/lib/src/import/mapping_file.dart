// SPDX-License-Identifier: Apache-2.0

/// The mapping file of an import source: the owner's YAML of schema `import_source.discord.v1`
/// converted to the JSON that `createImport` takes, and the server's JSON pointers translated
/// back to places in that file (docs/spec/22-import-migration.md 5.2; brief M8-08 Design
/// note 2).
///
/// The conversion is strict and decides nothing about the schema: the file must be one YAML
/// mapping with text keys and no key twice; everything else (unknown members, IDs, handles) is
/// the server's check, whose errors come back with their pointers. Nothing is stored: the text
/// stays in memory until the dialog closes.
library;

import 'dart:convert';

import 'package:yaml/yaml.dart';

/// Why a mapping file could not be converted.
enum MappingFileProblem {
  /// The text is no valid YAML (a syntax error or a key named twice).
  invalidYaml,

  /// The document is not a mapping, or a mapping has a key that is not a text.
  notMapping,
}

/// A mapping file that could not be converted: [problem], and for invalid YAML the [line] and
/// [column] (1-based) where the parser stopped.
final class MappingFileError implements Exception {
  /// Creates the error.
  const MappingFileError(this.problem, {this.line, this.column});

  /// What is wrong.
  final MappingFileProblem problem;

  /// The line of the error, or null.
  final int? line;

  /// The column of the error, or null.
  final int? column;

  @override
  String toString() => 'MappingFileError(${problem.name} $line:$column)';
}

/// Converts the YAML [text] of a mapping file to the JSON object of `ImportSourceCreate.config`.
///
/// Throws a [MappingFileError] for invalid YAML (also for a key named twice in one mapping,
/// which the YAML parser refuses) or a document that is not a mapping with text keys.
///
/// Example: `mappingJsonOf('version: 1\nguild_id: "100"\n')` is
/// `{'version': 1, 'guild_id': '100'}`.
Map<String, Object?> mappingJsonOf(String text) {
  final Object? doc;
  try {
    doc = loadYaml(text);
  } on YamlException catch (e) {
    final start = e.span?.start;
    throw MappingFileError(
      MappingFileProblem.invalidYaml,
      line: start == null ? null : start.line + 1,
      column: start == null ? null : start.column + 1,
    );
  }
  final json = _plain(doc);
  if (json is! Map<String, Object?>) {
    throw const MappingFileError(MappingFileProblem.notMapping);
  }
  // A round trip through JSON proves the value is plain JSON (numbers, texts, lists, maps).
  return (jsonDecode(jsonEncode(json)) as Map).cast<String, Object?>();
}

Object? _plain(Object? node) => switch (node) {
  final YamlMap m => <String, Object?>{
    for (final e in m.entries)
      switch (e.key) {
        final String k => k,
        _ => throw const MappingFileError(MappingFileProblem.notMapping),
      }: _plain(e.value),
  },
  final YamlList l => [for (final e in l) _plain(e)],
  _ => node,
};

/// The place in the mapping file of the server's JSON [pointer] (RFC 6901): the members below
/// `/config` joined with dots, a list index in brackets.
///
/// Example: `/config/channels/2/planned_operator` is `channels[2].planned_operator`; `/name`
/// is `name`; `/config` is the whole file, an empty text.
String mappingPathOf(String pointer) {
  var parts = pointer
      .split('/')
      .skip(1)
      .map((p) => p.replaceAll('~1', '/').replaceAll('~0', '~'))
      .toList();
  if (parts.isNotEmpty && parts.first == 'config') parts = parts.sublist(1);
  final out = StringBuffer();
  for (final p in parts) {
    if (int.tryParse(p) != null && out.isNotEmpty) {
      out.write('[$p]');
    } else {
      if (out.isNotEmpty) out.write('.');
      out.write(p);
    }
  }
  return out.toString();
}
