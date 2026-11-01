// SPDX-License-Identifier: Apache-2.0

// The delete dialog over a scripted port (01 6.18 items 3 and 4; D-DEL-6, D-DEL-10): it reads
// the counts when it opens with a loading state, names them, offers "Try again" after a failed
// read, shows the folder box of a project to owners only, unchecked, disabled with the reason
// of every refusal and of a check never reported, and sends the box's value with the delete.

import 'dart:async';

import 'package:client/delete/delete_dialog.dart';
import 'package:client/delete/delete_port.dart';
import 'package:client/new_project/new_project_flow.dart';
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../support/delete_fakes.dart';
import '../support/widgets.dart';

const DeleteTarget project = DeleteTarget(
  kind: DeleteKind.project,
  id: 'forum-1',
  name: 'website',
);

const DeleteTarget chat = DeleteTarget(
  kind: DeleteKind.chat,
  id: 'thread-1',
  name: 'Release notes',
);

const FolderView okFolder = FolderView(name: 'website', checkOk: true);

/// Opens the dialog of [target]; the outcome lands in [results].
Future<void> open(
  WidgetTester tester,
  ScriptedDeletePort port, {
  DeleteTarget target = project,
  bool owner = true,
  List<DeleteOutcome?>? results,
  bool settle = true,
}) async {
  await tester.pumpWidget(
    hostScreen(
      Builder(
        builder: (context) => TextButton(
          onPressed: () async {
            final outcome = await showDialog<DeleteOutcome>(
              context: context,
              builder: (_) =>
                  DeleteDialog(target: target, port: port, owner: owner),
            );
            results?.add(outcome);
          },
          child: const Text('open'),
        ),
      ),
    ),
  );
  await tester.tap(find.text('open'));
  if (settle) {
    await tester.pumpAndSettle();
  } else {
    // The loading state spins; let only the dialog's route come in.
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 500));
  }
}

Finder byKey(String key) => find.byKey(ValueKey(key));

CheckboxListTile box(WidgetTester tester) =>
    tester.widget<CheckboxListTile>(byKey('delete.folder'));

DsButton confirm(WidgetTester tester) =>
    tester.widget<DsButton>(byKey('delete.confirm'));

void main() {
  testWidgets('the title and the counts show; Delete is enabled after them', (
    tester,
  ) async {
    final port = ScriptedDeletePort()
      ..infos[0] = const DeletionView(
        chats: 2,
        messages: 140,
        files: 3,
        sessions: 2,
      )
      ..gate = Completer<void>();
    await open(tester, port, target: chat, settle: false);
    expect(find.text(en.deleteTitle(dsIsolate('Release notes'))), findsOne);
    expect(byKey('delete.loading'), findsOneWidget);
    expect(confirm(tester).onPressed, isNull, reason: 'counts not shown yet');
    port.gate!.complete();
    await tester.pumpAndSettle();
    expect(
      find.text(
        'This deletes 2 chats, 140 messages, 3 files and 2 agent sessions.',
      ),
      findsOneWidget,
    );
    expect(find.text('You can undo this for a short time.'), findsOneWidget);
    expect(confirm(tester).style, DsButtonStyle.danger);
    expect(confirm(tester).onPressed, isNotNull);
    expect(port.calls, ['info thread-1']);
  });

  testWidgets('a failed read shows the sentence and Try again reads again', (
    tester,
  ) async {
    final port = ScriptedDeletePort()
      ..infos.replaceRange(0, 1, [
        const NewProjectFailure.offline(),
        const DeletionView(messages: 3),
      ]);
    await open(tester, port, target: chat);
    expect(find.text('Could not read what this deletes.'), findsOneWidget);
    expect(confirm(tester).onPressed, isNull);
    await tester.tap(byKey('delete.retry'));
    await tester.pumpAndSettle();
    expect(find.text('This deletes 3 messages.'), findsOneWidget);
    expect(port.calls, ['info thread-1', 'info thread-1']);
  });

  testWidgets('the folder box is unchecked by default and sends false', (
    tester,
  ) async {
    final port = ScriptedDeletePort()
      ..infos[0] = const DeletionView(chats: 1, folder: okFolder);
    final results = <DeleteOutcome?>[];
    await open(tester, port, results: results);
    expect(find.text(en.deleteFolderBox(dsIsolate('website'))), findsOneWidget);
    expect(box(tester).value, isFalse);
    expect(box(tester).onChanged, isNotNull);
    await tester.tap(byKey('delete.confirm'));
    await tester.pumpAndSettle();
    expect(port.calls.last, startsWith('delete forum-1 trash=false key=key-1'));
    expect(port.calls.last, endsWith('purpose=Delete the project website'));
    expect(results.single!.trashFolder, isFalse);
  });

  testWidgets('a checked folder box sends true', (tester) async {
    final port = ScriptedDeletePort()
      ..infos[0] = const DeletionView(chats: 1, folder: okFolder);
    final results = <DeleteOutcome?>[];
    await open(tester, port, results: results);
    await tester.tap(byKey('delete.folder'));
    await tester.pumpAndSettle();
    expect(box(tester).value, isTrue);
    await tester.tap(byKey('delete.confirm'));
    await tester.pumpAndSettle();
    expect(port.calls.last, startsWith('delete forum-1 trash=true'));
    expect(results.single!.trashFolder, isTrue);
    expect(results.single!.view, port.deleted);
  });

  for (final (code, sentence) in [
    (null, 'The runner has not checked the folder yet.'),
    ('outside_root', "The folder is outside the runner's projects folder."),
    ('symlink', 'The folder is a link to another place.'),
    (
      'foreign_worktree',
      'The folder is a Git work tree of another repository.',
    ),
    ('uncommitted_changes', 'The folder has changes that are not committed.'),
    ('unpushed_commits', 'The folder has commits that are not pushed.'),
    ('contains_other_project', 'The folder contains another project.'),
    ('workspace_missing', "The folder does not exist on the runner's host."),
    ('check_failed', 'The runner could not check the folder.'),
    ('a_new_code', 'The runner does not move this folder.'),
  ]) {
    testWidgets('the box is disabled with its reason: ${code ?? 'no check'}', (
      tester,
    ) async {
      final port = ScriptedDeletePort()
        ..infos[0] = DeletionView(
          folder: FolderView(
            name: 'website',
            checkOk: code == null ? null : false,
            checkReason: code,
          ),
        );
      await open(tester, port);
      expect(box(tester).onChanged, isNull);
      expect(box(tester).value, isFalse);
      expect(find.text(sentence), findsOneWidget);
      await tester.tap(byKey('delete.confirm'));
      await tester.pumpAndSettle();
      expect(port.calls.last, startsWith('delete forum-1 trash=false'));
    });
  }

  testWidgets('a person who is not an owner sees no folder box', (
    tester,
  ) async {
    final port = ScriptedDeletePort()
      ..infos[0] = const DeletionView(chats: 1, folder: okFolder);
    await open(tester, port, owner: false);
    expect(byKey('delete.counts'), findsOneWidget);
    expect(byKey('delete.folder'), findsNothing);
  });

  testWidgets('a refused delete keeps the dialog open with the reason', (
    tester,
  ) async {
    final port = ScriptedDeletePort()
      ..infos[0] = const DeletionView(chats: 1, folder: okFolder)
      ..deleteFailures.add(
        const NewProjectFailure.problem(
          'conflict',
          reason: 'project_folder_refused',
        ),
      );
    final results = <DeleteOutcome?>[];
    await open(tester, port, results: results);
    await tester.tap(byKey('delete.folder'));
    await tester.pump();
    await tester.tap(byKey('delete.confirm'));
    await tester.pumpAndSettle();
    expect(find.text(en.problemConflictProjectFolderRefused), findsOneWidget);
    expect(results, isEmpty);
    // The same request again: the same idempotency key.
    await tester.tap(byKey('delete.confirm'));
    await tester.pumpAndSettle();
    expect([
      for (final c in port.calls.where((c) => c.startsWith('delete'))) c,
    ], everyElement(contains('key=key-1')));
    expect(results, hasLength(1));
  });

  testWidgets('Cancel closes without a delete', (tester) async {
    final port = ScriptedDeletePort();
    final results = <DeleteOutcome?>[];
    await open(tester, port, target: chat, results: results);
    expect(find.text('It is empty.'), findsOneWidget);
    await tester.tap(byKey('delete.cancel'));
    await tester.pumpAndSettle();
    expect(results, [null]);
    expect(port.calls, ['info thread-1']);
  });
}
