// SPDX-License-Identifier: Apache-2.0

/// The visible steps of "New agent" (docs/spec/01-product.md 6.20 item 4; brief A1-08 Design
/// notes 3 and 6): each step with its state in words and a mark, one sentence for the cause of
/// a failed step, and an announcement of the step list to screen readers at most once per
/// `a11y.live_announce_min_interval`.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:l10n/l10n.dart';

import 'new_agent_flow.dart';
import 'new_agent_texts.dart';

/// The steps of [flow].
///
/// Example: `NewAgentSteps(flow: flow, announceMinInterval: config.liveAnnounceMinInterval)`.
class NewAgentSteps extends StatefulWidget {
  /// Creates the list; [announceMinInterval] is `a11y.live_announce_min_interval`, [clock] the
  /// time of the announcer.
  const NewAgentSteps({
    super.key,
    required this.flow,
    required this.announceMinInterval,
    this.clock = DateTime.now,
  });

  /// The flow.
  final NewAgentFlow flow;

  /// `a11y.live_announce_min_interval`.
  final Duration announceMinInterval;

  /// The current time.
  final DateTime Function() clock;

  @override
  State<NewAgentSteps> createState() => _NewAgentStepsState();
}

class _NewAgentStepsState extends State<NewAgentSteps> {
  LiveAnnouncer? _announcer;
  Timer? _flush;

  @override
  void dispose() {
    _flush?.cancel();
    super.dispose();
  }

  static String _stateWord(NewAgentStepState s) => switch (s) {
    NewAgentStepState.waiting => 'waiting',
    NewAgentStepState.running => 'running',
    NewAgentStepState.done => 'finished',
    NewAgentStepState.failed => 'failed',
  };

  void _observe(String summary) {
    final view = View.of(context);
    final direction = Directionality.of(context);
    final announcer = _announcer ??= LiveAnnouncer(
      minInterval: widget.announceMinInterval,
      clock: widget.clock,
      sink: (message) => SemanticsService.sendAnnouncement(
        view,
        message,
        direction,
        assertiveness: Assertiveness.polite,
      ),
    );
    announcer.observe(state: summary, questionId: null);
    _scheduleFlush(announcer);
  }

  void _scheduleFlush(LiveAnnouncer announcer) {
    _flush?.cancel();
    final due = announcer.pendingDue;
    if (due == null) return;
    var wait = due.difference(widget.clock());
    if (wait < const Duration(milliseconds: 1)) {
      wait = const Duration(milliseconds: 1);
    }
    _flush = Timer(wait, () {
      if (!mounted) return;
      announcer.flush();
      _scheduleFlush(announcer);
    });
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final flow = widget.flow;
    final colors = context.dsColors;
    final lines = [
      for (final s in flow.steps)
        (step: s, label: newAgentStepLabel(l, flow, s), state: flow.stateOf(s)),
    ];
    // The step that changes is the news: the running one, else the failed one, else the last
    // finished one.
    final current =
        lines.where((x) => x.state == NewAgentStepState.running).firstOrNull ??
        lines.where((x) => x.state == NewAgentStepState.failed).firstOrNull ??
        lines.lastWhere(
          (x) => x.state == NewAgentStepState.done,
          orElse: () => lines.first,
        );
    _observe(l.newAgentStepState(current.label, _stateWord(current.state)));
    final failure = flow.failure;
    final failed = flow.failedStep;
    return Column(
      key: const ValueKey('newAgent.steps'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        for (final x in lines)
          _StepLine(
            key: ValueKey('newAgent.step.${x.step.name}'),
            label: x.label,
            state: x.state,
            stateWord: _stateWord(x.state),
          ),
        if (failure != null && failed != null)
          Padding(
            padding: const EdgeInsets.only(top: DsSpace.s8),
            child: Text(
              newAgentFailureText(l, failed, failure, mode: flow.mode),
              key: const ValueKey('newAgent.failure'),
              style: dsTextStyle(DsType.body).copyWith(color: colors.dangerFg),
            ),
          ),
      ],
    );
  }
}

/// One step: a mark and the label, read as one sentence with its state.
class _StepLine extends StatelessWidget {
  const _StepLine({
    super.key,
    required this.label,
    required this.state,
    required this.stateWord,
  });

  final String label;
  final NewAgentStepState state;
  final String stateWord;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final mark = switch (state) {
      NewAgentStepState.running => const SizedBox.square(
        dimension: DsSpace.s16,
        child: CircularProgressIndicator(strokeWidth: 2),
      ),
      NewAgentStepState.done => Icon(
        Icons.check,
        size: DsSpace.s16,
        color: colors.textPrimary,
      ),
      NewAgentStepState.failed => Icon(
        Icons.close,
        size: DsSpace.s16,
        color: colors.dangerFg,
      ),
      NewAgentStepState.waiting => Icon(
        Icons.circle_outlined,
        size: DsSpace.s16,
        color: colors.textSecondary,
      ),
    };
    return MergeSemantics(
      child: Semantics(
        label: l.newAgentStepState(label, stateWord),
        excludeSemantics: true,
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: DsSpace.s4),
          child: Row(
            children: [
              mark,
              const SizedBox(width: DsSpace.s8),
              Expanded(child: Text(label, style: dsTextStyle(DsType.body))),
            ],
          ),
        ),
      ),
    );
  }
}
