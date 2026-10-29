// SPDX-License-Identifier: Apache-2.0

/// "Rename project" of a forum with a project binding (01 6.11; docs/decisions.d/M4-44.md
/// D-M4-44-6): a short dialog with the current name that renames the forum through
/// `updateChannel` with its version as `If-Match`. The folder, the binding and the sessions
/// stay. A changed version reads the forum again and asks the person to confirm once more.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:l10n/l10n.dart';

import '../app/providers/session.dart';
import 'new_project_flow.dart';
import 'new_project_port.dart';
import 'project_sync_dialog.dart' show serverFailureText;

/// Opens the dialog for [forum] with the client of the signed-in session; returns the new name
/// after a rename, null when the person cancelled. Does nothing without a session.
Future<String?> showRenameProjectDialog(
  BuildContext context,
  WidgetRef ref,
  Channel forum,
) async {
  final services = ref.read(signedInServicesProvider);
  if (services == null) return null;
  final dialog = RenameProjectDialog(
    forum: forum.id,
    name: forum.name,
    port: ApiRenameProjectPort(services.auth.api),
    newKey: services.ids.next,
  );
  return showDialog<String>(context: context, builder: (_) => dialog);
}

/// The dialog over [port].
class RenameProjectDialog extends StatefulWidget {
  /// Creates the dialog of [forum] with its current [name]; [newKey] makes the
  /// `Idempotency-Key` of a rename.
  const RenameProjectDialog({
    super.key,
    required this.forum,
    required this.name,
    required this.port,
    required this.newKey,
  });

  /// The forum.
  final ChannelId forum;

  /// Its current name.
  final String name;

  /// The server side.
  final RenameProjectPort port;

  /// Makes an idempotency key.
  final String Function() newKey;

  @override
  State<RenameProjectDialog> createState() => _RenameProjectDialogState();
}

class _RenameProjectDialogState extends State<RenameProjectDialog> {
  late final TextEditingController _name = TextEditingController(
    text: widget.name,
  );
  int? _version;
  String? _key;
  String? _keyFor;
  bool _busy = false;
  bool _changed = false;
  NewProjectFailure? _failure;

  String get _trimmed => _name.text.trim();

  bool get _canRename =>
      !_busy && _trimmed.isNotEmpty && _trimmed != widget.name;

  Future<void> _rename() async {
    final name = _trimmed;
    setState(() {
      _busy = true;
      _failure = null;
    });
    try {
      final version = _version ??= await widget.port.version(widget.forum);
      // One key per name and version: a repeated press after a lost answer is the same request.
      final keyFor = '$version:$name';
      if (_key == null || _keyFor != keyFor) {
        _key = widget.newKey();
        _keyFor = keyFor;
      }
      await widget.port.rename(
        widget.forum,
        name,
        version: version,
        idempotencyKey: _key!,
      );
      if (mounted) Navigator.of(context).pop(name);
      return;
    } on NewProjectFailure catch (e) {
      if (e.kind == NewProjectFailureKind.problem &&
          e.code == 'version_conflict') {
        await _reread();
        return;
      }
      if (mounted) setState(() => _failure = e);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  /// Reads the forum's version again after a conflict; the typed name stays for one more press.
  Future<void> _reread() async {
    try {
      final version = await widget.port.version(widget.forum);
      if (mounted) {
        setState(() {
          _version = version;
          _changed = true;
        });
      }
    } on NewProjectFailure catch (e) {
      if (mounted) setState(() => _failure = e);
    }
  }

  @override
  void dispose() {
    _name.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final failure = _failure;
    return AlertDialog(
      key: const ValueKey('renameProject.dialog'),
      title: Text(l.renameProjectTitle),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 420),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
              key: const ValueKey('renameProject.name'),
              controller: _name,
              autofocus: true,
              enabled: !_busy,
              onChanged: (_) => setState(() {}),
              onSubmitted: (_) {
                if (_canRename) unawaited(_rename());
              },
              decoration: InputDecoration(
                labelText: l.renameProjectName,
                helperText: l.renameProjectHint,
                helperMaxLines: 3,
              ),
            ),
            if (_changed && failure == null) ...[
              const SizedBox(height: DsSpace.s8),
              Text(
                l.renameProjectChanged,
                key: const ValueKey('renameProject.changed'),
                style: dsTextStyle(DsType.body)
                    .copyWith(color: colors.textSecondary),
              ),
            ],
            if (failure != null) ...[
              const SizedBox(height: DsSpace.s8),
              Text(
                serverFailureText(l, failure),
                key: const ValueKey('renameProject.failure'),
                style: dsTextStyle(DsType.body)
                    .copyWith(color: colors.dangerFg),
              ),
            ],
          ],
        ),
      ),
      actions: [
        TextButton(
          key: const ValueKey('renameProject.cancel'),
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l.renameProjectCancel),
        ),
        FilledButton(
          key: const ValueKey('renameProject.submit'),
          onPressed: _canRename ? () => unawaited(_rename()) : null,
          child: Text(l.renameProjectSubmit),
        ),
      ],
    );
  }
}
