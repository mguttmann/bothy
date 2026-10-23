// SPDX-License-Identifier: Apache-2.0

/// One import source: state and its steps, the counters of the server and of the import tool,
/// the failed items and the channel table with the lead and operator steps (docs/spec/
/// 22-import-migration.md 8, 11, 12.1, 13; brief M8-08 Goal and Design notes 1, 3 and 4).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../agents/guided_flow.dart' show GuidedResult;
import '../common/problem_banner.dart';
import '../data/admin_repository.dart';
import '../self/self_feedback.dart';
import 'import_channel_table.dart';
import 'import_controller.dart';
import 'import_dialogs.dart';
import 'import_labels.dart';
import 'import_repository.dart';

/// The page of the source [sourceId].
///
/// It shows only counters and structure, never content, file names or names of people (22 11,
/// 14): the counters are the numbers of `stats.server` and `stats.tool`, a failed item shows
/// its kind, origin key (Discord IDs) and an error code, a channel row its Discord ID and its
/// Bothy channel. Every step (start, pause, cancel, complete, a lead change, the operator) runs
/// after its confirmation with the step-up the server asks for, and the page reads the source
/// again after each step, on the refresh control and when the browser tab becomes visible
/// again; there is no timer (fixed requirement 16).
///
/// Keys: `import.source`, `import.back`, `import.refresh`, `import.name`, `import.state`,
/// `import.action.<start|pause|cancel|complete>`, `import.completeHint`, `import.next`,
/// `import.counters.server`, `import.counters.tool`, `import.counter.<server|tool>.<path>`,
/// `import.failed`, `import.failed.<item id>`, `self.notice`.
class ImportSourcePage extends StatefulWidget {
  /// Creates the page.
  const ImportSourcePage({
    super.key,
    required this.repository,
    required this.sourceId,
    required this.onBack,
    this.onChanged,
  });

  /// The administration's calls.
  final AdminRepository repository;

  /// The source.
  final String sourceId;

  /// Returns to the list.
  final VoidCallback onBack;

  /// Runs after a change.
  final VoidCallback? onChanged;

  @override
  State<ImportSourcePage> createState() => _ImportSourcePageState();
}

class _ImportSourcePageState extends State<ImportSourcePage> {
  late final ImportController _controller = ImportController(widget.repository);
  late final AppLifecycleListener _lifecycle;
  Object? _error;
  bool _busy = false;
  SelfNotice? _notice;

  String get _id => widget.sourceId;

  @override
  void initState() {
    super.initState();
    _lifecycle = AppLifecycleListener(onShow: () => unawaited(_refresh()));
    unawaited(_refresh());
  }

  @override
  void dispose() {
    _lifecycle.dispose();
    _controller.dispose();
    super.dispose();
  }

  Future<void> _refresh() async {
    try {
      await _controller.refresh(_id);
      if (mounted) setState(() => _error = null);
    } on Object catch (e) {
      if (mounted) setState(() => _error = e);
    }
  }

  /// Runs [step]; shows its notice or its failure (the `409` reasons in plain language), then
  /// reads the source again. Returns whether the step succeeded.
  Future<bool> _run(Future<String> Function() step) async {
    final l = AppLocalizations.of(context);
    setState(() => _busy = true);
    var ok = false;
    try {
      final text = await step();
      ok = true;
      if (mounted) setState(() => _notice = SelfNotice(text));
      widget.onChanged?.call();
    } on Object catch (e) {
      final failure = importFailure(l, e);
      if (mounted && failure != null) {
        setState(() => _notice = SelfNotice(failure.text, tone: DsTone.danger));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
    // The server is the source: the page shows its state after every step, also a refused one.
    if (mounted) await _refresh();
    return ok;
  }

  Future<void> _stateAction(
    ImportSourceSnapshot source,
    ImportStateAction action,
  ) async {
    final l = AppLocalizations.of(context);
    final name = source.name;
    if (!await ImportConfirm.stateAction(context, action, name) || !mounted) {
      return;
    }
    final r = widget.repository;
    await _run(() async {
      final ImportSourceSnapshot after;
      final String done;
      switch (action) {
        case ImportStateAction.start:
          after = await r.startImport(
            _id,
            purpose: l.stepUpPurposeImportStart(name),
          );
          done = l.importStarted(name);
        case ImportStateAction.pause:
          after = await r.pauseImport(
            _id,
            purpose: l.stepUpPurposeImportPause(name),
          );
          done = l.importPausedDone(name);
        case ImportStateAction.cancel:
          after = await r.cancelImport(
            _id,
            purpose: l.stepUpPurposeImportCancel(name),
          );
          done = l.importCancelledDone(name);
        case ImportStateAction.complete:
          after = await r.completeImport(
            _id,
            purpose: l.stepUpPurposeImportComplete(name),
          );
          done = l.importCompletedDone(name);
      }
      _controller.apply(after);
      return done;
    });
  }

  Future<void> _leadBothy(ImportChannelRow row) async {
    final l = AppLocalizations.of(context);
    if (!await ImportConfirm.leadBothy(context, row) || !mounted) return;
    final name = row.bothyName ?? row.mapped.discordId;
    final ok = await _run(() async {
      _controller.apply(
        await widget.repository.setImportLead(
          _id,
          discordChannel: row.mapped.discordId,
          lead: ImportLead.bothy,
          purpose: l.stepUpPurposeImportLead(name),
        ),
      );
      return l.importLeadBothyDone(name);
    });
    if (!ok || !mounted) return;
    // 22 13.3 step 3: the operator follows the lead, preselected with the planned one.
    final fresh = _rowOf(row.mapped.discordId);
    if (fresh != null && fresh.phase != ImportChannelPhase.bothyOperatorOk) {
      await _operator(fresh);
    }
  }

  Future<void> _leadDiscord(ImportChannelRow row) async {
    final l = AppLocalizations.of(context);
    if (!await ImportConfirm.leadDiscord(context, row) || !mounted) return;
    final name = row.bothyName ?? row.mapped.discordId;
    final ok = await _run(() async {
      _controller.apply(
        await widget.repository.setImportLead(
          _id,
          discordChannel: row.mapped.discordId,
          lead: ImportLead.discord,
          purpose: l.stepUpPurposeImportLead(name),
        ),
      );
      return l.importLeadDiscordDone(name);
    });
    if (!ok || !mounted) return;
    // 22 13.4 step 1: then the offer to remove the operator.
    final fresh = _rowOf(row.mapped.discordId);
    if (fresh?.channel?.operatorAgentId != null) {
      await _operator(fresh!, remove: true);
    }
  }

  ImportChannelRow? _rowOf(String discordId) => _controller
      .details(_id)
      ?.rows
      .where((r) => r.mapped.discordId == discordId)
      .firstOrNull;

  Future<void> _operator(ImportChannelRow row, {bool remove = false}) async {
    final l = AppLocalizations.of(context);
    final details = _controller.details(_id);
    if (row.channel == null || details == null) return;
    final result = await ImportOperatorDialog.show(
      context,
      repository: widget.repository,
      row: row,
      agents: details.agents,
      remove: remove,
    );
    if (result == null || !mounted) return;
    final name = result.channel.name;
    final agent = details.agents
        .where((a) => a.id == result.channel.operatorAgentId)
        .firstOrNull
        ?.displayName;
    setState(
      () => _notice = SelfNotice(
        agent == null
            ? l.importOperatorRemoved(name)
            : l.importOperatorDone(agent, name),
      ),
    );
    widget.onChanged?.call();
    await _refresh();
    if (result.switchedAgents.isNotEmpty && mounted) {
      await GuidedResult.show(
        context,
        repository: widget.repository,
        agentIds: result.switchedAgents,
      );
    }
  }

  Widget _counters(
    AppLocalizations l,
    String key,
    String title,
    List<ImportCounter> counters,
  ) {
    final colors = context.dsColors;
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    return Column(
      key: ValueKey('import.counters.$key'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Semantics(
          header: true,
          child: Text(
            title,
            style: dsTextStyle(DsType.heading)
                .copyWith(color: colors.textPrimary),
          ),
        ),
        const SizedBox(height: DsSpace.s4),
        if (counters.isEmpty) Text(l.importNoCounters, style: secondary),
        Wrap(
          spacing: DsSpace.s16,
          runSpacing: DsSpace.s4,
          children: [
            for (final c in counters)
              Text(
                '${c.path}: ${c.value}',
                key: ValueKey('import.counter.$key.${c.path}'),
                style: dsTextStyle(DsType.bodySm)
                    .copyWith(color: colors.textPrimary),
              ),
          ],
        ),
        const SizedBox(height: DsSpace.s16),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return ListenableBuilder(
      listenable: _controller,
      builder: (context, _) {
        final colors = context.dsColors;
        final source = _controller.source(_id);
        final details = _controller.details(_id);
        final error = _error;
        final secondary = dsTextStyle(DsType.bodySm)
            .copyWith(color: colors.textSecondary);
        return Column(
          key: const ValueKey('import.source'),
          crossAxisAlignment: CrossAxisAlignment.stretch,
          mainAxisSize: MainAxisSize.min,
          children: [
            Row(
              children: [
                TextButton.icon(
                  key: const ValueKey('import.back'),
                  onPressed: widget.onBack,
                  icon: const Icon(Icons.arrow_back),
                  label: Text(l.importBack),
                ),
                const Spacer(),
                IconButton(
                  key: const ValueKey('import.refresh'),
                  tooltip: l.importRefresh,
                  onPressed: _busy ? null : _refresh,
                  icon: const Icon(Icons.refresh),
                ),
              ],
            ),
            if (_notice case final n?) selfNoticeBanner(n),
            if (error != null && (source == null || details == null))
              ProblemBanner(problem: loadProblem(error), onRetry: _refresh)
            else if (source == null || details == null)
              const Center(child: CircularProgressIndicator())
            else
              ..._body(l, source, details, secondary),
          ],
        );
      },
    );
  }

  List<Widget> _body(
    AppLocalizations l,
    ImportSourceSnapshot source,
    ImportSourceDetails details,
    TextStyle secondary,
  ) {
    final colors = context.dsColors;
    final ledHere = details.allImportedLedHere;
    final actions = importStateActions(source, allImportedLedHere: ledHere);
    final guild = switch (source.config['guild_id']) {
      final String g => g,
      _ => '<guild>',
    };
    final command = 'bothy import discord --guild $guild --map <file>';
    return [
      Semantics(
        header: true,
        child: Text(
          source.name,
          key: const ValueKey('import.name'),
          style: dsTextStyle(DsType.title).copyWith(color: colors.textPrimary),
        ),
      ),
      Text(
        '${source.kind}  |  ${importStateLabel(l, source)}',
        key: const ValueKey('import.state'),
        style: secondary,
      ),
      const SizedBox(height: DsSpace.s12),
      if (actions.isNotEmpty)
        Wrap(
          spacing: DsSpace.s8,
          runSpacing: DsSpace.s8,
          children: [
            for (final a in actions)
              switch (a) {
                ImportStateAction.cancel => OutlinedButton(
                  key: ValueKey('import.action.${a.name}'),
                  style: OutlinedButton.styleFrom(
                    foregroundColor: colors.dangerFg,
                  ),
                  onPressed: _busy ? null : () => _stateAction(source, a),
                  child: Text(l.importCancel),
                ),
                _ => FilledButton(
                  key: ValueKey('import.action.${a.name}'),
                  onPressed: _busy ? null : () => _stateAction(source, a),
                  child: Text(switch (a) {
                    ImportStateAction.start => l.importStart,
                    ImportStateAction.pause => l.importPause,
                    _ => l.importComplete,
                  }),
                ),
              },
          ],
        ),
      if ((source.state == ImportState.running ||
              source.state == ImportState.paused) &&
          !ledHere)
        Padding(
          padding: const EdgeInsets.only(top: DsSpace.s8),
          child: Text(
            l.importCompleteHint,
            key: const ValueKey('import.completeHint'),
            style: secondary,
          ),
        ),
      if (source.state == ImportState.draft)
        Padding(
          padding: const EdgeInsets.only(top: DsSpace.s12),
          child: Column(
            key: const ValueKey('import.next'),
            crossAxisAlignment: CrossAxisAlignment.stretch,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(l.importNextSteps, style: dsTextStyle(DsType.body)),
              Text(l.importNextCheck(command), style: secondary),
              Text(l.importNextStart, style: secondary),
              Text(l.importNextApply('$command --apply'), style: secondary),
            ],
          ),
        ),
      const SizedBox(height: DsSpace.s16),
      _counters(l, 'server', l.importServerCounters, source.serverCounters),
      _counters(l, 'tool', l.importToolCounters, source.toolCounters),
      Semantics(
        header: true,
        child: Text(
          l.importFailedItems,
          style: dsTextStyle(DsType.heading).copyWith(color: colors.textPrimary),
        ),
      ),
      const SizedBox(height: DsSpace.s4),
      Column(
        key: const ValueKey('import.failed'),
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          if (details.failedItems.isEmpty)
            Text(l.importNoFailedItems, style: secondary),
          for (final i in details.failedItems)
            Text(
              l.importFailedItem(
                i.itemKind,
                i.originKey,
                importItemErrorCode(i.error) ?? '',
              ),
              key: ValueKey('import.failed.${i.id}'),
              style: secondary,
            ),
        ],
      ),
      const SizedBox(height: DsSpace.s16),
      ImportChannelTable(
        rows: details.rows,
        actionable:
            source.state != ImportState.completed &&
            source.state != ImportState.failed,
        busy: _busy,
        onLeadBothy: _leadBothy,
        onLeadDiscord: _leadDiscord,
        onSetOperator: _operator,
      ),
    ];
  }
}
