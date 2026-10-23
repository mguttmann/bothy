// SPDX-License-Identifier: Apache-2.0

// The update of an app (brief M6-08 Ruling 2; 11 4.2): a new manifest file, the server's
// validation, the diff of scopes, events, destinations, requested grants and commands with the
// notice that removed items take effect at once, then upgradeManifest with the granted scopes
// and events the new manifest still names, and the new consent through setScopes when the new
// manifest asks for more.

import 'dart:convert';
import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart' show ApiProblem, ProblemFieldError;
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

PickedFile _file(Map<String, Object?> manifest) => PickedFile(
  'repo-bridge.json',
  Uint8List.fromList(utf8.encode(jsonEncode(manifest))),
);

/// Version 2.0.0: `files:write` is new and required, `commands:write` and the command are
/// gone, `reaction.added` is a new event, the delivery moves to another host, and the channel
/// grant became a space grant.
Map<String, Object?> _v2() => Map.of(
  repoBridgeManifest(
    version: '2.0.0',
    required: const ['messages:write', 'files:write'],
    events: const ['message.created', 'reaction.added'],
    eventsUrl: 'https://relay.example.com/hooks/events',
    grants: const [
      {
        'target_kind': 'space',
        'level': 'read',
        'capabilities': <String>[],
        'purpose': 'Read the space',
      },
    ],
  ),
)..['commands'] = const <Object?>[];

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;
  late List<PickedFile?> files;
  late int changed;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = FakeAdminRepository(
      prompt: StepUpDialogPrompt(() => navigator.currentContext),
    );
    final seeded = repository.seedApp(
      repoBridgeManifest(),
      grantedScopes: const [
        'messages:write',
        'commands:write',
        'messages:read',
      ],
    );
    // The installation subscribes to message.created.
    repository.appsById[seeded.id] = AppRecord(
      id: seeded.id,
      slug: seeded.slug,
      displayName: seeded.displayName,
      manifestVersion: seeded.manifestVersion,
      manifest: seeded.manifest,
      installationId: seeded.installationId,
      grantedScopes: seeded.grantedScopes,
      eventSubscriptions: const ['message.created'],
      state: seeded.state,
      installedAt: seeded.installedAt,
      installedBy: seeded.installedBy,
    );
    files = [_file(_v2())];
    changed = 0;
  });

  Future<void> tap(WidgetTester tester, String key) async {
    final f = find.byKey(ValueKey(key)).last;
    await tester.ensureVisible(f);
    await tester.pumpAndSettle();
    await tester.tap(f);
    await tester.pumpAndSettle();
  }

  Future<void> page(WidgetTester tester, {bool picker = true}) async {
    await pumpInApp(
      tester,
      AppPage(
        repository: repository,
        appId: 'app-repo-bridge',
        pickManifest: picker ? () async => files.removeAt(0) : null,
        clock: testClock,
        onChanged: () => changed++,
      ),
      navigatorKey: navigator,
    );
    await tester.pumpAndSettle();
  }

  String textOf(WidgetTester tester, String key) => [
    for (final t in tester.widgetList<Text>(
      find.descendant(
        of: find.byKey(ValueKey(key)),
        matching: find.byType(Text),
      ),
    ))
      t.data ?? t.textSpan?.toPlainText() ?? '',
  ].join(' | ');

  CheckboxListTile box(WidgetTester tester, String key) =>
      tester.widget<CheckboxListTile>(find.byKey(ValueKey(key)));

  testWidgets('without a file dialog Update is disabled', (tester) async {
    await page(tester, picker: false);
    expect(
      tester
          .widget<ButtonStyleButton>(
            find.byKey(const ValueKey('appPage.update')),
          )
          .onPressed,
      isNull,
    );
  });

  testWidgets('the diff names added and removed items before anything is sent', (
    tester,
  ) async {
    await page(tester);
    await tap(tester, 'appPage.update');
    expect(find.byKey(const ValueKey('update.dialog')), findsOneWidget);
    await tap(tester, 'update.pick');

    expect(repository.validated.single.info.version, '2.0.0');
    final diff = textOf(tester, 'update.diff');
    expect(diff, contains('Changes from version 1.2.0 to 2.0.0'));
    expect(diff, contains('Added: files:write'));
    expect(diff, contains('Removed: commands:write'));
    expect(diff, contains('Added: reaction.added'));
    expect(diff, contains('Added: relay.example.com'));
    expect(diff, contains('Removed: bridge.example.com'));
    expect(diff, contains('Added: Read in one space'));
    expect(diff, contains('Removed: Write on one channel'));
    expect(diff, contains('Removed: /issue'));
    expect(diff, contains('Removed items take effect at once.'));
    expect(diff, contains('You confirm new destinations with this update.'));
    expect(
      diff,
      contains(
        'An update never adds an access grant: create added ones under Access '
        'grants.',
      ),
    );
    expect(diff, contains('Added scopes and events need a new consent'));
    expect(repository.calls, isEmpty);
  });

  testWidgets('update keeps the granted scopes the new manifest names, then the new consent '
      'goes through setScopes', (tester) async {
    await page(tester);
    await tap(tester, 'appPage.update');
    await tap(tester, 'update.pick');
    await tester.enterText(
      find.byKey(const ValueKey('update.reason')),
      'New release',
    );
    await tester.pump();
    await tap(tester, 'update.submit');
    expect(find.byKey(const ValueKey('stepUp.dialog')), findsOneWidget);
    await tap(tester, 'stepUp.continue');

    expect(repository.calls, [
      'upgradeManifest app-repo-bridge 2.0.0 messages:write,messages:read '
          'message.created New release',
    ]);

    // The new consent: the new required scope fixed, the added event checked, an optional
    // scope the owner never granted unchecked.
    expect(box(tester, 'update.consent.scope.files:write').value, isTrue);
    expect(box(tester, 'update.consent.scope.files:write').onChanged, isNull);
    expect(box(tester, 'update.consent.scope.messages:read').value, isTrue);
    expect(box(tester, 'update.consent.scope.events:subscribe').value, isFalse);
    expect(box(tester, 'update.consent.event.reaction.added').value, isTrue);
    expect(box(tester, 'update.consent.event.message.created').value, isTrue);
    await tap(tester, 'update.consent.scope.events:subscribe');
    await tap(tester, 'update.consentSubmit');

    expect(
      repository.calls.last,
      'setScopes app-repo-bridge messages:write,files:write,messages:read,'
      'events:subscribe message.created,reaction.added New release',
    );
    // One step-up for both changes: the confirmation stays fresh.
    expect(repository.stepUps, ['Update the app Repo Bridge']);
    expect(find.byKey(const ValueKey('update.dialog')), findsNothing);
    expect(
      repository.appsById['app-repo-bridge']!.grantedScopes,
      contains('files:write'),
    );
    expect(changed, 1);
    // The page shows the new version.
    expect(find.textContaining('2.0.0'), findsWidgets);
  });

  testWidgets('"Later" keeps the current scopes', (tester) async {
    repository.stepUpFresh = true;
    await page(tester);
    await tap(tester, 'appPage.update');
    await tap(tester, 'update.pick');
    await tester.enterText(
      find.byKey(const ValueKey('update.reason')),
      'New release',
    );
    await tester.pump();
    await tap(tester, 'update.submit');
    await tap(tester, 'update.consentLater');
    expect(repository.calls.where((c) => c.startsWith('setScopes')), isEmpty);
    expect(find.byKey(const ValueKey('update.dialog')), findsNothing);
    expect(changed, 1);
  });

  testWidgets('a manifest that asks for nothing new needs no new consent', (
    tester,
  ) async {
    repository.stepUpFresh = true;
    files = [_file(repoBridgeManifest(version: '1.2.1'))];
    await page(tester);
    await tap(tester, 'appPage.update');
    await tap(tester, 'update.pick');
    expect(
      textOf(tester, 'update.diff'),
      contains('Nothing changes in scopes, events, destinations'),
    );
    await tester.enterText(
      find.byKey(const ValueKey('update.reason')),
      'Patch',
    );
    await tester.pump();
    await tap(tester, 'update.submit');
    expect(repository.calls, [
      'upgradeManifest app-repo-bridge 1.2.1 messages:write,commands:write,'
          'messages:read message.created Patch',
    ]);
    expect(find.byKey(const ValueKey('update.dialog')), findsNothing);
  });

  testWidgets('a refused manifest shows the failed rules; nothing is sent', (
    tester,
  ) async {
    repository.manifestCheck = (_) => const ManifestCheck(
      valid: false,
      errors: [ProblemFieldError(pointer: '/slug', code: 'pattern')],
    );
    await page(tester);
    await tap(tester, 'appPage.update');
    await tap(tester, 'update.pick');
    expect(textOf(tester, 'update.errors'), contains('/slug: rule "pattern"'));
    expect(find.byKey(const ValueKey('update.diff')), findsNothing);
    expect(repository.calls, isEmpty);
  });

  testWidgets('a refused update shows the server\'s reason', (tester) async {
    repository.stepUpFresh = true;
    repository.refuse['upgradeManifest'] = const ApiProblem(
      status: 400,
      code: 'webhook_host_not_allowed',
    );
    await page(tester);
    await tap(tester, 'appPage.update');
    await tap(tester, 'update.pick');
    await tester.enterText(
      find.byKey(const ValueKey('update.reason')),
      'New release',
    );
    await tester.pump();
    await tap(tester, 'update.submit');
    expect(find.byKey(const ValueKey('update.failure')), findsOneWidget);
    expect(find.byKey(const ValueKey('update.dialog')), findsOneWidget);
    expect(changed, 0);
  });
}
