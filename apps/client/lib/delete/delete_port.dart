// SPDX-License-Identifier: Apache-2.0

/// The server side of "Delete..." (01 6.18; docs/design/delete.md D-DEL-4, D-DEL-6): what a
/// deletion removes (`getSpaceDeletion`, `getChannelDeletion`, `getThreadDeletion`), the delete
/// (`deleteSpace`, `deleteChannel` with `ChannelDelete.trash_project_folder`, `deleteThread`) and
/// the undo (`restoreSpace`, `restoreChannel`, `restoreThread`) through the client of the
/// signed-in session. DPoP, refresh and the step-up interceptor apply: a space and a channel
/// carry the purpose text of the step-up prompt (01 6.18 item 5), a chat needs none.
library;

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart' show stepUpPurposeExtra;
import 'package:flutter/foundation.dart';

import '../new_project/new_project_flow.dart' show NewProjectFailure;
import '../new_project/new_project_port.dart' show newProjectFailureOf;

/// What is deleted: the entry of the menu it came from (01 6.18 item 1).
enum DeleteKind {
  /// A space ("Delete space...").
  space,

  /// A text channel or a forum without a project binding ("Delete channel...").
  channel,

  /// A forum with a project binding ("Delete project..."), a channel with the folder box.
  project,

  /// A chat or thread, never a main thread ("Delete chat...").
  chat,
}

/// The item a deletion is about: its [kind], its server [id] and its [name] for the texts.
@immutable
final class DeleteTarget {
  /// Creates the target.
  const DeleteTarget({
    required this.kind,
    required this.id,
    required this.name,
  });

  /// The kind.
  final DeleteKind kind;

  /// The ID of the space, channel or thread.
  final String id;

  /// The name or title shown in the dialog and the snack bars.
  final String name;

  /// Whether the server asks for the step-up (a space or a channel, D-DEL-4).
  bool get needsStepUp => kind != DeleteKind.chat;

  @override
  bool operator ==(Object other) =>
      other is DeleteTarget &&
      other.kind == kind &&
      other.id == id &&
      other.name == name;

  @override
  int get hashCode => Object.hash(kind, id, name);
}

/// The state of a requested folder move (`ProjectFolderDeletion.trash_state`).
enum FolderTrashState {
  /// The runner has not reported yet.
  requested,

  /// The runner moved the folder to the trash of its host.
  trashed,

  /// The runner refused, with [FolderView.trashReason].
  refused,
}

/// The project folder of a deletion (`DeletionInfo.project_folder`, D-DEL-10).
@immutable
final class FolderView {
  /// Creates the view.
  const FolderView({
    required this.name,
    this.checkOk,
    this.checkReason,
    this.trashState,
    this.trashReason,
  });

  /// `workspace_ref`, the folder's name under the runner's projects root.
  final String name;

  /// The runner's last check: true ok, false refused, null never reported.
  final bool? checkOk;

  /// The reason code of a refused check, or null.
  final String? checkReason;

  /// The outcome of a requested move, or null when none was requested.
  final FolderTrashState? trashState;

  /// The refusal code with [trashState] `refused`, or null.
  final String? trashReason;
}

/// What a deletion removes or removed (`DeletionInfo`, D-DEL-6): the counts, the end of the
/// undo window ([undoUntil], null while the item is live) and the project folder.
@immutable
final class DeletionView {
  /// Creates the view.
  const DeletionView({
    this.channels = 0,
    this.chats = 0,
    this.messages = 0,
    this.files = 0,
    this.sessions = 0,
    this.undoUntil,
    this.folder,
  });

  /// Channels and forums (a space only).
  final int channels;

  /// Chats and threads, without main threads.
  final int chats;

  /// Messages.
  final int messages;

  /// Files attached to them.
  final int files;

  /// Agent sessions of those chats.
  final int sessions;

  /// The end of the undo window, or null.
  final DateTime? undoUntil;

  /// The project folder (a forum with a binding, shown to owners), or null.
  final FolderView? folder;
}

/// The server side of a deletion; every method throws [NewProjectFailure] (the client's
/// failure of a server call, with the problem's code and reason).
abstract interface class DeletePort {
  /// What deleting [target] removes, or after a delete its state and the folder outcome.
  Future<DeletionView> info(DeleteTarget target);

  /// Deletes [target] with [idempotencyKey]; [trashFolder] asks the runner to move the project
  /// folder to the trash (a project only); [purpose] is the text of the step-up prompt.
  Future<DeletionView> delete(
    DeleteTarget target, {
    required bool trashFolder,
    required String idempotencyKey,
    required String purpose,
  });

  /// Undoes the deletion of [target] with [idempotencyKey] and the step-up text [purpose].
  Future<void> restore(
    DeleteTarget target, {
    required String idempotencyKey,
    required String purpose,
  });

  /// A new idempotency key.
  String newKey();
}

/// [DeletePort] over [ApiClient].
///
/// Example:
/// ```dart
/// final port = ApiDeletePort(services.auth.api, newKey: services.ids.next);
/// final view = await port.info(target);
/// ```
final class ApiDeletePort implements DeletePort {
  /// Creates the port over the client of the signed-in session; [newKey] makes the
  /// idempotency keys.
  ApiDeletePort(this._api, {required this._newKey});

  final ApiClient _api;
  final String Function() _newKey;

  @override
  String newKey() => _newKey();

  @override
  Future<DeletionView> info(DeleteTarget target) => _guard(() async {
    final answer = switch (target.kind) {
      DeleteKind.space => await _api.getSpacesApi().getSpaceDeletion(
        spaceId: target.id,
      ),
      DeleteKind.channel || DeleteKind.project =>
        await _api.getChannelsApi().getChannelDeletion(channelId: target.id),
      DeleteKind.chat => await _api.getThreadsApi().getThreadDeletion(
        threadId: target.id,
      ),
    };
    return deletionViewOf(answer.data!);
  });

  @override
  Future<DeletionView> delete(
    DeleteTarget target, {
    required bool trashFolder,
    required String idempotencyKey,
    required String purpose,
  }) => _guard(() async {
    final extra = {stepUpPurposeExtra: purpose};
    final answer = switch (target.kind) {
      DeleteKind.space => await _api.getSpacesApi().deleteSpace(
        spaceId: target.id,
        idempotencyKey: idempotencyKey,
        extra: extra,
      ),
      DeleteKind.channel ||
      DeleteKind.project => await _api.getChannelsApi().deleteChannel(
        channelId: target.id,
        idempotencyKey: idempotencyKey,
        // Always sent, false without the box (01 6.18 item 4).
        channelDelete: ChannelDelete(
          (b) => b
            ..trashProjectFolder =
                trashFolder && target.kind == DeleteKind.project,
        ),
        extra: extra,
      ),
      DeleteKind.chat => await _api.getThreadsApi().deleteThread(
        threadId: target.id,
        idempotencyKey: idempotencyKey,
      ),
    };
    return deletionViewOf(answer.data!);
  });

  @override
  Future<void> restore(
    DeleteTarget target, {
    required String idempotencyKey,
    required String purpose,
  }) => _guard(() async {
    final extra = {stepUpPurposeExtra: purpose};
    switch (target.kind) {
      case DeleteKind.space:
        await _api.getSpacesApi().restoreSpace(
          spaceId: target.id,
          idempotencyKey: idempotencyKey,
          extra: extra,
        );
      case DeleteKind.channel || DeleteKind.project:
        await _api.getChannelsApi().restoreChannel(
          channelId: target.id,
          idempotencyKey: idempotencyKey,
          extra: extra,
        );
      case DeleteKind.chat:
        await _api.getThreadsApi().restoreThread(
          threadId: target.id,
          idempotencyKey: idempotencyKey,
        );
    }
  });

  /// Runs [call] and turns its errors into [NewProjectFailure].
  static Future<T> _guard<T>(Future<T> Function() call) async {
    try {
      return await call();
    } on Object catch (e) {
      final failure = newProjectFailureOf(e);
      if (failure == null) rethrow;
      throw failure;
    }
  }
}

/// The [DeletionView] of the server's [info].
DeletionView deletionViewOf(DeletionInfo info) {
  final c = info.contents;
  final f = info.projectFolder;
  return DeletionView(
    channels: c.channels,
    chats: c.threads,
    messages: c.messages,
    files: c.files,
    sessions: c.sessions,
    undoUntil: info.undoUntil?.toUtc(),
    folder: f == null
        ? null
        : FolderView(
            name: f.workspaceRef,
            checkOk: f.trashCheck?.ok,
            checkReason: f.trashCheck?.reason,
            trashState: _trashState(f.trashState),
            trashReason: f.trashReason,
          ),
  );
}

/// The state of [state]; a value of a newer server counts as no outcome yet.
FolderTrashState? _trashState(ProjectFolderDeletionTrashStateEnum? state) {
  if (state == ProjectFolderDeletionTrashStateEnum.requested) {
    return FolderTrashState.requested;
  }
  if (state == ProjectFolderDeletionTrashStateEnum.trashed) {
    return FolderTrashState.trashed;
  }
  if (state == ProjectFolderDeletionTrashStateEnum.refused) {
    return FolderTrashState.refused;
  }
  return null;
}
