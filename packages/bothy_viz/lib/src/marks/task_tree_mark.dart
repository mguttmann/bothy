// SPDX-License-Identifier: Apache-2.0

/// The agent tree below a run card of the live topology (docs/spec/25-visualization.md 6.1;
/// docs/design/topology-agent-tree.md 6).
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';

import '../model/run_card_model.dart';
import '../model/task_tree_model.dart';
import 'run_card_mark.dart';
import 'viz_texts.dart';

/// The words of a task node for one locale: its name, state and facts.
extension TaskTreeTexts on VizTexts {
  /// The kind word of [kind].
  String taskKind(TaskNodeKind kind) => switch (kind) {
    TaskNodeKind.subagent => l10n.toolChipKindSubagent,
    TaskNodeKind.workflow => l10n.toolChipKindWorkflow,
    TaskNodeKind.backgroundJob => l10n.toolChipKindBackgroundJob,
    TaskNodeKind.unknown => l10n.toolChipKindUnknown,
  };

  /// The name of [node]: its label, else kind and position ("Subagent 2").
  String taskName(TaskNodeModel node) =>
      node.label ??
      l10n.runGraphRowLabel(taskKind(node.kind), '${node.position}');

  /// The state word of [state].
  String taskState(TaskNodeState state) => switch (state) {
    TaskNodeState.running => l10n.liveBoxStateRunning,
    TaskNodeState.completed => l10n.liveBoxStateCompleted,
    TaskNodeState.failed => l10n.liveBoxStateFailed,
    TaskNodeState.cancelled => l10n.liveBoxStateCancelled,
    TaskNodeState.unknown => l10n.vizRunStateUnknown,
  };

  /// The facts of [node] in reading order: kind (when a label names it), state, runtime and the
  /// current tool.
  List<String> taskFacts(TaskNodeModel node) => [
    if (node.label != null) taskKind(node.kind),
    taskState(node.state),
    format.elapsed(node.runtime),
    if (node.tool != null && !node.state.isFinished)
      l10n.vizStepToolCall(node.tool!),
  ];
}

/// The run state role that colors [state].
DsRunState taskDsState(TaskNodeState state) => switch (state) {
  TaskNodeState.running => DsRunState.running,
  TaskNodeState.completed => DsRunState.completed,
  TaskNodeState.failed => DsRunState.failed,
  TaskNodeState.cancelled => DsRunState.cancelled,
  TaskNodeState.unknown => DsRunState.unknown,
};

/// The icon of a node [kind].
DsGlyph taskKindGlyph(TaskNodeKind kind) => switch (kind) {
  TaskNodeKind.subagent => DsIcons.stepSubagent,
  TaskNodeKind.workflow => DsIcons.stepWorkflow,
  TaskNodeKind.backgroundJob => DsIcons.stepBackgroundJob,
  TaskNodeKind.unknown => DsIcons.stepStatus,
};

/// One visible row of the tree: the node, its level and the guide lines to its left.
final class TaskTreeRow {
  /// Creates a row.
  const TaskTreeRow({
    required this.node,
    required this.depth,
    required this.position,
    required this.siblings,
    required this.guides,
    required this.expanded,
  });

  /// The node.
  final TaskNodeModel node;

  /// The level: 1 directly under the run.
  final int depth;

  /// The 1-based position among its siblings and their number.
  final int position, siblings;

  /// Per level above the node (1 to depth - 1): whether the line of that level continues below
  /// this row (the ancestor on that level has later siblings).
  final List<bool> guides;

  /// Whether its children are shown.
  final bool expanded;

  /// Whether it is the last of its siblings (the elbow ends here).
  bool get last => position == siblings;

  /// The rows of [roots] in tree order; the children of a node in [collapsed] are left out.
  static List<TaskTreeRow> of(
    List<TaskNodeModel> roots,
    Set<String> collapsed,
  ) {
    final out = <TaskTreeRow>[];
    void walk(List<TaskNodeModel> nodes, int depth, List<bool> guides) {
      for (var i = 0; i < nodes.length; i++) {
        final n = nodes[i];
        final expanded = !collapsed.contains(n.id);
        out.add(
          TaskTreeRow(
            node: n,
            depth: depth,
            position: i + 1,
            siblings: nodes.length,
            guides: guides,
            expanded: expanded,
          ),
        );
        if (expanded && n.children.isNotEmpty) {
          walk(n.children, depth + 1, [...guides, i < nodes.length - 1]);
        }
      }
    }

    walk(roots, 1, const []);
    return out;
  }
}

/// The agent tree of one run: node cards indented by level, with elbow lines from the run and
/// from each parent to its children (an org chart read top to bottom).
///
/// Each node card shows the kind icon, the name (label, else "Subagent 2"), state as color,
/// icon and word, the runtime and the current tool chip; a finished node is faded. A node
/// with children has a toggle that collapses its branch; the choice stays for the life of the
/// widget, so it survives new frames. A tap on a node reports [threadId] to [onTap].
///
/// The tree is a semantics container "3 agents and jobs started"; each node reads its name,
/// facts and its position in the tree ("level 2, 1 of 2").
class TaskTreeMark extends StatefulWidget {
  /// Creates the tree of [tasks].
  const TaskTreeMark({
    super.key,
    required this.tasks,
    required this.threadId,
    this.truncated = false,
    this.onTap,
  });

  /// The nodes directly under the run.
  final List<TaskNodeModel> tasks;

  /// The thread of the run; a tap on a node reports it.
  final String threadId;

  /// The server left nodes out ("More agents and jobs than shown").
  final bool truncated;

  /// Called with [threadId] on a tap.
  final RunTapCallback? onTap;

  /// The indentation of one level.
  static const double indent = DsSpace.s16;

  @override
  State<TaskTreeMark> createState() => _TaskTreeMarkState();
}

class _TaskTreeMarkState extends State<TaskTreeMark> {
  final Set<String> _collapsed = {};

  void _toggle(String id) => setState(() {
    if (!_collapsed.remove(id)) _collapsed.add(id);
  });

  @override
  Widget build(BuildContext context) {
    final texts = VizTexts.of(context);
    final colors = context.dsColors;
    final rows = TaskTreeRow.of(widget.tasks, _collapsed);
    final count = widget.tasks.fold(0, (n, t) => n + t.count);
    return Semantics(
      container: true,
      label: texts.l10n.vizTaskTree(count),
      explicitChildNodes: true,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          for (final row in rows)
            _NodeRow(
              key: ValueKey('taskTree.${row.node.id}'),
              row: row,
              threadId: widget.threadId,
              onTap: widget.onTap,
              onToggle: () => _toggle(row.node.id),
            ),
          if (widget.truncated)
            Padding(
              padding: const EdgeInsetsDirectional.only(
                start: TaskTreeMark.indent,
                top: DsSpace.s4,
              ),
              child: Text(
                texts.l10n.vizMoreTasksThanShown,
                style: dsTextStyle(DsType.caption)
                    .copyWith(color: colors.textSecondary),
              ),
            ),
        ],
      ),
    );
  }
}

class _NodeRow extends StatelessWidget {
  const _NodeRow({
    super.key,
    required this.row,
    required this.threadId,
    required this.onTap,
    required this.onToggle,
  });

  final TaskTreeRow row;
  final String threadId;
  final RunTapCallback? onTap;
  final VoidCallback onToggle;

  /// The opacity of a finished node.
  static const double finishedOpacity = 0.55;

  /// The space above each node card; the guide lines run through it.
  static const double _gap = DsSpace.s4;

  @override
  Widget build(BuildContext context) {
    final texts = VizTexts.of(context);
    final colors = context.dsColors;
    final node = row.node;
    final stateColor = dsRunStateColor(context, taskDsState(node.state));
    final name = texts.taskName(node);
    final facts = texts.taskFacts(node);
    final hasChildren = node.children.isNotEmpty;
    final callback = onTap;
    final secondary = dsTextStyle(DsType.caption)
        .copyWith(color: colors.textSecondary);
    Widget card = CustomPaint(
      painter: RunCardPainter(
        fill: colors.surfaceRaised,
        border: colors.borderSubtle,
        spine: stateColor,
      ),
      child: Padding(
        padding: const EdgeInsetsDirectional.fromSTEB(
          DsSpace.s8 + DsBorderWidth.spine,
          DsSpace.s4,
          DsSpace.s8,
          DsSpace.s4,
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            DsIcon(
              taskKindGlyph(node.kind),
              size: DsIconSizeRole.sm,
              color: colors.textSecondary,
              decorative: true,
            ),
            const SizedBox(width: DsSpace.s6),
            Flexible(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    name,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: dsTextStyle(DsType.label)
                        .copyWith(color: colors.textPrimary),
                  ),
                  Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      DsIcon(
                        dsRunStateGlyph(taskDsState(node.state)),
                        size: DsIconSizeRole.sm,
                        color: stateColor,
                        decorative: true,
                      ),
                      const SizedBox(width: DsSpace.s4),
                      Flexible(
                        child: Text(
                          [
                            texts.taskState(node.state),
                            texts.format.elapsed(node.runtime),
                          ].join(' · '),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: secondary,
                        ),
                      ),
                    ],
                  ),
                  if (node.tool != null && !node.state.isFinished)
                    Padding(
                      padding: const EdgeInsets.only(top: DsSpace.s2),
                      child: _ToolChip(
                        text: texts.l10n.vizStepToolCall(node.tool!),
                      ),
                    ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
    if (node.state.isFinished) {
      card = Opacity(opacity: finishedOpacity, child: card);
    }
    final semantics = [
      name,
      ...facts,
      texts.l10n.runGraphTreePosition(row.depth, row.position, row.siblings),
    ].join(', ');
    return IntrinsicHeight(
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          SizedBox(
            width: row.depth * TaskTreeMark.indent,
            child: CustomPaint(
              painter: TaskTreeGuidePainter(
                guides: row.guides,
                last: row.last,
                color: colors.vizAxis,
                top: _gap,
              ),
            ),
          ),
          Container(
            width: DsIconSize.sizeSm + DsSpace.s4,
            padding: const EdgeInsets.only(top: _gap),
            child: hasChildren
                ? Semantics(
                    button: true,
                    expanded: row.expanded,
                    label: row.expanded
                        ? texts.l10n.runGraphCollapse
                        : texts.l10n.runGraphExpand,
                    onTap: onToggle,
                    child: GestureDetector(
                      key: ValueKey('taskTree.toggle.${node.id}'),
                      behavior: HitTestBehavior.opaque,
                      onTap: onToggle,
                      child: Center(
                        child: DsIcon(
                          row.expanded
                              ? DsIcons.actionCollapse
                              : DsIcons.actionExpand,
                          size: DsIconSizeRole.sm,
                          color: colors.textSecondary,
                          decorative: true,
                        ),
                      ),
                    ),
                  )
                : null,
          ),
          Flexible(
            child: Padding(
              padding: const EdgeInsets.only(top: _gap),
              child: ConstrainedBox(
                constraints: const BoxConstraints(
                  maxWidth: DsLayout.sessionWidth,
                ),
                child: Semantics(
                  container: true,
                  button: callback != null,
                  label: semantics,
                  onTap: callback == null ? null : () => callback(threadId),
                  child: ExcludeSemantics(
                    child: callback == null
                        ? card
                        : GestureDetector(
                            behavior: HitTestBehavior.opaque,
                            onTap: () => callback(threadId),
                            child: card,
                          ),
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _ToolChip extends StatelessWidget {
  const _ToolChip({required this.text});

  final String text;

  @override
  Widget build(BuildContext context) {
    final colors = context.dsColors;
    return DecoratedBox(
      decoration: BoxDecoration(
        color: colors.surfaceSunken,
        borderRadius: BorderRadius.circular(DsRadius.sm),
        border: Border.all(
          color: colors.borderSubtle,
          width: DsBorderWidth.hairline,
        ),
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(
          horizontal: DsSpace.s6,
          vertical: DsSpace.s2,
        ),
        child: Text(
          text,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: dsTextStyle(DsType.caption)
              .copyWith(color: colors.textPrimary),
        ),
      ),
    );
  }
}

/// Paints the guide lines left of one row: a vertical line for each level whose ancestor has
/// later siblings, and the elbow of the row itself, from the parent down to the middle of the
/// row and across to the node (an org chart's connecting line).
class TaskTreeGuidePainter extends CustomPainter {
  /// Creates the painter.
  const TaskTreeGuidePainter({
    required this.guides,
    required this.last,
    required this.color,
    this.top = 0,
  });

  /// The space above the node card; the elbow meets the middle of the card below it.
  final double top;

  /// Per level above the row: the line continues below it.
  final List<bool> guides;

  /// The row is the last of its siblings: its own line ends at the elbow.
  final bool last;

  /// `color.viz.axis`.
  final Color color;

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = color
      ..strokeWidth = DsBorderWidth.hairline * 2
      ..style = PaintingStyle.stroke;
    const step = TaskTreeMark.indent;
    double x(int level) => (level + 0.5) * step;
    for (var level = 0; level < guides.length; level++) {
      if (guides[level]) {
        canvas.drawLine(
          Offset(x(level), 0),
          Offset(x(level), size.height),
          paint,
        );
      }
    }
    final own = x(guides.length);
    final mid = (size.height + top) / 2;
    canvas
      ..drawLine(Offset(own, 0), Offset(own, last ? mid : size.height), paint)
      ..drawLine(Offset(own, mid), Offset(size.width, mid), paint);
  }

  @override
  bool shouldRepaint(TaskTreeGuidePainter oldDelegate) =>
      oldDelegate.last != last ||
      oldDelegate.color != color ||
      oldDelegate.top != top ||
      oldDelegate.guides.length != guides.length ||
      !_same(oldDelegate.guides, guides);

  static bool _same(List<bool> a, List<bool> b) {
    for (var i = 0; i < a.length; i++) {
      if (a[i] != b[i]) return false;
    }
    return true;
  }
}

/// A run card with its agent tree below it; without tree nodes only the card.
class RunCardTreeMark extends StatelessWidget {
  /// Creates the card of [model] with the tree of [RunCardModel.tasks].
  const RunCardTreeMark({super.key, required this.model, this.onTap});

  /// The card and its tree.
  final RunCardModel model;

  /// Called with [RunCardModel.threadId] on a tap on the card or a node.
  final RunTapCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final card = RunCardMark(model: model, onTap: onTap);
    if (model.tasks.isEmpty && !model.tasksTruncated) return card;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        card,
        TaskTreeMark(
          key: ValueKey('taskTree.run.${model.runId}'),
          tasks: model.tasks,
          threadId: model.threadId,
          truncated: model.tasksTruncated,
          onTap: onTap,
        ),
      ],
    );
  }
}
