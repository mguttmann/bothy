// SPDX-License-Identifier: Apache-2.0

/// The interview as a conversation (docs/spec/01-product.md 6.20 item 2; brief A1-08 Design
/// note 5 and requirement 15): the builder's questions and the person's answers, a spinner while
/// the request runs, the cost estimate so far, "Use what we have", and a fallback with its reason
/// and the ways to the templates and the empty form.
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'agent_interview.dart';
import 'new_agent_port.dart';
import 'new_agent_texts.dart';

/// The view of [interview]; [onTemplates] and [onEmpty] leave a fallback for the other ways in.
///
/// Example:
/// ```dart
/// NewAgentInterviewView(interview: interview, onTemplates: showTemplates, onEmpty: showForm)
/// ```
class NewAgentInterviewView extends StatefulWidget {
  /// Creates the view.
  const NewAgentInterviewView({
    super.key,
    required this.interview,
    required this.onTemplates,
    required this.onEmpty,
  });

  /// The interview.
  final AgentInterview interview;

  /// Opens the template list.
  final VoidCallback onTemplates;

  /// Opens the empty form.
  final VoidCallback onEmpty;

  @override
  State<NewAgentInterviewView> createState() => _NewAgentInterviewViewState();
}

class _NewAgentInterviewViewState extends State<NewAgentInterviewView> {
  final TextEditingController _answer = TextEditingController();

  @override
  void dispose() {
    _answer.dispose();
    super.dispose();
  }

  Future<void> _send({required bool draft}) async {
    final text = _answer.text;
    final i = widget.interview;
    if (draft) {
      await i.useWhatWeHave(text);
    } else {
      await i.answer(text);
    }
    // The text stays when the server refused it, so the person can shorten it.
    if (mounted && i.error == null) _answer.clear();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final i = widget.interview;
    final fallback = i.fallbackReason;
    final cost = i.costMicros;
    final error = i.error;
    return Column(
      key: const ValueKey('newAgent.interview'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          l.newAgentInterviewIntro,
          style: dsTextStyle(DsType.bodySm)
              .copyWith(color: colors.textSecondary),
        ),
        const SizedBox(height: DsSpace.s12),
        for (final (n, t) in i.turns.indexed)
          _Bubble(
            key: ValueKey('newAgent.interview.turn.$n'),
            who: t.role == InterviewRole.builder
                ? l.newAgentInterviewBuilder
                : l.newAgentInterviewYou,
            text: t.text,
            mine: t.role == InterviewRole.owner,
          ),
        if (i.pending)
          Semantics(
            key: const ValueKey('newAgent.interview.pending'),
            liveRegion: true,
            label: l.newAgentInterviewWaiting,
            child: const Padding(
              padding: EdgeInsets.all(DsSpace.s8),
              child: Center(
                child: SizedBox.square(
                  dimension: DsSpace.s24,
                  child: CircularProgressIndicator(strokeWidth: 2),
                ),
              ),
            ),
          ),
        if (cost != null)
          Padding(
            padding: const EdgeInsets.only(top: DsSpace.s8),
            child: Text(
              l.newAgentInterviewCost(
                dsFormat(context).money(cost / 1000000, i.currency ?? 'USD'),
              ),
              key: const ValueKey('newAgent.interview.cost'),
              style: dsTextStyle(DsType.bodySm)
                  .copyWith(color: colors.textSecondary),
            ),
          ),
        if (error != null)
          Padding(
            padding: const EdgeInsets.only(top: DsSpace.s8),
            child: Text(
              error.kind == NewAgentFailureKind.offline
                  ? l.newAgentErrOffline
                  : error.errors.isNotEmpty
                  ? error.errors
                        .map((e) => newAgentFieldErrorText(l, e.code))
                        .join(' ')
                  : problemText(l, error.code ?? '', reason: error.reason),
              key: const ValueKey('newAgent.interview.error'),
              style: dsTextStyle(DsType.bodySm)
                  .copyWith(color: colors.dangerFg),
            ),
          ),
        if (fallback != null)
          Container(
            key: const ValueKey('newAgent.interview.fallback'),
            margin: const EdgeInsets.only(top: DsSpace.s12),
            padding: const EdgeInsets.all(DsSpace.s12),
            decoration: BoxDecoration(
              border: Border.all(color: colors.borderSubtle),
              borderRadius: BorderRadius.circular(DsSpace.s8),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Semantics(
                  liveRegion: true,
                  child: Text(
                    interviewFallbackText(l, fallback),
                    style: dsTextStyle(DsType.bodyStrong),
                  ),
                ),
                const SizedBox(height: DsSpace.s4),
                Text(
                  l.newAgentInterviewFallbackHint,
                  style: dsTextStyle(DsType.bodySm),
                ),
                const SizedBox(height: DsSpace.s8),
                Wrap(
                  spacing: DsSpace.s8,
                  runSpacing: DsSpace.s8,
                  children: [
                    OutlinedButton(
                      key: const ValueKey('newAgent.interview.toTemplates'),
                      onPressed: widget.onTemplates,
                      child: Text(l.newAgentWayTemplate),
                    ),
                    OutlinedButton(
                      key: const ValueKey('newAgent.interview.toEmpty'),
                      onPressed: widget.onEmpty,
                      child: Text(l.newAgentWayEmpty),
                    ),
                  ],
                ),
              ],
            ),
          )
        else if (i.turns.isNotEmpty || !i.pending) ...[
          const SizedBox(height: DsSpace.s12),
          TextField(
            key: const ValueKey('newAgent.interview.input'),
            controller: _answer,
            enabled: !i.pending,
            minLines: 2,
            maxLines: null,
            decoration: InputDecoration(labelText: l.newAgentInterviewAnswer),
          ),
          const SizedBox(height: DsSpace.s8),
          Wrap(
            alignment: WrapAlignment.end,
            spacing: DsSpace.s8,
            runSpacing: DsSpace.s8,
            children: [
              TextButton(
                key: const ValueKey('newAgent.interview.useWhatWeHave'),
                onPressed: i.pending
                    ? null
                    : () => unawaited(_send(draft: true)),
                child: Text(l.newAgentInterviewUseWhatWeHave),
              ),
              FilledButton(
                key: const ValueKey('newAgent.interview.send'),
                onPressed: i.pending
                    ? null
                    : () => unawaited(_send(draft: false)),
                child: Text(l.newAgentInterviewSend),
              ),
            ],
          ),
        ],
      ],
    );
  }
}

/// One turn of the conversation.
class _Bubble extends StatelessWidget {
  const _Bubble({
    super.key,
    required this.who,
    required this.text,
    required this.mine,
  });

  final String who;
  final String text;
  final bool mine;

  @override
  Widget build(BuildContext context) {
    final colors = context.dsColors;
    return MergeSemantics(
      child: Align(
        alignment: mine
            ? AlignmentDirectional.centerEnd
            : AlignmentDirectional.centerStart,
        child: Container(
          constraints: const BoxConstraints(maxWidth: 480),
          margin: const EdgeInsets.only(bottom: DsSpace.s8),
          padding: const EdgeInsets.all(DsSpace.s8),
          decoration: BoxDecoration(
            color: mine ? colors.surfaceSelected : colors.surfaceRaised,
            borderRadius: BorderRadius.circular(DsSpace.s8),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                who,
                style: dsTextStyle(DsType.label)
                    .copyWith(color: colors.textSecondary),
              ),
              Text(
                text,
                style: dsTextStyle(DsType.body)
                    .copyWith(color: colors.textPrimary),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
