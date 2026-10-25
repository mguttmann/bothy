// SPDX-License-Identifier: Apache-2.0

/// The run graph D14: an indented trace view on a shared time axis and its list form
/// (docs/spec/01-product.md 5.1 D14, 8; docs/spec/09-agents.md 5.6; design note
/// run-visualization D-RV-1, D-RV-6, D-RV-12).
library;

import 'dart:async';
import 'dart:math' as math;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter/services.dart';

import '../views/view_scaffold.dart' show VizToggleButton;
import 'run_graph_announcer.dart';
import 'run_graph_model.dart';
import 'run_graph_texts.dart';

/// The two layouts of the trace view (brief M5-14 Design note 1; 01 5.3 M12).
enum RunGraphLayout {
  /// The bar of a row beside its label on the full time axis: the desktop, a phone in
  /// landscape and tablets.
  wide,

  /// The bar of a row in a narrow track below its label: a phone in portrait.
  narrow,
}

/// The layout for an available [width]: [RunGraphLayout.narrow] below the design token
/// `DsLayout.overlayWidth`, the width of an overlay, which no portrait phone reaches and every
/// landscape phone and desktop pane does; [RunGraphLayout.wide] from it on
/// (docs/decisions.d/M5-14.md D-M5-14-1).
///
/// Example: `runGraphLayoutFor(390)` is [RunGraphLayout.narrow].
RunGraphLayout runGraphLayoutFor(double width) => width < DsLayout.overlayWidth
    ? RunGraphLayout.narrow
    : RunGraphLayout.wide;

/// Sends one polite announcement; the default goes to the platform screen reader.
typedef RunGraphAnnounceSink = void Function(
  BuildContext context,
  String message,
);

/// The trace view of [graph]: one row per node in tree order, indented by depth, with state
/// icon and word, duration, current tool, markers, tokens and cost, and to the right a bar per
/// row on a time axis from the run's start to its end or now (a running bar ends in an arrow).
///
/// Rows are built lazily, so a run with `run.max_steps_persisted` steps stays smooth. A node
/// with more than [visibleSteps] (`ui.livebox_visible_steps`) finished children shows the row
/// "n earlier steps" (09 5.6 rule 7). A row with children opens and closes with a tap on its
/// arrow or with the arrow keys (Up and Down move, Right opens, Left closes or goes to the
/// parent; Enter or Space opens "n earlier steps"). The switch "As list" shows the same rows
/// without bars as an outline whose semantics name level, position, state, duration and cost.
///
/// State changes and the end of a node are announced politely, at most once per
/// [announceMinInterval] (`a11y.live_announce_min_interval`), never every patch; the time
/// comes from [clock]. Nothing animates, so the reduced motion context needs no other path.
///
/// [layout] places the bars: beside the labels ([RunGraphLayout.wide]) or below each label on
/// one shared track ([RunGraphLayout.narrow], a phone in portrait, 01 5.3 M12); null picks it
/// from the available width ([runGraphLayoutFor]). Both layouts show the same rows with the
/// same values and semantics; only the place of the bar differs. With [touch] (density
/// `touch`, M12) a tap anywhere on a row with children closes or opens it, else a tap selects
/// the row and only its arrow toggles.
///
/// The view has no scroll view around it of its own size: give it a bounded height.
///
/// ```dart
/// RunGraphView(graph: RunGraphModel.fromRun(run, now, agentName: name),
///     visibleSteps: config.liveboxVisibleSteps,
///     announceMinInterval: config.liveAnnounceMinInterval, clock: clock.now)
/// ```
class RunGraphView extends StatefulWidget {
  /// Creates the view of [graph].
  const RunGraphView({
    super.key,
    required this.graph,
    required this.visibleSteps,
    required this.announceMinInterval,
    required this.clock,
    this.initiallyAsList = false,
    this.onAnnounce,
    this.layout,
    this.touch = false,
  });

  /// The derived graph; the host derives it again on every applied patch.
  final RunGraphModel graph;

  /// `ui.livebox_visible_steps`.
  final int visibleSteps;

  /// `a11y.live_announce_min_interval`.
  final Duration announceMinInterval;

  /// The current time of the announcer's throttle.
  final DateTime Function() clock;

  /// Whether the list form shows first.
  final bool initiallyAsList;

  /// Receives the announcements instead of the platform screen reader (tests).
  final RunGraphAnnounceSink? onAnnounce;

  /// Where the bars stand; null picks the layout from the available width.
  final RunGraphLayout? layout;

  /// Whether a tap on a row with children toggles it (density `touch`).
  final bool touch;

  @override
  State<RunGraphView> createState() => RunGraphViewState();
}

/// The state of [RunGraphView]: the list switch, the rows the person opened or closed and the
/// selected row of the keyboard.
class RunGraphViewState extends State<RunGraphView> {
  late bool _asList = widget.initiallyAsList;
  final Set<String> _expanded = {};
  final Set<String> _collapsed = {};
  final FocusNode _focus = FocusNode(debugLabel: 'runGraph');
  late RunGraphAnnouncer _announcer;
  Timer? _flush;
  int _selected = 0;
  List<RunGraphRow> _rows = const [];

  /// Whether the list form shows.
  bool get asList => _asList;

  /// The rows shown now.
  List<RunGraphRow> get rows => _rows;

  @override
  void initState() {
    super.initState();
    // The selection shows only while the graph holds the keyboard focus.
    _focus.addListener(() {
      if (mounted) setState(() {});
    });
    _announcer = _newAnnouncer();
    _announcer.observe(widget.graph);
  }

  RunGraphAnnouncer _newAnnouncer() => RunGraphAnnouncer(
    minInterval: widget.announceMinInterval,
    clock: widget.clock,
    sink: _announce,
    text: (node) {
      final texts = RunGraphTexts.of(context);
      return texts.l10n.runGraphAnnounce(
        texts.label(node),
        texts.state(node.state),
      );
    },
  );

  void _announce(String message) {
    final sink = widget.onAnnounce;
    if (sink != null) {
      sink(context, message);
      return;
    }
    unawaited(
      SemanticsService.sendAnnouncement(
        View.of(context),
        message,
        Directionality.of(context),
        assertiveness: Assertiveness.polite,
      ),
    );
  }

  @override
  void didUpdateWidget(RunGraphView oldWidget) {
    super.didUpdateWidget(oldWidget);
    final old = oldWidget;
    if (old.announceMinInterval != widget.announceMinInterval) {
      _announcer = _newAnnouncer()..observe(old.graph);
    }
    if (!identical(old.graph, widget.graph)) {
      _announcer.observe(widget.graph);
      _scheduleFlush();
    }
  }

  void _scheduleFlush() {
    _flush?.cancel();
    final due = _announcer.pendingDue;
    if (due == null) return;
    var wait = due.difference(widget.clock());
    if (wait.isNegative) wait = Duration.zero;
    _flush = Timer(wait, () {
      if (!mounted) return;
      _announcer.flush();
      _scheduleFlush();
    });
  }

  @override
  void dispose() {
    _flush?.cancel();
    _focus.dispose();
    super.dispose();
  }

  void _toggle(RunGraphNodeRow row) => setState(() {
    final id = row.node.id;
    if (row.expanded) {
      _collapsed.add(id);
    } else {
      _collapsed.remove(id);
    }
  });

  void _showEarlier(RunGraphEarlierRow row) =>
      setState(() => _expanded.add(row.parentId));

  KeyEventResult _onKey(FocusNode node, KeyEvent event) {
    if (event is! KeyDownEvent && event is! KeyRepeatEvent) {
      return KeyEventResult.ignored;
    }
    if (_rows.isEmpty) return KeyEventResult.ignored;
    final key = event.logicalKey;
    final row = _rows[_selected.clamp(0, _rows.length - 1)];
    if (key == LogicalKeyboardKey.arrowDown) {
      setState(() => _selected = math.min(_selected + 1, _rows.length - 1));
    } else if (key == LogicalKeyboardKey.arrowUp) {
      setState(() => _selected = math.max(_selected - 1, 0));
    } else if (key == LogicalKeyboardKey.arrowRight) {
      if (row is RunGraphNodeRow && row.hasChildren && !row.expanded) {
        _toggle(row);
      } else if (row is RunGraphEarlierRow) {
        _showEarlier(row);
      }
    } else if (key == LogicalKeyboardKey.arrowLeft) {
      if (row is RunGraphNodeRow && row.hasChildren && row.expanded) {
        _toggle(row);
      } else {
        // To the parent: the nearest row above with a smaller depth.
        for (var i = _selected - 1; i >= 0; i--) {
          if (_rows[i].depth < row.depth) {
            setState(() => _selected = i);
            break;
          }
        }
      }
    } else if (key == LogicalKeyboardKey.enter ||
        key == LogicalKeyboardKey.space) {
      switch (row) {
        case RunGraphEarlierRow():
          _showEarlier(row);
        case RunGraphNodeRow(hasChildren: true):
          _toggle(row);
        case RunGraphNodeRow():
          return KeyEventResult.ignored;
      }
    } else {
      return KeyEventResult.ignored;
    }
    return KeyEventResult.handled;
  }

  @override
  Widget build(BuildContext context) {
    final texts = RunGraphTexts.of(context);
    final graph = widget.graph;
    _rows = graph.rows(
      visibleSteps: widget.visibleSteps,
      expanded: _expanded,
      collapsed: _collapsed,
    );
    if (_selected >= _rows.length) _selected = _rows.length - 1;
    return LayoutBuilder(
      builder: (context, constraints) {
        final width = constraints.maxWidth.isFinite
            ? constraints.maxWidth
            : DsLayout.messageMaxWidth;
        final narrow =
            (widget.layout ?? runGraphLayoutFor(width)) ==
            RunGraphLayout.narrow;
        // The labels take a share of the width, never less than a column and never so much
        // that the time axis disappears. In the narrow layout the bars share one track below
        // the labels that starts after the arrow of the top level, so the times of all rows
        // stay comparable; the label then takes the whole row.
        final double labelWidth = _asList || narrow
            ? width
            : (width * 0.45).clamp(
                math.min(DsLayout.columnMin, width / 2),
                DsLayout.listWidth + DsSpace.s64,
              );
        const trackStart = DsIconSize.sizeSm + DsSpace.s4;
        final trackWidth = narrow
            ? math.max(0.0, width - 2 * DsSpace.s4 - trackStart)
            : math.max(0.0, width - labelWidth - DsSpace.s8);
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Padding(
              padding: const EdgeInsets.symmetric(vertical: DsSpace.s4),
              child: Align(
                alignment: AlignmentDirectional.centerEnd,
                child: VizToggleButton(
                  key: const ValueKey('runGraph.asList'),
                  label: texts.l10n.runGraphAsList,
                  selected: _asList,
                  onPressed: () => setState(() => _asList = !_asList),
                ),
              ),
            ),
            if (!_asList)
              _Axis(
                graph: graph,
                // The axis starts where the bars start: after the label, or in the narrow
                // layout after the row's padding and the arrow.
                labelWidth: narrow ? trackStart - DsSpace.s4 : labelWidth,
                trackWidth: trackWidth,
              ),
            Expanded(
              child: Focus(
                focusNode: _focus,
                onKeyEvent: _onKey,
                child: ListView.builder(
                  key: ValueKey(
                    _asList ? 'runGraph.outline' : 'runGraph.trace',
                  ),
                  itemCount: _rows.length,
                  itemBuilder: (context, i) => switch (_rows[i]) {
                    final RunGraphNodeRow row => _NodeRow(
                      key: ValueKey('runGraph.row.${row.node.id}'),
                      graph: graph,
                      row: row,
                      asList: _asList,
                      narrow: narrow,
                      touch: widget.touch,
                      labelWidth: labelWidth,
                      trackStart: trackStart,
                      selected: i == _selected && _focus.hasFocus,
                      onToggle: () {
                        _focus.requestFocus();
                        setState(() => _selected = i);
                        _toggle(row);
                      },
                      onSelect: () {
                        _focus.requestFocus();
                        setState(() => _selected = i);
                      },
                    ),
                    final RunGraphEarlierRow row => _EarlierRow(
                      key: ValueKey('runGraph.earlier.${row.parentId}'),
                      row: row,
                      selected: i == _selected && _focus.hasFocus,
                      onTap: () {
                        _focus.requestFocus();
                        setState(() => _selected = i);
                        _showEarlier(row);
                      },
                    ),
                  },
                ),
              ),
            ),
          ],
        );
      },
    );
  }
}

/// The labels of the time axis over the bars.
class _Axis extends StatelessWidget {
  const _Axis({
    required this.graph,
    required this.labelWidth,
    required this.trackWidth,
  });

  final RunGraphModel graph;
  final double labelWidth;
  final double trackWidth;

  @override
  Widget build(BuildContext context) {
    final texts = RunGraphTexts.of(context);
    final f = texts.viz.format;
    final span = graph.axisEnd.difference(graph.axisStart);
    final ticks = runGraphAxisTicks(span, trackWidth);
    final style = dsTextStyle(DsType.codeSm)
        .copyWith(color: context.dsColors.textSecondary);
    return Semantics(
      label: texts.l10n.runGraphTimeAxis(
        f.elapsed(Duration.zero),
        f.elapsed(span),
      ),
      child: ExcludeSemantics(
        child: Padding(
          padding: const EdgeInsets.only(bottom: DsSpace.s4),
          child: Row(
            children: [
              SizedBox(width: labelWidth + DsSpace.s8),
              SizedBox(
                width: trackWidth,
                height: dsTextStyle(DsType.codeSm).fontSize! * 2,
                child: Stack(
                  clipBehavior: Clip.none,
                  children: [
                    for (final t in ticks)
                      Positioned(
                        left: span.inMilliseconds <= 0
                            ? 0
                            : trackWidth *
                                  t.inMilliseconds /
                                  span.inMilliseconds,
                        top: 0,
                        child: Text(f.elapsed(t), style: style),
                      ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// The steps of the time axis: the smallest step of a fixed list of round durations that
/// leaves at least one label width ([DsSpace.s64] plus [DsSpace.s16]) between two labels on a
/// track of [width]; the labels read as run times (`0:00`, `1:00`, `1:00:00`), so seconds,
/// minutes and hours follow the length of the run (docs/decisions.d/M4-39.md D-M4-39-1).
///
/// Returns the ticks from zero up to [span]; a span of zero gives only zero.
List<Duration> runGraphAxisTicks(Duration span, double width) {
  if (span <= Duration.zero || width <= 0) return const [Duration.zero];
  const steps = [
    Duration(seconds: 1),
    Duration(seconds: 5),
    Duration(seconds: 10),
    Duration(seconds: 15),
    Duration(seconds: 30),
    Duration(minutes: 1),
    Duration(minutes: 2),
    Duration(minutes: 5),
    Duration(minutes: 10),
    Duration(minutes: 15),
    Duration(minutes: 30),
    Duration(hours: 1),
    Duration(hours: 2),
    Duration(hours: 3),
    Duration(hours: 6),
    Duration(hours: 12),
    Duration(hours: 24),
  ];
  const minGap = DsSpace.s64 + DsSpace.s16;
  final fit = math.max(1, (width / minGap).floor());
  var step = steps.last;
  for (final s in steps) {
    if (span.inMilliseconds / s.inMilliseconds <= fit) {
      step = s;
      break;
    }
  }
  return [for (var t = Duration.zero; t <= span; t += step) t];
}

class _NodeRow extends StatelessWidget {
  const _NodeRow({
    super.key,
    required this.graph,
    required this.row,
    required this.asList,
    required this.narrow,
    required this.touch,
    required this.labelWidth,
    required this.trackStart,
    required this.selected,
    required this.onToggle,
    required this.onSelect,
  });

  final RunGraphModel graph;
  final RunGraphNodeRow row;
  final bool asList;
  final bool narrow;
  final bool touch;
  final double labelWidth;
  final double trackStart;
  final bool selected;
  final VoidCallback onToggle;
  final VoidCallback onSelect;

  @override
  Widget build(BuildContext context) {
    final texts = RunGraphTexts.of(context);
    final colors = context.dsColors;
    final node = row.node;
    final label = texts.label(node);
    final facts = texts.facts(node, now: graph.axisEnd);
    final stateColor = runGraphStateColor(context, node.state);
    final isSummary = node.kind == RunGraphNodeKind.summary;
    final semantics = [
      label,
      ...facts,
      texts.l10n.runGraphTreePosition(
        row.depth + 1,
        row.position,
        row.siblings,
      ),
    ].join(', ');
    final indent = row.depth * DsSpace.s16;
    final arrowSize = DsIconSize.sizeSm;
    final labelPart = Padding(
      padding: EdgeInsetsDirectional.only(start: indent),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (row.hasChildren)
            GestureDetector(
              key: ValueKey('runGraph.toggle.${node.id}'),
              behavior: HitTestBehavior.opaque,
              onTap: onToggle,
              child: DsIcon(
                row.expanded ? DsIcons.actionCollapse : DsIcons.actionExpand,
                size: DsIconSizeRole.sm,
                color: colors.textSecondary,
                decorative: true,
              ),
            )
          else
            SizedBox(width: arrowSize),
          const SizedBox(width: DsSpace.s4),
          DsIcon(
            isSummary
                ? runGraphKindGlyph(node.kind)
                : runGraphStateGlyph(node.state),
            size: DsIconSizeRole.sm,
            color: isSummary ? colors.textSecondary : stateColor,
            decorative: true,
          ),
          const SizedBox(width: DsSpace.s6),
          Expanded(
            child: Column(
              key: ValueKey('runGraph.label.${node.id}'),
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  label,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: dsTextStyle(
                    node.kind == RunGraphNodeKind.run
                        ? DsType.bodyStrong
                        : DsType.bodySm,
                  ).copyWith(color: colors.textPrimary),
                ),
                if (facts.isNotEmpty)
                  Text(
                    facts.join(' · '),
                    maxLines: asList ? null : 1,
                    overflow: asList ? null : TextOverflow.ellipsis,
                    style: dsTextStyle(DsType.caption)
                        .copyWith(color: colors.textSecondary),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
    final bar = SizedBox(
      key: ValueKey('runGraph.bar.${node.id}'),
      height: DsIconSize.sizeSm,
      child: CustomPaint(
        painter: RunGraphBarPainter(
          from: graph.axisStart,
          to: graph.axisEnd,
          start: node.start,
          end: node.end,
          running: node.isRunning,
          color: stateColor,
          grid: colors.vizGrid,
        ),
      ),
    );
    return Semantics(
      container: true,
      label: semantics,
      selected: selected,
      expanded: row.hasChildren ? row.expanded : null,
      onTap: row.hasChildren ? onToggle : null,
      onTapHint: row.hasChildren
          ? (row.expanded
                ? texts.l10n.runGraphCollapse
                : texts.l10n.runGraphExpand)
          : null,
      child: ExcludeSemantics(
        child: GestureDetector(
          behavior: HitTestBehavior.opaque,
          onTap: touch && row.hasChildren ? onToggle : onSelect,
          child: Container(
            constraints: BoxConstraints(minHeight: context.dsDensity.targetMin),
            padding: const EdgeInsets.symmetric(
              horizontal: DsSpace.s4,
              vertical: DsSpace.s2,
            ),
            decoration: BoxDecoration(
              color: selected ? colors.surfaceSelected : null,
              border: Border(
                bottom: BorderSide(
                  color: colors.borderSubtle,
                  width: DsBorderWidth.hairline,
                ),
              ),
            ),
            child: asList
                ? labelPart
                : narrow
                ? Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      labelPart,
                      if (!isSummary)
                        Padding(
                          padding: EdgeInsetsDirectional.only(
                            start: trackStart,
                            top: DsSpace.s2,
                          ),
                          child: bar,
                        ),
                    ],
                  )
                : Row(
                    children: [
                      SizedBox(width: labelWidth, child: labelPart),
                      const SizedBox(width: DsSpace.s8),
                      Expanded(
                        child: isSummary
                            ? const SizedBox(height: DsIconSize.sizeSm)
                            : bar,
                      ),
                    ],
                  ),
          ),
        ),
      ),
    );
  }
}

class _EarlierRow extends StatelessWidget {
  const _EarlierRow({
    super.key,
    required this.row,
    required this.selected,
    required this.onTap,
  });

  final RunGraphEarlierRow row;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final texts = RunGraphTexts.of(context);
    final text = texts.l10n.liveBoxEarlierSteps(row.count);
    return Container(
      color: selected ? context.dsColors.surfaceSelected : null,
      padding: EdgeInsetsDirectional.only(
        start: row.depth * DsSpace.s16 + DsIconSize.sizeSm + DsSpace.s8,
      ),
      alignment: AlignmentDirectional.centerStart,
      child: DsTappable(
        onTap: onTap,
        label: text,
        selected: selected,
        excludeChildSemantics: true,
        child: Align(
          alignment: AlignmentDirectional.centerStart,
          widthFactor: 1,
          child: Text(
            text,
            style: dsTextStyle(DsType.label)
                .copyWith(color: context.dsAccent.textLink),
          ),
        ),
      ),
    );
  }
}

/// Paints the bar of one row on the shared time axis: from [start] to [end], or to the end
/// of the axis with an arrow while the node runs.
class RunGraphBarPainter extends CustomPainter {
  /// Creates the painter.
  const RunGraphBarPainter({
    required this.from,
    required this.to,
    required this.start,
    required this.end,
    required this.running,
    required this.color,
    required this.grid,
  });

  /// Start of the axis.
  final DateTime from;

  /// End of the axis.
  final DateTime to;

  /// Start of the bar.
  final DateTime start;

  /// End of the bar; null while the node runs.
  final DateTime? end;

  /// Whether the bar ends in an arrow.
  final bool running;

  /// The state role.
  final Color color;

  /// `color.viz.grid`, the track.
  final Color grid;

  double _fraction(DateTime t) {
    final span = to.difference(from).inMilliseconds;
    if (span <= 0) return 0;
    return (t.difference(from).inMilliseconds / span).clamp(0, 1).toDouble();
  }

  @override
  void paint(Canvas canvas, Size size) {
    canvas.drawLine(
      Offset(0, size.height / 2),
      Offset(size.width, size.height / 2),
      Paint()
        ..color = grid
        ..strokeWidth = DsBorderWidth.hairline,
    );
    final left = size.width * _fraction(start);
    var right = size.width * _fraction(end ?? to);
    final paint = Paint()..color = color;
    final head = size.height / 2;
    if (running && end == null) right = math.max(left, right - head);
    canvas.drawRRect(
      RRect.fromLTRBR(
        left,
        size.height / 4,
        right <= left ? left + DsBorderWidth.spine : right,
        size.height * 3 / 4,
        const Radius.circular(DsRadius.xs),
      ),
      paint,
    );
    if (running && end == null) {
      final tip = math.max(left, right) + head;
      canvas.drawPath(
        Path()
          ..moveTo(math.max(left, right), 0)
          ..lineTo(tip, size.height / 2)
          ..lineTo(math.max(left, right), size.height)
          ..close(),
        paint,
      );
    }
  }

  @override
  bool shouldRepaint(RunGraphBarPainter old) =>
      old.from != from ||
      old.to != to ||
      old.start != start ||
      old.end != end ||
      old.running != running ||
      old.color != color ||
      old.grid != grid;
}
