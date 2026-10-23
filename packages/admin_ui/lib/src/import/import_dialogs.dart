// SPDX-License-Identifier: Apache-2.0

/// The confirmations of the import steps and the operator step of the switch-over
/// (docs/spec/01-product.md 5.2; 22 13.3 to 13.5; brief M8-08 fixed requirements 11, 13 to 15
/// and Design notes 3 and 4).
///
/// Every `import.manage` step shows its confirmation with the plain-language effect first; the
/// step-up follows from the interceptor when the server asks for it.
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../agents/guided_flow.dart';
import '../agents/i1_explanation.dart' show readableName;
import '../common/confirm_change.dart';
import '../common/problem_banner.dart';
import '../data/access_repository.dart';
import '../data/admin_repository.dart';
import '../data/agents_repository.dart';
import '../data/self_repository.dart';
import '../grants/grant_labels.dart' show AccessNames;
import 'import_controller.dart';
import 'import_repository.dart';

/// A state step of an import source (M8-03 note 3).
enum ImportStateAction {
  /// `startImport`: `draft` or `paused` to `running`.
  start,

  /// `pauseImport`: `running` to `paused`.
  pause,

  /// `cancelImport`: `draft`, `running` or `paused` to `failed` (cancelled).
  cancel,

  /// `completeImport`: `running` or `paused` to `completed`.
  complete,
}

/// The state steps of [source] in the order they are shown (brief M8-08 Design note 4):
/// `draft` start and cancel; `running` pause, cancel, complete; `paused` start, cancel,
/// complete; `completed` and `failed` none. Completion is offered only while
/// [allImportedLedHere] (requirement 15); the server still decides.
List<ImportStateAction> importStateActions(
  ImportSourceSnapshot source, {
  required bool allImportedLedHere,
}) => switch (source.state) {
  ImportState.draft => const [
    ImportStateAction.start,
    ImportStateAction.cancel,
  ],
  ImportState.running => [
    ImportStateAction.pause,
    ImportStateAction.cancel,
    if (allImportedLedHere) ImportStateAction.complete,
  ],
  ImportState.paused => [
    ImportStateAction.start,
    ImportStateAction.cancel,
    if (allImportedLedHere) ImportStateAction.complete,
  ],
  ImportState.completed || ImportState.failed => const [],
};

/// The confirmations of the import screens; each shows an [ImportConfirmDialog] and returns
/// whether the owner confirmed.
abstract final class ImportConfirm {
  /// The confirmation of [action] on the source named [name].
  static Future<bool> stateAction(
    BuildContext context,
    ImportStateAction action,
    String name,
  ) => _show(context, ImportConfirmDialog.state(action: action, name: name));

  /// "Bothy leads" for [row] (requirement 13).
  static Future<bool> leadBothy(BuildContext context, ImportChannelRow row) =>
      _show(context, ImportConfirmDialog.leadBothy(row: row));

  /// "Discord leads" for [row] (22 13.4).
  static Future<bool> leadDiscord(BuildContext context, ImportChannelRow row) =>
      _show(context, ImportConfirmDialog.leadDiscord(row: row));

  /// The confirmation of adding the source named [name].
  static Future<bool> create(BuildContext context, String name) =>
      _show(context, ImportConfirmDialog.create(name: name));

  static Future<bool> _show(
    BuildContext context,
    ImportConfirmDialog dialog,
  ) async =>
      await showDialog<String>(context: context, builder: (_) => dialog) !=
      null;
}

/// What an [ImportConfirmDialog] confirms.
enum ImportConfirmKind {
  /// A state step ([ImportStateAction]).
  state,

  /// "Bothy leads" for a channel.
  leadBothy,

  /// "Discord leads" for a channel.
  leadDiscord,

  /// Adding a source.
  create,
}

/// The [ConfirmChange] of one import step with its plain-language effect (01 5.2; fixed
/// requirements 11, 13 to 15):
///
/// - a state step: cancel is destructive; complete warns that the mapping in `import_items`
///   stays until the retention class `import` is deleted and that a repeated import is no
///   longer idempotent after that (22 13.5);
/// - "Bothy leads": the order of 22 13.3 (Discord locked by the owner, the `switch` command
///   with `--discord-locked` succeeded, then the lead, then the planned operator) and, when the
///   channel already has another operator than the planned one, the warning that imported
///   threads keep the planned operator (R-IM-08);
/// - "Discord leads": read-only for everyone, answers in progress cancelled, messages written
///   in Bothy stay and the `revert` command counts them (22 13.4);
/// - adding a source: the mapping is stored without secrets and starts as a draft.
///
/// Pops the reason text (empty) on "Confirm", null on "Cancel".
class ImportConfirmDialog extends StatelessWidget {
  /// The confirmation of the state step [action] on the source [name].
  const ImportConfirmDialog.state({
    super.key,
    required ImportStateAction this.action,
    required String this.name,
  }) : kind = ImportConfirmKind.state,
       row = null;

  /// "Bothy leads" for [row].
  const ImportConfirmDialog.leadBothy({
    super.key,
    required ImportChannelRow this.row,
  }) : kind = ImportConfirmKind.leadBothy,
       action = null,
       name = null;

  /// "Discord leads" for [row].
  const ImportConfirmDialog.leadDiscord({
    super.key,
    required ImportChannelRow this.row,
  }) : kind = ImportConfirmKind.leadDiscord,
       action = null,
       name = null;

  /// Adding the source [name].
  const ImportConfirmDialog.create({super.key, required String this.name})
    : kind = ImportConfirmKind.create,
      action = null,
      row = null;

  /// What is confirmed.
  final ImportConfirmKind kind;

  /// The state step, for [ImportConfirmKind.state].
  final ImportStateAction? action;

  /// The source name, for a state step and for adding.
  final String? name;

  /// The channel, for a lead step.
  final ImportChannelRow? row;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return switch (kind) {
      ImportConfirmKind.state => _state(l, action!, name!),
      ImportConfirmKind.leadBothy => _leadBothy(l, row!),
      ImportConfirmKind.leadDiscord => ConfirmChange(
        title: l.importLeadDiscordTitle(_channelName(row!)),
        body: l.importLeadDiscordBody(
          'bothy import discord revert --channel ${row!.mapped.discordId}',
        ),
        confirmLabel: l.importLeadDiscordAction,
        destructive: true,
      ),
      ImportConfirmKind.create => ConfirmChange(
        title: l.importCreateConfirmTitle(name!),
        body: l.importCreateConfirmBody,
        confirmLabel: l.importCreateSubmit,
      ),
    };
  }

  static String _channelName(ImportChannelRow row) =>
      row.bothyName ?? row.mapped.discordId;

  static Widget _state(
    AppLocalizations l,
    ImportStateAction action,
    String name,
  ) {
    final (title, body, label) = switch (action) {
      ImportStateAction.start => (
        l.importStartTitle(name),
        l.importStartBody,
        l.importStart,
      ),
      ImportStateAction.pause => (
        l.importPauseTitle(name),
        l.importPauseBody,
        l.importPause,
      ),
      ImportStateAction.cancel => (
        l.importCancelTitle(name),
        l.importCancelBody,
        l.importCancel,
      ),
      ImportStateAction.complete => (
        l.importCompleteTitle(name),
        l.importCompleteBody,
        l.importComplete,
      ),
    };
    return ConfirmChange(
      title: title,
      body: body,
      confirmLabel: label,
      destructive: action == ImportStateAction.cancel,
    );
  }

  static Widget _leadBothy(AppLocalizations l, ImportChannelRow row) {
    final planned =
        row.plannedAgent?.displayName ?? row.mapped.plannedAgentHandle;
    final operator = row.channel?.operatorAgentId;
    final other =
        planned != null && operator != null && operator != row.plannedAgent?.id;
    return ConfirmChange(
      title: l.importLeadBothyTitle(_channelName(row)),
      body: [
        l.importLeadBothyOrder,
        l.importLeadBothyStepDiscord,
        l.importLeadBothyStepSwitch(
          'bothy import discord switch --channel ${row.mapped.discordId} '
          '--discord-locked',
        ),
        l.importLeadBothyStepLead,
        if (planned != null)
          l.importLeadBothyStepOperator(planned)
        else
          l.importLeadBothyStepNoOperator,
      ].join('\n'),
      confirmLabel: l.importLeadBothyAction,
      fields: other
          ? DsBanner(
              key: const ValueKey('import.confirm.warning'),
              tone: DsTone.warning,
              text: l.importLeadBothyOtherOperator(planned),
            )
          : null,
    );
  }
}

/// The answer of [ImportOperatorDialog]: the channel after the change and the agents the
/// guided flow switched to organization (empty without it).
final class ImportOperatorResult {
  /// Creates the result.
  const ImportOperatorResult(this.channel, {this.switchedAgents = const []});

  /// The channel after `setChannelOperator`.
  final ImportChannelState channel;

  /// The agents `resolve_i1` switched.
  final List<String> switchedAgents;
}

/// The operator step of the switch-over (22 13.3 step 3) or of the revert (22 13.4 step 1):
/// with [remove] false the agent (preselected with the planned operator, a warning when another
/// one is chosen, R-IM-08) and a required reason, then `setChannelOperator` with `If-Match`; with
/// [remove] true the removal of the operator with a reason.
///
/// When the server answers `409 i1_violation`, the dialog shows the guided option of 10 11
/// ([GuidedSwitch]): after the reason of the switch it sends the same request once more with
/// `resolve_i1`, so the agent is switched and set in one transaction (BR-07). "Not now" closes
/// without a change. Pops an [ImportOperatorResult] on success, null otherwise.
///
/// Keys: `import.operator.dialog`, `import.operator.agent`, `import.operator.agent.<id>`,
/// `import.operator.warning`, `import.operator.reason`, `import.operator.submit`,
/// `import.operator.skip`, `import.operator.failed`.
class ImportOperatorDialog extends StatefulWidget {
  /// Creates the dialog.
  const ImportOperatorDialog({
    super.key,
    required this.repository,
    required this.row,
    required this.agents,
    this.remove = false,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The imported channel; its [ImportChannelRow.channel] must be known.
  final ImportChannelRow row;

  /// The agents the server lists; the active ones can be chosen.
  final List<AgentRecord> agents;

  /// Whether the operator is removed instead of set.
  final bool remove;

  /// Shows the dialog over [context].
  ///
  /// Example: `await ImportOperatorDialog.show(context, repository: r, row: row, agents: a);`
  static Future<ImportOperatorResult?> show(
    BuildContext context, {
    required AdminRepository repository,
    required ImportChannelRow row,
    required List<AgentRecord> agents,
    bool remove = false,
  }) => showDialog<ImportOperatorResult>(
    context: context,
    builder: (_) => ImportOperatorDialog(
      repository: repository,
      row: row,
      agents: agents,
      remove: remove,
    ),
  );

  @override
  State<ImportOperatorDialog> createState() => _ImportOperatorDialogState();
}

class _ImportOperatorDialogState extends State<ImportOperatorDialog> {
  final TextEditingController _reason = TextEditingController();
  // The planned agent is preselected while it is active; the list holds only active agents.
  late String? _agentId = switch (widget.row.plannedAgent) {
    final a? when a.active => a.id,
    _ => null,
  };
  bool _busy = false;
  String? _error;
  List<InvariantFinding> _conflicts = const [];
  AccessNames? _names;

  List<AgentRecord> get _active => [
    for (final a in widget.agents)
      if (a.active) a,
  ];

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

  String get _name => widget.row.bothyName ?? widget.row.mapped.discordId;

  Future<void> _send({I1Resolution? resolve}) async {
    final l = AppLocalizations.of(context);
    final channel = widget.row.channel!;
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final after = await widget.repository.setImportChannelOperator(
        channel.id,
        agentId: widget.remove ? null : _agentId,
        version: channel.version,
        reason: _reason.text.trim(),
        resolveI1: resolve,
        purpose: l.stepUpPurposeImportOperator(_name),
      );
      if (mounted) {
        Navigator.of(context).pop(
          ImportOperatorResult(
            after,
            switchedAgents: resolve?.agentIds ?? const [],
          ),
        );
      }
    } on SelfActionFailed catch (e) {
      if (resolve == null &&
          e.problem?.code == 'i1_violation' &&
          guidedAgentIds(e.conflicts).isNotEmpty) {
        final names = await AccessNames.load(widget.repository);
        if (mounted) {
          setState(() {
            _conflicts = e.conflicts;
            _names = names;
          });
        }
      } else if (mounted) {
        setState(() => _error = adminFailure(l, e)?.text);
      }
    } on Object catch (e) {
      if (mounted) setState(() => _error = adminFailure(l, e)?.text);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _takeGuided() async {
    final names = _names!;
    final reason = await GuidedFlow.askReason(
      context,
      agents: guidedAgentNames(names, _conflicts),
    );
    if (reason == null || !mounted) return;
    final ids = guidedAgentIds(_conflicts);
    setState(() => _conflicts = const []);
    await _send(resolve: I1Resolution(ids, reason: reason));
  }

  String _people(AccessNames names) => {
    for (final c in _conflicts)
      for (final p in c.principalIds) readableName(names, p) ?? p,
  }.join(', ');

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final planned = widget.row.plannedAgent;
    final chosen = _agentId;
    final other =
        !widget.remove &&
        planned != null &&
        chosen != null &&
        chosen != planned.id;
    final canSend =
        !_busy &&
        _reason.text.trim().isNotEmpty &&
        (widget.remove || chosen != null);
    final names = _names;
    return AlertDialog(
      key: const ValueKey('import.operator.dialog'),
      backgroundColor: colors.surfaceRaised,
      title: Text(
        widget.remove
            ? l.importOperatorRemoveTitle(_name)
            : l.importOperatorTitle(_name),
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
                widget.remove
                    ? l.importOperatorRemoveBody
                    : l.importOperatorBody,
                style: dsTextStyle(DsType.body)
                    .copyWith(color: colors.textSecondary),
              ),
              const SizedBox(height: DsSpace.s16),
              if (!widget.remove)
                DropdownButtonFormField<String>(
                  key: const ValueKey('import.operator.agent'),
                  initialValue: chosen,
                  isExpanded: true,
                  decoration: InputDecoration(labelText: l.importOperatorAgent),
                  items: [
                    for (final a in _active)
                      DropdownMenuItem(
                        key: ValueKey('import.operator.agent.${a.id}'),
                        value: a.id,
                        child: Text(a.displayName),
                      ),
                  ],
                  onChanged: _busy
                      ? null
                      : (v) => setState(() {
                          _agentId = v;
                          _conflicts = const [];
                        }),
                ),
              if (other) ...[
                const SizedBox(height: DsSpace.s8),
                DsBanner(
                  key: const ValueKey('import.operator.warning'),
                  tone: DsTone.warning,
                  text: l.importOperatorNotPlanned(
                    _active.firstWhere((a) => a.id == chosen).displayName,
                    planned.displayName,
                  ),
                ),
              ],
              const SizedBox(height: DsSpace.s8),
              TextField(
                key: const ValueKey('import.operator.reason'),
                controller: _reason,
                enabled: !_busy,
                decoration: InputDecoration(labelText: l.adminReason),
              ),
              if (_error case final error?) ...[
                const SizedBox(height: DsSpace.s8),
                DsBanner(
                  key: const ValueKey('import.operator.failed'),
                  tone: DsTone.danger,
                  text: error,
                  liveRegion: true,
                ),
              ],
              if (names != null && _conflicts.isNotEmpty) ...[
                const SizedBox(height: DsSpace.s8),
                GuidedSwitch(
                  conflicts: _conflicts,
                  person: _people(names),
                  names: names,
                  grant: false,
                  onTake: _busy ? null : _takeGuided,
                ),
              ],
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          key: const ValueKey('import.operator.skip'),
          onPressed: _busy ? null : () => Navigator.of(context).pop(),
          child: Text(l.importOperatorSkip),
        ),
        FilledButton(
          key: const ValueKey('import.operator.submit'),
          onPressed: canSend ? _send : null,
          child: Text(
            widget.remove ? l.importOperatorRemove : l.importOperatorSet,
          ),
        ),
      ],
    );
  }
}
