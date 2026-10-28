// SPDX-License-Identifier: Apache-2.0

// The new project dialog over a fake port (01 6.9 items 2 to 7; docs/decisions.d/M4-43.md
// D-M4-43-8 to D-M4-43-11).

import 'package:client/new_project/new_project_dialog.dart';
import 'package:client/new_project/new_project_flow.dart';
import 'package:client/new_project/new_project_port.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../support/widgets.dart';

final _space = SpaceId.parse('0192f4a0-7b1c-72d4-8bae-000000000001');
final _forum = ChannelId.parse('0192f4a0-7b1c-72d4-8bae-000000000002');
final _runnerA = PrincipalId.parse('0192f4a0-7b1c-72d4-8bae-000000000003');
final _runnerB = PrincipalId.parse('0192f4a0-7b1c-72d4-8bae-000000000004');

/// The dialog's clock; the slow test moves it with the pumps.
DateTime fakeNow = DateTime.utc(2026, 10, 28, 9);

final class ScriptedPort implements NewProjectPort {
  final List<String> calls = [];
  final List<BindingReadiness?> answers = [];
  NewProjectFailure? failCreate;

  @override
  Future<ChannelId> createForum(
    SpaceId space,
    String name, {
    required String idempotencyKey,
  }) async {
    calls.add('createForum $name');
    final f = failCreate;
    if (f != null) {
      failCreate = null;
      throw f;
    }
    return _forum;
  }

  @override
  Future<void> assignRunner(ChannelId forum, PrincipalId agent) async =>
      calls.add('assignRunner');

  @override
  Future<void> bind(
    ChannelId forum, {
    required String workspaceRef,
    required String provision,
    String? repositoryUrl,
  }) async =>
      calls.add('bind $workspaceRef $provision ${repositoryUrl ?? '-'}');

  @override
  Future<BindingReadiness?> readiness(ChannelId forum) async {
    calls.add('readiness');
    return answers.length > 1 ? answers.removeAt(0) : answers.firstOrNull;
  }

  @override
  Future<void> saveIdea(ChannelId forum, String idea) async =>
      calls.add('saveIdea');

  @override
  Future<String> postIdea(ChannelId forum, String idea) async {
    calls.add('postIdea');
    return 'entry-1';
  }
}

class Opener extends StatefulWidget {
  const Opener({super.key, required this.port, this.results});

  final ScriptedPort port;
  final List<NewProjectResult?>? results;

  @override
  State<Opener> createState() => _OpenerState();
}

class _OpenerState extends State<Opener> {
  @override
  Widget build(BuildContext context) => Center(
    child: TextButton(
      onPressed: () async {
        final r = await showDialog<NewProjectResult>(
          context: context,
          barrierDismissible: false,
          builder: (_) => NewProjectDialog(
            spaceId: _space,
            spaceName: 'Engineering',
            spaceRunner: _runnerA,
            port: widget.port,
            runners: () async => [
              ProjectRunner(id: _runnerA, name: 'Code Runner'),
              ProjectRunner(id: _runnerB, name: 'Lab Runner'),
            ],
            pollInterval: const Duration(seconds: 2),
            wait: const Duration(seconds: 10),
            nameMaxChars: 40,
            now: () => fakeNow,
            newKey: () => 'key',
          ),
        );
        widget.results?.add(r);
      },
      child: const Text('open'),
    ),
  );
}

Future<AppLocalizations> open(
  WidgetTester tester,
  ScriptedPort port, [
  List<NewProjectResult?>? results,
]) async {
  tester.view.physicalSize = const Size(1400, 1200);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(hostScreen(Opener(port: port, results: results)));
  await tester.tap(find.text('open'));
  await tester.pumpAndSettle();
  return AppLocalizations.delegate.load(const Locale('en'));
}

void main() {
  testWidgets(
    'clone: the name follows the URL, then the steps run and the forum opens',
    (tester) async {
      final port = ScriptedPort()
        ..answers.addAll([
          const BindingReadiness(ready: false, reason: 'provisioning'),
          const BindingReadiness(ready: true),
        ]);
      final results = <NewProjectResult?>[];
      final l = await open(tester, port, results);
      expect(
        find.text(l.newProjectIn(dsIsolate('Engineering'))),
        findsOneWidget,
      );
      await tester.tap(find.byKey(const ValueKey('newProject.mode.clone')));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byKey(const ValueKey('newProject.url')),
        'https://git.example.com/team/web-site.git',
      );
      await tester.pump();
      final name = tester.widget<TextField>(
        find.byKey(const ValueKey('newProject.name')),
      );
      expect(name.controller!.text, 'web-site');
      await tester.tap(find.byKey(const ValueKey('newProject.create')));
      await tester.pump();
      for (var i = 0; i < 5; i++) {
        await tester.pump(const Duration(seconds: 2));
      }
      await tester.pumpAndSettle();
      expect(port.calls, [
        'createForum web-site',
        'bind web-site clone https://git.example.com/team/web-site.git',
        'readiness',
        'readiness',
      ]);
      expect(find.byType(NewProjectDialog), findsNothing);
      // Closed once: the page below the dialog is still there.
      expect(find.text('open'), findsOneWidget);
      expect(results.single?.forum, _forum);
      expect(results.single?.sentEntry, isNull);
    },
  );

  testWidgets('invalid fields show their errors and start nothing', (
    tester,
  ) async {
    final port = ScriptedPort();
    final l = await open(tester, port);
    await tester.tap(find.byKey(const ValueKey('newProject.create')));
    await tester.pumpAndSettle();
    expect(find.text(l.newProjectNameInvalid(40)), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('newProject.mode.clone')));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(const ValueKey('newProject.url')),
      'http://git.example.com/a',
    );
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('newProject.create')));
    await tester.pumpAndSettle();
    expect(find.text(l.newProjectUrlInvalid), findsOneWidget);
    expect(port.calls, isEmpty);
  });

  testWidgets('a failed step shows its sentence; try again resumes', (
    tester,
  ) async {
    final port = ScriptedPort()
      ..failCreate = const NewProjectFailure.problem('conflict')
      ..answers.add(const BindingReadiness(ready: true));
    final l = await open(tester, port);
    await tester.enterText(
      find.byKey(const ValueKey('newProject.name')),
      'website',
    );
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('newProject.create')));
    await tester.pumpAndSettle();
    expect(find.text(l.newProjectErrNameTaken), findsOneWidget);
    expect(
      find.bySemanticsLabel(
        l.newProjectStepState(l.newProjectStepForum, 'failed'),
      ),
      findsOneWidget,
    );
    await tester.enterText(
      find.byKey(const ValueKey('newProject.name')),
      'website-2',
    );
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('newProject.retry')));
    await tester.pumpAndSettle();
    expect(port.calls.first, 'createForum website');
    expect(port.calls[1], 'createForum website-2');
    expect(find.byType(NewProjectDialog), findsNothing);
  });

  testWidgets(
    'a readiness reason names the cause; the runner field and the idea flow',
    (tester) async {
      final port = ScriptedPort()
        ..answers.add(
          const BindingReadiness(ready: false, reason: 'workspace_exists'),
        );
      final l = await open(tester, port);
      await tester.tap(find.byKey(const ValueKey('newProject.mode.idea')));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byKey(const ValueKey('newProject.idea')),
        'A shopping list app',
      );
      await tester.pump();
      expect(
        tester
            .widget<TextField>(find.byKey(const ValueKey('newProject.name')))
            .controller!
            .text,
        'a-shopping-list-app',
      );
      await tester.tap(find.byKey(const ValueKey('newProject.create')));
      await tester.pumpAndSettle();
      expect(find.text(l.newProjectErrWorkspaceExists), findsOneWidget);
      expect(port.calls, [
        'createForum a-shopping-list-app',
        'saveIdea',
        'bind a-shopping-list-app init -',
        'readiness',
      ]);
      expect(find.text(l.newProjectStepCreating), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('newProject.close')));
      await tester.pumpAndSettle();
      expect(find.byType(NewProjectDialog), findsNothing);
    },
  );

  testWidgets('without readiness after the wait the dialog says so', (
    tester,
  ) async {
    final port = ScriptedPort()..answers.add(null);
    final l = await open(tester, port);
    await tester.enterText(
      find.byKey(const ValueKey('newProject.name')),
      'website',
    );
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('newProject.create')));
    await tester.pump();
    expect(find.text(l.newProjectSlow), findsNothing);
    for (var i = 0; i < 7; i++) {
      fakeNow = fakeNow.add(const Duration(seconds: 2));
      await tester.pump(const Duration(seconds: 2));
    }
    expect(find.text(l.newProjectSlow), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('newProject.close')));
    await tester.pumpAndSettle();
    expect(find.byType(NewProjectDialog), findsNothing);
  });

  test('every readiness reason of 09 18.12 has its own sentence', () async {
    final l = await AppLocalizations.delegate.load(const Locale('en'));
    final texts = {
      for (final code in [
        'outside_root',
        'workspace_missing',
        'not_a_git_worktree',
        'not_writable',
        'workspace_exists',
        'init_failed',
        'clone_failed',
        'provision_timeout',
        'provision_refused',
      ])
        newProjectFailureText(
          l,
          NewProjectStep.prepareFolder,
          NewProjectFailure.notReady(code),
        ),
    };
    expect(texts, hasLength(9));
    expect(
      newProjectFailureText(
        l,
        NewProjectStep.prepareFolder,
        const NewProjectFailure.notReady('later_code'),
      ),
      l.newProjectErrNotReady('later_code'),
    );
    expect(
      newProjectFailureText(
        l,
        NewProjectStep.bindProject,
        const NewProjectFailure.problem('conflict'),
      ),
      l.newProjectErrFolderBound,
    );
    expect(
      newProjectFailureText(
        l,
        NewProjectStep.bindProject,
        const NewProjectFailure.stepUpCancelled(),
      ),
      l.newProjectErrStepUp,
    );
    expect(
      newProjectFailureText(
        l,
        NewProjectStep.createForum,
        const NewProjectFailure.offline(),
      ),
      l.newProjectErrOffline,
    );
  });
}
