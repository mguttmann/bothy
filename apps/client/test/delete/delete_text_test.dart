// SPDX-License-Identifier: Apache-2.0

// The sentences of a deletion (01 6.18 items 3, 4 and 6): only the counts that are not zero in
// one sentence with plural forms, the empty item, the reason sentence of every refusal code
// and of a check never reported, and the time of the undo snack bar.

import 'package:client/delete/delete_flow.dart';
import 'package:client/delete/delete_port.dart';
import 'package:client/delete/delete_text.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

void main() {
  final l = lookupAppLocalizations(const Locale('en'));
  final de = lookupAppLocalizations(const Locale('de'));

  test('the counts that are not zero form one sentence', () {
    expect(
      deletionSummary(
        l,
        const DeletionView(chats: 2, messages: 140, files: 3, sessions: 2),
      ),
      'This deletes 2 chats, 140 messages, 3 files and 2 agent sessions.',
    );
    expect(
      deletionSummary(
        l,
        const DeletionView(channels: 1, chats: 1, messages: 1, files: 1),
      ),
      'This deletes 1 channel, 1 chat, 1 message and 1 file.',
    );
    expect(
      deletionSummary(l, const DeletionView(messages: 4, sessions: 1)),
      'This deletes 4 messages and 1 agent session.',
    );
    expect(
      deletionSummary(l, const DeletionView(messages: 7)),
      'This deletes 7 messages.',
    );
    // language-check: off
    expect(
      deletionSummary(de, const DeletionView(channels: 3, chats: 1)),
      'Gelöscht werden: 3 Kanäle und 1 Chat.',
    );
    // language-check: on
  });

  test('an item with every count zero reads as empty', () {
    expect(deletionSummary(l, const DeletionView()), 'It is empty.');
  });

  test('every refusal code has its sentence, an unknown one the generic', () {
    final sentences = {
      'outside_root': "The folder is outside the runner's projects folder.",
      'symlink': 'The folder is a link to another place.',
      'foreign_worktree':
          'The folder is a Git work tree of another repository.',
      'uncommitted_changes': 'The folder has changes that are not committed.',
      'unpushed_commits': 'The folder has commits that are not pushed.',
      'contains_other_project': 'The folder contains another project.',
      'workspace_missing': "The folder does not exist on the runner's host.",
      'check_failed': 'The runner could not check the folder.',
      'something_new': 'The runner does not move this folder.',
    };
    for (final MapEntry(:key, :value) in sentences.entries) {
      expect(folderReasonText(l, key), value, reason: key);
    }
    expect(
      folderReasonText(l, null),
      'The runner has not checked the folder yet.',
    );
  });

  test('the box is blocked unless the last check is ok', () {
    expect(
      folderBoxBlocked(l, const FolderView(name: 'website', checkOk: true)),
      isNull,
    );
    expect(
      folderBoxBlocked(l, const FolderView(name: 'website')),
      'The runner has not checked the folder yet.',
    );
    expect(
      folderBoxBlocked(
        l,
        const FolderView(
          name: 'website',
          checkOk: false,
          checkReason: 'unpushed_commits',
        ),
      ),
      'The folder has commits that are not pushed.',
    );
    expect(
      folderBoxBlocked(l, const FolderView(name: 'website', checkOk: false)),
      'The runner does not move this folder.',
    );
  });

  group('the undo snack bar ends 2 seconds before undo_until', () {
    final now = DateTime.utc(2026, 11, 1, 9);
    const max = Duration(seconds: 10);

    test('at most ui.delete_undo_toast', () {
      expect(
        undoToastDuration(
          undoUntil: now.add(const Duration(seconds: 30)),
          now: now,
          max: max,
        ),
        max,
      );
    });

    test('a shorter window ends earlier', () {
      expect(
        undoToastDuration(
          undoUntil: now.add(const Duration(seconds: 5)),
          now: now,
          max: max,
        ),
        const Duration(seconds: 3),
      );
    });

    test('without undo_until the configured time', () {
      expect(undoToastDuration(undoUntil: null, now: now, max: max), max);
    });

    test('a window that is about to end leaves no time', () {
      expect(
        undoToastDuration(
          undoUntil: now.add(const Duration(seconds: 1)),
          now: now,
          max: max,
        ),
        isNot(greaterThan(Duration.zero)),
      );
    });
  });

  test('a space and a channel name their step-up purpose, a chat none', () {
    const space = DeleteTarget(kind: DeleteKind.space, id: 's', name: 'Team');
    const project = DeleteTarget(
      kind: DeleteKind.project,
      id: 'c',
      name: 'website',
    );
    const chat = DeleteTarget(kind: DeleteKind.chat, id: 't', name: 'Notes');
    expect(deletePurpose(l, space), 'Delete the space Team');
    expect(restorePurpose(l, project), 'Restore the project website');
    expect(space.needsStepUp && project.needsStepUp, isTrue);
    expect(chat.needsStepUp, isFalse);
    expect(deletePurpose(l, chat), isEmpty);
  });
}
