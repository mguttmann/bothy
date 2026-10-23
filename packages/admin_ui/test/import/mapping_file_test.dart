// SPDX-License-Identifier: Apache-2.0

// The mapping file of brief M8-08 Design note 2: the owner's YAML becomes the JSON of
// createImport (strict: a key named twice is an error), and the server's JSON pointers read as
// places in that file.

import 'package:admin_ui/admin_ui.dart';
import 'package:flutter_test/flutter_test.dart';

const String _yaml = '''
version: 1
guild_id: "100"
defaults:
  unmapped_human: placeholder
  unmapped_bot: placeholder
  uncategorized_space: skip
channels:
  - discord_channel: "301"
    target: {create: {name: tickets}}
    planned_operator: "agent:texter"
  - discord_channel: "302"
    target: skip
''';

void main() {
  test('YAML becomes the JSON object of the mapping', () {
    expect(mappingJsonOf(_yaml), {
      'version': 1,
      'guild_id': '100',
      'defaults': {
        'unmapped_human': 'placeholder',
        'unmapped_bot': 'placeholder',
        'uncategorized_space': 'skip',
      },
      'channels': [
        {
          'discord_channel': '301',
          'target': {
            'create': {'name': 'tickets'},
          },
          'planned_operator': 'agent:texter',
        },
        {'discord_channel': '302', 'target': 'skip'},
      ],
    });
  });

  test(
    'an unquoted ID stays a number, so the server reports it at its pointer',
    () {
      expect(mappingJsonOf('guild_id: 100\n')['guild_id'], 100);
    },
  );

  test('a key named twice is refused with its line', () {
    expect(
      () => mappingJsonOf('version: 1\nguild_id: "1"\nguild_id: "2"\n'),
      throwsA(
        isA<MappingFileError>()
            .having((e) => e.problem, 'problem', MappingFileProblem.invalidYaml)
            .having((e) => e.line, 'line', 3),
      ),
    );
  });

  test('broken YAML, a list and a number key are refused', () {
    expect(
      () => mappingJsonOf('channels: [\n'),
      throwsA(
        isA<MappingFileError>().having(
          (e) => e.problem,
          'problem',
          MappingFileProblem.invalidYaml,
        ),
      ),
    );
    for (final text in ['- a\n- b\n', '1: x\n', 'a:\n  2: x\n', '']) {
      expect(
        () => mappingJsonOf(text),
        throwsA(
          isA<MappingFileError>().having(
            (e) => e.problem,
            'problem',
            MappingFileProblem.notMapping,
          ),
        ),
        reason: text,
      );
    }
  });

  test('pointers read as places in the file', () {
    expect(
      mappingPathOf('/config/channels/2/planned_operator'),
      'channels[2].planned_operator',
    );
    expect(mappingPathOf('/config/guild_id'), 'guild_id');
    expect(
      mappingPathOf('/config/spaces/0/target/create/name'),
      'spaces[0].target.create.name',
    );
    expect(mappingPathOf('/name'), 'name');
    expect(mappingPathOf('/config'), '');
    expect(mappingPathOf(''), '');
    expect(mappingPathOf('/config/a~1b/c~0d'), 'a/b.c~d');
  });
}
