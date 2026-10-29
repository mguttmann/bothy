// SPDX-License-Identifier: Apache-2.0

// "Rename project" over a fake port (01 6.11; docs/decisions.d/M4-44.md D-M4-44-6).

import 'package:client/new_project/new_project_flow.dart';
import 'package:client/new_project/new_project_port.dart';
import 'package:client/new_project/rename_project_dialog.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

import '../support/widgets.dart';

final _forum = ChannelId.parse('0192f4a0-7b1c-72d4-8bae-000000000002');

final class ScriptedRename implements RenameProjectPort {
  final List<String> calls = [];
  final List<int> versions = [3];
  final List<NewProjectFailure> failures = [];

  @override
  Future<int> version(ChannelId forum) async {
    calls.add('version');
    return versions.length > 1 ? versions.removeAt(0) : versions.first;
  }

  @override
  Future<void> rename(
    ChannelId forum,
    String name, {
    required int version,
    required String idempotencyKey,
  }) async {
    calls.add('rename $name $version $idempotencyKey');
    if (failures.isNotEmpty) throw failures.removeAt(0);
  }
}

Future<AppLocalizations> open(
  WidgetTester tester,
  ScriptedRename port,
  List<String?> results,
) async {
  var keys = 0;
  await tester.pumpWidget(
    hostScreen(
      Builder(
        builder: (context) => TextButton(
          onPressed: () async => results.add(
            await showDialog<String>(
              context: context,
              builder: (_) => RenameProjectDialog(
                forum: _forum,
                name: 'website',
                port: port,
                newKey: () => 'key-${++keys}',
              ),
            ),
          ),
          child: const Text('open'),
        ),
      ),
    ),
  );
  await tester.tap(find.text('open'));
  await tester.pumpAndSettle();
  return AppLocalizations.delegate.load(const Locale('en'));
}

TextField nameField(WidgetTester tester) =>
    tester.widget<TextField>(find.byKey(const ValueKey('renameProject.name')));

FilledButton submit(WidgetTester tester) => tester.widget<FilledButton>(
  find.byKey(const ValueKey('renameProject.submit')),
);

void main() {
  testWidgets('renames the forum with its version and closes with the name', (
    tester,
  ) async {
    final port = ScriptedRename();
    final results = <String?>[];
    await open(tester, port, results);
    expect(nameField(tester).controller!.text, 'website');
    expect(submit(tester).onPressed, isNull, reason: 'unchanged name');
    await tester.enterText(
      find.byKey(const ValueKey('renameProject.name')),
      '  Website 2026  ',
    );
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('renameProject.submit')));
    await tester.pumpAndSettle();
    expect(port.calls, ['version', 'rename Website 2026 3 key-1']);
    expect(results, ['Website 2026']);
  });

  testWidgets('an empty name cannot be sent', (tester) async {
    await open(tester, ScriptedRename(), []);
    await tester.enterText(
      find.byKey(const ValueKey('renameProject.name')),
      '   ',
    );
    await tester.pump();
    expect(submit(tester).onPressed, isNull);
  });

  testWidgets(
    'a version conflict reads the forum again and the next press uses the new version',
    (tester) async {
      final port = ScriptedRename()
        ..versions.addAll([3, 4])
        ..versions.removeAt(0)
        ..failures.add(const NewProjectFailure.problem('version_conflict'));
      final results = <String?>[];
      await open(tester, port, results);
      await tester.enterText(
        find.byKey(const ValueKey('renameProject.name')),
        'site',
      );
      await tester.pump();
      await tester.tap(find.byKey(const ValueKey('renameProject.submit')));
      await tester.pumpAndSettle();
      expect(
        find.byKey(const ValueKey('renameProject.changed')),
        findsOneWidget,
      );
      expect(results, isEmpty);
      await tester.tap(find.byKey(const ValueKey('renameProject.submit')));
      await tester.pumpAndSettle();
      expect(port.calls, [
        'version',
        'rename site 3 key-1',
        'version',
        'rename site 4 key-2',
      ]);
      expect(results, ['site']);
    },
  );

  testWidgets('a refusal shows its sentence and keeps the dialog', (
    tester,
  ) async {
    final port = ScriptedRename()
      ..failures.add(const NewProjectFailure.offline());
    final results = <String?>[];
    final l = await open(tester, port, results);
    await tester.enterText(
      find.byKey(const ValueKey('renameProject.name')),
      'site',
    );
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('renameProject.submit')));
    await tester.pumpAndSettle();
    expect(find.text(l.newProjectErrOffline), findsOneWidget);
    expect(results, isEmpty);
  });
}
