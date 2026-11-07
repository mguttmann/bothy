// SPDX-License-Identifier: Apache-2.0

/// The dialog "New agent" of the desktop, the phones and the console (docs/spec/01-product.md
/// 6.20; docs/spec/26-builtin-agents.md 8; brief A1-08; docs/decisions.d/A1-08.md).
///
/// Three ways in lead to one preview: a template, the interview with the builder agent, the
/// empty form (01 6.20 item 2). "Create" runs the steps of [NewAgentFlow]; a failed step shows
/// its cause and keeps the preview editable, "Try again" resumes, "Close" keeps what exists
/// (26 8.3). When every step is done the dialog closes with a [NewAgentResult], and the host
/// opens the new channel or the agent card (01 6.20 item 5). On the phone the same dialog fills
/// the screen ([NewAgentDialog.fullScreen]).
library;

import 'dart:async';

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'agent_interview.dart';
import 'new_agent_flow.dart';
import 'new_agent_interview_view.dart';
import 'new_agent_port.dart';
import 'new_agent_preview.dart';
import 'new_agent_steps.dart';

/// Opens the dialog over [port] with [offer]; returns the result when every step is done, null
/// when the person closed it (what exists stays, 26 8.3).
///
/// [newKey] makes an `Idempotency-Key` (UUIDv7), [announceMinInterval] is
/// `a11y.live_announce_min_interval`, [fullScreen] is the phone layout.
///
/// Example:
/// ```dart
/// final result = await showNewAgentDialog(context, port: port, offer: offer,
///     newKey: ids.next, announceMinInterval: config.liveAnnounceMinInterval);
/// ```
Future<NewAgentResult?> showNewAgentDialog(
  BuildContext context, {
  required NewAgentPort port,
  required NewAgentOffer offer,
  required String Function() newKey,
  required Duration announceMinInterval,
  bool fullScreen = false,
}) => showDialog<NewAgentResult>(
  context: context,
  barrierDismissible: false,
  useSafeArea: !fullScreen,
  builder: (_) => NewAgentDialog(
    port: port,
    offer: offer,
    newKey: newKey,
    announceMinInterval: announceMinInterval,
    fullScreen: fullScreen,
  ),
);

/// What a host (the desktop, a phone, the console) hands to an entry of "New agent": the port,
/// the key maker, the announcement interval, the layout and what to open once the agent exists.
///
/// Example:
/// ```dart
/// NewAgentHost(port: port, newKey: ids.next,
///     announceMinInterval: config.liveAnnounceMinInterval,
///     onCreated: (r) => r.channelId == null ? openAgent(r.agentId) : openChannel(r.channelId!));
/// ```
final class NewAgentHost {
  /// Creates the host's part.
  const NewAgentHost({
    required this.port,
    required this.newKey,
    required this.announceMinInterval,
    this.fullScreen = false,
    this.onCreated,
  });

  /// The server side.
  final NewAgentPort port;

  /// Makes an `Idempotency-Key`.
  final String Function() newKey;

  /// `a11y.live_announce_min_interval`.
  final Duration announceMinInterval;

  /// Whether the dialog fills the screen (the phones).
  final bool fullScreen;

  /// Opens the new channel or, without one, the agent card (01 6.20 item 5); null leaves it to
  /// the entry.
  final void Function(NewAgentResult result)? onCreated;

  /// Opens the dialog with [offer]; returns the result once every step is done.
  Future<NewAgentResult?> open(BuildContext context, NewAgentOffer offer) =>
      showNewAgentDialog(
        context,
        port: port,
        offer: offer,
        newKey: newKey,
        announceMinInterval: announceMinInterval,
        fullScreen: fullScreen,
      );
}

/// Where the dialog is.
enum _Stage { start, templates, interview, preview }

/// The dialog.
class NewAgentDialog extends StatefulWidget {
  /// Creates the dialog; the parameters are those of [showNewAgentDialog], [clock] is the time
  /// of the step announcements.
  const NewAgentDialog({
    super.key,
    required this.port,
    required this.offer,
    required this.newKey,
    required this.announceMinInterval,
    this.fullScreen = false,
    this.clock = DateTime.now,
  });

  /// The server side.
  final NewAgentPort port;

  /// The templates and runners.
  final NewAgentOffer offer;

  /// Makes an `Idempotency-Key`.
  final String Function() newKey;

  /// `a11y.live_announce_min_interval`.
  final Duration announceMinInterval;

  /// Whether the dialog fills the screen (the phones).
  final bool fullScreen;

  /// The current time.
  final DateTime Function() clock;

  @override
  State<NewAgentDialog> createState() => _NewAgentDialogState();
}

class _NewAgentDialogState extends State<NewAgentDialog> {
  late final NewAgentFlow _flow;
  AgentInterview? _interview;
  _Stage _stage = _Stage.start;
  bool _popped = false;

  @override
  void initState() {
    super.initState();
    _flow = NewAgentFlow(
      port: widget.port,
      offer: widget.offer,
      newKey: widget.newKey,
    )..addListener(_changed);
    unawaited(_loadContext());
  }

  Future<void> _loadContext() async {
    try {
      final c = await widget.port.context();
      if (mounted) _flow.setContext(c);
    } on Object {
      // Without the context the preview offers no personal billing and no space; the rest works.
    }
  }

  void _changed() {
    if (!mounted) return;
    setState(() {});
    final result = _flow.result;
    if (result != null && !_popped) {
      _popped = true;
      Navigator.of(context).pop(result);
    }
  }

  void _interviewChanged() {
    final i = _interview;
    if (!mounted || i == null) return;
    final draft = i.draft;
    if (draft != null && _stage == _Stage.interview) {
      _flow.useInterview(draft, costMicros: i.costMicros, currency: i.currency);
      _stage = _Stage.preview;
    }
    setState(() {});
  }

  void _go(_Stage stage) {
    setState(() => _stage = stage);
    if (stage == _Stage.interview && _interview == null) {
      _interview = AgentInterview(widget.port)..addListener(_interviewChanged);
      unawaited(_interview!.start());
    }
  }

  @override
  void dispose() {
    _flow
      ..removeListener(_changed)
      ..dispose();
    _interview
      ?..removeListener(_interviewChanged)
      ..dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final body = switch (_stage) {
      _Stage.start => _start(l),
      _Stage.templates => _templates(l),
      _Stage.interview => NewAgentInterviewView(
        interview: _interview!,
        onTemplates: () => _go(_Stage.templates),
        onEmpty: () {
          _flow.useEmpty();
          _go(_Stage.preview);
        },
      ),
      _Stage.preview => Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          if (_flow.started) ...[
            NewAgentSteps(
              flow: _flow,
              announceMinInterval: widget.announceMinInterval,
              clock: widget.clock,
            ),
            const Divider(height: DsSpace.s24),
          ],
          NewAgentPreview(flow: _flow),
        ],
      ),
    };
    final content = Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(
            DsSpace.s24,
            DsSpace.s20,
            DsSpace.s24,
            DsSpace.s12,
          ),
          child: Semantics(
            header: true,
            child: Text(
              l.newAgentTitle,
              style: dsTextStyle(DsType.title)
                  .copyWith(color: context.dsColors.textPrimary),
            ),
          ),
        ),
        Flexible(
          child: SingleChildScrollView(
            padding: const EdgeInsets.symmetric(horizontal: DsSpace.s24),
            child: body,
          ),
        ),
        Padding(
          padding: const EdgeInsets.all(DsSpace.s16),
          child: Wrap(
            alignment: WrapAlignment.end,
            spacing: DsSpace.s8,
            runSpacing: DsSpace.s8,
            children: _actions(l),
          ),
        ),
      ],
    );
    return Dialog(
      key: const ValueKey('newAgent.dialog'),
      insetPadding: widget.fullScreen
          ? EdgeInsets.zero
          : const EdgeInsets.symmetric(
              horizontal: DsSpace.s40,
              vertical: DsSpace.s24,
            ),
      shape: widget.fullScreen ? const RoundedRectangleBorder() : null,
      child: widget.fullScreen
          ? SafeArea(child: SizedBox.expand(child: content))
          : ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 720),
              child: content,
            ),
    );
  }

  List<Widget> _actions(AppLocalizations l) {
    final flow = _flow;
    final started = flow.started;
    return [
      if (_stage != _Stage.start && !started)
        TextButton(
          key: const ValueKey('newAgent.back'),
          onPressed: () => _go(_Stage.start),
          child: Text(l.newAgentBack),
        ),
      TextButton(
        key: ValueKey(started ? 'newAgent.close' : 'newAgent.cancel'),
        onPressed: flow.busy ? null : () => Navigator.of(context).pop(),
        child: Text(started ? l.newAgentClose : l.newAgentCancel),
      ),
      if (_stage == _Stage.preview && !started)
        FilledButton(
          key: const ValueKey('newAgent.create'),
          onPressed: flow.busy ? null : () => unawaited(flow.run()),
          child: Text(l.newAgentCreate),
        )
      else if (_stage == _Stage.preview && flow.failure != null)
        FilledButton(
          key: const ValueKey('newAgent.retry'),
          onPressed: flow.busy ? null : () => unawaited(flow.run()),
          child: Text(l.newAgentRetry),
        ),
    ];
  }

  Widget _start(AppLocalizations l) {
    Widget way(String id, String title, String hint, VoidCallback onTap) =>
        Card(
          margin: const EdgeInsets.only(bottom: DsSpace.s8),
          child: ListTile(
            key: ValueKey('newAgent.way.$id'),
            title: Text(title),
            subtitle: Text(hint),
            onTap: onTap,
          ),
        );

    return Column(
      key: const ValueKey('newAgent.start'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        way('template', l.newAgentWayTemplate, l.newAgentWayTemplateHint, () {
          _go(_Stage.templates);
        }),
        way(
          'interview',
          l.newAgentWayInterview,
          l.newAgentWayInterviewHint,
          () {
            _go(_Stage.interview);
          },
        ),
        way('empty', l.newAgentWayEmpty, l.newAgentWayEmptyHint, () {
          _flow.useEmpty();
          _go(_Stage.preview);
        }),
      ],
    );
  }

  Widget _templates(AppLocalizations l) {
    final templates = widget.offer.templates;
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: context.dsColors.textSecondary);
    return Column(
      key: const ValueKey('newAgent.templates'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        if (templates.isEmpty) Text(l.newAgentTemplatesEmpty, style: secondary),
        for (final t in templates)
          Card(
            margin: const EdgeInsets.only(bottom: DsSpace.s8),
            child: ListTile(
              key: ValueKey('newAgent.template.${t.slug}'),
              title: Text(t.name),
              subtitle: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(t.description),
                  if (t.requiredSecrets.isNotEmpty)
                    Text(
                      l.newAgentTemplateSecrets(t.requiredSecrets.join(', ')),
                      style: secondary,
                    ),
                  if (t.privateSource)
                    Text(l.newAgentTemplatePrivate, style: secondary),
                ],
              ),
              onTap: () {
                _flow.useTemplate(t);
                _go(_Stage.preview);
              },
            ),
          ),
      ],
    );
  }
}
