// SPDX-License-Identifier: Apache-2.0

/// The apps of the administration (docs/spec/01-product.md 5.2, 5.5; 11 4; brief M6-08).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import '../data/apps_repository.dart';
import 'app_labels.dart';
import 'install_dialog.dart';
import 'manifest_picker.dart';

/// Every app the server lists (`listApps`) with name, slug, version, publisher and the state
/// of its installation; a row opens the app ([onOpen] receives its ID).
///
/// "Install app" opens the [InstallAppDialog] with the host's file dialog [pickManifest]; a
/// host without one ([pickManifest] null) shows the button disabled with a short hint, because
/// an app is installed from a file only (11 4.1). After an installation the list loads again,
/// [onChanged] runs and nothing else opens. A list the server refuses shows its reason and
/// nothing else (15 1 principle 1).
///
/// Example: `AppsList(repository: admin, onOpen: (id) => open(id), pickManifest: pick)`.
class AppsList extends StatefulWidget {
  /// Creates the list.
  const AppsList({
    super.key,
    required this.repository,
    required this.onOpen,
    this.pickManifest,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.onChanged,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// Opens the page of an app by its ID.
  final ValueChanged<String> onOpen;

  /// The host's file dialog for a manifest, or null when the host has none.
  final ManifestPicker? pickManifest;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Runs after every change, or null.
  final VoidCallback? onChanged;

  @override
  State<AppsList> createState() => _AppsListState();
}

class _AppsListState extends State<AppsList> {
  List<AppRecord>? _apps;
  Object? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final apps = await widget.repository.apps();
      if (mounted) {
        setState(() {
          _apps = apps;
          _error = null;
        });
      }
    } on Object catch (e) {
      if (mounted) {
        setState(() {
          _apps = null;
          _error = e;
        });
      }
    }
  }

  Future<void> _install(ManifestPicker pick) async {
    final changed = await InstallAppDialog.show(
      context,
      repository: widget.repository,
      pickManifest: pick,
      timeZone: widget.timeZone,
    );
    if (changed != true || !mounted) return;
    await _load();
    widget.onChanged?.call();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final apps = _apps;
    final error = _error;
    final pick = widget.pickManifest;
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    final Widget content;
    if (error != null) {
      content = ProblemBanner(problem: loadProblem(error), onRetry: _load);
    } else if (apps == null) {
      content = const Center(child: CircularProgressIndicator());
    } else {
      content = Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              Expanded(
                child: Semantics(
                  header: true,
                  child: Text(
                    l.adminAppListTitle,
                    style: dsTextStyle(DsType.title)
                        .copyWith(color: colors.textPrimary),
                  ),
                ),
              ),
              OutlinedButton(
                key: const ValueKey('apps.install'),
                onPressed: pick == null ? null : () => _install(pick),
                child: Text(l.adminAppInstall),
              ),
            ],
          ),
          if (pick == null)
            Align(
              alignment: AlignmentDirectional.centerEnd,
              child: Text(l.adminAppInstallNoPicker, style: secondary),
            ),
          const SizedBox(height: DsSpace.s12),
          if (apps.isEmpty)
            Text(
              l.adminAppListEmpty,
              style: dsTextStyle(DsType.body)
                  .copyWith(color: colors.textSecondary),
            ),
          for (final a in apps) _row(l, secondary, a),
        ],
      );
    }
    return KeyedSubtree(key: const ValueKey('apps.list'), child: content);
  }

  Widget _row(AppLocalizations l, TextStyle secondary, AppRecord a) => ListTile(
    key: ValueKey('apps.row.${a.id}'),
    contentPadding: EdgeInsets.zero,
    title: Text(a.displayName),
    subtitle: Padding(
      padding: const EdgeInsets.only(top: DsSpace.s4),
      child: Wrap(
        spacing: DsSpace.s8,
        runSpacing: DsSpace.s4,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          AppStateBadge(state: a.state),
          Text(a.slug, style: secondary),
          Text(l.adminAppVersion(a.manifestVersion), style: secondary),
          Text(l.adminAppPublisher(a.manifest.publisher), style: secondary),
        ],
      ),
    ),
    onTap: () => widget.onOpen(a.id),
  );
}
