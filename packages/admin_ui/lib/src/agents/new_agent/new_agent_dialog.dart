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
///
/// The same dialog edits an existing built-in agent ([ExistingAgent], brief A1-09): "Edit" and
/// "Restore" open the preview with the stored definition and the list of changed parts and
/// save with "Save"; a `412` offers "Reload", `definition_needs_host` offers "Remove the local
/// servers"; "Show" is read-only; "Continue setup" starts at the ways in and runs the missing
/// steps.
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
import 'new_agent_texts.dart';

/// An existing built-in agent the dialog opens for "Edit", "Restore", "Show" or "Continue
/// setup" ([NewAgentFlow.forAgent]).
///
/// Example: `ExistingAgent(agent: state, runner: runner, stored: current)` edits the current
/// definition; with `content: older.json, restoredVersion: older.version` it restores it.
final class ExistingAgent {
  /// Creates the description.
  const ExistingAgent({
    required this.agent,
    required this.runner,
    this.stored,
    this.content,
    this.restoredVersion,
    this.shownVersion,
  });

  /// The agent as the server has it.
  final NewAgentState agent;

  /// Its hosting runner.
  final HostingRunner runner;

  /// The current definition, or null for "Continue setup".
  final StoredDefinition? stored;

  /// An older content to restore or show, or null to edit [stored].
  final Map<String, Object?>? content;

  /// The version of [content] for "Restore", or null.
  final int? restoredVersion;

  /// The version of [content] for "Show" (read-only), or null.
  final int? shownVersion;
}

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
  ExistingAgent? existing,
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
    existing: existing,
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

  /// Opens the dialog for the existing agent [existing] (A1-09); returns the result once
  /// every step is done, null when it was closed or only shown.
  Future<NewAgentResult?> openExisting(
    BuildContext context,
    NewAgentOffer offer,
    ExistingAgent existing,
  ) => showNewAgentDialog(
    context,
    port: port,
    offer: offer,
    newKey: newKey,
    announceMinInterval: announceMinInterval,
    fullScreen: fullScreen,
    existing: existing,
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
    this.existing,
  });

  /// The existing agent to edit, restore, show or set up, or null for "New agent".
  final ExistingAgent? existing;

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
    final existing = widget.existing;
    _flow =
        (existing == null
              ? NewAgentFlow(
                  port: widget.port,
                  offer: widget.offer,
                  newKey: widget.newKey,
                )
              : NewAgentFlow.forAgent(
                  port: widget.port,
                  offer: widget.offer,
                  newKey: widget.newKey,
                  agent: existing.agent,
                  runner: existing.runner,
                  stored: existing.stored,
                  content: existing.content,
                  readOnly: existing.shownVersion != null,
                ))
          ..addListener(_changed);
    // An edit, a restore and a shown version start at the preview.
    if (_flow.draft != null) _stage = _Stage.preview;
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

  String _title(AppLocalizations l) {
    final existing = widget.existing;
    final name = existing?.agent.name ?? '';
    return switch (_flow.mode) {
      NewAgentMode.create => l.newAgentTitle,
      NewAgentMode.setup => l.agentSetupTitle(name),
      NewAgentMode.view => l.agentViewTitle(name, existing!.shownVersion!),
      NewAgentMode.edit => switch (existing?.restoredVersion) {
        final v? => l.agentRestoreTitle(v, name),
        null => l.agentEditTitle(name),
      },
    };
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
          if (_flow.mode == NewAgentMode.edit) ...[
            AgentEditChanges(flow: _flow),
            const Divider(height: DsSpace.s24),
          ],
          if (_flow.started) ...[
            NewAgentSteps(
              flow: _flow,
              announceMinInterval: widget.announceMinInterval,
              clock: widget.clock,
            ),
            const Divider(height: DsSpace.s24),
          ],
          // A reload gives a new draft; the fields start again from it.
          KeyedSubtree(
            key: ObjectKey(_flow.draft),
            child: NewAgentPreview(flow: _flow),
          ),
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
              _title(l),
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
    switch (flow.mode) {
      case NewAgentMode.view:
        return [
          TextButton(
            key: const ValueKey('newAgent.close'),
            onPressed: () => Navigator.of(context).pop(),
            child: Text(l.newAgentClose),
          ),
        ];
      case NewAgentMode.edit:
        return _editActions(l);
      case NewAgentMode.create || NewAgentMode.setup:
        break;
    }
    final primary = flow.mode == NewAgentMode.setup
        ? FilledButton(
            key: const ValueKey('agentEdit.continue'),
            onPressed: flow.busy ? null : () => unawaited(flow.run()),
            child: Text(l.agentEditContinue),
          )
        : FilledButton(
            key: const ValueKey('newAgent.create'),
            onPressed: flow.busy ? null : () => unawaited(flow.run()),
            child: Text(l.newAgentCreate),
          );
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
        primary
      else if (_stage == _Stage.preview && flow.failure != null)
        FilledButton(
          key: const ValueKey('newAgent.retry'),
          onPressed: flow.busy ? null : () => unawaited(flow.run()),
          child: Text(l.newAgentRetry),
        ),
    ];
  }

  /// "Cancel" or "Close", "Save" until it was pressed, then after a failure "Reload" for a
  /// stale version, "Remove the local servers" for `definition_needs_host` and "Try again".
  List<Widget> _editActions(AppLocalizations l) {
    final flow = _flow;
    final failure = flow.failure;
    final stale = failure?.code == 'version_conflict';
    final needsHost =
        failure?.reason == 'definition_needs_host' && flow.hasLocalServers;
    return [
      TextButton(
        key: ValueKey(flow.started ? 'newAgent.close' : 'newAgent.cancel'),
        onPressed: flow.busy ? null : () => Navigator.of(context).pop(),
        child: Text(flow.started ? l.newAgentClose : l.newAgentCancel),
      ),
      if (!flow.started)
        FilledButton(
          key: const ValueKey('agentEdit.save'),
          onPressed: flow.busy || !flow.hasChanges
              ? null
              : () => unawaited(flow.run()),
          child: Text(l.agentEditSave),
        ),
      if (stale)
        FilledButton(
          key: const ValueKey('agentEdit.reload'),
          onPressed: flow.busy ? null : () => unawaited(flow.reload()),
          child: Text(l.agentEditReload),
        ),
      if (needsHost)
        OutlinedButton(
          key: const ValueKey('agentEdit.removeLocal'),
          onPressed: flow.busy ? null : flow.removeLocalServers,
          child: Text(l.agentEditRemoveLocal),
        ),
      if (failure != null && !stale)
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

/// The changed parts of an edit against the version it started from (brief A1-09 Design note
/// 2), or "No changes yet", and when a saved change applies.
///
/// Example: `AgentEditChanges(flow: flow)` above the preview of an edit.
class AgentEditChanges extends StatelessWidget {
  /// Creates the list.
  const AgentEditChanges({super.key, required this.flow});

  /// The flow of the edit.
  final NewAgentFlow flow;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final changes = flow.changes;
    final secondary = dsTextStyle(DsType.bodySm)
        .copyWith(color: colors.textSecondary);
    return Column(
      key: const ValueKey('agentEdit.changes'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Semantics(
          header: true,
          child: Text(
            l.agentEditChangesTitle,
            style: dsTextStyle(DsType.bodyStrong)
                .copyWith(color: colors.textPrimary),
          ),
        ),
        const SizedBox(height: DsSpace.s4),
        if (changes.isEmpty)
          Text(
            l.agentEditNoChanges,
            key: const ValueKey('agentEdit.noChanges'),
            style: secondary,
          )
        else
          for (final (i, c) in changes.indexed)
            Text(
              agentEditChangeText(l, c),
              key: ValueKey('agentEdit.change.$i'),
              style: dsTextStyle(DsType.body)
                  .copyWith(color: colors.textPrimary),
            ),
        const SizedBox(height: DsSpace.s4),
        Text(l.agentEditChangesHint, style: secondary),
      ],
    );
  }
}
