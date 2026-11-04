// SPDX-License-Identifier: Apache-2.0

/// The run history and the detail of a finished run (docs/spec/25-visualization.md 6.7).
library;

import 'dart:async';

import 'package:api_client/api_client.dart' as api;
import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart' show AppConfigValues;
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../marks/run_label_mark.dart';
import '../marks/viz_colors.dart';
import '../marks/viz_texts.dart';
import '../model/run_history_model.dart';
import '../run_graph/run_graph_model.dart';
import '../run_graph/run_graph_view.dart';
import '../sources/live_frame_source.dart';
import '../sources/stats_query_source.dart';
import 'table/run_history_table.dart';
import 'view_scaffold.dart';
import 'view_texts.dart';
import 'viz_host_options.dart';

/// Opens the thread [threadId] at the message [messageId].
typedef VizOpenMessage = void Function(String threadId, String messageId);

/// The end states of the filter, `null` for all.
const List<api.RunState?> _filterStates = [
  null,
  api.RunState.completed,
  api.RunState.failed,
  api.RunState.cancelled,
];

/// The finished runs the person may read, newest end first, page by page.
///
/// Reads `listRunHistory` for the scope of the host and the chosen end state; "Load more runs"
/// follows `next_cursor`, and the last page ends with the note that there are no older runs.
/// Above the list a note says how far the history reaches: runs stay as long as their chat
/// (docs/spec/20-privacy.md 2). Each row names the chat with the shared run label and shows
/// end state, start, end, duration, cost (as "Estimate" or "information only"), the tokens, the
/// subagents and the model; a tap opens [RunHistoryDetail]. The table form has the same values.
class RunHistoryView extends StatefulWidget {
  /// Creates the view.
  const RunHistoryView({
    super.key,
    required this.live,
    required this.queries,
    required this.options,
    required this.onOpenThread,
    this.onOpenMessage,
  });

  /// Not used by this view; the view contract is the same for all views.
  final LiveFrameSource? live;

  /// The REST queries (`listRunHistory`, `getRun`).
  final StatsQuerySource queries;

  /// What the host knows.
  final VizHostOptions options;

  /// Opens the thread of a run.
  final void Function(String threadId) onOpenThread;

  /// Opens the thread at the triggering message; `null` hides the jump.
  final VizOpenMessage? onOpenMessage;

  @override
  State<RunHistoryView> createState() => _RunHistoryViewState();
}

class _RunHistoryViewState extends State<RunHistoryView> {
  final List<api.RunHistoryEntry> _entries = [];
  String? _next;
  bool _loaded = false;
  bool _loadingMore = false;
  String? _error;
  api.RunState? _state;
  RunHistoryRow? _selected;
  int _generation = 0;

  @override
  void initState() {
    super.initState();
    unawaited(_load(reset: true));
  }

  Future<void> _load({required bool reset}) async {
    final generation = reset ? ++_generation : _generation;
    try {
      final page = await widget.queries.runHistory(
        spaceId: widget.options.scope.spaceId,
        state: _state?.name,
        cursor: reset ? null : _next,
      );
      if (!mounted || generation != _generation) return;
      setState(() {
        if (reset) _entries.clear();
        _entries.addAll(page.items);
        _next = page.nextCursor;
        _loaded = true;
        _loadingMore = false;
      });
    } on Object catch (error) {
      if (!mounted || generation != _generation) return;
      setState(() {
        _error = vizProblemCode(error);
        _loadingMore = false;
      });
    }
  }

  void _filter(api.RunState? state) {
    if (state == _state) return;
    setState(() {
      _state = state;
      _entries.clear();
      _next = null;
      _loaded = false;
      _error = null;
    });
    unawaited(_load(reset: true));
  }

  void _more() {
    if (_loadingMore || _next == null) return;
    setState(() => _loadingMore = true);
    unawaited(_load(reset: false));
  }

  void _open(RunHistoryRow row) => setState(() => _selected = row);

  @override
  Widget build(BuildContext context) {
    final selected = _selected;
    if (selected != null) {
      return RunHistoryDetail(
        key: ValueKey('viz-history-detail-${selected.runId}'),
        row: selected,
        queries: widget.queries,
        options: widget.options,
        onBack: () => setState(() => _selected = null),
        onOpenThread: widget.onOpenThread,
        onOpenMessage: widget.onOpenMessage,
      );
    }
    final texts = VizTexts.of(context);
    final l10n = texts.l10n;
    final costs = VizTvScope.costsVisible(context);
    final model = RunHistoryModel.fromEntries(
      _entries,
      names: widget.options.names,
    );
    final now = widget.options.clock.now();
    final timeZone = widget.options.timeZone;
    final error = _error;
    return VizViewScaffold(
      title: l10n.vizViewRunHistory,
      options: widget.options,
      status: error != null
          ? VizFailed(error)
          : _loaded
          ? const VizReady()
          : const VizLoading(),
      notes: [(tone: DsTone.info, text: l10n.vizHistoryRetention)],
      controls: _StateFilter(selected: _state, onSelected: _filter),
      tables: (texts) => runHistoryTables(
        texts,
        model,
        now: now,
        timeZone: timeZone,
        costsVisible: costs,
        onOpenThread: widget.onOpenThread,
        onOpenRun: (id) => _open(model.rows.firstWhere((r) => r.runId == id)),
      ),
      chart: (context) => Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          if (model.rows.isEmpty)
            Text(
              l10n.vizHistoryEmpty,
              style: dsTextStyle(DsType.body)
                  .copyWith(color: context.dsColors.textSecondary),
            ),
          for (final row in model.rows)
            Padding(
              padding: const EdgeInsets.only(bottom: DsSpace.s8),
              child: _HistoryRow(
                row: row,
                now: now,
                timeZone: timeZone,
                costsVisible: costs,
                onOpen: () => _open(row),
              ),
            ),
          if (_next != null)
            Align(
              alignment: AlignmentDirectional.centerStart,
              child: OutlinedButton(
                key: const ValueKey('viz-history-more'),
                style: vizButtonStyle(context),
                onPressed: _loadingMore ? null : _more,
                child: Text(l10n.vizHistoryLoadMore),
              ),
            )
          else if (model.rows.isNotEmpty)
            Text(
              l10n.vizHistoryEnd,
              style: dsTextStyle(DsType.bodySm)
                  .copyWith(color: context.dsColors.textSecondary),
            ),
        ],
      ),
    );
  }
}

/// The filter by end state: all, completed, failed, cancelled.
class _StateFilter extends StatelessWidget {
  const _StateFilter({required this.selected, required this.onSelected});

  final api.RunState? selected;
  final ValueChanged<api.RunState?> onSelected;

  @override
  Widget build(BuildContext context) {
    final texts = VizTexts.of(context);
    final colors = context.dsColors;
    return Wrap(
      spacing: DsSpace.s8,
      runSpacing: DsSpace.s8,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        Text(
          texts.l10n.vizHistoryStateFilter,
          style: dsTextStyle(DsType.label)
              .copyWith(color: colors.textSecondary),
        ),
        for (final state in _filterStates)
          VizToggleButton(
            key: ValueKey('viz-history-state-${state?.name ?? 'all'}'),
            label: state == null
                ? texts.l10n.vizHistoryAll
                : texts.timelineState(state),
            selected: state == selected,
            onPressed: () => onSelected(state),
          ),
      ],
    );
  }
}

/// One figure: its name above its value.
class _Figure extends StatelessWidget {
  const _Figure({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    final colors = context.dsColors;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          label,
          style: dsTextStyle(DsType.caption)
              .copyWith(color: colors.textSecondary),
        ),
        Text(
          value,
          style: dsTextStyle(DsType.bodySm).copyWith(color: colors.textPrimary),
        ),
      ],
    );
  }
}

/// The figures of [row] in the order of the table, cost only with [costsVisible].
List<Widget> _figures(
  VizTexts texts,
  RunHistoryRow row, {
  required DateTime now,
  required String timeZone,
  required bool costsVisible,
}) {
  final l10n = texts.l10n;
  return [
    _Figure(
      label: l10n.vizColStart,
      value: row.startedAt == null
          ? l10n.vizNone
          : vizDayTime(texts, row.startedAt!, now, timeZone),
    ),
    _Figure(
      label: l10n.vizColEnd,
      value: vizDayTime(texts, row.finishedAt, now, timeZone),
    ),
    _Figure(
      label: l10n.vizColDuration,
      value: row.duration == null
          ? l10n.vizNone
          : texts.format.durationShort(row.duration!),
    ),
    if (costsVisible)
      _Figure(label: l10n.vizColRunCost, value: texts.cost(row.cost)),
    _Figure(
      label: l10n.vizColTokensIn,
      value: vizOptionalCount(texts, row.tokens.input),
    ),
    _Figure(
      label: l10n.vizColTokensOut,
      value: vizOptionalCount(texts, row.tokens.output),
    ),
    _Figure(
      label: l10n.vizColCacheRead,
      value: vizOptionalCount(texts, row.tokens.cacheRead),
    ),
    _Figure(
      label: l10n.vizColCacheWrite,
      value: vizOptionalCount(texts, row.tokens.cacheWrite),
    ),
    _Figure(
      label: l10n.vizColSubagents,
      value: vizOptionalCount(texts, row.subagents),
    ),
    _Figure(label: l10n.vizColModel, value: row.model ?? l10n.vizNone),
  ];
}

/// The end state of a run as icon and word, never color alone (25 7.4).
class _StateTag extends StatelessWidget {
  const _StateTag({required this.state});

  final api.RunState state;

  @override
  Widget build(BuildContext context) {
    final colors = context.dsColors;
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        DsIcon(
          runStateGlyph(state),
          size: DsIconSizeRole.sm,
          color: runStateColor(colors, state),
          decorative: true,
        ),
        const SizedBox(width: DsSpace.s4),
        Text(
          VizTexts.of(context).timelineState(state),
          style: dsTextStyle(DsType.label).copyWith(color: colors.textPrimary),
        ),
      ],
    );
  }
}

/// One finished run of the list: label and end state, then the figures; a tap opens the
/// detail.
class _HistoryRow extends StatelessWidget {
  const _HistoryRow({
    required this.row,
    required this.now,
    required this.timeZone,
    required this.costsVisible,
    required this.onOpen,
  });

  final RunHistoryRow row;
  final DateTime now;
  final String timeZone;
  final bool costsVisible;
  final VoidCallback onOpen;

  @override
  Widget build(BuildContext context) {
    final texts = VizTexts.of(context);
    final colors = context.dsColors;
    return Semantics(
      button: true,
      label: texts.l10n.vizHistoryOpenRun(row.label.shortId),
      child: Material(
        color: colors.surfaceRaised,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(DsRadius.md),
          side: BorderSide(color: colors.borderSubtle),
        ),
        clipBehavior: Clip.antiAlias,
        child: InkWell(
          key: ValueKey('viz-history-row-${row.runId}'),
          onTap: onOpen,
          child: Padding(
            padding: const EdgeInsets.all(DsSpace.s12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              mainAxisSize: MainAxisSize.min,
              children: [
                Row(
                  children: [
                    Expanded(child: VizRunLabelText(label: row.label)),
                    const SizedBox(width: DsSpace.s8),
                    _StateTag(state: row.state),
                  ],
                ),
                const SizedBox(height: DsSpace.s8),
                Wrap(
                  spacing: DsSpace.s16,
                  runSpacing: DsSpace.s8,
                  children: _figures(
                    texts,
                    row,
                    now: now,
                    timeZone: timeZone,
                    costsVisible: costsVisible,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// The detail of a finished run: the figures of its row with the model and the step count,
/// the summary and the last text of the agent, the steps as the tree of the run graph
/// (subagents, workflows, background jobs and tool calls with duration and state, from
/// `getRun`), and the jumps into the chat and to the triggering message.
class RunHistoryDetail extends StatefulWidget {
  /// Creates the detail of [row].
  const RunHistoryDetail({
    super.key,
    required this.row,
    required this.queries,
    required this.options,
    required this.onBack,
    required this.onOpenThread,
    this.onOpenMessage,
  });

  /// The run from the list.
  final RunHistoryRow row;

  /// The REST queries (`getRun`).
  final StatsQuerySource queries;

  /// What the host knows.
  final VizHostOptions options;

  /// Returns to the list.
  final VoidCallback onBack;

  /// Opens the thread of the run.
  final void Function(String threadId) onOpenThread;

  /// Opens the thread at the triggering message; `null` hides the jump.
  final VizOpenMessage? onOpenMessage;

  @override
  State<RunHistoryDetail> createState() => _RunHistoryDetailState();
}

class _RunHistoryDetailState extends State<RunHistoryDetail> {
  api.Run? _run;
  String? _error;

  /// The height of the step tree; the view has no scroll view of its own size.
  static const double _graphHeight = 480;

  @override
  void initState() {
    super.initState();
    unawaited(_load());
  }

  Future<void> _load() async {
    try {
      final run = await widget.queries.run(widget.row.runId);
      if (mounted) setState(() => _run = run);
    } on Object catch (error) {
      if (mounted) setState(() => _error = vizProblemCode(error));
    }
  }

  Widget _section(BuildContext context, String title, Widget child) => Padding(
    padding: const EdgeInsets.only(top: DsSpace.s16),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Semantics(
          header: true,
          child: Text(
            title,
            style: dsTextStyle(DsType.label)
                .copyWith(color: context.dsColors.textSecondary),
          ),
        ),
        const SizedBox(height: DsSpace.s4),
        child,
      ],
    ),
  );

  @override
  Widget build(BuildContext context) {
    final texts = VizTexts.of(context);
    final l10n = texts.l10n;
    final colors = context.dsColors;
    final row = widget.row;
    final run = _run;
    final error = _error;
    final options = widget.options;
    final now = options.clock.now();
    final message = run?.triggeringMessageId;
    final openMessage = widget.onOpenMessage;
    final summary = run?.summary ?? row.summary;
    final lastText = run?.lastTextExcerpt;
    final bodyStyle = dsTextStyle(DsType.body)
        .copyWith(color: colors.textPrimary);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Align(
          alignment: AlignmentDirectional.centerStart,
          child: TextButton(
            key: const ValueKey('viz-history-back'),
            style: vizButtonStyle(context),
            onPressed: widget.onBack,
            child: Text(l10n.vizHistoryBack),
          ),
        ),
        const SizedBox(height: DsSpace.s8),
        Row(
          children: [
            Expanded(
              child: Semantics(
                header: true,
                label: VizRunLabelText.full(texts, row.label),
                child: ExcludeSemantics(
                  child: VizRunLabelText(
                    label: row.label,
                    titleStyle: dsTextStyle(DsType.title)
                        .copyWith(color: colors.textPrimary),
                  ),
                ),
              ),
            ),
            const SizedBox(width: DsSpace.s8),
            _StateTag(state: row.state),
          ],
        ),
        const SizedBox(height: DsSpace.s12),
        Wrap(
          spacing: DsSpace.s16,
          runSpacing: DsSpace.s8,
          children: [
            ..._figures(
              texts,
              row,
              now: now,
              timeZone: options.timeZone,
              costsVisible: VizTvScope.costsVisible(context),
            ),
            if (run != null)
              _Figure(
                label: l10n.vizHistorySteps,
                value: texts.format.count(run.steps.length),
              ),
          ],
        ),
        const SizedBox(height: DsSpace.s12),
        Wrap(
          spacing: DsSpace.s8,
          runSpacing: DsSpace.s8,
          children: [
            OutlinedButton(
              key: const ValueKey('viz-history-open-chat'),
              style: vizButtonStyle(context),
              onPressed: () => widget.onOpenThread(row.threadId),
              child: Text(l10n.vizHistoryOpenChat),
            ),
            if (message != null && openMessage != null)
              OutlinedButton(
                key: const ValueKey('viz-history-open-message'),
                style: vizButtonStyle(context),
                onPressed: () => openMessage(row.threadId, message),
                child: Text(l10n.vizHistoryOpenMessage),
              ),
          ],
        ),
        if (summary != null && summary.isNotEmpty)
          _section(
            context,
            l10n.vizHistorySummary,
            Text(summary, style: bodyStyle),
          ),
        if (lastText != null && lastText.isNotEmpty)
          _section(
            context,
            l10n.vizHistoryLastText,
            Text(lastText, style: bodyStyle),
          ),
        _section(context, l10n.vizHistorySteps, switch ((run, error)) {
          (_, final String code) => DsBanner(
            tone: DsTone.danger,
            text: problemText(l10n, code),
          ),
          (final api.Run run, _) => SizedBox(
            height: _graphHeight,
            child: RunGraphView(
              graph: RunGraphModel.fromRun(
                run,
                now,
                agentName: row.label.agent,
              ),
              visibleSteps: options.config.liveboxVisibleSteps,
              announceMinInterval: options.config.liveAnnounceMinInterval,
              clock: options.clock.now,
              touch: options.formFactor == VizFormFactor.mobile,
            ),
          ),
          _ => Semantics(
            label: l10n.runGraphLoading,
            child: const SizedBox(height: DsSpace.s48),
          ),
        }),
      ],
    );
  }
}
