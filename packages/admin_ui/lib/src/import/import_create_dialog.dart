// SPDX-License-Identifier: Apache-2.0

/// "Add source": a name and the mapping file, converted from YAML to JSON in the browser and
/// sent with `createImport` after the confirmation and the step-up (docs/spec/
/// 22-import-migration.md 5.2 and 8; brief M8-08 Design note 2).
library;

import 'dart:convert';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../apps/manifest_picker.dart' show PickedFile;
import '../data/admin_repository.dart';
import '../data/self_repository.dart';
import 'import_dialogs.dart';
import 'import_labels.dart';
import 'import_repository.dart';
import 'mapping_file.dart';

/// Opens the host's file dialog for a mapping file (`.yaml`, `.yml`) and returns the chosen
/// file, or null when the person cancels. The console opens a file input of the browser.
typedef MappingPicker = Future<PickedFile?> Function();

/// The dialog that adds an import source of `kind = discord`.
///
/// The YAML is parsed strictly (a key named twice is an error) and converted to JSON; the text
/// and the JSON stay in the dialog's state and nothing reaches browser storage. A file that is
/// no YAML mapping says why with its line. `400 validation_failed` lists every error of the
/// server with its place in the file (`channels[2].planned_operator`); the owner fixes the file,
/// chooses it again and adds. A file larger than the server's request limit is refused by the
/// server (`413`), which the dialog shows. Pops the created [ImportSourceSnapshot], or null.
///
/// Keys: `import.create.dialog`, `import.create.name`, `import.create.pick`,
/// `import.create.file`, `import.create.fileError`, `import.create.errors`,
/// `import.create.error.<n>`, `import.create.failed`, `import.create.submit`,
/// `import.create.cancel`.
class ImportCreateDialog extends StatefulWidget {
  /// Creates the dialog.
  const ImportCreateDialog({
    super.key,
    required this.repository,
    required this.pickMapping,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The host's file dialog.
  final MappingPicker pickMapping;

  /// Shows the dialog over [context].
  ///
  /// Example: `final created = await ImportCreateDialog.show(context, repository: r, pickMapping: p);`
  static Future<ImportSourceSnapshot?> show(
    BuildContext context, {
    required AdminRepository repository,
    required MappingPicker pickMapping,
  }) => showDialog<ImportSourceSnapshot>(
    context: context,
    builder: (_) =>
        ImportCreateDialog(repository: repository, pickMapping: pickMapping),
  );

  @override
  State<ImportCreateDialog> createState() => _ImportCreateDialogState();
}

class _ImportCreateDialogState extends State<ImportCreateDialog> {
  final TextEditingController _name = TextEditingController();
  Map<String, Object?>? _config;
  String? _fileError;
  List<String> _serverErrors = const [];
  String? _failure;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    _name.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _name.dispose();
    super.dispose();
  }

  Future<void> _pick() async {
    final l = AppLocalizations.of(context);
    // A new choice replaces the old file at once: until it is read, nothing counts as chosen,
    // so "Add" never sends the file of the choice before.
    setState(() {
      _serverErrors = const [];
      _failure = null;
      _config = null;
      _fileError = null;
    });
    final file = await widget.pickMapping();
    if (file == null || !mounted) return;
    setState(() {
      try {
        _config = mappingJsonOf(utf8.decode(file.bytes));
      } on FormatException {
        _fileError = l.importFileUnreadable;
      } on MappingFileError catch (e) {
        _fileError = importFileErrorText(l, e);
      }
    });
  }

  Future<void> _submit() async {
    final l = AppLocalizations.of(context);
    final name = _name.text.trim();
    final config = _config!;
    if (!await ImportConfirm.create(context, name) || !mounted) return;
    setState(() {
      _busy = true;
      _serverErrors = const [];
      _failure = null;
    });
    try {
      final created = await widget.repository.createImportSource(
        name: name,
        config: config,
        purpose: l.stepUpPurposeImportCreate(name),
      );
      if (mounted) Navigator.of(context).pop(created);
    } on SelfActionFailed catch (e) {
      final problem = e.problem;
      if (!mounted) return;
      setState(() {
        if (problem != null &&
            problem.code == 'validation_failed' &&
            problem.errors.isNotEmpty) {
          _serverErrors = importFieldErrors(l, problem);
        } else {
          _failure = importFailure(l, e)?.text;
        }
      });
    } on Object catch (e) {
      if (mounted) setState(() => _failure = importFailure(l, e)?.text);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final canSubmit = !_busy && _config != null && _name.text.trim().isNotEmpty;
    return AlertDialog(
      key: const ValueKey('import.create.dialog'),
      backgroundColor: colors.surfaceRaised,
      title: Text(
        l.importCreateTitle,
        style: dsTextStyle(DsType.title).copyWith(color: colors.textPrimary),
      ),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: DsLayout.overlayWidth),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              TextField(
                key: const ValueKey('import.create.name'),
                controller: _name,
                enabled: !_busy,
                decoration: InputDecoration(labelText: l.importCreateName),
              ),
              const SizedBox(height: DsSpace.s12),
              Text(l.importCreateHint, style: secondary),
              const SizedBox(height: DsSpace.s8),
              Align(
                alignment: Alignment.centerLeft,
                child: OutlinedButton(
                  key: const ValueKey('import.create.pick'),
                  onPressed: _busy ? null : _pick,
                  child: Text(l.importCreatePick),
                ),
              ),
              if (_config != null) ...[
                const SizedBox(height: DsSpace.s8),
                Text(
                  l.importCreateFileRead,
                  key: const ValueKey('import.create.file'),
                  style: secondary,
                ),
              ],
              if (_fileError case final error?) ...[
                const SizedBox(height: DsSpace.s8),
                DsBanner(
                  key: const ValueKey('import.create.fileError'),
                  tone: DsTone.danger,
                  text: error,
                  liveRegion: true,
                ),
              ],
              if (_serverErrors.isNotEmpty) ...[
                const SizedBox(height: DsSpace.s8),
                Semantics(
                  liveRegion: true,
                  child: Column(
                    key: const ValueKey('import.create.errors'),
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(
                        l.importServerErrors,
                        style: dsTextStyle(DsType.body)
                            .copyWith(color: colors.dangerFg),
                      ),
                      for (final (i, e) in _serverErrors.indexed)
                        Text(
                          e,
                          key: ValueKey('import.create.error.$i'),
                          style: dsTextStyle(DsType.bodySm)
                              .copyWith(color: colors.dangerFg),
                        ),
                    ],
                  ),
                ),
              ],
              if (_failure case final failure?) ...[
                const SizedBox(height: DsSpace.s8),
                DsBanner(
                  key: const ValueKey('import.create.failed'),
                  tone: DsTone.danger,
                  text: failure,
                  liveRegion: true,
                ),
              ],
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          key: const ValueKey('import.create.cancel'),
          onPressed: _busy ? null : () => Navigator.of(context).pop(),
          child: Text(l.selfCancel),
        ),
        FilledButton(
          key: const ValueKey('import.create.submit'),
          onPressed: canSubmit ? _submit : null,
          child: Text(l.importCreateSubmit),
        ),
      ],
    );
  }
}
