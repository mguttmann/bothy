// SPDX-License-Identifier: Apache-2.0

// The incoming webhooks per channel (brief M6-08 Design notes 4 and 5, Ruling 5; 11 8.1): the
// list follows the chosen channel, create with an existing app, create with the composed
// "incoming app" (install, grant, webhook in this order under one confirmation and one
// step-up), a failure after the install with Retry and Uninstall, revoke after one
// confirmation, and the option "New incoming app" only for owners.

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';
import 'incoming_support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;
  late int changes;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
    seedIncoming(repository);
    changes = 0;
  });

  Future<void> tap(WidgetTester tester, String key) async {
    await tester.ensureVisible(find.byKey(ValueKey(key)).last);
    await tester.tap(find.byKey(ValueKey(key)).last);
    await tester.pumpAndSettle();
  }

  Future<void> type(WidgetTester tester, String key, String text) async {
    await tester.enterText(find.byKey(ValueKey(key)), text);
    await tester.pump();
  }

  Future<void> view(WidgetTester tester, {Key? key}) async {
    await pumpInApp(
      tester,
      IncomingWebhooksView(
        key: key,
        repository: repository,
        timeZone: 'Europe/Berlin',
        clock: testClock,
        onChanged: () => changes++,
        newSlugSuffix: () => 'abc12345',
      ),
      navigatorKey: navigator,
    );
  }

  Finder inRow(String id, Finder f) => find.descendant(
    of: find.byKey(ValueKey('incoming.row.$id')),
    matching: f,
  );

  String step(WidgetTester tester, String name) =>
      tester.widget<Text>(find.byKey(ValueKey('incoming.step.$name'))).data!;

  bool canSubmit(WidgetTester tester) =>
      tester
          .widget<FilledButton>(find.byKey(const ValueKey('incoming.submit')))
          .onPressed !=
      null;

  testWidgets('the list follows the chosen channel; spaces are no choice', (
    tester,
  ) async {
    await view(tester);

    expect(find.byKey(const ValueKey('incoming.view')), findsOneWidget);
    expect(find.text('Incoming webhooks'), findsOneWidget);
    // The first channel is chosen; it reads as its path.
    expect(find.text('Marketing/Kampagnen'), findsOneWidget);
    expect(inRow('hook-a', find.text('Build alerts')), findsOneWidget);
    expect(
      inRow('hook-a', find.textContaining('writes as Repo Bridge')),
      findsOneWidget,
    );
    expect(
      inRow('hook-a', find.textContaining('created Oct 1, 2026')),
      findsOneWidget,
    );
    expect(
      inRow('hook-a', find.textContaining('last used Today')),
      findsOneWidget,
    );
    expect(find.byKey(const ValueKey('incoming.row.hook-b')), findsNothing);

    await tap(tester, 'incoming.channel');
    expect(
      find.byKey(const ValueKey('incoming.channel.marketing')),
      findsNothing,
    );
    await tap(tester, 'incoming.channel.internal');
    expect(find.byKey(const ValueKey('incoming.row.hook-a')), findsNothing);
    expect(inRow('hook-b', find.text('Old pager')), findsOneWidget);
    expect(inRow('hook-b', find.textContaining('never used')), findsOneWidget);
    // The app is no longer listed, so its name is unknown: no "writes as".
    expect(inRow('hook-b', find.textContaining('writes as')), findsNothing);
  });

  testWidgets('create with an existing active app shows the address once', (
    tester,
  ) async {
    await view(tester);
    await tap(tester, 'incoming.create');

    expect(find.byKey(const ValueKey('incoming.dialog')), findsOneWidget);
    expect(find.text('New incoming webhook in Marketing/Kampagnen'), findsOne);
    expect(canSubmit(tester), isFalse);
    await type(tester, 'incoming.name', 'Deploy alerts');
    await tap(tester, 'incoming.mode.existing');
    expect(canSubmit(tester), isFalse, reason: 'no app chosen');
    await tap(tester, 'incoming.app');
    // A suspended app cannot write, so it is no choice.
    expect(
      find.byKey(const ValueKey('incoming.app.app-paused-bot')),
      findsNothing,
    );
    await tap(tester, 'incoming.app.app-repo-bridge');
    expect(canSubmit(tester), isTrue);

    await tap(tester, 'incoming.submit');
    expect(find.byKey(const ValueKey('stepUp.dialog')), findsOneWidget);
    await tap(tester, 'stepUp.continue');

    expect(repository.calls, [
      'createIncomingWebhook campaigns app-repo-bridge Deploy alerts',
    ]);
    expect(repository.stepUps, [
      'Create an incoming webhook in Marketing/Kampagnen',
    ]);
    final token = repository.handedOut.single;
    expect(find.byKey(const ValueKey('incoming.dialog')), findsNothing);
    expect(find.byKey(const ValueKey('incoming.secretDialog')), findsOneWidget);
    expect(
      find.text('https://chat.example.com/api/v1/hooks/hook-0/$token'),
      findsOneWidget,
    );
    expect(
      find.text(
        'Senders that can set a header drop the last segment of the address '
        'and send it as Authorization: Bearer <token>.',
      ),
      findsOneWidget,
    );

    await tap(tester, 'incoming.secretClose');
    expect(inRow('hook-0', find.text('Deploy alerts')), findsOneWidget);
    expect(find.textContaining(token), findsNothing);
    expect(changes, 1);
  });

  testWidgets(
    'a back action cannot close the dialog while a run is in progress',
    (tester) async {
      bool canPop() => (tester.widget(
        find
            .ancestor(
              of: find.byKey(const ValueKey('incoming.dialog')),
              matching: find.byWidgetPredicate((w) => w is PopScope),
            )
            .first,
      ) as PopScope).canPop;
      await view(tester);
      await tap(tester, 'incoming.create');
      expect(canPop(), isTrue);
      await type(tester, 'incoming.name', 'Deploy alerts');
      await tap(tester, 'incoming.mode.existing');
      await tap(tester, 'incoming.app');
      await tap(tester, 'incoming.app.app-repo-bridge');
      await tap(tester, 'incoming.submit');
      // The run waits for the step-up: the address it will return must not be lost.
      expect(find.byKey(const ValueKey('stepUp.dialog')), findsOneWidget);
      expect(canPop(), isFalse);
      await tap(tester, 'stepUp.continue');
      expect(
        find.byKey(const ValueKey('incoming.secretDialog')),
        findsOneWidget,
      );
    },
  );

  testWidgets('create with the incoming app: install, grant write on exactly '
      'this channel, webhook, in this order with one step-up', (tester) async {
    await view(tester);
    await tap(tester, 'incoming.create');
    await type(tester, 'incoming.name', 'Alerts');
    await tap(tester, 'incoming.mode.newApp');
    expect(find.byKey(const ValueKey('incoming.app')), findsNothing);
    expect(canSubmit(tester), isFalse, reason: 'no reason');
    await type(tester, 'incoming.reason', 'Monitoring');
    await tap(tester, 'incoming.submit');
    await tap(tester, 'stepUp.continue');

    expect(repository.calls, [
      'installApp incoming-abc12345 messages:write  Monitoring',
      'createGrant app-incoming-abc12345-0 channel.campaigns write Monitoring',
      'createIncomingWebhook campaigns app-incoming-abc12345-0 Alerts',
    ]);
    expect(repository.stepUps, hasLength(1));
    final manifest = repository.appsById['app-incoming-abc12345-0']!.manifest;
    expect(manifest.slug, 'incoming-abc12345');
    expect(manifest.name, 'Alerts');
    expect(manifest.description, 'Incoming webhook Alerts');
    expect(manifest.publisher, 'Built-in');
    expect(manifest.version, '1.0.0');
    expect(manifest.minApiVersion, '0.2');
    expect(manifest.requiredScopes, ['messages:write']);
    expect(manifest.optionalScopes, isEmpty);
    expect(manifest.events, isEmpty);
    expect(manifest.targetHosts, isEmpty);
    final grant = repository.grantList.single;
    expect(grant.subjectKind, AdminSubjectKind.app);
    expect(grant.targetKind, AdminTargetKind.channel);
    expect(grant.targetId, 'campaigns');
    expect(grant.level, AccessLevel.write);

    expect(find.byKey(const ValueKey('incoming.secretDialog')), findsOneWidget);
    await tap(tester, 'incoming.secretClose');
    expect(inRow('hook-0', find.text('Alerts')), findsOneWidget);
  });

  testWidgets('a failure after the install shows the steps, and Retry runs '
      'the failed step and the rest', (tester) async {
    repository.refuse['createGrant'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
    );
    await view(tester);
    await tap(tester, 'incoming.create');
    await type(tester, 'incoming.name', 'Alerts');
    await tap(tester, 'incoming.mode.newApp');
    await type(tester, 'incoming.reason', 'Monitoring');
    await tap(tester, 'incoming.submit');
    await tap(tester, 'stepUp.continue');

    expect(step(tester, 'install'), 'Install the incoming app: done');
    expect(
      step(tester, 'grant'),
      'Grant write on Marketing/Kampagnen: You are not allowed to do this',
    );
    expect(step(tester, 'webhook'), 'Create the webhook: waiting');
    expect(find.byKey(const ValueKey('incoming.retry')), findsOneWidget);
    expect(find.byKey(const ValueKey('incoming.uninstall')), findsOneWidget);
    expect(find.byKey(const ValueKey('incoming.secretDialog')), findsNothing);

    repository.refuse.remove('createGrant');
    await tap(tester, 'incoming.retry');
    expect(repository.calls, [
      'installApp incoming-abc12345 messages:write  Monitoring',
      'createGrant app-incoming-abc12345-0 channel.campaigns write Monitoring',
      'createGrant app-incoming-abc12345-0 channel.campaigns write Monitoring',
      'createIncomingWebhook campaigns app-incoming-abc12345-0 Alerts',
    ]);
    expect(repository.stepUps, hasLength(1));
    expect(find.byKey(const ValueKey('incoming.secretDialog')), findsOneWidget);
  });

  testWidgets('Uninstall removes the half-installed incoming app', (
    tester,
  ) async {
    repository.refuse['createIncomingWebhook'] = const ApiProblem(
      status: 400,
      code: 'validation_failed',
    );
    await view(tester);
    await tap(tester, 'incoming.create');
    await type(tester, 'incoming.name', 'Alerts');
    await tap(tester, 'incoming.mode.newApp');
    await type(tester, 'incoming.reason', 'Monitoring');
    await tap(tester, 'incoming.submit');
    await tap(tester, 'stepUp.continue');
    expect(step(tester, 'grant'), 'Grant write on Marketing/Kampagnen: done');
    expect(step(tester, 'webhook'), startsWith('Create the webhook: '));
    expect(step(tester, 'webhook'), isNot(endsWith('waiting')));

    await tap(tester, 'incoming.uninstall');
    expect(
      repository.calls.last,
      'uninstallApp app-incoming-abc12345-0 Monitoring',
    );
    expect(
      repository.appsById['app-incoming-abc12345-0']!.state,
      AppInstallState.uninstalled,
    );
    expect(find.text('The incoming app is uninstalled.'), findsOneWidget);
    expect(find.byKey(const ValueKey('incoming.retry')), findsNothing);
    expect(find.byKey(const ValueKey('incoming.uninstall')), findsNothing);
  });

  testWidgets('a refused install shows the reason; Retry installs again and '
      'there is nothing to uninstall', (tester) async {
    repository.stepUpFresh = true;
    repository.refuse['installApp'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
      reason: 'owner_only',
    );
    await view(tester);
    await tap(tester, 'incoming.create');
    await type(tester, 'incoming.name', 'Alerts');
    await tap(tester, 'incoming.mode.newApp');
    await type(tester, 'incoming.reason', 'Monitoring');
    await tap(tester, 'incoming.submit');

    expect(step(tester, 'install'), startsWith('Install the incoming app: '));
    expect(
      step(tester, 'grant'),
      'Grant write on Marketing/Kampagnen: waiting',
    );
    expect(find.byKey(const ValueKey('incoming.uninstall')), findsNothing);

    repository.refuse.remove('installApp');
    await tap(tester, 'incoming.retry');
    expect(
      repository.calls.where((c) => c.startsWith('installApp')),
      hasLength(2),
    );
    expect(find.byKey(const ValueKey('incoming.secretDialog')), findsOneWidget);
  });

  testWidgets('"New incoming app" is offered only when the server says the '
      'caller is an owner', (tester) async {
    repository.callerOwner = false;
    await view(tester);
    await tap(tester, 'incoming.create');
    expect(find.byKey(const ValueKey('incoming.mode.newApp')), findsNothing);
    expect(find.byKey(const ValueKey('incoming.app')), findsOneWidget);
  });

  testWidgets('revoke after one confirmation', (tester) async {
    await view(tester);
    await tap(tester, 'incoming.revoke.hook-a');
    expect(
      find.text('Revoke the incoming webhook Build alerts?'),
      findsOneWidget,
    );
    await tap(tester, 'confirm.submit');
    await tap(tester, 'stepUp.continue');

    expect(repository.calls, ['revokeIncomingWebhook hook-a']);
    expect(repository.stepUps, ['Revoke the incoming webhook Build alerts']);
    expect(find.byKey(const ValueKey('incoming.row.hook-a')), findsNothing);
    expect(changes, 1);
  });

  testWidgets('a refused list shows the reason and no control', (tester) async {
    repository.refuse['incomingWebhooks'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
    );
    await view(tester);
    expect(find.byKey(const ValueKey('admin.denied')), findsOneWidget);
    expect(find.byKey(const ValueKey('incoming.create')), findsNothing);
  });
}
