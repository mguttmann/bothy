// SPDX-License-Identifier: Apache-2.0

/// The delete dialog (01 6.18 items 3 to 5; docs/design/delete.md D-DEL-6, D-DEL-10): the title
/// "Delete `name`?", the counts that go in one sentence, read when the dialog opens (a loading
/// state, an error sentence with "Try again"), the hint about the undo window, for an owner of a
/// project the unchecked box that moves its folder to the trash (disabled with its reason while
/// the runner's check does not allow it), and "Cancel" and "Delete" in the danger style.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../new_project/new_project_flow.dart' show NewProjectFailure;
import '../new_project/project_sync_dialog.dart' show serverFailureText;
import 'delete_port.dart';
import 'delete_text.dart';

/// What the dialog did: the server's answer to the delete and whether the folder move was
/// asked for.
final class DeleteOutcome {
  /// Creates the outcome.
  const DeleteOutcome({required this.view, required this.trashFolder});

  /// The answer of the delete (`pending` with `undo_until`).
  final DeletionView view;

  /// Whether the box "Also move the project folder ... to the trash" was checked.
  final bool trashFolder;
}

/// The dialog of [target] over [port]; pops with a [DeleteOutcome] after the delete, with null
/// on "Cancel". [owner] shows the folder box of a project (D-DEL-4: the move needs
/// `channel.project.bind`).
class DeleteDialog extends StatefulWidget {
  /// Creates the dialog.
  const DeleteDialog({
    super.key,
    required this.target,
    required this.port,
    required this.owner,
  });

  /// What is deleted.
  final DeleteTarget target;

  /// The server side.
  final DeletePort port;

  /// Whether the person is an owner.
  final bool owner;

  @override
  State<DeleteDialog> createState() => _DeleteDialogState();
}

class _DeleteDialogState extends State<DeleteDialog> {
  DeletionView? _view;
  bool _loadFailed = false;
  bool _busy = false;
  bool _trashFolder = false;
  NewProjectFailure? _failure;

  /// One key per box state: a repeated press after a lost answer is the same request.
  final Map<bool, String> _keys = {};

  @override
  void initState() {
    super.initState();
    unawaited(_load());
  }

  Future<void> _load() async {
    setState(() {
      _loadFailed = false;
      _view = null;
    });
    try {
      final view = await widget.port.info(widget.target);
      if (mounted) setState(() => _view = view);
    } on Object {
      if (mounted) setState(() => _loadFailed = true);
    }
  }

  /// The folder of a project the person may move, or null for no box.
  FolderView? get _folder =>
      widget.owner && widget.target.kind == DeleteKind.project
      ? _view?.folder
      : null;

  Future<void> _delete(AppLocalizations l) async {
    final folder = _folder;
    final trash =
        _trashFolder && folder != null && folderBoxBlocked(l, folder) == null;
    setState(() {
      _busy = true;
      _failure = null;
    });
    try {
      final view = await widget.port.delete(
        widget.target,
        trashFolder: trash,
        idempotencyKey: _keys.putIfAbsent(trash, widget.port.newKey),
        purpose: deletePurpose(l, widget.target),
      );
      if (mounted) {
        Navigator.of(context)
            .pop(DeleteOutcome(view: view, trashFolder: trash));
      }
    } on NewProjectFailure catch (e) {
      if (mounted) setState(() => _failure = e);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final view = _view;
    final folder = _folder;
    final failure = _failure;
    final body = dsTextStyle(DsType.body).copyWith(color: colors.textPrimary);
    final quiet = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final List<Widget> content;
    if (_loadFailed) {
      content = [
        Text(
          l.deleteLoadFailed,
          key: const ValueKey('delete.loadFailed'),
          style: body.copyWith(color: colors.dangerFg),
        ),
        const SizedBox(height: DsSpace.s8),
        Align(
          alignment: AlignmentDirectional.centerStart,
          child: DsButton(
            key: const ValueKey('delete.retry'),
            label: l.deleteRetry,
            onPressed: () => unawaited(_load()),
          ),
        ),
      ];
    } else if (view == null) {
      content = [
        Semantics(
          liveRegion: true,
          child: Row(
            key: const ValueKey('delete.loading'),
            children: [
              const SizedBox.square(
                dimension: DsSpace.s16,
                child: CircularProgressIndicator(strokeWidth: 2),
              ),
              const SizedBox(width: DsSpace.s8),
              Expanded(child: Text(l.deleteLoading, style: quiet)),
            ],
          ),
        ),
      ];
    } else {
      final blocked = folder == null ? null : folderBoxBlocked(l, folder);
      content = [
        Text(
          deletionSummary(l, view),
          key: const ValueKey('delete.counts'),
          style: body,
        ),
        const SizedBox(height: DsSpace.s8),
        Text(
          l.deleteUndoHint,
          key: const ValueKey('delete.hint'),
          style: quiet,
        ),
        if (folder != null) ...[
          const SizedBox(height: DsSpace.s12),
          CheckboxListTile(
            key: const ValueKey('delete.folder'),
            contentPadding: EdgeInsets.zero,
            controlAffinity: ListTileControlAffinity.leading,
            value: _trashFolder && blocked == null,
            onChanged: blocked != null || _busy
                ? null
                : (v) => setState(() => _trashFolder = v ?? false),
            title: Text(l.deleteFolderBox(dsIsolate(folder.name)), style: body),
            subtitle: blocked == null
                ? null
                : Text(
                    blocked,
                    key: const ValueKey('delete.folderBlocked'),
                    style: quiet,
                  ),
          ),
        ],
        if (failure != null) ...[
          const SizedBox(height: DsSpace.s8),
          Text(
            serverFailureText(l, failure),
            key: const ValueKey('delete.failure'),
            style: body.copyWith(color: colors.dangerFg),
          ),
        ],
      ];
    }
    return AlertDialog(
      key: const ValueKey('delete.dialog'),
      title: Text(l.deleteTitle(dsIsolate(widget.target.name))),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 420),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          mainAxisSize: MainAxisSize.min,
          children: content,
        ),
      ),
      actions: [
        DsButton(
          key: const ValueKey('delete.cancel'),
          label: l.deleteCancel,
          onPressed: () => Navigator.of(context).pop(),
        ),
        DsButton(
          key: const ValueKey('delete.confirm'),
          label: l.deleteConfirm,
          style: DsButtonStyle.danger,
          loading: _busy,
          // Enabled once the counts show (01 6.18 item 3).
          onPressed: view == null || _busy ? null : () => unawaited(_delete(l)),
        ),
      ],
    );
  }
}
