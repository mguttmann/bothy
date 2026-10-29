// SPDX-License-Identifier: Apache-2.0

// The steps of the new project dialog over a fake port (01 6.9; docs/decisions.d/M4-43.md
// D-M4-43-1, D-M4-43-8, D-M4-43-9, D-M4-43-11).

import 'package:client/new_project/new_project_flow.dart';
import 'package:domain/domain.dart';
import 'package:flutter_test/flutter_test.dart';

final _space = SpaceId.parse('0192f4a0-7b1c-72d4-8bae-000000000001');
final _forum = ChannelId.parse('0192f4a0-7b1c-72d4-8bae-000000000002');
final _spaceRunner = PrincipalId.parse('0192f4a0-7b1c-72d4-8bae-000000000003');
final _otherRunner = PrincipalId.parse('0192f4a0-7b1c-72d4-8bae-000000000004');

/// A port that records every call and answers from scripted values.
final class FakePort implements NewProjectPort {
  final List<String> calls = [];
  final List<String> keys = [];
  final List<BindingReadiness?> answers = [];
  NewProjectFailure? failCreate;
  NewProjectFailure? failBind;
  Object? failAssign;

  @override
  Future<ChannelId> createForum(
    SpaceId space,
    String name, {
    required String idempotencyKey,
  }) async {
    calls.add('createForum $name');
    keys.add(idempotencyKey);
    final f = failCreate;
    if (f != null) {
      failCreate = null;
      throw f;
    }
    return _forum;
  }

  @override
  Future<void> assignRunner(ChannelId forum, PrincipalId agent) async {
    calls.add('assignRunner ${agent.value}');
    final f = failAssign;
    if (f != null) {
      failAssign = null;
      throw f;
    }
  }

  @override
  Future<void> bind(
    ChannelId forum, {
    required String workspaceRef,
    required String provision,
    String? repositoryUrl,
  }) async {
    calls.add('bind $workspaceRef $provision ${repositoryUrl ?? '-'}');
    final f = failBind;
    if (f != null) {
      failBind = null;
      throw f;
    }
  }

  @override
  Future<BindingReadiness?> readiness(ChannelId forum) async {
    calls.add('readiness');
    return answers.length > 1 ? answers.removeAt(0) : answers.firstOrNull;
  }

  @override
  Future<void> saveIdea(ChannelId forum, String idea) async =>
      calls.add('saveIdea $idea');

  @override
  Future<String> postIdea(ChannelId forum, String idea) async {
    calls.add('postIdea $idea');
    return 'entry-1';
  }
}

const _ready = BindingReadiness(ready: true);
const _provisioning = BindingReadiness(ready: false, reason: 'provisioning');

final class Harness {
  Harness() {
    flow = NewProjectFlow(
      port: port,
      space: _space,
      spaceRunner: _spaceRunner,
      pollInterval: const Duration(seconds: 2),
      wait: const Duration(seconds: 10),
      nameMaxChars: 40,
      now: () => now,
      sleep: (d) async {
        sleeps.add(d);
        now = now.add(d);
        // A timer turn, so the test's own waits get the event loop between two reads.
        await Future<void>.delayed(Duration.zero);
      },
      newKey: () => 'key-${++_keys}',
    );
  }

  final FakePort port = FakePort();
  late final NewProjectFlow flow;
  final List<Duration> sleeps = [];
  DateTime now = DateTime.utc(2026, 10, 28, 9);
  int _keys = 0;
}

void main() {
  test(
    'empty project with the space runner: forum, binding init, wait, done',
    () async {
      final h = Harness();
      h.port.answers.addAll([null, _provisioning, _ready]);
      h.flow
        ..mode = NewProjectMode.empty
        ..name = 'website';
      expect(h.flow.runner, _spaceRunner);
      expect(h.flow.steps, [
        NewProjectStep.createForum,
        NewProjectStep.bindProject,
        NewProjectStep.prepareFolder,
      ]);
      await h.flow.run();
      expect(h.port.calls, [
        'createForum website',
        'bind website init -',
        'readiness',
        'readiness',
        'readiness',
      ]);
      expect(h.flow.finished, isTrue);
      expect(h.flow.forum, _forum);
      expect(h.sleeps, everyElement(const Duration(seconds: 2)));
      for (final s in h.flow.steps) {
        expect(h.flow.stateOf(s), NewProjectStepState.done);
      }
    },
  );

  test('clone with another runner assigns it before the binding', () async {
    final h = Harness();
    h.port.answers.add(_ready);
    h.flow
      ..mode = NewProjectMode.clone
      ..url = 'https://git.example.com/team/web-site.git'
      ..runner = _otherRunner;
    expect(h.flow.name, 'web-site');
    expect(h.flow.folderCloning, isTrue);
    await h.flow.run();
    expect(h.port.calls, [
      'createForum web-site',
      'assignRunner ${_otherRunner.value}',
      'bind web-site clone https://git.example.com/team/web-site.git',
      'readiness',
    ]);
  });

  test('existing folder binds without creation', () async {
    final h = Harness();
    h.port.answers.add(_ready);
    h.flow
      ..mode = NewProjectMode.existing
      ..name = 'website'
      ..folder = 'team/website';
    await h.flow.run();
    expect(h.port.calls, [
      'createForum website',
      'bind team/website none -',
      'readiness',
    ]);
  });

  test('an idea waits as the draft and is posted when ready', () async {
    final h = Harness();
    h.port.answers.addAll([_provisioning, _ready]);
    h.flow
      ..mode = NewProjectMode.idea
      ..idea = 'A shopping list app';
    expect(h.flow.name, 'a-shopping-list-app');
    expect(h.flow.steps.last, NewProjectStep.postIdea);
    await h.flow.run();
    expect(h.port.calls, [
      'createForum a-shopping-list-app',
      'saveIdea A shopping list app',
      'bind a-shopping-list-app init -',
      'readiness',
      'readiness',
      'postIdea A shopping list app',
    ]);
    expect(h.flow.sentEntry, 'entry-1');
    expect(h.flow.finished, isTrue);
  });

  test('the idea is the draft before the runner is assigned, so a failure keeps it', () async {
    final h = Harness();
    h.port
      ..failAssign = const NewProjectFailure.offline()
      ..answers.add(_ready);
    h.flow
      ..mode = NewProjectMode.idea
      ..idea = 'A shopping list app'
      ..runner = _otherRunner;
    await h.flow.run();
    expect(h.flow.failedStep, NewProjectStep.assignRunner);
    expect(h.port.calls, [
      'createForum a-shopping-list-app',
      'saveIdea A shopping list app',
      'assignRunner ${_otherRunner.value}',
    ]);
    await h.flow.run();
    expect(h.flow.finished, isTrue);
    expect(h.port.calls.where((c) => c.startsWith('saveIdea')), hasLength(1));
  });

  test(
    'an error other than a step failure fails the step and resumes there',
    () async {
      final h = Harness();
      h.port
        ..failAssign = StateError('outbox closed')
        ..answers.add(_ready);
      h.flow
        ..mode = NewProjectMode.empty
        ..name = 'website'
        ..runner = _otherRunner;
      await h.flow.run();
      expect(h.flow.failedStep, NewProjectStep.assignRunner);
      expect(h.flow.failure?.kind, NewProjectFailureKind.problem);
      expect(h.flow.failure?.code, 'internal_error');
      expect(
        h.flow.stateOf(NewProjectStep.assignRunner),
        NewProjectStepState.failed,
      );
      await h.flow.run();
      expect(h.flow.finished, isTrue);
    },
  );

  test('a failed step stops, keeps what exists and resumes; a new name is a new key', () async {
    final h = Harness();
    h.port
      ..failCreate = const NewProjectFailure.offline()
      ..answers.add(_ready);
    h.flow
      ..mode = NewProjectMode.empty
      ..name = 'website';
    await h.flow.run();
    expect(h.flow.failedStep, NewProjectStep.createForum);
    expect(h.flow.failure?.kind, NewProjectFailureKind.offline);
    expect(
      h.flow.stateOf(NewProjectStep.createForum),
      NewProjectStepState.failed,
    );
    expect(
      h.flow.stateOf(NewProjectStep.bindProject),
      NewProjectStepState.waiting,
    );
    h.port.failCreate = const NewProjectFailure.problem('conflict');
    await h.flow.run();
    expect(h.flow.failure?.code, 'conflict');
    expect(h.flow.canEdit(NewProjectField.name), isTrue);
    h.flow.name = 'website-2';
    await h.flow.run();
    expect(h.port.calls.where((c) => c.startsWith('createForum')), [
      'createForum website',
      'createForum website',
      'createForum website-2',
    ]);
    expect(h.port.keys, ['key-1', 'key-1', 'key-2']);
    expect(h.flow.finished, isTrue);
    expect(h.flow.failure, isNull);
  });

  test('a readiness reason fails the folder step; try again binds the edited fields', () async {
    final h = Harness();
    h.port.answers.addAll([
      _provisioning,
      const BindingReadiness(ready: false, reason: 'clone_failed'),
    ]);
    h.flow
      ..mode = NewProjectMode.clone
      ..url = 'https://git.example.com/team/website.git';
    await h.flow.run();
    expect(h.flow.failedStep, NewProjectStep.prepareFolder);
    expect(h.flow.failure?.kind, NewProjectFailureKind.notReady);
    expect(h.flow.failure?.code, 'clone_failed');
    expect(h.flow.canEdit(NewProjectField.url), isTrue);
    expect(h.flow.canEdit(NewProjectField.name), isFalse);
    expect(h.flow.canEdit(NewProjectField.mode), isFalse);
    h.port.answers
      ..clear()
      ..add(_ready);
    h.flow.url = 'https://git.example.com/team/website-new.git';
    expect(h.flow.name, 'website', reason: 'the forum exists; its name stays');
    await h.flow.run();
    expect(
      h.port.calls.where((c) => c.startsWith('createForum')),
      hasLength(1),
    );
    expect(
      h.port.calls.where((c) => c.startsWith('bind')).last,
      'bind website clone https://git.example.com/team/website-new.git',
    );
    expect(h.flow.finished, isTrue);
  });

  test(
    'a taken folder: the folder changes, binds again and the project is ready',
    () async {
      final h = Harness();
      h.port.answers.add(
        const BindingReadiness(ready: false, reason: 'workspace_exists'),
      );
      h.flow
        ..mode = NewProjectMode.idea
        ..idea = 'A website for the club';
      await h.flow.run();
      expect(h.flow.failedStep, NewProjectStep.prepareFolder);
      expect(h.flow.failure?.code, 'workspace_exists');
      expect(h.flow.canEdit(NewProjectField.name), isFalse);
      expect(h.flow.canEdit(NewProjectField.folder), isTrue);
      expect(h.flow.folder, 'a-website-for-the');
      h.port.answers
        ..clear()
        ..add(_ready);
      h.flow.folder = 'club-website';
      await h.flow.run();
      expect(h.port.calls.where((c) => c.startsWith('bind')), [
        'bind a-website-for-the init -',
        'bind club-website init -',
      ]);
      expect(
        h.port.calls.where((c) => c.startsWith('createForum')),
        hasLength(1),
      );
      expect(h.flow.finished, isTrue);
      expect(h.flow.sentEntry, 'entry-1');
    },
  );

  test(
    'a folder bound to another forum: the folder changes and binds',
    () async {
      final h = Harness();
      h.port
        ..failBind = const NewProjectFailure.problem('conflict')
        ..answers.add(_ready);
      h.flow
        ..mode = NewProjectMode.empty
        ..name = 'website';
      await h.flow.run();
      expect(h.flow.failedStep, NewProjectStep.bindProject);
      expect(h.flow.failure?.code, 'conflict');
      expect(h.flow.canEdit(NewProjectField.folder), isTrue);
      h.flow.folder = 'website-2';
      expect(
        h.flow.name,
        'website',
        reason: 'the forum exists; its name stays',
      );
      await h.flow.run();
      expect(h.port.calls.where((c) => c.startsWith('bind')), [
        'bind website init -',
        'bind website-2 init -',
      ]);
      expect(h.flow.finished, isTrue);
    },
  );

  test(
    'a created folder must be one segment; a mode change drops a typed folder',
    () async {
      final h = Harness();
      h.flow
        ..mode = NewProjectMode.existing
        ..name = 'website'
        ..folder = 'team/website'
        ..mode = NewProjectMode.empty;
      expect(h.flow.folder, 'website');
      expect(h.flow.invalidFields, isEmpty);
      h.port.failBind = const NewProjectFailure.problem('conflict');
      await h.flow.run();
      h.flow.folder = 'team/website';
      expect(h.flow.invalidFields, {NewProjectField.folder});
    },
  );

  test('a cancelled step-up fails its step and resumes there', () async {
    final h = Harness();
    h.port
      ..failBind = const NewProjectFailure.stepUpCancelled()
      ..answers.add(_ready);
    h.flow
      ..mode = NewProjectMode.empty
      ..name = 'website';
    await h.flow.run();
    expect(h.flow.failedStep, NewProjectStep.bindProject);
    expect(h.flow.failure?.kind, NewProjectFailureKind.stepUpCancelled);
    expect(
      h.flow.stateOf(NewProjectStep.createForum),
      NewProjectStepState.done,
    );
    await h.flow.run();
    expect(
      h.port.calls.where((c) => c.startsWith('createForum')),
      hasLength(1),
    );
    expect(h.flow.finished, isTrue);
  });

  test(
    'without readiness after the wait the flow says so and keeps reading',
    () async {
      final h = Harness();
      h.port.answers.add(null);
      h.flow
        ..mode = NewProjectMode.empty
        ..name = 'website';
      final run = h.flow.run();
      await pumpUntil(() => h.flow.slow);
      expect(h.flow.finished, isFalse);
      expect(h.flow.failure, isNull);
      expect(
        h.flow.stateOf(NewProjectStep.prepareFolder),
        NewProjectStepState.running,
      );
      h.port.answers
        ..clear()
        ..add(_ready);
      await run;
      expect(h.flow.finished, isTrue);
    },
  );

  test('closing stops the reading', () async {
    final h = Harness();
    h.port.answers.add(null);
    h.flow
      ..mode = NewProjectMode.empty
      ..name = 'website';
    final run = h.flow.run();
    await pumpUntil(
      () => h.port.calls.where((c) => c == 'readiness').length > 2,
    );
    h.flow.dispose();
    await run;
    final reads = h.port.calls.where((c) => c == 'readiness').length;
    await Future<void>.delayed(Duration.zero);
    expect(h.port.calls.where((c) => c == 'readiness').length, reads);
  });

  test('invalid fields block the start', () async {
    final h = Harness();
    h.flow
      ..mode = NewProjectMode.clone
      ..url = 'http://git.example.com/a'
      ..name = '.hidden';
    expect(h.flow.invalidFields, {NewProjectField.name, NewProjectField.url});
    await h.flow.run();
    expect(h.port.calls, isEmpty);
    h.flow
      ..mode = NewProjectMode.idea
      ..name = 'ok';
    expect(h.flow.invalidFields, {NewProjectField.idea});
    h.flow
      ..mode = NewProjectMode.existing
      ..folder = '../x';
    expect(h.flow.invalidFields, {NewProjectField.folder});
    final none = NewProjectFlow(
      port: h.port,
      space: _space,
      spaceRunner: null,
      pollInterval: const Duration(seconds: 2),
      wait: const Duration(seconds: 10),
      nameMaxChars: 40,
      now: DateTime.now,
      newKey: () => 'k',
    )..name = 'website';
    expect(none.invalidFields, {NewProjectField.runner});
  });

  test('a name the person typed is not replaced by the suggestion', () {
    final h = Harness();
    h.flow
      ..mode = NewProjectMode.clone
      ..url = 'https://git.example.com/team/one.git';
    expect(h.flow.name, 'one');
    h.flow.name = 'mine';
    h.flow.url = 'https://git.example.com/team/two.git';
    expect(h.flow.name, 'mine');
  });
}

/// Lets the flow's microtasks run until [done] holds.
Future<void> pumpUntil(bool Function() done) async {
  for (var i = 0; i < 1000 && !done(); i++) {
    await Future<void>.delayed(Duration.zero);
  }
  expect(done(), isTrue);
}
