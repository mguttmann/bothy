// SPDX-License-Identifier: Apache-2.0

// "Add forums for existing folders" over a fake port (01 6.10; docs/decisions.d/M4-44.md
// D-M4-44-4, D-M4-44-5).

import 'package:client/new_project/new_project_flow.dart';
import 'package:client/new_project/project_sync_flow.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

final _space = SpaceId.parse('0192f4a0-7b1c-72d4-8bae-000000000001');
final _spaceRunner = PrincipalId.parse('0192f4a0-7b1c-72d4-8bae-000000000003');
final _otherRunner = PrincipalId.parse('0192f4a0-7b1c-72d4-8bae-000000000004');

/// The forum the fake creates for the folder [name] (names of at most six letters).
ChannelId _forumOf(String name) => ChannelId.parse(
  '0192f4a0-7b1c-72d4-8bae-${name.codeUnits.map((c) => c.toRadixString(16)).join().padLeft(12, '0')}',
);

/// A port that records every call; failures are scripted per folder name.
final class FakePort implements NewProjectPort, ProjectFolderSource {
  final List<String> calls = [];
  final Map<String, List<String>> keysByName = {};
  final Map<PrincipalId?, RunnerFolders> lists = {};
  final Map<String, NewProjectFailure> failCreate = {};
  final Map<String, NewProjectFailure> failBind = {};
  final Map<ChannelId, List<BindingReadiness?>> answers = {};
  NewProjectFailure? failFolders;
  final Map<ChannelId, String> names = {};

  @override
  Future<RunnerFolders> folders(PrincipalId runner) async {
    calls.add('folders ${runner.value.substring(runner.value.length - 1)}');
    final f = failFolders;
    if (f != null) {
      failFolders = null;
      throw f;
    }
    return lists[runner] ?? const RunnerFolders(names: [], reported: true);
  }

  @override
  Future<ChannelId> createForum(
    SpaceId space,
    String name, {
    required String idempotencyKey,
  }) async {
    calls.add('createForum $name');
    keysByName.putIfAbsent(name, () => []).add(idempotencyKey);
    final f = failCreate.remove(name);
    if (f != null) throw f;
    final id = _forumOf(name);
    names[id] = name;
    return id;
  }

  @override
  Future<void> assignRunner(ChannelId forum, PrincipalId agent) async =>
      calls.add('assignRunner ${names[forum]}');

  @override
  Future<void> bind(
    ChannelId forum, {
    required String workspaceRef,
    required String provision,
    String? repositoryUrl,
  }) async {
    calls.add('bind $workspaceRef $provision');
    final f = failBind.remove(workspaceRef);
    if (f != null) throw f;
  }

  @override
  Future<BindingReadiness?> readiness(ChannelId forum) async {
    calls.add('readiness ${names[forum]}');
    final list = answers[forum];
    if (list == null) return const BindingReadiness(ready: true);
    return list.length > 1 ? list.removeAt(0) : list.first;
  }

  @override
  Future<void> saveIdea(ChannelId forum, String idea) async {}

  @override
  Future<String> postIdea(ChannelId forum, String idea) async => 'entry';
}

final class Harness {
  Harness({PrincipalId? spaceRunner}) {
    flow = ProjectSyncFlow(
      port: port,
      source: port,
      space: _space,
      spaceRunner: spaceRunner ?? _spaceRunner,
      pollInterval: const Duration(seconds: 2),
      wait: const Duration(seconds: 10),
      now: () => now,
      sleep: (d) async {
        now = now.add(d);
        await Future<void>.delayed(Duration.zero);
      },
      newKey: () => 'key-${++_keys}',
    );
  }

  final FakePort port = FakePort();
  late final ProjectSyncFlow flow;
  DateTime now = DateTime.utc(2026, 10, 29, 9);
  int _keys = 0;
}

void main() {
  test('loads the folders of the space runner, all checked', () async {
    final h = Harness();
    h.port.lists[_spaceRunner] = const RunnerFolders(
      names: ['api', 'docs'],
      reported: true,
    );
    await h.flow.load();
    expect(h.flow.runner, _spaceRunner);
    expect(h.flow.rows.map((r) => r.name), ['api', 'docs']);
    expect(h.flow.rows.every((r) => r.checked), isTrue);
    expect(h.flow.reported, isTrue);
    expect(h.flow.canStart, isTrue);
  });

  test('a changed runner loads its own list', () async {
    final h = Harness();
    h.port.lists[_spaceRunner] = const RunnerFolders(
      names: ['api'],
      reported: true,
    );
    h.port.lists[_otherRunner] = const RunnerFolders(
      names: [],
      reported: false,
    );
    await h.flow.load();
    h.flow.runner = _otherRunner;
    await h.flow.load();
    expect(h.port.calls, ['folders 3', 'folders 4']);
    expect(h.flow.rows, isEmpty);
    expect(h.flow.reported, isFalse);
    expect(h.flow.canStart, isFalse);
  });

  test('a failed load keeps its failure', () async {
    final h = Harness();
    h.port.failFolders = const NewProjectFailure.offline();
    await h.flow.load();
    expect(h.flow.loadFailure?.kind, NewProjectFailureKind.offline);
    expect(h.flow.rows, isEmpty);
  });

  test(
    'checked folders get forum and binding none in list order, then readiness',
    () async {
      final h = Harness();
      h.port.lists[_spaceRunner] = const RunnerFolders(
        names: ['api', 'docs', 'web'],
        reported: true,
      );
      await h.flow.load();
      h.flow.setChecked('docs', checked: false);
      await h.flow.run();
      expect(h.port.calls.skip(1), [
        'createForum api',
        'bind api none',
        'createForum web',
        'bind web none',
        'readiness api',
        'readiness web',
      ]);
      expect(h.flow.finished, isTrue);
      expect(h.flow.row('api').state, ProjectSyncRowState.ready);
      expect(h.flow.row('docs').state, ProjectSyncRowState.skipped);
      expect(h.flow.created, [_forumOf('api'), _forumOf('web')]);
    },
  );

  test('another runner than the space default is assigned per forum', () async {
    final h = Harness();
    h.port.lists[_otherRunner] = const RunnerFolders(
      names: ['api'],
      reported: true,
    );
    h.flow.runner = _otherRunner;
    await h.flow.load();
    await h.flow.run();
    expect(h.port.calls.skip(1).take(3), [
      'createForum api',
      'assignRunner api',
      'bind api none',
    ]);
  });

  test('a failed folder does not stop the others; Try again resumes it without a second forum', () async {
    final h = Harness();
    h.port.lists[_spaceRunner] = const RunnerFolders(
      names: ['api', 'docs'],
      reported: true,
    );
    await h.flow.load();
    h.port.failBind['api'] = const NewProjectFailure.problem('conflict');
    await h.flow.run();
    final api = h.flow.row('api');
    expect(api.state, ProjectSyncRowState.failed);
    expect(api.failedStep, NewProjectStep.bindProject);
    expect(api.failure?.code, 'conflict');
    expect(h.flow.row('docs').state, ProjectSyncRowState.ready);
    expect(h.flow.finished, isFalse);
    expect(h.flow.hasFailures, isTrue);
    h.port.calls.clear();
    await h.flow.run();
    expect(h.port.calls, ['bind api none', 'readiness api']);
    expect(h.flow.finished, isTrue);
  });

  test('a failed forum is created again with the same key', () async {
    final h = Harness();
    h.port.lists[_spaceRunner] = const RunnerFolders(
      names: ['api'],
      reported: true,
    );
    await h.flow.load();
    h.port.failCreate['api'] = const NewProjectFailure.offline();
    await h.flow.run();
    expect(h.flow.row('api').failedStep, NewProjectStep.createForum);
    await h.flow.run();
    expect(h.port.keysByName['api'], ['key-1', 'key-1']);
    expect(h.flow.finished, isTrue);
  });

  test('a reason of the runner fails the row at the folder step', () async {
    final h = Harness();
    h.port.lists[_spaceRunner] = const RunnerFolders(
      names: ['api'],
      reported: true,
    );
    h.port.answers[_forumOf('api')] = [
      null,
      const BindingReadiness(ready: false, reason: 'not_a_git_worktree'),
    ];
    await h.flow.load();
    await h.flow.run();
    final api = h.flow.row('api');
    expect(api.state, ProjectSyncRowState.failed);
    expect(api.failedStep, NewProjectStep.prepareFolder);
    expect(api.failure?.code, 'not_a_git_worktree');
  });

  test(
    'rows the runner has not finished within the wait say so and the run ends',
    () async {
      final h = Harness();
      h.port.lists[_spaceRunner] = const RunnerFolders(
        names: ['api'],
        reported: true,
      );
      h.port.answers[_forumOf('api')] = [null];
      await h.flow.load();
      await h.flow.run();
      expect(h.flow.row('api').state, ProjectSyncRowState.notFinished);
      expect(h.flow.busy, isFalse);
      expect(h.flow.finished, isFalse);
    },
  );

  test(
    'the checkboxes and the runner are locked once a forum exists',
    () async {
      final h = Harness();
      h.port.lists[_spaceRunner] = const RunnerFolders(
        names: ['api', 'docs'],
        reported: true,
      );
      await h.flow.load();
      h.port.failCreate['docs'] = const NewProjectFailure.offline();
      await h.flow.run();
      h.flow
        ..setChecked('api', checked: false)
        ..runner = _otherRunner;
      expect(h.flow.row('api').checked, isTrue);
      expect(h.flow.runner, _spaceRunner);
    },
  );
}
