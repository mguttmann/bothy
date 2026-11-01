// SPDX-License-Identifier: Apache-2.0

import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support/views.dart';

/// Every key of [json], recursively.
Iterable<String> _keys(Object? json) sync* {
  if (json is Map<String, Object?>) {
    for (final MapEntry(:key, :value) in json.entries) {
      yield key;
      yield* _keys(value);
    }
  } else if (json is List<Object?>) {
    for (final item in json) {
      yield* _keys(item);
    }
  }
}

/// Every value of the members [names] in [json], recursively.
Iterable<String> _values(Object? json, Set<String> names) sync* {
  if (json is Map<String, Object?>) {
    for (final MapEntry(:key, :value) in json.entries) {
      if (names.contains(key) && value is String) yield value;
      yield* _values(value, names);
    }
  } else if (json is List<Object?>) {
    for (final item in json) {
      yield* _values(item, names);
    }
  }
}

/// The fixtures of the statistics views. The run documents `Run.*.json` of the run graph are
/// run content under `run.read`, not level a statistics (25 1, R-VZ-14), so they carry titles.
final Map<String, String> _statsFixtures = {
  for (final MapEntry(:key, :value) in fixtureFiles.entries)
    if (!key.startsWith('Run.')) key: value,
};

void main() {
  test('no fixture schema has a member for a person or for content', () {
    // The schemas carry no person and no content (25 1 rules 2 and 3), so there is no field
    // a canary could go into: the check is by schema, over every member of every fixture.
    // `name` is the tool name of a step, which the views may show (25 1 rule 3).
    // The one exception is the chat of a run (`StatsRunChat`, 25 4 rule 6): its channel name
    // and thread title, which the server sends only to a person who may read the thread.
    final person = RegExp(
      r'principal|person|people_id|triggered|actor|^user_id$|title|text|'
      r'prompt|detail|content|body|message$',
    );
    const chat = {'thread_title'};
    for (final MapEntry(key: file, value: text) in _statsFixtures.entries) {
      final keys = _keys(jsonDecode(text)).toSet().difference(chat);
      expect(keys.where(person.hasMatch), isEmpty, reason: file);
    }
  });

  final ids = <String>{
    for (final text in _statsFixtures.values)
      ..._values(jsonDecode(text), {'thread_id', 'run_id'}),
  };

  for (final MapEntry(key: kind, value: states) in viewStates.entries) {
    for (final state in states) {
      for (final asTable in [false, true]) {
        // A run label shows the short form of a run or chat ID as its secondary text (25 4
        // rule 6); a full ID never appears.
        testWidgets('${kind.name} $state shows no full thread or run ID and no '
            'person (${asTable ? 'table' : 'view'})', (tester) async {
          final world = ViewWorld(state: state);
          await tester.runAsync(world.load);
          await pumpWorld(tester, world, world.view(kind));
          final toggle = find.byKey(const ValueKey('viz-as-table'));
          if (asTable && toggle.evaluate().isNotEmpty) {
            await tester.tap(toggle);
            await tester.pumpAndSettle();
          }
          final text = renderedTexts(tester).join('\n');
          for (final id in ids) {
            expect(text, isNot(contains(id)));
          }
        });
      }
    }
  }
}
