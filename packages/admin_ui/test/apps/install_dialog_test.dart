// SPDX-License-Identifier: Apache-2.0

// The install flow of Ruling 1 (brief M6-08 Design note 1; 11 4.1): a manifest file, the
// server's validation, the consent in plain words with optional scopes and events and a target
// per requested grant, "Install" with step-up, the server's preview of each chosen grant before
// any grant is created, then grants and token; the token is shown once with the key ID. A failed
// step stops the flow with the progress; Retry runs the failed step and the rest, Uninstall
// removes the half-installed app; nothing is retried silently.

import 'dart:convert';
import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

const _secret = 'bothy_agt_EXAMP0EXAMPLESecretShownOnce000000000000000_000000';

PickedFile _file(Map<String, Object?> manifest, {String name = 'app.json'}) =>
    PickedFile(name, Uint8List.fromList(utf8.encode(jsonEncode(manifest))));

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;
  late List<PickedFile?> files;
  late int changed;
  late List<String> opened;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
    files = [_file(repoBridgeManifest(), name: 'repo-bridge.json')];
    changed = 0;
    opened = [];
  });

  Future<void> tap(WidgetTester tester, String key) async {
    final f = find.byKey(ValueKey(key)).last;
    await tester.ensureVisible(f);
    await tester.pumpAndSettle();
    await tester.tap(f);
    await tester.pumpAndSettle();
  }

  Future<void> type(WidgetTester tester, String key, String text) async {
    await tester.ensureVisible(find.byKey(ValueKey(key)));
    await tester.enterText(find.byKey(ValueKey(key)), text);
    await tester.pump();
  }

  Future<void> openDialog(WidgetTester tester) async {
    await pumpInApp(
      tester,
      AppsList(
        repository: repository,
        onOpen: opened.add,
        pickManifest: () async => files.removeAt(0),
        timeZone: 'Europe/Berlin',
        clock: testClock,
        onChanged: () => changed++,
      ),
      navigatorKey: navigator,
    );
    await tap(tester, 'apps.install');
    expect(find.byKey(const ValueKey('install.dialog')), findsOneWidget);
  }

  CheckboxListTile box(WidgetTester tester, String key) =>
      tester.widget<CheckboxListTile>(find.byKey(ValueKey(key)));

  String textOf(WidgetTester tester, String key) => [
    for (final t in tester.widgetList<Text>(
      find.descendant(
        of: find.byKey(ValueKey(key)),
        matching: find.byType(Text),
      ),
    ))
      t.data ?? t.textSpan?.toPlainText() ?? '',
  ].join(' ');

  /// Picks the file, chooses the channel for the grant, fills the token and the reason.
  Future<void> consent(WidgetTester tester, {bool pickChannel = true}) async {
    await tap(tester, 'install.pick');
    expect(find.byKey(const ValueKey('consent.view')), findsOneWidget);
    if (pickChannel) {
      await tap(tester, 'consent.grant.0');
      await tap(tester, 'consent.grant.0.channel.campaigns');
    }
    await type(tester, 'consent.tokenName', 'Bridge');
    await type(tester, 'consent.tokenLastDay', '2026-12-31');
    await type(tester, 'consent.reason', 'Ticket bridge');
  }

  testWidgets('the consent says in plain words what the app asks for', (
    tester,
  ) async {
    await openDialog(tester);
    await tap(tester, 'install.pick');

    expect(repository.validated.single.fileName, 'repo-bridge.json');
    expect(repository.calls, isEmpty);
    final view = textOf(tester, 'consent.view');
    expect(
      view,
      contains('Repo Bridge by Example Org, version 1.2.0, asks for'),
    );
    // Scopes with their meaning (05 2.6), events, destinations, commands, components.
    expect(view, contains('Write, edit and delete messages'));
    expect(view, contains('messages:write'));
    expect(view, contains('Read messages and threads and search in them'));
    expect(view, contains('message.created'));
    expect(view, contains('bridge.example.com'));
    expect(view, contains('/issue'));
    expect(view, contains('Create an issue'));
    expect(view, contains('Buttons'));
    expect(view, contains('Forms'));
    // The requested grant with its purpose and the publisher's data_egress text.
    expect(view, contains('Write on one channel'));
    expect(view, contains('Purpose: Create posts per ticket'));
    expect(
      view,
      contains('Messages of the granted channels go to bridge.example.com'),
    );

    // Required scopes are checked and fixed; optional scopes and events start checked.
    expect(box(tester, 'consent.scope.messages:write').value, isTrue);
    expect(box(tester, 'consent.scope.messages:write').onChanged, isNull);
    expect(box(tester, 'consent.scope.messages:read').value, isTrue);
    expect(box(tester, 'consent.scope.messages:read').onChanged, isNotNull);
    expect(box(tester, 'consent.event.message.created').value, isTrue);

    // The target picker of a channel grant offers channels and "Do not create" only.
    await tap(tester, 'consent.grant.0');
    expect(
      find.byKey(const ValueKey('consent.grant.0.channel.campaigns')),
      findsWidgets,
    );
    expect(
      find.byKey(const ValueKey('consent.grant.0.channel.internal')),
      findsWidgets,
    );
    expect(find.byKey(const ValueKey('consent.grant.0.none')), findsWidgets);
    expect(
      find.byKey(const ValueKey('consent.grant.0.space.marketing')),
      findsNothing,
    );
  });

  testWidgets('install sends the chosen scopes, previews each grant, then creates grants and '
      'token and shows the token once', (tester) async {
    await openDialog(tester);
    await tap(tester, 'install.pick');
    await tap(tester, 'consent.scope.messages:read');
    expect(box(tester, 'consent.scope.messages:read').value, isFalse);
    await tap(tester, 'consent.grant.0');
    await tap(tester, 'consent.grant.0.channel.campaigns');
    await type(tester, 'consent.tokenName', 'Bridge');
    await type(tester, 'consent.tokenLastDay', '2026-12-31');
    await type(tester, 'consent.reason', 'Ticket bridge');
    await tap(tester, 'install.submit');

    expect(find.byKey(const ValueKey('stepUp.dialog')), findsOneWidget);
    await tap(tester, 'stepUp.continue');

    // The unchecked optional scope does not reach the server.
    expect(repository.calls, [
      'installApp repo-bridge messages:write,commands:write,events:subscribe '
          'message.created Ticket bridge',
    ]);
    // The server's preview of the grant is shown before any grant exists.
    final preview = repository.previews.single;
    expect(preview.subjectKind, AdminSubjectKind.app);
    expect(preview.subjectId, 'app-repo-bridge-0');
    expect(preview.target, campaigns);
    expect(preview.level, AccessLevel.write);
    expect(preview.reason, 'Ticket bridge');
    expect(
      textOf(tester, 'install.preview.0'),
      contains('Preview of app-repo-bridge-0'),
    );
    expect(repository.grantList, isEmpty);

    await tap(tester, 'install.createGrants');
    expect(repository.calls.skip(1), [
      'createGrant app-repo-bridge-0 channel.campaigns write Ticket bridge',
      // The token holds through the typed day in Europe/Berlin.
      'createToken app-repo-bridge-0 Bridge 2026-12-31T23:00:00.000Z',
    ]);
    // One step-up for the whole flow: the confirmation stays fresh.
    expect(repository.stepUps, ['Install the app Repo Bridge']);

    expect(find.byKey(const ValueKey('appToken.secretDialog')), findsOneWidget);
    expect(find.text(_secret), findsOneWidget);
    expect(
      find.text('Key ID of the webhook signatures: whk-2026-10'),
      findsOneWidget,
    );
    expect(find.byKey(const ValueKey('appToken.keyId')), findsOneWidget);
    await tap(tester, 'appToken.secretClose');

    // Gone: the secret, the dialog; the list shows the new app and opens nothing.
    expect(find.textContaining(_secret), findsNothing);
    expect(find.byKey(const ValueKey('install.dialog')), findsNothing);
    expect(
      find.byKey(const ValueKey('apps.row.app-repo-bridge-0')),
      findsOneWidget,
    );
    expect(opened, isEmpty);
    expect(changed, 1);
  });

  testWidgets('"Do not create" skips the grant; only the token is created', (
    tester,
  ) async {
    repository.stepUpFresh = true;
    await openDialog(tester);
    await consent(tester, pickChannel: false);
    await tap(tester, 'install.submit');
    expect(repository.previews, isEmpty);
    expect(
      find.text('No access grant chosen; only the token is created.'),
      findsOneWidget,
    );
    await tap(tester, 'install.createGrants');
    expect(repository.calls.last, startsWith('createToken app-repo-bridge-0'));
    expect(repository.calls.where((c) => c.startsWith('createGrant')), isEmpty);
    await tap(tester, 'appToken.secretClose');
  });

  testWidgets('a space grant offers spaces; an instance grant has no picker', (
    tester,
  ) async {
    repository.stepUpFresh = true;
    files = [
      _file(
        repoBridgeManifest(
          grants: const [
            {
              'target_kind': 'space',
              'level': 'read',
              'capabilities': <String>[],
              'purpose': 'Read the space',
            },
            {
              'target_kind': 'instance',
              'level': 'read',
              'capabilities': <String>[],
              'purpose': 'Read everything',
            },
          ],
        ),
      ),
    ];
    await openDialog(tester);
    await tap(tester, 'install.pick');
    expect(find.byKey(const ValueKey('consent.grant.1')), findsNothing);
    expect(
      textOf(tester, 'consent.view'),
      contains('Read on the whole instance'),
    );
    await tap(tester, 'consent.grant.0');
    expect(
      find.byKey(const ValueKey('consent.grant.0.channel.campaigns')),
      findsNothing,
    );
    await tap(tester, 'consent.grant.0.space.marketing');
    await type(tester, 'consent.tokenName', 'Bridge');
    await type(tester, 'consent.tokenLastDay', '2026-12-31');
    await type(tester, 'consent.reason', 'Ticket bridge');
    await tap(tester, 'install.submit');
    expect(repository.previews.map((p) => p.target), [
      marketing,
      AdminTarget.instance,
    ]);
    expect(find.byKey(const ValueKey('install.preview.1')), findsOneWidget);
  });

  testWidgets(
    'a failed grant stops the flow; Retry runs only the failed step and the rest',
    (tester) async {
      repository.stepUpFresh = true;
      await openDialog(tester);
      await consent(tester);
      await tap(tester, 'install.submit');
      repository.refuse['createGrant'] = const ApiProblem(
        status: 403,
        code: 'forbidden',
      );
      await tap(tester, 'install.createGrants');

      expect(textOf(tester, 'install.step.install'), contains('finished'));
      expect(textOf(tester, 'install.step.grant.0'), contains('failed: '));
      expect(textOf(tester, 'install.step.token'), contains('waiting'));
      expect(
        repository.calls.where((c) => c.startsWith('createToken')),
        isEmpty,
      );
      expect(find.byKey(const ValueKey('appToken.secretDialog')), findsNothing);

      repository.refuse.remove('createGrant');
      await tap(tester, 'install.retry');
      expect(repository.calls, [
        startsWith('installApp '),
        startsWith('createGrant app-repo-bridge-0 channel.campaigns'),
        startsWith('createGrant app-repo-bridge-0 channel.campaigns'),
        startsWith('createToken app-repo-bridge-0'),
      ]);
      await tap(tester, 'appToken.secretClose');
      expect(changed, 1);
    },
  );

  testWidgets('a failed token keeps the grants; Retry creates only the token', (
    tester,
  ) async {
    repository.stepUpFresh = true;
    await openDialog(tester);
    await consent(tester);
    await tap(tester, 'install.submit');
    repository.refuse['createToken'] = const ApiProblem(
      status: 400,
      code: 'validation_failed',
    );
    await tap(tester, 'install.createGrants');
    expect(textOf(tester, 'install.step.grant.0'), contains('finished'));
    expect(textOf(tester, 'install.step.token'), contains('failed: '));
    repository.refuse.remove('createToken');
    await tap(tester, 'install.retry');
    expect(
      repository.calls.where((c) => c.startsWith('createGrant')),
      hasLength(1),
    );
    expect(
      repository.calls.where((c) => c.startsWith('createToken')),
      hasLength(2),
    );
    expect(find.text(_secret), findsOneWidget);
    await tap(tester, 'appToken.secretClose');
  });

  testWidgets('Uninstall removes the half-installed app', (tester) async {
    repository.stepUpFresh = true;
    await openDialog(tester);
    await consent(tester);
    await tap(tester, 'install.submit');
    repository.refuse['createGrant'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
    );
    await tap(tester, 'install.createGrants');
    await tap(tester, 'install.uninstall');

    expect(
      repository.calls.last,
      'uninstallApp app-repo-bridge-0 Ticket bridge',
    );
    expect(find.byKey(const ValueKey('install.dialog')), findsNothing);
    expect(
      find.byKey(const ValueKey('apps.row.app-repo-bridge-0')),
      findsNothing,
    );
    expect(changed, 1);
  });

  testWidgets('a refused install shows the reason and keeps the consent', (
    tester,
  ) async {
    repository.stepUpFresh = true;
    await openDialog(tester);
    await consent(tester);
    repository.refuse['installApp'] = const ApiProblem(
      status: 400,
      code: 'webhook_host_not_allowed',
    );
    await tap(tester, 'install.submit');
    expect(textOf(tester, 'install.step.install'), contains('failed: '));
    expect(find.byKey(const ValueKey('consent.view')), findsOneWidget);
    expect(repository.previews, isEmpty);
  });

  testWidgets('the server\'s failed rules show pointer, rule and detail', (
    tester,
  ) async {
    repository.manifestCheck = (_) => const ManifestCheck(
      valid: false,
      errors: [
        ProblemFieldError(
          pointer: '/scopes/required/0',
          code: 'unknown_scope',
          detail: 'not in the catalogue',
        ),
        ProblemFieldError(pointer: '', code: 'too_large'),
      ],
    );
    await openDialog(tester);
    await tap(tester, 'install.pick');

    final errors = textOf(tester, 'install.errors');
    expect(errors, contains('/scopes/required/0: rule "unknown_scope"'));
    expect(errors, contains('not in the catalogue'));
    expect(errors, contains('the whole manifest: rule "too_large"'));
    expect(find.byKey(const ValueKey('consent.view')), findsNothing);
    expect(repository.calls, isEmpty);
  });

  testWidgets('a file the client cannot read never reaches the server', (
    tester,
  ) async {
    files = [
      PickedFile('broken.json', Uint8List.fromList(const [0xff, 0xfe, 0x00])),
    ];
    await openDialog(tester);
    await tap(tester, 'install.pick');
    expect(
      textOf(tester, 'install.fileError'),
      contains('The file is not text (UTF-8).'),
    );
    expect(repository.validated, isEmpty);
  });

  testWidgets('a cancelled step-up installs nothing and says nothing', (
    tester,
  ) async {
    await openDialog(tester);
    await consent(tester);
    await tap(tester, 'install.submit');
    await tap(tester, 'stepUp.cancel');
    expect(repository.appsById, isEmpty);
    expect(find.byKey(const ValueKey('consent.view')), findsOneWidget);
    expect(find.textContaining('failed'), findsNothing);
  });
}
