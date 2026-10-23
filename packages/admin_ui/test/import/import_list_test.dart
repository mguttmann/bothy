// SPDX-License-Identifier: Apache-2.0

// The import list and "Add source" (brief M8-08 Design notes 1 and 2): the sources with kind,
// state and progress, the mapping file read strictly in the dialog, the server's errors at
// their places in the file, a fixed file added after the confirmation and the step-up, and
// the list read again on the refresh control and when the tab becomes visible.

import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support.dart';

void main() {
  late GlobalKey<NavigatorState> navigator;
  late FakeAdminRepository repository;
  late List<String> opened;

  setUp(() {
    navigator = GlobalKey<NavigatorState>();
    repository = instanceRepository(navigator);
    opened = [];
  });

  Future<void> pumpList(WidgetTester tester, {MappingPicker? pick}) async {
    await pumpInstance(
      tester,
      ImportListPage(
        repository: repository,
        pickMapping: pick,
        onOpen: opened.add,
      ),
      navigator,
    );
    await tester.pumpAndSettle();
  }

  testWidgets('the list shows kind, state and progress; a row opens', (
    tester,
  ) async {
    final draft = repository.seedImport('First');
    final running = repository.seedImport(
      'Old community server',
      state: 'running',
      stats: const {
        'server': {
          'channel': {'posted': 2},
          'message': {'posted': 40, 'failed': 1},
        },
      },
    );
    await pumpList(tester);
    expect(
      find.text('discord  |  Started  |  42 items imported'),
      findsOneWidget,
    );
    expect(find.text('discord  |  Draft  |  0 items imported'), findsOneWidget);
    // Newest first.
    final rows = find.byWidgetPredicate(
      (w) => w is ListTile && '${w.key}'.contains('import.row.'),
    );
    expect(
      [for (final e in rows.evaluate()) '${e.widget.key}'],
      ["[<'import.row.$running'>]", "[<'import.row.$draft'>]"],
    );
    await tester.tap(find.byKey(ValueKey('import.row.$running')));
    expect(opened, [running]);
    // Without a file dialog there is no "Add source".
    expect(find.byKey(const ValueKey('import.add')), findsNothing);
  });

  testWidgets('an empty list says how to start', (tester) async {
    await pumpList(tester, pick: QueuedPicker([]).call);
    expect(find.byKey(const ValueKey('import.empty')), findsOneWidget);
  });

  testWidgets('a refused list shows the reason and no add button', (
    tester,
  ) async {
    repository.refuse['importSources'] = const ApiProblem(
      status: 403,
      code: 'forbidden',
      reason: 'owner_only',
    );
    await pumpList(tester, pick: QueuedPicker([]).call);
    expect(find.byKey(const ValueKey('admin.denied')), findsOneWidget);
    expect(find.byKey(const ValueKey('import.add')), findsNothing);
  });

  testWidgets('add: a wrong handle at its place, fixed, then created', (
    tester,
  ) async {
    final picker = QueuedPicker([
      'version: 1\nguild_id: "1"\nguild_id: "2"\n',
      mappingYaml(operator: 'agent:nobody'),
      mappingYaml(),
    ]);
    await pumpList(tester, pick: picker.call);
    await tapKey(tester, 'import.add');
    final submit = find.byKey(const ValueKey('import.create.submit'));
    expect(tester.widget<FilledButton>(submit).onPressed, isNull);
    await tester.enterText(
      find.byKey(const ValueKey('import.create.name')),
      'Old community server',
    );
    await tapKey(tester, 'import.create.pick');
    expect(
      find.byKey(const ValueKey('import.create.fileError')),
      findsOneWidget,
    );
    expect(
      find.textContaining('(line 3, column 1). A key named twice'),
      findsOneWidget,
    );
    expect(tester.widget<FilledButton>(submit).onPressed, isNull);

    await tapKey(tester, 'import.create.pick');
    expect(find.byKey(const ValueKey('import.create.file')), findsOneWidget);
    await tapKey(tester, 'import.create.submit');
    expect(find.text('Add Old community server?'), findsOneWidget);
    await confirmChange(tester);
    await confirmStepUp(tester, 'Add the import source Old community server');
    expect(
      textOf(tester, 'import.create.error.0'),
      'channels[0].planned_operator: names no person or agent with this handle',
    );
    expect(repository.importsById, isEmpty);

    await tapKey(tester, 'import.create.pick');
    // A new file clears the errors of the old one.
    expect(find.byKey(const ValueKey('import.create.errors')), findsNothing);
    await tapKey(tester, 'import.create.submit');
    await confirmChange(tester);
    expect(find.byKey(const ValueKey('import.create.dialog')), findsNothing);
    final id = repository.importsById.keys.single;
    expect(opened, [id]);
    final config = repository.importsById[id]!['config']! as Map;
    expect(config['guild_id'], '100');
    expect((config['channels']! as List).first, {
      'discord_channel': '301',
      'target': {
        'create': {'name': 'tickets'},
      },
      'planned_operator': 'agent:texter',
    });
    expect(picker.opened, 3);
  });

  testWidgets('a file that is no UTF-8 is refused before any request', (
    tester,
  ) async {
    Future<PickedFile?> pick() async =>
        PickedFile('mapping.yaml', Uint8List.fromList(const [0xff, 0xfe]));
    await pumpList(tester, pick: pick);
    await tapKey(tester, 'import.add');
    await tapKey(tester, 'import.create.pick');
    expect(find.text('The file is no UTF-8 text.'), findsOneWidget);
    expect(repository.calls, isEmpty);
  });

  testWidgets(
    'the list reads again on refresh and when the tab becomes visible',
    (tester) async {
      await pumpList(tester, pick: QueuedPicker([]).call);
      expect(find.byKey(const ValueKey('import.empty')), findsOneWidget);
      repository.seedImport('First');
      await tapKey(tester, 'import.refresh');
      expect(find.text('First'), findsOneWidget);

      repository.seedImport('Second');
      await setLifecycle(tester, [
        AppLifecycleState.inactive,
        AppLifecycleState.hidden,
      ]);
      expect(find.text('Second'), findsNothing);
      await setLifecycle(tester, [
        AppLifecycleState.inactive,
        AppLifecycleState.resumed,
      ]);
      expect(find.text('Second'), findsOneWidget);
    },
  );
}
