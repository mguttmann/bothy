// SPDX-License-Identifier: Apache-2.0

/// The versions of the definition of a built-in agent (docs/spec/26-builtin-agents.md 4.1 item
/// 2, 8.4; brief A1-09 requirement 13 and Design note 3): newest first with version, time,
/// author and origin; "Show" opens a version read-only in the editor, "Restore" opens it in the
/// editor against the current version, where "Save" writes its content as a new version
/// through the same save path as an edit.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../new_agent/new_agent_port.dart';

/// The dialog of the versions of [agent].
///
/// [onShow] opens a loaded version read-only; [onRestore] opens it for saving and returns
/// whether it was saved, which closes the dialog with true. [ownerNames] names the authors
/// (an unknown author shows by ID); [timeZone] and [clock] format the times.
///
/// Example:
/// ```dart
/// final restored = await AgentVersionsDialog.show(context, port: port, agent: agent,
///     ownerNames: names, onShow: show, onRestore: restore);
/// ```
class AgentVersionsDialog extends StatefulWidget {
  /// Creates the dialog.
  const AgentVersionsDialog({
    super.key,
    required this.port,
    required this.agent,
    required this.onShow,
    required this.onRestore,
    this.ownerNames = const {},
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
  });

  /// The server side.
  final NewAgentPort port;

  /// The agent.
  final NewAgentState agent;

  /// Opens a version read-only.
  final Future<void> Function(StoredDefinition version) onShow;

  /// Opens a version for saving; true when it was saved.
  final Future<bool> Function(StoredDefinition version) onRestore;

  /// The display names of the owners by ID.
  final Map<String, String> ownerNames;

  /// The time zone of the signed-in person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Shows the dialog; returns true when a version was restored.
  static Future<bool?> show(
    BuildContext context, {
    required NewAgentPort port,
    required NewAgentState agent,
    required Future<void> Function(StoredDefinition version) onShow,
    required Future<bool> Function(StoredDefinition version) onRestore,
    Map<String, String> ownerNames = const {},
    String timeZone = 'UTC',
    DateTime Function() clock = DateTime.now,
  }) => showDialog<bool>(
    context: context,
    builder: (_) => AgentVersionsDialog(
      port: port,
      agent: agent,
      onShow: onShow,
      onRestore: onRestore,
      ownerNames: ownerNames,
      timeZone: timeZone,
      clock: clock,
    ),
  );

  @override
  State<AgentVersionsDialog> createState() => _AgentVersionsDialogState();
}

class _AgentVersionsDialogState extends State<AgentVersionsDialog> {
  List<DefinitionVersionInfo>? _versions;
  NewAgentFailure? _failure;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    unawaited(_load());
  }

  Future<void> _load() async {
    try {
      final versions = await widget.port.versions(widget.agent.id);
      if (mounted) setState(() => _versions = versions);
    } on Object catch (e) {
      if (mounted) setState(() => _failure = _failureOf(e));
    }
  }

  static NewAgentFailure _failureOf(Object e) =>
      e is NewAgentFailure ? e : const NewAgentFailure.offline();

  Future<void> _open(int version, {required bool restore}) async {
    setState(() {
      _busy = true;
      _failure = null;
    });
    try {
      final stored = await widget.port.definitionVersion(
        widget.agent.id,
        version,
      );
      if (!mounted) return;
      if (restore) {
        if (await widget.onRestore(stored) && mounted) {
          Navigator.of(context).pop(true);
        }
      } else {
        await widget.onShow(stored);
      }
    } on Object catch (e) {
      if (mounted) setState(() => _failure = _failureOf(e));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  String _when(DateTime at) {
    final format = L10nFormat(Localizations.localeOf(context));
    String in_(String zone) =>
        '${format.relativeDay(at, now: widget.clock(), timeZone: zone)} '
        '${format.time(at, timeZone: zone)}';
    try {
      return in_(widget.timeZone);
    } on ArgumentError {
      return in_('UTC');
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final versions = _versions;
    final failure = _failure;
    final current = versions?.firstOrNull?.version;
    return AlertDialog(
      key: const ValueKey('agentVersions.dialog'),
      title: Text(l.agentVersionsTitle(widget.agent.name)),
      content: SizedBox(
        width: 560,
        child: SingleChildScrollView(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            mainAxisSize: MainAxisSize.min,
            children: [
              if (failure != null)
                Padding(
                  padding: const EdgeInsets.only(bottom: DsSpace.s8),
                  child: Text(
                    failure.kind == NewAgentFailureKind.offline
                        ? l.newAgentErrOffline
                        : problemText(
                            l,
                            failure.code ?? '',
                            reason: failure.reason,
                          ),
                    key: const ValueKey('agentVersions.error'),
                    style: dsTextStyle(DsType.body)
                        .copyWith(color: colors.dangerFg),
                  ),
                ),
              if (versions == null && failure == null)
                const Center(child: CircularProgressIndicator())
              else if (versions != null && versions.isEmpty)
                Text(l.agentVersionsEmpty)
              else
                for (final v in versions ?? const <DefinitionVersionInfo>[])
                  _row(l, v, current: v.version == current),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          key: const ValueKey('agentVersions.close'),
          onPressed: () => Navigator.of(context).pop(false),
          child: Text(l.newAgentClose),
        ),
      ],
    );
  }

  Widget _row(
    AppLocalizations l,
    DefinitionVersionInfo v, {
    required bool current,
  }) {
    final colors = context.dsColors;
    final author = widget.ownerNames[v.createdBy] ?? v.createdBy;
    // Text first, the actions below: the row stays readable at a large text scale.
    return Padding(
      key: ValueKey('agentVersions.row.${v.version}'),
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s4),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(
            l.agentVersionLine(
              v.version,
              _when(v.createdAt),
              author,
              v.origin.name,
            ),
            style: dsTextStyle(DsType.body).copyWith(color: colors.textPrimary),
          ),
          if (current)
            Text(
              l.agentVersionCurrent,
              key: ValueKey('agentVersions.current.${v.version}'),
              style: dsTextStyle(DsType.bodySm)
                  .copyWith(color: colors.textSecondary),
            ),
          Wrap(
            spacing: DsSpace.s8,
            children: [
              TextButton(
                key: ValueKey('agentVersions.show.${v.version}'),
                onPressed: _busy
                    ? null
                    : () => _open(v.version, restore: false),
                child: Semantics(
                  label: l.agentVersionShowLabel(v.version),
                  excludeSemantics: true,
                  child: Text(l.agentVersionShow),
                ),
              ),
              if (!current)
                TextButton(
                  key: ValueKey('agentVersions.restore.${v.version}'),
                  onPressed: _busy
                      ? null
                      : () => _open(v.version, restore: true),
                  child: Semantics(
                    label: l.agentVersionRestoreLabel(v.version),
                    excludeSemantics: true,
                    child: Text(l.agentVersionRestore),
                  ),
                ),
            ],
          ),
        ],
      ),
    );
  }
}
