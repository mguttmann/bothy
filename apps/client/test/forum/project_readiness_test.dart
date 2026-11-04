// SPDX-License-Identifier: Apache-2.0

// The banner of a project forum that is not ready (01 6.9 item 8; 09 18.12 and 18.16;
// F-PROJEKT-BEREIT): the cause, an offline runner, the action for an owner, and nothing once
// the project is ready.

import 'package:client/forum/project_readiness.dart';
import 'package:client/forum/providers.dart';
import 'package:client/new_project/new_project_flow.dart';
import 'package:client/notifications_settings/providers.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../support/widgets.dart';

final _forum = ChannelId.parse('0192f4a0-7b1c-72d4-8bae-000000000002');
final _runner = PrincipalId.parse('0192f4a0-7b1c-72d4-8bae-000000000003');

/// A port with scripted readiness that records the actions.
final class FakeReadiness implements ProjectReadinessPort {
  FakeReadiness(this.views);

  /// The answers of [read] in order; the last one repeats.
  final List<ProjectReadinessView?> views;
  final List<String> calls = [];

  @override
  Future<ProjectReadinessView?> read(ChannelId forum) async {
    calls.add('read');
    return views.length > 1 ? views.removeAt(0) : views.first;
  }

  @override
  Future<void> setRunner(
    ChannelId forum,
    PrincipalId agent, {
    required String reason,
    required String purpose,
  }) async => calls.add('setRunner ${agent.value}');

  @override
  Future<void> renameFolder(
    ChannelId forum,
    String workspaceRef, {
    required String reason,
    required String purpose,
  }) async => calls.add('renameFolder $workspaceRef');
}

Future<AppLocalizations> pumpBanner(
  WidgetTester tester,
  FakeReadiness port, {
  bool owner = true,
}) async {
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        projectReadinessPortProvider.overrideWithValue(port),
        isOwnerProvider.overrideWith((ref) async => owner),
      ],
      child: hostScreen(
        Scaffold(
          body: ProjectReadinessBanner(forum: _forum, forumName: 'Shop'),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return AppLocalizations.delegate.load(const Locale('en'));
}

void main() {
  testWidgets(
    'project_not_ready without a reason names the cause and an offline runner',
    (tester) async {
      final port = FakeReadiness([
        const ProjectReadinessView(
          ready: false,
          runnerName: 'Claude Code',
          runnerOffline: true,
        ),
      ]);
      final l = await pumpBanner(tester, port);
      expect(
        find.text(l.projectNotReady(l.projectReasonWaiting)),
        findsOneWidget,
      );
      expect(find.text(l.projectRunnerOffline('Claude Code')), findsOneWidget);
      expect(
        find.byKey(const ValueKey('project.banner.setRunner')),
        findsNothing,
      );
      await tester.tap(find.byKey(const ValueKey('project.banner.check')));
      await tester.pumpAndSettle();
      expect(port.calls, ['read', 'read']);
    },
  );

  testWidgets(
    'a chat agent as operator: the owner makes the runner the operator and the banner goes',
    (tester) async {
      final port = FakeReadiness([
        ProjectReadinessView(
          ready: false,
          reason: reasonOperatorCannotServe,
          runnerName: 'Hermes',
          fixRunner: ProjectRunner(id: _runner, name: 'Claude Code'),
        ),
        const ProjectReadinessView(ready: true),
      ]);
      final l = await pumpBanner(tester, port);
      expect(
        find.text(l.projectNotReady(l.projectReasonOperatorCannotServe)),
        findsOneWidget,
      );
      await tester.tap(find.text(l.projectActionSetRunner('Claude Code')));
      await tester.pumpAndSettle();
      expect(port.calls, ['read', 'setRunner ${_runner.value}', 'read']);
      expect(find.byKey(const ValueKey('project.banner')), findsNothing);
    },
  );

  testWidgets('a taken folder: the owner names another folder', (tester) async {
    final port = FakeReadiness([
      const ProjectReadinessView(ready: false, reason: reasonWorkspaceExists),
      const ProjectReadinessView(ready: false, reason: 'provisioning'),
    ]);
    final l = await pumpBanner(tester, port);
    await tester.tap(find.byKey(const ValueKey('project.banner.rename')));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(const ValueKey('project.rename.field')),
      'shop-2',
    );
    await tester.tap(find.byKey(const ValueKey('project.rename.save')));
    await tester.pumpAndSettle();
    expect(port.calls, ['read', 'renameFolder shop-2', 'read']);
    expect(
      find.text(l.projectNotReady(l.projectReasonProvisioning)),
      findsOneWidget,
    );
  });

  testWidgets(
    'a person who is not an owner sees the cause without the owner action',
    (tester) async {
      final port = FakeReadiness([
        ProjectReadinessView(
          ready: false,
          reason: reasonOperatorCannotServe,
          fixRunner: ProjectRunner(id: _runner, name: 'Claude Code'),
        ),
      ]);
      await pumpBanner(tester, port, owner: false);
      expect(
        find.byKey(const ValueKey('project.banner.reason')),
        findsOneWidget,
      );
      expect(
        find.byKey(const ValueKey('project.banner.setRunner')),
        findsNothing,
      );
    },
  );

  testWidgets('a ready project and an unreadable binding show nothing', (
    tester,
  ) async {
    await pumpBanner(
      tester,
      FakeReadiness([const ProjectReadinessView(ready: true)]),
    );
    expect(find.byKey(const ValueKey('project.banner')), findsNothing);
    await pumpBanner(tester, FakeReadiness([null]));
    expect(find.byKey(const ValueKey('project.banner')), findsNothing);
  });
}
