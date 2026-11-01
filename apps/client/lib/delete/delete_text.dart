// SPDX-License-Identifier: Apache-2.0

/// The sentences of a deletion (01 6.18 items 3, 4 and 7): the counts that go, the reason a
/// project folder is not moved, and the menu entry and step-up texts of each kind.
library;

import 'package:l10n/l10n.dart';

import 'delete_port.dart';

/// The sentence of the counts in [view] that are not zero, in the order channels, chats,
/// messages, files, agent sessions; with every count zero the sentence that the item is empty.
///
/// Example: 2 chats, 140 messages, 3 files and 2 agent sessions read "This deletes 2 chats,
/// 140 messages, 3 files and 2 agent sessions."
String deletionSummary(AppLocalizations l, DeletionView view) {
  final parts = [
    if (view.channels > 0) l.deleteCountChannels(view.channels),
    if (view.chats > 0) l.deleteCountChats(view.chats),
    if (view.messages > 0) l.deleteCountMessages(view.messages),
    if (view.files > 0) l.deleteCountFiles(view.files),
    if (view.sessions > 0) l.deleteCountSessions(view.sessions),
  ];
  if (parts.isEmpty) return l.deleteSummaryEmpty;
  final joined = parts.length == 1
      ? parts.single
      : l.deleteListAnd(
          parts.sublist(0, parts.length - 1).join(', '),
          parts.last,
        );
  return l.deleteSummary(joined);
}

/// The sentence why the folder is not moved for the reason [code] of the runner's check or
/// move (09 18.15); null is a check the runner has not reported yet, an unknown code the
/// generic sentence.
String folderReasonText(AppLocalizations l, String? code) => switch (code) {
  null => l.deleteFolderUnchecked,
  'outside_root' => l.deleteFolderOutsideRoot,
  'symlink' => l.deleteFolderSymlink,
  'foreign_worktree' => l.deleteFolderForeignWorktree,
  'uncommitted_changes' => l.deleteFolderUncommittedChanges,
  'unpushed_commits' => l.deleteFolderUnpushedCommits,
  'contains_other_project' => l.deleteFolderContainsOtherProject,
  'workspace_missing' => l.deleteFolderWorkspaceMissing,
  'check_failed' => l.deleteFolderCheckFailed,
  _ => l.deleteFolderRefusedOther,
};

/// Why the folder box of [folder] is disabled, or null when the last check is ok.
String? folderBoxBlocked(
  AppLocalizations l,
  FolderView folder,
) => switch (folder.checkOk) {
  true => null,
  // Never reported: the runner has not checked the folder yet.
  null => folderReasonText(l, null),
  // A refusal without a code still blocks; it reads as the generic sentence.
  false => folderReasonText(l, folder.checkReason ?? ''),
};

/// The menu entry of [kind] (01 6.18 item 1).
String deleteMenuLabel(AppLocalizations l, DeleteKind kind) => switch (kind) {
  DeleteKind.space => l.menuDeleteSpace,
  DeleteKind.channel => l.menuDeleteChannel,
  DeleteKind.project => l.menuDeleteProject,
  DeleteKind.chat => l.menuDeleteChat,
};

/// The purpose of the step-up prompt of deleting [target]; a chat has none.
String deletePurpose(AppLocalizations l, DeleteTarget target) =>
    switch (target.kind) {
      DeleteKind.space => l.stepUpPurposeDeleteSpace(target.name),
      DeleteKind.channel => l.stepUpPurposeDeleteChannel(target.name),
      DeleteKind.project => l.stepUpPurposeDeleteProject(target.name),
      DeleteKind.chat => '',
    };

/// The purpose of the step-up prompt of restoring [target]; a chat has none.
String restorePurpose(AppLocalizations l, DeleteTarget target) =>
    switch (target.kind) {
      DeleteKind.space => l.stepUpPurposeRestoreSpace(target.name),
      DeleteKind.channel => l.stepUpPurposeRestoreChannel(target.name),
      DeleteKind.project => l.stepUpPurposeRestoreProject(target.name),
      DeleteKind.chat => '',
    };
