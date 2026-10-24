// SPDX-License-Identifier: Apache-2.0

/// The live box of a run: in progress, collapsed and the mobile line (components.md section 2).
library;

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';

import '../generated/design_tokens.g.dart';
import '../icons/ds_icon.dart';
import '../theme/ds_theme.dart';
import 'ds_banner.dart';
import 'ds_billing_badge.dart';
import 'ds_common.dart';
import 'ds_status_line.dart';
import 'ds_tool_chip.dart';
import 'live_announcer.dart';
import 'view_models.dart';

enum _Variant { full, collapsed, mobile }

/// The display of a run (`runs`, run document `run_document.v1`).
///
/// The default constructor draws the anatomy of components.md 2.1: header (state icon and
/// word, run time, context, tokens, cost in `type.code-sm`, backend policy badge, optional
/// collapse arrow), the reason of a failed or canceled run (translated, an unknown code in
/// `type.code-sm`), the status line, the last [visibleSteps] steps (`ui.livebox_visible_steps`)
/// with "n earlier steps" above them, the last text as a quote, the stall banner while
/// [DsRunData.stalled], and the footer with "Stop" and the follow-up note or, for
/// `waiting_input`, the [question]. The spine on the left has the color of the state.
///
/// Screen readers get a container labeled "Run of {Agent}: {State}, for {Duration}, context
/// {n} percent"; state changes and new questions are announced politely through a
/// [LiveAnnouncer], at most once per [announceMinInterval] (`a11y.live_announce_min_interval`);
/// patches to steps and numbers never.
///
/// [DsLiveBox.collapsed] is the line of a finished run (2.2) and [DsLiveBox.mobile] the compact
/// line under the navigation bar (2.3).
///
/// ```dart
/// DsLiveBox(run, visibleSteps: config.liveboxVisibleSteps,
///     announceMinInterval: config.liveAnnounceMinInterval, onStop: stop)
/// ```
class DsLiveBox extends StatefulWidget {
  /// The live box of an active or expanded run.
  const DsLiveBox(
    this.run, {
    super.key,
    required int this.visibleSteps,
    required Duration this.announceMinInterval,
    this.onStop,
    this.onCollapse,
    this.question,
    this.announcer,
  }) : _variant = _Variant.full,
       onExpand = null,
       onOpen = null;

  /// The collapsed line of a finished run: icon, "Ran {Duration}", tools, subagents and the
  /// cost with "Estimate" while it is an estimate; [onExpand] loads the document (`run.read`).
  ///
  /// [announceMinInterval] (`a11y.live_announce_min_interval`) keeps the announcements going
  /// when the box collapses at the end of the run (01 6.2), so the final state is announced.
  const DsLiveBox.collapsed(
    this.run, {
    super.key,
    required Duration this.announceMinInterval,
    this.onExpand,
    this.announcer,
  }) : _variant = _Variant.collapsed,
       visibleSteps = null,
       onStop = null,
       onCollapse = null,
       onOpen = null,
       question = null;

  /// The mobile line: state icon, state and run time, the status line in one line and the
  /// stop button with the hit area `density.target-min`; [onOpen] opens the sheet.
  ///
  /// [announceMinInterval] (`a11y.live_announce_min_interval`): the mobile line announces
  /// state changes and new questions like the full box.
  const DsLiveBox.mobile(
    this.run, {
    super.key,
    required Duration this.announceMinInterval,
    this.onOpen,
    this.onStop,
    this.announcer,
  }) : _variant = _Variant.mobile,
       visibleSteps = null,
       onCollapse = null,
       onExpand = null,
       question = null;

  /// The run.
  final DsRunData run;

  /// `ui.livebox_visible_steps`: steps shown before "n earlier steps".
  final int? visibleSteps;

  /// `a11y.live_announce_min_interval`.
  final Duration? announceMinInterval;

  /// Stops the run, or null to hide the button (final states).
  final VoidCallback? onStop;

  /// Collapses the box, or null to hide the arrow.
  final VoidCallback? onCollapse;

  /// Expands the collapsed line.
  final VoidCallback? onExpand;

  /// Opens the full box from the mobile line.
  final VoidCallback? onOpen;

  /// The form or buttons of the open question of a `waiting_input` run.
  final Widget? question;

  /// The announcer to observe instead of one of the box's own; the caller keeps it across
  /// boxes of the same run (a collapse, the end of the run), so the throttle and a merged
  /// announcement survive the box. Its clock and sink are the caller's.
  final LiveAnnouncer? announcer;

  final _Variant _variant;

  @override
  State<DsLiveBox> createState() => DsLiveBoxState();
}

/// State of [DsLiveBox]: the earlier-steps toggle and the announcer.
class DsLiveBoxState extends State<DsLiveBox> {
  bool _allSteps = false;
  LiveAnnouncer? _announcer;
  Timer? _flush;

  /// The spine color of the current state.
  @visibleForTesting
  Color get spineColor => dsRunStateColor(context, widget.run.state);

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    _observe();
  }

  @override
  void didUpdateWidget(DsLiveBox oldWidget) {
    super.didUpdateWidget(oldWidget);
    _observe();
  }

  @override
  void dispose() {
    _flush?.cancel();
    super.dispose();
  }

  void _observe() {
    final interval = widget.announceMinInterval;
    if (interval == null) return;
    final l = dsL10n(context);
    final view = View.of(context);
    final direction = Directionality.of(context);
    final announcer =
        widget.announcer ??
        (_announcer ??= LiveAnnouncer(
          minInterval: interval,
          clock: DateTime.now,
          sink: (message) => SemanticsService.sendAnnouncement(
            view,
            message,
            direction,
            assertiveness: Assertiveness.polite,
          ),
        ));
    final run = widget.run;
    announcer.observe(
      state: l.liveBoxAnnounceState(run.agentName, dsRunStateWord(l, run)),
      questionId: run.questionId,
      question: l.liveBoxAnnounceQuestion(run.agentName),
    );
    _scheduleFlush(announcer);
  }

  /// Arms the timer of a waiting announcement. A timer may fire a little early (it rounds to
  /// whole milliseconds), in which case `flush` sends nothing; the timer is then armed again,
  /// so a merged announcement is never lost.
  void _scheduleFlush(LiveAnnouncer announcer) {
    _flush?.cancel();
    final due = announcer.pendingDue;
    if (due == null) return;
    var wait = due.difference(DateTime.now());
    // At least one millisecond, so an early timer never spins on the same instant.
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
    return switch (widget._variant) {
      _Variant.full => _full(context),
      _Variant.collapsed => _collapsed(context),
      _Variant.mobile => _mobile(context),
    };
  }

  TextStyle _mono(BuildContext context) =>
      dsTextStyle(DsType.codeSm)
          .copyWith(color: context.dsColors.textSecondary);

  Widget _stateIcon(BuildContext context) => DsIcon(
    dsRunStateGlyph(widget.run.state),
    size: DsIconSizeRole.sm,
    color: spineColor,
    decorative: true,
  );

  String _label(BuildContext context) {
    final l = dsL10n(context);
    final run = widget.run;
    return l.liveBoxSemanticsLabel(
      run.agentName,
      dsRunStateWord(l, run),
      dsFormat(context).elapsed(run.elapsed),
      (run.contextPercent ?? 0).round(),
    );
  }

  Widget _stopButton(BuildContext context, {required bool compact}) {
    final l = dsL10n(context);
    final colors = context.dsColors;
    return DsTappable(
      onTap: widget.onStop,
      label: l.liveBoxStopRun(widget.run.agentName),
      excludeChildSemantics: true,
      radius: DsRadius.md,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: DsSpace.s8),
        decoration: BoxDecoration(
          border: Border.all(
            color: colors.dangerBorder,
            width: DsBorderWidth.hairline,
          ),
          borderRadius: BorderRadius.circular(DsRadius.md),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            DsIcon(
              DsIcons.actionStop,
              size: DsIconSizeRole.sm,
              color: colors.dangerFg,
              decorative: true,
            ),
            if (!compact) ...[
              const SizedBox(width: DsSpace.s4),
              Text(
                l.liveBoxStop,
                style: dsTextStyle(DsType.label)
                    .copyWith(color: colors.dangerFg),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _full(BuildContext context) {
    final l = dsL10n(context);
    final f = dsFormat(context);
    final colors = context.dsColors;
    final run = widget.run;
    final mono = _mono(context);
    final visible = widget.visibleSteps!;
    final steps = run.steps;
    final hidden = _allSteps || steps.length <= visible
        ? 0
        : steps.length - visible;
    final shown = steps.sublist(hidden);

    final header = Wrap(
      spacing: DsSpace.s8,
      runSpacing: DsSpace.s4,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            _stateIcon(context),
            const SizedBox(width: DsSpace.s6),
            Text(
              dsRunStateWord(l, run),
              style: dsTextStyle(DsType.label)
                  .copyWith(color: colors.textPrimary),
            ),
          ],
        ),
        Text(f.elapsed(run.elapsed), style: mono),
        if (run.contextPercent != null)
          Text(l.liveBoxContext(f.percent(run.contextPercent!)), style: mono),
        if (run.tokens != null)
          Text(l.liveBoxTokens(f.count(run.tokens!)), style: mono),
        if (run.cost != null) DsCostText(run.cost!),
        if (run.billingMode != null)
          DsBillingBadge(
            mode: run.billingMode!,
            holderName: run.billingHolderName,
          ),
        if (widget.onCollapse != null)
          DsTappable(
            onTap: widget.onCollapse,
            label: l.liveBoxCollapse,
            excludeChildSemantics: true,
            child: Center(
              widthFactor: 1,
              heightFactor: 1,
              child: DsIcon(
                DsIcons.actionExpand,
                size: DsIconSizeRole.sm,
                color: colors.textSecondary,
                decorative: true,
              ),
            ),
          ),
      ],
    );

    Widget? reason;
    if (run.state == DsRunState.failed || run.state == DsRunState.cancelled) {
      final code = run.stateReason;
      if (code != null) {
        final text = dsReasonText(l, code, name: run.cancelledByName);
        reason = text != null
            ? Text(
                text,
                style: dsTextStyle(DsType.bodySm)
                    .copyWith(color: colors.textPrimary),
              )
            : Text(
                code,
                style: dsTextStyle(DsType.codeSm)
                    .copyWith(color: colors.textPrimary),
              );
      }
    }

    final children = <Widget>[
      header,
      ?reason,
      if ((run.statusLine ?? '').trim().isNotEmpty)
        DsStatusLine(run.statusLine),
      if (hidden > 0)
        Align(
          alignment: AlignmentDirectional.centerStart,
          child: DsTappable(
            onTap: () => setState(() => _allSteps = true),
            label: l.liveBoxEarlierSteps(hidden),
            excludeChildSemantics: true,
            child: Align(
              alignment: AlignmentDirectional.centerStart,
              widthFactor: 1,
              child: Text(
                l.liveBoxEarlierSteps(hidden),
                style: dsTextStyle(DsType.label)
                    .copyWith(color: context.dsAccent.textLink),
              ),
            ),
          ),
        ),
      for (final step in shown) DsToolChip(step, key: ValueKey(step.id)),
      if ((run.lastTextExcerpt ?? '').isNotEmpty)
        Container(
          padding: const EdgeInsets.only(left: DsSpace.s8),
          decoration: BoxDecoration(
            border: Border(
              left: BorderSide(
                color: colors.borderDefault,
                width: DsBorderWidth.spine,
              ),
            ),
          ),
          child: Text(
            run.lastTextExcerpt!,
            style: dsTextStyle(DsType.bodySm)
                .copyWith(color: colors.textSecondary),
          ),
        ),
      if (run.stalled)
        DsBanner(
          tone: DsTone.warning,
          text: l.liveBoxStall(run.stalledFor!.inMinutes),
        ),
      if (run.state.isActive)
        if (run.state == DsRunState.waitingInput && widget.question != null)
          widget.question!
        else
          Row(
            children: [
              if (widget.onStop != null) _stopButton(context, compact: false),
              const SizedBox(width: DsSpace.s8),
              Expanded(
                child: Text(
                  l.liveBoxFollowupHint,
                  style: dsTextStyle(DsType.caption)
                      .copyWith(color: colors.textSecondary),
                ),
              ),
            ],
          ),
    ];

    return Semantics(
      container: true,
      label: _label(context),
      child: Container(
        decoration: BoxDecoration(
          color: colors.surfaceRaised,
          border: Border.all(
            color: colors.borderDefault,
            width: DsBorderWidth.hairline,
          ),
          borderRadius: BorderRadius.circular(DsRadius.md),
        ),
        clipBehavior: Clip.antiAlias,
        child: IntrinsicHeight(
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Container(width: DsBorderWidth.spine, color: spineColor),
              Expanded(
                child: Padding(
                  padding: EdgeInsets.all(context.dsDensity.liveboxPad),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      for (var i = 0; i < children.length; i++) ...[
                        if (i > 0) const SizedBox(height: DsSpace.s8),
                        children[i],
                      ],
                    ],
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _collapsed(BuildContext context) {
    final l = dsL10n(context);
    final f = dsFormat(context);
    final colors = context.dsColors;
    final run = widget.run;
    final style = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    const dot = ' · ';
    return DsTappable(
      onTap: widget.onExpand,
      label: _label(context),
      hint: l.liveBoxExpand,
      radius: DsRadius.md,
      excludeChildSemantics: true,
      child: Container(
        padding: EdgeInsets.symmetric(
          horizontal: context.dsDensity.liveboxPad,
          vertical: DsSpace.s4,
        ),
        decoration: BoxDecoration(
          color: colors.surfaceSunken,
          borderRadius: BorderRadius.circular(DsRadius.md),
        ),
        child: Row(
          children: [
            _stateIcon(context),
            const SizedBox(width: DsSpace.s8),
            Expanded(
              child: Text.rich(
                TextSpan(
                  style: style,
                  children: [
                    TextSpan(text: l.liveBoxRan(f.durationShort(run.elapsed))),
                    TextSpan(text: '$dot${l.liveBoxTools(run.toolCount)}'),
                    if (run.subagentCount > 0)
                      TextSpan(
                        text: '$dot${l.liveBoxSubagents(run.subagentCount)}',
                      ),
                    if (run.cost != null) ...[
                      const TextSpan(text: dot),
                      TextSpan(
                        text: f.money(
                          run.cost!.money.amount,
                          run.cost!.money.currency,
                        ),
                        style: _mono(context),
                      ),
                      if (DsCostText.suffix(context, run.cost!) case final s?)
                        TextSpan(text: ' $s'),
                    ],
                  ],
                ),
              ),
            ),
            DsIcon(
              DsIcons.actionCollapse,
              size: DsIconSizeRole.sm,
              color: colors.textSecondary,
              decorative: true,
            ),
          ],
        ),
      ),
    );
  }

  Widget _mobile(BuildContext context) {
    final l = dsL10n(context);
    final f = dsFormat(context);
    final colors = context.dsColors;
    final run = widget.run;
    final density = context.dsDensity;
    return Container(
      constraints: BoxConstraints(minHeight: density.controlHeight),
      padding: const EdgeInsets.symmetric(
        horizontal: DsSpace.s8,
        vertical: DsSpace.s4,
      ),
      decoration: BoxDecoration(
        color: colors.surfaceRaised,
        border: Border(
          bottom: BorderSide(
            color: colors.borderSubtle,
            width: DsBorderWidth.hairline,
          ),
          left: BorderSide(color: spineColor, width: DsBorderWidth.spine),
        ),
      ),
      child: Row(
        children: [
          Expanded(
            child: DsTappable(
              onTap: widget.onOpen,
              label: _label(context),
              hint: l.liveBoxOpen,
              excludeChildSemantics: true,
              child: Row(
                children: [
                  _stateIcon(context),
                  const SizedBox(width: DsSpace.s6),
                  Text(
                    dsRunStateWord(l, run),
                    style: dsTextStyle(DsType.label)
                        .copyWith(color: colors.textPrimary),
                  ),
                  const SizedBox(width: DsSpace.s6),
                  Text(f.elapsed(run.elapsed), style: _mono(context)),
                  const SizedBox(width: DsSpace.s8),
                  Expanded(child: DsStatusLine(run.statusLine)),
                ],
              ),
            ),
          ),
          if (widget.onStop != null && run.state.isActive) ...[
            const SizedBox(width: DsSpace.s8),
            _stopButton(context, compact: true),
          ],
        ],
      ),
    );
  }
}
