// SPDX-License-Identifier: Apache-2.0

/// The confirmation of suspend, resume and uninstall with the effects of 11 4.3 (brief M6-08
/// Design note 2, Ruling 3).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'app_labels.dart';

/// The one confirmation of [action] for the app [appName]: the column of the table of 11 4.3
/// for the action (rows tokens, deliveries, commands, components, grants and incoming
/// webhooks, messages; for resume what comes back), shown before anything is sent, and for
/// uninstall the reason the server stores in the audit log.
///
/// The dialog sends nothing: it pops the reason (empty for suspend and resume) for "Confirm"
/// and null for "Cancel". The caller sends the change with its purpose, and the step-up
/// interceptor asks for the step-up when the server answers `401 step_up_required`.
///
/// Keys: `lifecycle.dialog`, `lifecycle.effects`, `lifecycle.reason`, `lifecycle.submit`,
/// `lifecycle.cancel`.
///
/// Example:
/// ```dart
/// final reason = await AppLifecycleDialog.show(
///   context,
///   appName: 'Repo Bridge',
///   action: AppLifecycleAction.uninstall,
/// );
/// ```
class AppLifecycleDialog extends StatefulWidget {
  /// Creates the dialog.
  const AppLifecycleDialog({
    super.key,
    required this.appName,
    required this.action,
  });

  /// The name of the app.
  final String appName;

  /// The action to confirm.
  final AppLifecycleAction action;

  /// Shows the dialog over [context]; returns the reason (empty without one) or null.
  static Future<String?> show(
    BuildContext context, {
    required String appName,
    required AppLifecycleAction action,
  }) => showDialog<String>(
    context: context,
    builder: (_) => AppLifecycleDialog(appName: appName, action: action),
  );

  @override
  State<AppLifecycleDialog> createState() => _AppLifecycleDialogState();
}

class _AppLifecycleDialogState extends State<AppLifecycleDialog> {
  final TextEditingController _reason = TextEditingController();

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

  bool get _uninstall => widget.action == AppLifecycleAction.uninstall;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final name = widget.appName;
    final (title, body, confirm) = switch (widget.action) {
      AppLifecycleAction.suspend => (
        l.adminAppSuspendTitle(name),
        l.adminAppSuspendBody,
        l.adminAppSuspend,
      ),
      AppLifecycleAction.resume => (
        l.adminAppResumeTitle(name),
        l.adminAppResumeBody,
        l.adminAppResume,
      ),
      AppLifecycleAction.uninstall => (
        l.adminAppUninstallTitle(name),
        l.adminAppUninstallBody,
        l.adminAppUninstall,
      ),
    };
    final strong = dsTextStyle(DsType.bodyStrong)
        .copyWith(color: colors.textPrimary);
    final cell = dsTextStyle(DsType.body).copyWith(color: colors.textPrimary);
    const pad = EdgeInsets.symmetric(
      vertical: DsSpace.s4,
      horizontal: DsSpace.s8,
    );
    final reason = _reason.text.trim();
    return AlertDialog(
      key: const ValueKey('lifecycle.dialog'),
      backgroundColor: colors.surfaceRaised,
      title: Text(
        title,
        style: dsTextStyle(DsType.title).copyWith(color: colors.textPrimary),
      ),
      content: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: DsLayout.overlayWidth),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              if (_uninstall)
                DsBanner(tone: DsTone.danger, text: body)
              else
                Text(
                  body,
                  style: dsTextStyle(DsType.body)
                      .copyWith(color: colors.textSecondary),
                ),
              const SizedBox(height: DsSpace.s12),
              Table(
                key: const ValueKey('lifecycle.effects'),
                columnWidths: const {
                  0: IntrinsicColumnWidth(flex: 1),
                  1: FlexColumnWidth(2),
                },
                border: TableBorder(
                  horizontalInside: BorderSide(color: colors.borderSubtle),
                ),
                children: [
                  TableRow(
                    children: [
                      Padding(
                        padding: pad,
                        child: Text(l.adminAppEffectsPart, style: strong),
                      ),
                      Padding(
                        padding: pad,
                        child: Text(l.adminAppEffectsResult, style: strong),
                      ),
                    ],
                  ),
                  for (final row in lifecycleEffects(l, widget.action))
                    TableRow(
                      children: [
                        Padding(
                          padding: pad,
                          child: Text(row.part, style: cell),
                        ),
                        Padding(
                          padding: pad,
                          child: Text(row.effect, style: cell),
                        ),
                      ],
                    ),
                ],
              ),
              if (_uninstall) ...[
                const SizedBox(height: DsSpace.s16),
                TextField(
                  key: const ValueKey('lifecycle.reason'),
                  controller: _reason,
                  decoration: InputDecoration(labelText: l.adminReasonRequired),
                ),
              ],
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          key: const ValueKey('lifecycle.cancel'),
          autofocus: _uninstall,
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l.selfCancel),
        ),
        FilledButton(
          key: const ValueKey('lifecycle.submit'),
          style: _uninstall
              ? FilledButton.styleFrom(
                  backgroundColor: colors.dangerBg,
                  foregroundColor: colors.dangerOnBg,
                )
              : null,
          // The server requires the reason of an uninstall (`ReasonBody`).
          onPressed: _uninstall && reason.isEmpty
              ? null
              : () => Navigator.of(context).pop(reason),
          child: Text(confirm),
        ),
      ],
    );
  }
}
