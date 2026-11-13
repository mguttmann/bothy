// SPDX-License-Identifier: Apache-2.0

// The import controller of brief M8-08 Design note 1: sequences of ImportSource snapshots (the
// payload of getImport and of import.progress) replace each other, the newest wins even with
// lower counters, and a refresh joins the mapping with the posted channel items, the channels
// and the planned agents into the rows of the channel table. A refresh reads only the items it
// shows as far as the contract lets it, and one pass runs at a time (ruling AUD-AM1/5).

import 'dart:async';

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

/// A repository whose `getImport` waits for [gate] and counts the passes that [started].
class _GatedRepository extends FakeAdminRepository {
  _GatedRepository() : super(prompt: RecordingPrompt());

  Completer<void>? gate;
  int started = 0;

  @override
  Future<ImportSourceSnapshot> importSource(String id) async {
    started++;
    await gate?.future;
    return super.importSource(id);
  }
}

ImportItemEntry _message(int n, {String state = 'posted'}) => ImportItemEntry(
  id: 'item-m$n',
  originKey: 'message:9$n:0',
  itemKind: 'message',
  state: state,
  error: state == 'failed' ? 'target_gone' : null,
);

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
      expect(details.rows[1].targetName, 'questions');
      // An existing channel that is not imported has no name the console may read.
      expect(details.rows[2].targetName, isNull);
      expect(details.allImportedLedHere, isFalse);
    },
  );

  test('refresh reads the failed items by their state and the posted items '
      'only until every mapped channel has its item', () async {
    final id = repository.seedImport('Old', state: 'running');
    // As the import tool writes them: the channels first, then their content.
    repository.postChannels(id, ['301', '302', '303']);
    repository.importItemsById[id]!.addAll([
      for (var n = 0; n < 7; n++) _message(n),
      _message(7, state: 'failed'),
      _message(8, state: 'pending'),
    ]);
    repository.importItemPageSize = 2;

    await controller.refresh(id);

    // 3 channel items and 7 messages are posted: five pages of two. The read ends with the
    // page that holds the last mapped channel.
    expect(repository.importItemPages, ['failed 1', 'posted 1', 'posted 2']);
    final details = controller.details(id)!;
    expect([for (final r in details.rows) r.imported], [true, true, true]);
    expect([for (final i in details.failedItems) i.id], ['item-m7']);
  });

  test('while a mapped channel has no posted item the read goes through every '
      'posted item, never the others', () async {
    final id = repository.seedImport('Old', state: 'running');
    repository.postChannels(id, ['301']);
    repository.importItemsById[id]!.addAll([
      for (var n = 0; n < 4; n++) _message(n),
      _message(4, state: 'pending'),
      _message(5, state: 'pending'),
    ]);
    repository.importItemPageSize = 2;

    await controller.refresh(id);

    expect(repository.importItemPages, [
      'failed 1',
      'posted 1',
      'posted 2',
      'posted 3',
    ]);
    expect(
      [for (final r in controller.details(id)!.rows) r.imported],
      [true, false, false],
    );
  });

  test('a refresh that is asked while one runs waits for it, and one more pass '
      'serves every such request', () async {
    final gated = _GatedRepository()..stepUpFresh = true;
    final c = ImportController(gated);
    addTearDown(c.dispose);
    final id = gated.seedImport('Old', state: 'running');
    final gate = gated.gate = Completer<void>();
    var done = 0;

    final first = c.refresh(id).then((_) => done++);
    final second = c.refresh(id).then((_) => done++);
    final third = c.refresh(id).then((_) => done++);
    await pumpEventQueue();
    expect(gated.started, 1, reason: 'one pass runs at a time');
    expect(done, 0);

    // What changes while the pass runs is what the later requests asked for.
    gated.importsById[id]!['state'] = 'paused';
    gated.gate = null;
    gate.complete();
    await Future.wait([first, second, third]);
    expect(done, 3);
    expect(
      gated.started,
      2,
      reason: 'the running pass and one pass after it, not one per request',
    );
    expect(c.source(id)!.state, ImportState.paused);

    await c.refresh(id);
    expect(gated.started, 3, reason: 'a later request reads again');
  });

  test('a pass that fails tells every waiting request, and the next request '
      'reads again', () async {
    final gated = _GatedRepository()..stepUpFresh = true;
    final c = ImportController(gated);
    addTearDown(c.dispose);
    final id = gated.seedImport('Old', state: 'running');
    final gate = gated.gate = Completer<void>();
    gated.refuse['importSource'] = const ApiProblem(
      status: 503,
      code: 'unavailable',
    );

    final results = <Object>[];
    final first = c.refresh(id).catchError((Object e) => results.add(e));
    final second = c.refresh(id).catchError((Object e) => results.add(e));
    await pumpEventQueue();
    gated.gate = null;
    gate.complete();
    await Future.wait([first, second]);
    expect(results, hasLength(2));
    expect(results, everyElement(isA<SelfActionFailed>()));

    gated.refuse.remove('importSource');
    await c.refresh(id);
    expect(c.source(id)!.state, ImportState.running);
  });

  test('the phase of a channel led here follows its operator', () async {
    final id = repository.seedImport('Old', state: 'running');
    repository.postChannels(id, ['301', '302']);
    await repository.setImportLead(
      id,
      discordChannel: '301',
      lead: ImportLead.here,
      purpose: 'p',
    );
    await repository.setImportLead(
      id,
      discordChannel: '302',
      lead: ImportLead.here,
      purpose: 'p',
    );
    await controller.refresh(id);
    ImportChannelRow row(String d) =>
        controller.details(id)!.rows.firstWhere((r) => r.mapped.discordId == d);
    expect(row('301').phase, ImportChannelPhase.hereNoOperator);
    // Without a planned operator, no operator is the plan.
    expect(row('302').phase, ImportChannelPhase.hereOperatorOk);
    expect(controller.details(id)!.allImportedLedHere, isTrue);

    repository.operatorElsewhere(id, '301', 'support');
    await controller.refresh(id);
    expect(row('301').phase, ImportChannelPhase.hereOtherOperator);

    repository.operatorElsewhere(id, '301', 'texter');
    await controller.refresh(id);
    expect(row('301').phase, ImportChannelPhase.hereOperatorOk);
    expect(row('301').channel!.readOnly, isFalse);
  });

  test(
    'a channel deleted or archived here no longer blocks completion',
    () async {
      final id = repository.seedImport('Old', state: 'running');
      repository.postChannels(id, ['301', '302']);
      await repository.setImportLead(
        id,
        discordChannel: '302',
        lead: ImportLead.here,
        purpose: 'p',
      );
      // 301 still leads in Discord and its channel is live: completion is not offered.
      await controller.refresh(id);
      expect(controller.details(id)!.allImportedLedHere, isFalse);

      // Deleted here: the channel reads with deleted_at (pending, archived as well).
      repository.channelGoneElsewhere(id, '301');
      await controller.refresh(id);
      final row = controller.details(id)!.rows.first;
      expect(row.mapped.lead, ImportLead.discord);
      expect(row.notLive, isTrue);
      expect(controller.details(id)!.allImportedLedHere, isTrue);

      // Purged for good: getChannel answers 404.
      repository.importChannelsById.remove('ch-301');
      await controller.refresh(id);
      expect(controller.details(id)!.rows.first.notLive, isTrue);
      expect(controller.details(id)!.allImportedLedHere, isTrue);
    },
  );

  test(
    'an unreadable channel that is not gone still blocks completion',
    () async {
      final id = repository.seedImport('Old', state: 'running');
      repository.postChannels(id, ['301']);
      repository.refuse['importChannel'] = const ApiProblem(
        status: 403,
        code: 'forbidden',
      );
      await controller.refresh(id);
      final row = controller.details(id)!.rows.first;
      expect(row.channelError, isNotNull);
      expect(row.notLive, isFalse);
      expect(controller.details(id)!.allImportedLedHere, isFalse);
    },
  );

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
