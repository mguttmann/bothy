// SPDX-License-Identifier: Apache-2.0

import 'package:domain/domain.dart';
import 'package:test/test.dart';
import 'package:yaml/yaml.dart';

import 'support.dart';

void main() {
  final doc = loadYaml(readRepoFile('docs/spec/actions.yaml')) as YamlMap;
  final events = [for (final e in doc['events'] as YamlList) e as YamlMap];

  test('the generated catalogue equals docs/spec/actions.yaml', () {
    expect(EventCatalog.types.toSet(), {
      for (final e in events) e['type'] as String,
    });
    for (final e in events) {
      final info = EventCatalog.lookup(e['type'] as String)!;
      expect(info.type, e['type']);
      expect(info.durable, e['kind'] == 'durable', reason: '${e['type']}');
      expect(
        info.carriesResourceVersion,
        e['resource_version'],
        reason: '${e['type']}',
      );
      expect(info.personal, e['audience'] == 'self', reason: '${e['type']}');
      expect(
        (e['resource'] as String).startsWith(info.resourceKind),
        isTrue,
        reason: '${e['type']}',
      );
    }
  });

  test('every durable type is present with its resource.version flag', () {
    final durable = events.where((e) => e['kind'] == 'durable');
    expect(durable, isNotEmpty);
    for (final e in durable) {
      expect(EventCatalog.lookup(e['type'] as String)?.durable, isTrue);
    }
    expect(
      EventCatalog.lookup('message.created')!.carriesResourceVersion,
      isTrue,
    );
    expect(
      EventCatalog.lookup('reaction.added')!.carriesResourceVersion,
      isFalse,
    );
    expect(EventCatalog.lookup('read_state.updated')!.personal, isTrue);
  });
}
