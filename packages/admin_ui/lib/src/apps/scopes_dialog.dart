// SPDX-License-Identifier: Apache-2.0

/// "Change consent" of the app page: the scopes and events of an installed app after the
/// install (11 4.2 `app.scopes.update`), also when the new consent of an update was put off
/// with "Later".
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import '../data/apps_repository.dart';
import 'consent_view.dart';

/// The consent of [app] over its current manifest: the required scopes fixed, the optional
/// scopes and the events checked as granted now and changeable, a reason, and "Save consent"
/// (`setAppScopes`, step-up from the interceptor). A failure stays in the dialog; the dialog
/// cannot be closed while the call runs.
///
/// Example: `final saved = await AppScopesDialog.show(context, repository: admin, app: app);`
class AppScopesDialog extends StatefulWidget {
  /// Creates the dialog for [app].
  const AppScopesDialog({
    super.key,
    required this.repository,
    required this.app,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The installed app.
  final AppRecord app;

  /// Shows the dialog; true once the consent is saved, else null.
  static Future<bool?> show(
    BuildContext context, {
    required AdminRepository repository,
    required AppRecord app,
  }) => showDialog<bool>(
    context: context,
    barrierDismissible: false,
    builder: (_) => AppScopesDialog(repository: repository, app: app),
  );

  @override
  State<AppScopesDialog> createState() => _AppScopesDialogState();
}

class _AppScopesDialogState extends State<AppScopesDialog> {
  final TextEditingController _reason = TextEditingController();
  late final Set<String> _scopes = {
    ...widget.app.manifest.requiredScopes,
    ...widget.app.grantedScopes,
  };
  late final Set<String> _events = {...widget.app.eventSubscriptions};
  bool _busy = false;
  String? _failure;

  @override
  void initState() {
    super.initState();
    _reason.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _reason.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    final l = AppLocalizations.of(context);
    final app = widget.app;
    final m = app.manifest;
    setState(() {
      _busy = true;
      _failure = null;
    });
    try {
      await widget.repository.setScopes(
        app.id,
        grantedScopes: [
          ...m.requiredScopes,
          for (final s in m.optionalScopes)
            if (_scopes.contains(s)) s,
        ],
        eventSubscriptions: [
          for (final e in m.events)
            if (_events.contains(e)) e,
        ],
        reason: _reason.text.trim(),
        purpose: l.adminAppPurposeScopes(app.displayName),
      );
      if (mounted) Navigator.of(context).pop(true);
    } on Object catch (e) {
      if (mounted) setState(() => _failure = adminFailure(l, e)?.text);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return PopScope(
      canPop: !_busy,
      child: AlertDialog(
        key: const ValueKey('scopes.dialog'),
        backgroundColor: colors.surfaceRaised,
        title: Text(
          l.adminAppChangeConsentTitle(widget.app.displayName),
          style: dsTextStyle(DsType.title).copyWith(color: colors.textPrimary),
        ),
        content: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: DsLayout.overlayWidth),
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text(
                  l.adminAppChangeConsentBody,
                  style: dsTextStyle(DsType.body)
                      .copyWith(color: colors.textPrimary),
                ),
                const SizedBox(height: DsSpace.s12),
                ConsentView(
                  manifest: widget.app.manifest,
                  scopes: _scopes,
                  events: _events,
                  choicesOnly: true,
                  enabled: !_busy,
                  keyPrefix: 'scopes',
                  onScope: (s, on) =>
                      setState(() => on ? _scopes.add(s) : _scopes.remove(s)),
                  onEvent: (e, on) =>
                      setState(() => on ? _events.add(e) : _events.remove(e)),
                ),
                const SizedBox(height: DsSpace.s8),
                TextField(
                  key: const ValueKey('scopes.reason'),
                  controller: _reason,
                  enabled: !_busy,
                  decoration: InputDecoration(labelText: l.adminReasonRequired),
                ),
                if (_failure case final failure?) ...[
                  const SizedBox(height: DsSpace.s8),
                  DsBanner(
                    key: const ValueKey('scopes.failure'),
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
            key: const ValueKey('scopes.cancel'),
            onPressed: _busy ? null : () => Navigator.of(context).pop(),
            child: Text(l.selfCancel),
          ),
          FilledButton(
            key: const ValueKey('scopes.submit'),
            onPressed: _busy || _reason.text.trim().isEmpty ? null : _save,
            child: Text(l.adminAppNewConsentSubmit),
          ),
        ],
      ),
    );
  }
}
