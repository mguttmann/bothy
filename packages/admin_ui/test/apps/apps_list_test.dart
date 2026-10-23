// SPDX-License-Identifier: Apache-2.0

// The apps list (brief M6-08 Tests; 01 5.2, 5.5; 11 4): every installed app with name, slug,
// version, publisher and state, "Install app" only with a file dialog of the host, a row opens
// the app, and a list the server refuses shows its reason and nothing else.

import 'dart:convert';
import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../admin/support.dart';

void main() {
  late FakeAdminRepository repository;

  setUp(() {
    repository = FakeAdminRepository(prompt: StepUpDialogPrompt(() => null));
  });

  Future<void> list(
    WidgetTester tester, {
    ManifestPicker? pick,
    ValueChanged<String>? onOpen,
  }) async {
    await pumpInApp(
      tester,
      AppsList(
        repository: repository,
        onOpen: onOpen ?? (_) {},
        pickManifest: pick,
        clock: testClock,
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('each row shows name, slug, version, publisher and state', (
    tester,
  ) async {
    repository.seedApp(repoBridgeManifest());
    final other = Map.of(repoBridgeManifest())
      ..['slug'] = 'cron-report'
      ..['name'] = 'Cron Report'
      ..['version'] = '0.3.1';
    repository.seedApp(other, state: AppInstallState.suspended);
    await list(tester, pick: () async => null);

    expect(find.byKey(const ValueKey('apps.list')), findsOneWidget);
    Finder inRow(String id, String text) => find.descendant(
      of: find.byKey(ValueKey('apps.row.$id')),
      matching: find.textContaining(text),
    );
    expect(inRow('app-repo-bridge', 'Repo Bridge'), findsOneWidget);
    expect(inRow('app-repo-bridge', 'repo-bridge'), findsOneWidget);
    expect(inRow('app-repo-bridge', '1.2.0'), findsOneWidget);
    expect(inRow('app-repo-bridge', 'Example Org'), findsOneWidget);
    expect(inRow('app-repo-bridge', 'Active'), findsOneWidget);
    expect(inRow('app-cron-report', 'Cron Report'), findsOneWidget);
    expect(inRow('app-cron-report', '0.3.1'), findsOneWidget);
    expect(inRow('app-cron-report', 'Suspended'), findsOneWidget);
  });

  testWidgets('a tap on a row opens the app', (tester) async {
    repository.seedApp(repoBridgeManifest());
    final opened = <String>[];
    await list(tester, onOpen: opened.add);
    await tester.tap(find.byKey(const ValueKey('apps.row.app-repo-bridge')));
    await tester.pumpAndSettle();
    expect(opened, ['app-repo-bridge']);
  });

  testWidgets('without a file dialog "Install app" is disabled with a hint', (
    tester,
  ) async {
    await list(tester);
    final button = tester.widget<ButtonStyleButton>(
      find.byKey(const ValueKey('apps.install')),
    );
    expect(button.onPressed, isNull);
    expect(
      find.text('Installing needs a file dialog; use the web console.'),
      findsOneWidget,
    );
    expect(find.text('No apps installed.'), findsOneWidget);
  });

  testWidgets('with a file dialog "Install app" opens the install dialog', (
    tester,
  ) async {
    await list(
      tester,
      pick: () async => PickedFile(
        'repo-bridge.json',
        Uint8List.fromList(utf8.encode(jsonEncode(repoBridgeManifest()))),
      ),
    );
    await tester.tap(find.byKey(const ValueKey('apps.install')));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('install.dialog')), findsOneWidget);
  });

  testWidgets('a refused list shows the reason and nothing else', (
    tester,
  ) async {
    repository.seedApp(repoBridgeManifest());
    repository.refuse['apps'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
      reason: 'not_owner',
    );
    await list(tester, pick: () async => null);
    expect(find.byKey(const ValueKey('admin.denied')), findsOneWidget);
    expect(find.byKey(const ValueKey('apps.install')), findsNothing);
    expect(
      find.byKey(const ValueKey('apps.row.app-repo-bridge')),
      findsNothing,
    );
  });
}
