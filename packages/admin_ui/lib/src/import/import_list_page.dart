// SPDX-License-Identifier: Apache-2.0

/// The page "Import" of the console: the import sources with kind, state and progress, "Add
/// source" and the refresh control (docs/spec/01-product.md 3.7 and 5.5; brief M8-08 Goal and
/// Design note 1).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import 'import_controller.dart';
import 'import_create_dialog.dart';
import 'import_labels.dart';

/// The import sources the server lists (`listImports`), each with its kind, state and the
/// number of imported items; a row opens the source ([onOpen] receives its ID).
///
/// "Add source" needs the host's file dialog ([pickMapping]; null hides it) and opens the
/// created source. The list reads the server again on the refresh control and whenever the
/// browser tab becomes visible again; there is no timer (brief M8-08 Design note 1). A list the
/// server refuses shows its reason and nothing else (15 1 principle 1).
///
/// Keys: `import.list`, `import.add`, `import.refresh`, `import.empty`, `import.row.<id>`.
class ImportListPage extends StatefulWidget {
  /// Creates the page.
  const ImportListPage({
    super.key,
    required this.repository,
    required this.onOpen,
    this.pickMapping,
    this.onChanged,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// Opens a source.
  final ValueChanged<String> onOpen;

  /// The host's file dialog for the mapping file, or null.
  final MappingPicker? pickMapping;

  /// Runs after a change.
  final VoidCallback? onChanged;

  @override
  State<ImportListPage> createState() => _ImportListPageState();
}

class _ImportListPageState extends State<ImportListPage> {
  late final ImportController _controller = ImportController(widget.repository);
  late final AppLifecycleListener _lifecycle;
  bool _loaded = false;
  Object? _error;

  @override
  void initState() {
    super.initState();
    // The browser tab became visible again (visibilitychange): read the server again.
    _lifecycle = AppLifecycleListener(onShow: () => unawaited(_load()));
    unawaited(_load());
  }

  @override
  void dispose() {
    _lifecycle.dispose();
    _controller.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      await _controller.loadList();
      if (mounted) {
        setState(() {
          _loaded = true;
          _error = null;
        });
      }
    } on Object catch (e) {
      if (mounted) setState(() => _error = e);
    }
  }

  Future<void> _add(MappingPicker pick) async {
    final created = await ImportCreateDialog.show(
      context,
      repository: widget.repository,
      pickMapping: pick,
    );
    if (created == null || !mounted) return;
    _controller.apply(created);
    widget.onChanged?.call();
    widget.onOpen(created.id);
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final error = _error;
    final pick = widget.pickMapping;
    return ListenableBuilder(
      listenable: _controller,
      builder: (context, _) => Column(
        key: const ValueKey('import.list'),
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              Expanded(
                child: Semantics(
                  header: true,
                  child: Text(
                    l.importTitle,
                    style: dsTextStyle(DsType.title)
                        .copyWith(color: colors.textPrimary),
                  ),
                ),
              ),
              IconButton(
                key: const ValueKey('import.refresh'),
                tooltip: l.importRefresh,
                onPressed: _load,
                icon: const Icon(Icons.refresh),
              ),
              if (pick != null && error == null)
                FilledButton(
                  key: const ValueKey('import.add'),
                  onPressed: () => _add(pick),
                  child: Text(l.importAdd),
                ),
            ],
          ),
          const SizedBox(height: DsSpace.s12),
          if (error != null)
            ProblemBanner(problem: loadProblem(error), onRetry: _load)
          else if (!_loaded)
            const Center(child: CircularProgressIndicator())
          else if (_controller.sources.isEmpty)
            Text(
              l.importEmpty,
              key: const ValueKey('import.empty'),
              style: dsTextStyle(DsType.body)
                  .copyWith(color: colors.textSecondary),
            )
          else
            for (final s in _controller.sources)
              ListTile(
                key: ValueKey('import.row.${s.id}'),
                title: Text(s.name),
                subtitle: Text(
                  [
                    s.kind,
                    importStateLabel(l, s),
                    l.importItemsImported(importPostedCount(s)),
                  ].join('  |  '),
                ),
                onTap: () => widget.onOpen(s.id),
              ),
        ],
      ),
    );
  }
}
