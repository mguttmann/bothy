// SPDX-License-Identifier: Apache-2.0

// After the delete (01 6.18 items 6 and 7; D-DEL-15): the undo snack bar with its time, "Undo"
// restoring, the sentence after the window (409 undo_window_passed) and the generic one; the
// folder outcome read every 2 seconds after undo_until for at most ui.delete_folder_wait, shown
// once as "Moved the folder ..." or "Kept the folder ...: <reason>"; which routes show a
// deleted item.

import 'package:client/app/router.dart';
import 'package:client/delete/delete_flow.dart';
import 'package:client/delete/delete_port.dart';
import 'package:client/new_project/new_project_flow.dart';
import 'package:domain/domain.dart' show ChannelKind;
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/delete_fakes.dart';
import '../support/structure.dart';
import '../support/widgets.dart';

const DeleteTarget forum = DeleteTarget(
  kind: DeleteKind.project,
  id: 'forum-1',
  name: 'website',
);

/// Pumps a screen whose button shows the undo snack bar of [target] for [duration].
Future<void> pumpToast(
  WidgetTester tester,
  ScriptedDeletePort port, {
  DeleteTarget target = forum,
  Duration duration = const Duration(seconds: 8),
  List<String>? restored,
}) async {
  await tester.pumpWidget(
    hostScreen(
      Scaffold(
        body: Builder(
          builder: (context) => TextButton(
            onPressed: () => showUndoToast(
              ScaffoldMessenger.of(context),
              en,
              target: target,
              port: port,
              duration: duration,
              onRestored: () => restored?.add(target.id),
            ),
            child: const Text('deleted'),
          ),
        ),
      ),
    ),
  );
  await tester.tap(find.text('deleted'));
  await tester.pumpAndSettle();
}

Finder byKey(String key) => find.byKey(ValueKey(key));

void main() {
  group('the undo snack bar', () {
    testWidgets('names the item, ends with the window and Undo restores', (
      tester,
    ) async {
      final port = ScriptedDeletePort();
      final restored = <String>[];
      await pumpToast(tester, port, restored: restored);
      final bar = tester.widget<SnackBar>(byKey('delete.toast'));
      expect(bar.duration, const Duration(seconds: 8));
      expect(bar.persist, isFalse, reason: 'it must end with the window');
      expect(find.text('Deleted website'), findsOneWidget);
      await tester.tap(find.text('Undo'));
      await tester.pumpAndSettle();
      expect(port.calls, [
        'restore forum-1 purpose=Restore the project website',
      ]);
      expect(restored, ['forum-1']);
      expect(byKey('delete.undoFailed'), findsNothing);
    });

    testWidgets('goes away after its time', (tester) async {
      await pumpToast(tester, ScriptedDeletePort());
      await tester.pump(const Duration(seconds: 9));
      await tester.pumpAndSettle();
      expect(byKey('delete.toast'), findsNothing);
    });

    testWidgets('an undo after the window says so', (tester) async {
      final port = ScriptedDeletePort()
        ..restoreFailures.add(
          const NewProjectFailure.problem(
            'conflict',
            reason: 'undo_window_passed',
          ),
        );
      final restored = <String>[];
      await pumpToast(tester, port, restored: restored);
      await tester.tap(find.text('Undo'));
      await tester.pumpAndSettle();
      expect(
        find.text('The deletion can no longer be undone.'),
        findsOneWidget,
      );
      expect(restored, isEmpty);
    });

    testWidgets('another failed undo gives the generic sentence', (
      tester,
    ) async {
      final port = ScriptedDeletePort()
        ..restoreFailures.add(const NewProjectFailure.offline());
      await pumpToast(tester, port);
      await tester.tap(find.text('Undo'));
      await tester.pumpAndSettle();
      expect(
        find.text('Could not undo the deletion of website.'),
        findsOneWidget,
      );
    });

    testWidgets('without time left there is no Undo', (tester) async {
      await pumpToast(tester, ScriptedDeletePort(), duration: Duration.zero);
      expect(find.text('Deleted website'), findsOneWidget);
      expect(byKey('delete.undo'), findsNothing);
    });
  });

  group('the folder outcome', () {
    final now = DateTime.utc(2026, 11, 1, 9);

    ({FolderOutcomeWatch watch, List<Duration> slept, List<FolderView> shown})
    watchOf(
      ScriptedDeletePort port, {
      Duration wait = const Duration(minutes: 2),
    }) {
      final slept = <Duration>[];
      final shown = <FolderView>[];
      final watch = FolderOutcomeWatch(
        port: port,
        target: forum,
        undoUntil: now.add(const Duration(seconds: 30)),
        wait: wait,
        now: () => now,
        sleep: (d) async => slept.add(d),
        onOutcome: shown.add,
      );
      return (watch: watch, slept: slept, shown: shown);
    }

    FolderView folder(FolderTrashState state, {String? reason}) => FolderView(
      name: 'website',
      checkOk: true,
      trashState: state,
      trashReason: reason,
    );

    test(
      'waits for undo_until, then reads every 2 seconds until trashed',
      () async {
        final port = ScriptedDeletePort()
          ..infos.replaceRange(0, 1, [
            DeletionView(folder: folder(FolderTrashState.requested)),
            DeletionView(folder: folder(FolderTrashState.requested)),
            DeletionView(folder: folder(FolderTrashState.trashed)),
          ]);
        final w = watchOf(port);
        await w.watch.run();
        expect(w.slept, const [
          Duration(seconds: 30),
          Duration(seconds: 2),
          Duration(seconds: 2),
        ]);
        expect(port.calls, hasLength(3));
        expect(w.shown.single.trashState, FolderTrashState.trashed);
      },
    );

    test('a refusal is the outcome with its reason', () async {
      final port = ScriptedDeletePort()
        ..infos[0] = DeletionView(
          folder: folder(FolderTrashState.refused, reason: 'unpushed_commits'),
        );
      final w = watchOf(port);
      await w.watch.run();
      expect(w.shown.single.trashReason, 'unpushed_commits');
    });

    test('ends after ui.delete_folder_wait without an outcome', () async {
      final port = ScriptedDeletePort()
        ..infos[0] = DeletionView(folder: folder(FolderTrashState.requested));
      final w = watchOf(port, wait: const Duration(seconds: 10));
      await w.watch.run();
      // Reads at 0, 2, 4, 6, 8 and 10 seconds after undo_until.
      expect(port.calls, hasLength(6));
      expect(w.slept.skip(1), everyElement(const Duration(seconds: 2)));
      expect(
        w.slept.skip(1).fold(Duration.zero, (a, b) => a + b),
        const Duration(seconds: 10),
      );
      expect(w.shown, isEmpty);
    });

    test('a failed read is no outcome yet', () async {
      final port = ScriptedDeletePort()
        ..infos.replaceRange(0, 1, [
          const NewProjectFailure.offline(),
          DeletionView(folder: folder(FolderTrashState.trashed)),
        ]);
      final w = watchOf(port);
      await w.watch.run();
      expect(w.shown, hasLength(1));
    });

    test('an undo ends the watch', () async {
      final port = ScriptedDeletePort()
        ..infos[0] = DeletionView(folder: folder(FolderTrashState.trashed));
      final w = watchOf(port)..watch.cancel();
      await w.watch.run();
      expect(port.calls, isEmpty);
      expect(w.shown, isEmpty);
    });

    testWidgets('the sentences of the outcome', (tester) async {
      for (final (state, reason, text) in [
        (
          FolderTrashState.trashed,
          null,
          'Moved the folder website to the trash.',
        ),
        (
          FolderTrashState.refused,
          'uncommitted_changes',
          'Kept the folder website: The folder has changes that are not '
              'committed.',
        ),
      ]) {
        final port = ScriptedDeletePort()
          ..infos[0] = DeletionView(folder: folder(state, reason: reason));
        await tester.pumpWidget(
          hostScreen(
            Scaffold(
              body: Builder(
                builder: (context) => TextButton(
                  onPressed: () => FolderOutcomeWatch(
                    port: port,
                    target: forum,
                    undoUntil: null,
                    wait: const Duration(minutes: 2),
                    now: () => now,
                    sleep: (_) async {},
                    onOutcome: (f) =>
                        showFolderOutcome(ScaffoldMessenger.of(context), en, f),
                  ).run(),
                  child: const Text('watch'),
                ),
              ),
            ),
          ),
        );
        await tester.tap(find.text('watch'));
        await tester.pumpAndSettle();
        expect(find.text(text), findsOneWidget);
        ScaffoldMessenger.of(tester.element(find.text('watch')))
            .removeCurrentSnackBar();
        await tester.pumpAndSettle();
      }
    });
  });

  group('which routes show the deleted item', () {
    late LocalDatabase db;

    setUp(() async {
      db = openInMemoryForTests(clock: () => t0);
      await db.structureDao.upsertSpaceIfNewer(space(1));
      await db.structureDao.upsertChannelIfNewer(
        channel(1, kind: ChannelKind.forum),
      );
      await db.threadsDao.upsertIfNewer(thread(1));
    });
    tearDown(() => db.close());

    DeleteTarget of(DeleteKind kind, String id) =>
        DeleteTarget(kind: kind, id: id, name: 'x');

    test('a chat, its channel and its space', () async {
      final at = ThreadLocation(threadId(1));
      expect(
        await showsDeleted(at, of(DeleteKind.chat, threadId(1).value), db),
        isTrue,
      );
      expect(
        await showsDeleted(at, of(DeleteKind.chat, threadId(2).value), db),
        isFalse,
      );
      expect(
        await showsDeleted(at, of(DeleteKind.project, channelId(1).value), db),
        isTrue,
      );
      expect(
        await showsDeleted(at, of(DeleteKind.space, spaceId(1).value), db),
        isTrue,
      );
      expect(
        await showsDeleted(at, of(DeleteKind.space, spaceId(2).value), db),
        isFalse,
      );
    });

    test('a channel route and its new chat', () async {
      for (final at in [
        ChannelLocation(channelId(1)),
        NewChatLocation(channelId(1)),
      ]) {
        expect(
          await showsDeleted(
            at,
            of(DeleteKind.channel, channelId(1).value),
            db,
          ),
          isTrue,
        );
        expect(
          await showsDeleted(at, of(DeleteKind.chat, threadId(1).value), db),
          isFalse,
        );
      }
    });

    test('other views stay', () async {
      expect(
        await showsDeleted(
          const HomeLocation(),
          of(DeleteKind.space, spaceId(1).value),
          db,
        ),
        isFalse,
      );
    });
  });
}
