// SPDX-License-Identifier: Apache-2.0

// The dialog "Add forums for existing folders" over a fake port (01 6.10; docs/decisions.d/
// M4-44.md D-M4-44-4, D-M4-44-5).

import 'package:client/new_project/new_project_flow.dart';
import 'package:client/new_project/new_project_port.dart';
import 'package:client/new_project/project_sync_dialog.dart';
import 'package:client/new_project/project_sync_flow.dart';
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../support/widgets.dart';

final _space = SpaceId.parse('0192f4a0-7b1c-72d4-8bae-000000000001');
final _runner = PrincipalId.parse('0192f4a0-7b1c-72d4-8bae-000000000003');

ChannelId _forumOf(String name) => ChannelId.parse(
  '0192f4a0-7b1c-72d4-8bae-${name.codeUnits.map((c) => c.toRadixString(16)).join().padLeft(12, '0')}',
);

final class ScriptedPort implements NewProjectPort, ProjectFolderSource {
  ScriptedPort(this.list);

  final RunnerFolders list;
  final List<String> calls = [];
  final Map<String, NewProjectFailure> failBind = {};

  @override
  Future<RunnerFolders> folders(PrincipalId runner) async {
    calls.add('folders');
    return list;
  }

  @override
  Future<ChannelId> createForum(
    SpaceId space,
    String name, {
    required String idempotencyKey,
  }) async {
    calls.add('createForum $name');
    return _forumOf(name);
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
  }) async {
    calls.add('bind $workspaceRef $provision');
    final f = failBind.remove(workspaceRef);
    if (f != null) throw f;
  }

  @override
  Future<BindingReadiness?> readiness(ChannelId forum) async =>
      const BindingReadiness(ready: true);

  @override
  Future<void> saveIdea(ChannelId forum, String idea) async {}

  @override
  Future<String> postIdea(ChannelId forum, String idea) async => 'entry';
}

class Opener extends StatelessWidget {
  const Opener({super.key, required this.port, required this.results});

  final ScriptedPort port;
  final List<List<ChannelId>?> results;

  @override
  Widget build(BuildContext context) => Center(
    child: TextButton(
      onPressed: () async {
        results.add(
          await showDialog<List<ChannelId>>(
            context: context,
            barrierDismissible: false,
            builder: (_) => ProjectSyncDialog(
              spaceId: _space,
              spaceName: 'Engineering',
              spaceRunner: _runner,
              port: port,
              source: port,
              runners: () async => [
                ProjectRunner(id: _runner, name: 'Code Runner'),
              ],
              pollInterval: const Duration(seconds: 2),
              wait: const Duration(seconds: 10),
              now: () => DateTime.utc(2026, 10, 29, 9),
              newKey: () => 'key',
            ),
          ),
        );
      },
      child: const Text('open'),
    ),
  );
}

Future<AppLocalizations> open(
  WidgetTester tester,
  ScriptedPort port,
  List<List<ChannelId>?> results,
) async {
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
    'the folders show checked; only the checked ones get a forum, then it closes',
    (tester) async {
      final port = ScriptedPort(
        const RunnerFolders(names: ['api', 'docs', 'web'], reported: true),
      );
      final results = <List<ChannelId>?>[];
      final l = await open(tester, port, results);
      expect(
        find.text(l.projectSyncTitle(dsIsolate('Engineering'))),
        findsOneWidget,
      );
      expect(find.byKey(const ValueKey('projectSync.intro')), findsOneWidget);
      for (final n in ['api', 'docs', 'web']) {
        final box = tester.widget<Checkbox>(
          find.byKey(ValueKey('projectSync.check.$n')),
        );
        expect(box.value, isTrue);
      }
      await tester.tap(find.byKey(const ValueKey('projectSync.check.docs')));
      await tester.pump();
      expect(port.calls, ['folders']);
      await tester.tap(find.byKey(const ValueKey('projectSync.add')));
      await tester.pumpAndSettle();
      expect(port.calls.skip(1), [
        'createForum api',
        'bind api none',
        'createForum web',
        'bind web none',
      ]);
      expect(find.byType(ProjectSyncDialog), findsNothing);
      expect(results.single, [_forumOf('api'), _forumOf('web')]);
    },
  );

  testWidgets(
    'a failed folder shows its sentence and Try again resumes only it',
    (tester) async {
      final port = ScriptedPort(
        const RunnerFolders(names: ['api', 'web'], reported: true),
      )..failBind['api'] = const NewProjectFailure.problem('conflict');
      final results = <List<ChannelId>?>[];
      final l = await open(tester, port, results);
      await tester.tap(find.byKey(const ValueKey('projectSync.add')));
      await tester.pumpAndSettle();
      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('projectSync.detail.api')))
            .data,
        l.newProjectErrFolderBound,
      );
      expect(
        find.byKey(const ValueKey('projectSync.detail.web')),
        findsOneWidget,
      );
      port.calls.clear();
      await tester.tap(find.byKey(const ValueKey('projectSync.retry')));
      await tester.pumpAndSettle();
      expect(port.calls, ['bind api none']);
      expect(results.single, [_forumOf('api'), _forumOf('web')]);
    },
  );

  testWidgets('an empty list says every folder has a forum', (tester) async {
    final port = ScriptedPort(const RunnerFolders(names: [], reported: true));
    await open(tester, port, []);
    expect(find.byKey(const ValueKey('projectSync.empty')), findsOneWidget);
    final add = tester.widget<FilledButton>(
      find.byKey(const ValueKey('projectSync.add')),
    );
    expect(add.onPressed, isNull);
  });

  testWidgets('a runner that never reported its folders is named so', (
    tester,
  ) async {
    final port = ScriptedPort(const RunnerFolders(names: [], reported: false));
    await open(tester, port, []);
    expect(
      find.byKey(const ValueKey('projectSync.notReported')),
      findsOneWidget,
    );
  });
}
