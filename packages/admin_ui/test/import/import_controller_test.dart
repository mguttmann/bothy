// SPDX-License-Identifier: Apache-2.0

// The import controller of brief M8-08 Design note 1: sequences of ImportSource snapshots (the
// payload of getImport and of import.progress) replace each other, the newest wins even with
// lower counters, and a refresh joins the mapping with the posted channel items, the channels
// and the planned agents into the rows of the channel table.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

Map<String, Object?> _progress(
  String state,
  Map<String, Object?> stats, {
  String id = 'import-9',
}) => {
  'id': id,
  'kind': 'discord',
  'name': 'Old community server',
  'config': fakeMapping(),
  'state': state,
  'stats': stats,
  'created_at': '2026-10-17T08:00:00.000Z',
};

void main() {
  late FakeAdminRepository repository;
  late ImportController controller;

  setUp(() {
    repository = FakeAdminRepository(prompt: RecordingPrompt())
      ..stepUpFresh = true;
    controller = ImportController(repository);
  });

  tearDown(() => controller.dispose());

  test('snapshots replace each other in order: state and counters', () {
    var notified = 0;
    controller
      ..addListener(() => notified++)
      ..applyProgress(_progress('draft', const {}))
      ..applyProgress(
        _progress('running', const {
          'server': {
            'channel': {'posted': 1},
          },
        }),
      )
      ..applyProgress(
        _progress('running', const {
          'server': {
            'channel': {'posted': 2},
            'message': {'posted': 40, 'skipped': 3},
            'source_changed': 1,
          },
          'tool': {
            'threads': {'open': 4, 'done': 6},
          },
        }),
      );
    final s = controller.source('import-9')!;
    expect(notified, 3);
    expect(s.state, ImportState.running);
    expect(
      [for (final c in s.serverCounters) '${c.path}=${c.value}'],
      [
        'channel.posted=2',
        'message.posted=40',
        'message.skipped=3',
        'source_changed=1',
      ],
    );
    expect(
      [for (final c in s.toolCounters) '${c.path}=${c.value}'],
      ['threads.done=6', 'threads.open=4'],
    );
    expect(importPostedCount(s), 42);
    expect(controller.sources, hasLength(1));
  });

  test(
    'an older counter in a newer snapshot still wins: the server is the source',
    () {
      controller
        ..applyProgress(
          _progress('running', const {
            'server': {
              'message': {'posted': 40},
            },
          }),
        )
        ..applyProgress(
          _progress('paused', const {
            'server': {
              'message': {'posted': 12},
            },
          }),
        );
      final s = controller.source('import-9')!;
      expect(s.state, ImportState.paused);
      expect(s.serverCounters.single.value, 12);
    },
  );

  test('a cancelled source reads as failed with cancelled', () {
    controller.applyProgress(
      _progress('failed', const {
        'server': {'cancelled': true},
      }),
    );
    final s = controller.source('import-9')!;
    expect(s.state, ImportState.failed);
    expect(s.cancelled, isTrue);
    // A flag is no counter.
    expect(s.serverCounters, isEmpty);
  });

  test('texts in stats never become counters', () {
    controller.applyProgress(
      _progress('running', const {
        'server': {
          'message': {'posted': 2},
        },
        'tool': {
          'last_message': 'Hello from the old server',
          'attachment': 'contract.pdf',
          'author': 'oldfriend42',
          'people': ['oldfriend42'],
          'messages': 5,
        },
      }),
    );
    final s = controller.source('import-9')!;
    expect([for (final c in s.toolCounters) c.path], ['messages']);
  });

  test('an unknown state offers nothing: it reads as failed', () {
    controller.applyProgress(_progress('archived', const {}));
    expect(controller.source('import-9')!.state, ImportState.failed);
  });

  test('a payload without an ID is refused', () {
    expect(
      () => controller.applyProgress(const {'name': 'x'}),
      throwsFormatException,
    );
  });

  test(
    'loadList keeps the server order and replaces older snapshots',
    () async {
      final first = repository.seedImport('First');
      final second = repository.seedImport('Second', state: 'running');
      await controller.loadList();
      expect([for (final s in controller.sources) s.id], [second, first]);
      repository.importsById[first]!['state'] = 'running';
      await controller.loadList();
      expect(controller.source(first)!.state, ImportState.running);
    },
  );

  test(
    'refresh builds one row per mapped channel that is not skipped',
    () async {
      final id = repository.seedImport('Old', state: 'running');
      repository.postChannels(id, ['301']);
      await controller.refresh(id);
      final details = controller.details(id)!;
      expect(
        [for (final r in details.rows) r.mapped.discordId],
        ['301', '302', '303'],
      );
      final tickets = details.rows[0];
      expect(tickets.imported, isTrue);
      expect(tickets.channel!.readOnly, isTrue);
      expect(tickets.plannedAgent!.id, 'texter');
      expect(tickets.phase, ImportChannelPhase.mirror);
      expect(details.rows[1].phase, ImportChannelPhase.notImported);
      expect(details.rows[1].bothyName, 'questions');
      // An existing channel that is not imported has no name the console may read.
      expect(details.rows[2].bothyName, isNull);
      expect(details.allImportedLedHere, isFalse);
    },
  );

  test('the phase of a channel led here follows its operator', () async {
    final id = repository.seedImport('Old', state: 'running');
    repository.postChannels(id, ['301', '302']);
    await repository.setImportLead(
      id,
      discordChannel: '301',
      lead: ImportLead.bothy,
      purpose: 'p',
    );
    await repository.setImportLead(
      id,
      discordChannel: '302',
      lead: ImportLead.bothy,
      purpose: 'p',
    );
    await controller.refresh(id);
    ImportChannelRow row(String d) =>
        controller.details(id)!.rows.firstWhere((r) => r.mapped.discordId == d);
    expect(row('301').phase, ImportChannelPhase.bothyNoOperator);
    // Without a planned operator, no operator is the plan.
    expect(row('302').phase, ImportChannelPhase.bothyOperatorOk);
    expect(controller.details(id)!.allImportedLedHere, isTrue);

    repository.operatorElsewhere(id, '301', 'support');
    await controller.refresh(id);
    expect(row('301').phase, ImportChannelPhase.bothyOtherOperator);

    repository.operatorElsewhere(id, '301', 'texter');
    await controller.refresh(id);
    expect(row('301').phase, ImportChannelPhase.bothyOperatorOk);
    expect(row('301').channel!.readOnly, isFalse);
  });

  test(
    'failed items are kept; a refused agent list leaves the plan unresolved',
    () async {
      final id = repository.seedImport('Old', state: 'running');
      repository.importItemsById[id]!.add(
        const ImportItemEntry(
          id: 'item-x',
          originKey: 'forum_tag:302:9',
          itemKind: 'forum_tag',
          state: 'failed',
          error: 'name_collision',
        ),
      );
      repository.refuse['agentRecords'] = const ApiProblem(
        status: 403,
        code: 'forbidden',
      );
      await controller.refresh(id);
      final details = controller.details(id)!;
      expect(details.failedItems.single.originKey, 'forum_tag:302:9');
      expect(details.rows.first.plannedAgent, isNull);
      expect(details.agents, isEmpty);
    },
  );
}
