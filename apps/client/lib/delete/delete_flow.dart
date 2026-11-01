// SPDX-License-Identifier: Apache-2.0

/// What follows a "Delete..." entry (01 6.18 items 6 and 7; docs/design/delete.md D-DEL-15):
/// the dialog, then leaving a view that shows the deleted item, the snack bar "Deleted `name`"
/// with "Undo" until `undo_until` minus 2 seconds (at most `ui.delete_undo_toast`), and with a
/// requested folder move its outcome, read every 2 seconds after `undo_until` for at most
/// `ui.delete_folder_wait`.
library;

import 'dart:async';

import 'package:auth/auth.dart' show Clock;
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:l10n/l10n.dart';
import 'package:local_store/local_store.dart' show LocalDatabase;

import '../app/providers/ports.dart';
import '../app/providers/session.dart';
import '../app/router.dart';
import '../new_project/new_project_flow.dart'
    show NewProjectFailure, NewProjectFailureKind;
import '../notifications_settings/providers.dart' show isOwnerProvider;
import 'delete_dialog.dart';
import 'delete_port.dart';
import 'delete_text.dart';
import 'providers.dart';

/// The time the undo snack bar keeps clear of `undo_until`: the round trip of the restore
/// (01 6.18 item 6).
const Duration undoMargin = Duration(seconds: 2);

/// How often the folder outcome is read after the undo window (01 6.18 item 7).
const Duration folderPollInterval = Duration(seconds: 2);

/// How long the snack bar shows when no time is left to undo (D-DEL-UI-3).
const Duration deletedNoticeDuration = Duration(seconds: 4);

/// How long the undo snack bar shows: until [undoUntil] minus [undoMargin] from [now], at most
/// [max] (`ui.delete_undo_toast`); [max] without a known end of the window. Zero or less
/// means no time is left to undo.
///
/// Example: `undoToastDuration(undoUntil: t + 30s, now: t, max: 10s)` is 10 seconds,
/// `undoToastDuration(undoUntil: t + 5s, now: t, max: 10s)` 3 seconds.
Duration undoToastDuration({
  required DateTime? undoUntil,
  required DateTime now,
  required Duration max,
}) {
  if (undoUntil == null) return max;
  final left = undoUntil.difference(now) - undoMargin;
  return left < max ? left : max;
}

/// What the delete flow needs from the providers, read while a menu builds its entries: the
/// menu closes before an entry runs, and its `ref` goes with it.
final class DeleteSession {
  /// Creates the session.
  const DeleteSession({
    required this.port,
    required this.owner,
    required this.config,
    required this.clock,
    this.db,
  });

  /// The session of the signed-in person from [ref], or null without one (no entry shows).
  static DeleteSession? watch(WidgetRef ref) {
    final port = ref.watch(deletePortProvider);
    if (port == null) return null;
    return DeleteSession(
      port: port,
      // Watched: `getMe` may answer after the menu opened (D-DEL-4: the folder box).
      owner: ref.watch(isOwnerProvider).value ?? false,
      config: ref.watch(appConfigProvider),
      clock: ref.watch(appPortsProvider).clock,
      db: ref.watch(localDatabaseProvider),
    );
  }

  /// The server side.
  final DeletePort port;

  /// Whether the person is an owner (`getMe`).
  final bool owner;

  /// `ui.delete_undo_toast` and `ui.delete_folder_wait`.
  final AppConfig config;

  /// The clock of the app.
  final Clock clock;

  /// The local database, for the route check; null in tests without one.
  final LocalDatabase? db;
}

/// Opens the delete dialog of [target] from a menu entry and, after the delete, leaves a view
/// that shows the item, shows the undo snack bar and, when the folder move was asked for, its
/// outcome later. [context] is the menu region's, which outlives the open menu.
Future<void> startDelete(
  BuildContext context,
  DeleteSession session,
  DeleteTarget target,
) async {
  final port = session.port;
  final config = session.config;
  final clock = session.clock;
  final messenger = ScaffoldMessenger.maybeOf(context);
  final router = GoRouter.maybeOf(context);
  final l = AppLocalizations.of(context);
  // Decided before the delete: the live event may remove the local rows the check reads.
  final shown = router == null
      ? Future.value(false)
      : showsDeleted(
          parseLocation(router.routerDelegate.currentConfiguration.uri),
          target,
          session.db,
        );
  final outcome = await showDialog<DeleteOutcome>(
    context: context,
    builder: (_) =>
        DeleteDialog(target: target, port: port, owner: session.owner),
  );
  if (outcome == null) return;
  if (await shown) router?.go('/');
  if (messenger == null) return;
  final watch = outcome.trashFolder
      ? FolderOutcomeWatch(
          port: port,
          target: target,
          undoUntil: outcome.view.undoUntil,
          wait: config.deleteFolderWait,
          now: clock.now,
          sleep: clock.delay,
          onOutcome: (folder) => showFolderOutcome(messenger, l, folder),
        )
      : null;
  showUndoToast(
    messenger,
    l,
    target: target,
    port: port,
    duration: undoToastDuration(
      undoUntil: outcome.view.undoUntil,
      now: clock.now(),
      max: config.deleteUndoToast,
    ),
    onRestored: watch?.cancel,
  );
  if (watch != null) unawaited(watch.run());
}

/// Shows "Deleted `name`" of [target] on [messenger] with "Undo" for [duration]; without time
/// left the snack bar has no "Undo" and shows [deletedNoticeDuration]. "Undo" restores through
/// [port]; [onRestored] runs after a restore. A restore after the window says that the deletion
/// can no longer be undone, any other failure that it could not be undone.
void showUndoToast(
  ScaffoldMessengerState messenger,
  AppLocalizations l, {
  required DeleteTarget target,
  required DeletePort port,
  required Duration duration,
  VoidCallback? onRestored,
}) {
  Future<void> undo() async {
    try {
      await port.restore(
        target,
        idempotencyKey: port.newKey(),
        purpose: restorePurpose(l, target),
      );
      onRestored?.call();
    } on Object catch (e) {
      final late =
          e is NewProjectFailure &&
          e.kind == NewProjectFailureKind.problem &&
          e.code == 'conflict' &&
          e.reason == 'undo_window_passed';
      _notice(
        messenger,
        const ValueKey('delete.undoFailed'),
        late ? l.deleteUndoTooLate : l.deleteUndoFailed(target.name),
      );
    }
  }

  final canUndo = duration > Duration.zero;
  messenger
    ..hideCurrentSnackBar()
    ..showSnackBar(
      SnackBar(
        key: const ValueKey('delete.toast'),
        content: Text(l.deleteDone(target.name)),
        duration: canUndo ? duration : deletedNoticeDuration,
        // A snack bar with an action stays by default; this one must end with the window.
        persist: false,
        action: canUndo
            ? SnackBarAction(
                key: const ValueKey('delete.undo'),
                label: l.deleteUndo,
                onPressed: () => unawaited(undo()),
              )
            : null,
      ),
    );
}

/// Shows the outcome of the folder move of [folder] on [messenger] in one sentence: moved to
/// the trash, or kept with the reason sentence of the runner's refusal (01 6.18 item 7).
void showFolderOutcome(
  ScaffoldMessengerState messenger,
  AppLocalizations l,
  FolderView folder,
) => _notice(
  messenger,
  const ValueKey('delete.folderOutcome'),
  folder.trashState == FolderTrashState.trashed
      ? l.deleteFolderTrashed(folder.name)
      : l.deleteFolderKept(
          folder.name,
          folderReasonText(l, folder.trashReason ?? ''),
        ),
);

void _notice(ScaffoldMessengerState messenger, Key key, String text) {
  messenger
    ..hideCurrentSnackBar()
    ..showSnackBar(SnackBar(key: key, content: Text(text), persist: false));
}

/// Reads the folder outcome of a deleted project (01 6.18 item 7): waits until [undoUntil],
/// then reads [port] every [interval] until the runner reports `trashed` or `refused`, which
/// goes to [onOutcome] once, or until [wait] (`ui.delete_folder_wait`) has passed; without an
/// outcome it says nothing (D-DEL-UI-4). A failed read counts as no outcome yet. [now] and
/// [sleep] are the clock (tests pass fakes); [cancel] ends it after an undo.
final class FolderOutcomeWatch {
  /// Creates the watch.
  FolderOutcomeWatch({
    required this._port,
    required this._target,
    required this._undoUntil,
    required this._wait,
    required this._now,
    required this._sleep,
    required this._onOutcome,
    this._interval = folderPollInterval,
  });

  final DeletePort _port;
  final DeleteTarget _target;
  final DateTime? _undoUntil;
  final Duration _wait;
  final DateTime Function() _now;
  final Future<void> Function(Duration) _sleep;
  final void Function(FolderView folder) _onOutcome;
  final Duration _interval;
  bool _cancelled = false;

  /// Ends the watch; no outcome shows after it.
  void cancel() => _cancelled = true;

  /// Runs the watch until an outcome, [cancel] or the end of the wait.
  Future<void> run() async {
    final until = _undoUntil;
    if (until != null) {
      final left = until.difference(_now());
      if (left > Duration.zero) await _sleep(left);
    }
    // The bound counts the intervals waited, so a fixed test clock ends it too (D-DEL-UI-4).
    var waited = Duration.zero;
    while (!_cancelled) {
      FolderView? folder;
      try {
        folder = (await _port.info(_target)).folder;
      } on Object {
        // Offline or refused for now: the next read may answer.
      }
      if (_cancelled) return;
      final state = folder?.trashState;
      if (folder != null &&
          (state == FolderTrashState.trashed ||
              state == FolderTrashState.refused)) {
        _onOutcome(folder);
        return;
      }
      if (waited + _interval > _wait) return;
      await _sleep(_interval);
      waited += _interval;
    }
  }
}

/// Whether [location] shows [target] or something inside it: the chat itself, a channel or
/// its chat, or a view inside a space. The chat and channel of a thread route come from [db].
Future<bool> showsDeleted(
  AppLocation location,
  DeleteTarget target,
  LocalDatabase? db,
) async {
  ThreadId? thread;
  ChannelId? channel;
  switch (location) {
    case SpaceLocation(:final spaceId):
      return target.kind == DeleteKind.space && spaceId.value == target.id;
    case ChannelLocation(:final channelId) || NewChatLocation(:final channelId):
      channel = channelId;
    case ThreadLocation(:final threadId) || RunGraphLocation(:final threadId):
      thread = threadId;
    default:
      return false;
  }
  if (thread != null) {
    if (target.kind == DeleteKind.chat) return thread.value == target.id;
    channel = (await db?.threadsDao.byId(thread))?.channelId;
  }
  if (target.kind == DeleteKind.chat || channel == null) return false;
  if (target.kind != DeleteKind.space) return channel.value == target.id;
  final stored = await db?.structureDao.channelById(channel);
  return stored?.spaceId?.value == target.id;
}
