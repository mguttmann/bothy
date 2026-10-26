// SPDX-License-Identifier: Apache-2.0

/// The status page of the desktop app, the phone apps and the console (docs/spec/16-operations.md
/// 12; 01 status page row; brief M7-21 Scope): the components with their states, the open
/// incidents, the maintenance windows, the history, the maintenance mode, and for owners the
/// forms of `status.manage`.
library;

import 'dart:async';

import 'package:api_client/api_client.dart' as api;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/admin_action.dart';
import '../common/confirm_change.dart';
import '../common/problem_banner.dart';
import '../self/self_feedback.dart';
import 'status_controller.dart';
import 'status_forms.dart';
import 'status_labels.dart';
import 'status_repository.dart';
import 'status_source.dart';

/// The status page over [source].
///
/// It shows what the server answered and nothing more: the components of agents appear only
/// when the server returns them (brief M7-21 requirement 13). While the source reports the
/// server unreachable, or a read gets no answer, it shows "Server unreachable" and hides every
/// state, because the page runs in the server and cannot know its own outage (16 12 "Limit").
/// Maintenance (the maintenance mode, `overall_state` `maintenance`, and the incidents of kind
/// `maintenance`) is shown in its own place and role, never as an outage.
///
/// Parameters: [manage] offers the owner forms when [StatusRepository.callerIsOwner] says so
/// (null: a read-only page, also for the public page); [timeZone] and [clock] format the times;
/// [onOpenLink] opens an active link of an owner text (null: links stay inactive text);
/// [title] replaces the heading; [notFoundText] replaces the reason of a `404`. The [source] is read once when the page starts; a host that
/// switches sources gives the page a new key.
///
/// Example:
/// ```dart
/// StatusScreen(source: source, manage: repository, timeZone: 'Europe/Berlin');
/// ```
class StatusScreen extends StatefulWidget {
  /// Creates the page.
  const StatusScreen({
    super.key,
    required this.source,
    this.manage,
    this.timeZone = 'UTC',
    this.clock = DateTime.now,
    this.onOpenLink,
    this.title,
    this.notFoundText,
  });

  /// Where the page and its changes come from.
  final StatusSource source;

  /// The owner operations, or null for a page without forms.
  final StatusRepository? manage;

  /// The time zone of the person.
  final String timeZone;

  /// The current time.
  final DateTime Function() clock;

  /// Opens an active link of an owner text, or null.
  final ValueChanged<Uri>? onOpenLink;

  /// The heading, or null for "Status page".
  final String? title;

  /// What a `404` of the read says instead of the server's reason, or null: the public page
  /// says that the instance shows its status only after login.
  final String? notFoundText;

  @override
  State<StatusScreen> createState() => _StatusScreenState();
}

class _StatusScreenState extends State<StatusScreen>
    with AdminActionState<StatusScreen> {
  late StatusController _controller;
  bool _owner = false;

  @override
  void initState() {
    super.initState();
    _controller = StatusController(widget.source)..start();
    unawaited(_readOwner());
  }

  @override
  void didUpdateWidget(StatusScreen old) {
    super.didUpdateWidget(old);
    if (!identical(old.manage, widget.manage)) unawaited(_readOwner());
  }

  Future<void> _readOwner() async {
    final manage = widget.manage;
    var owner = false;
    if (manage != null) {
      try {
        owner = await manage.callerIsOwner();
      } on Object {
        // No forms when the server cannot say; the page itself still shows.
        owner = false;
      }
    }
    if (mounted) setState(() => _owner = owner);
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _change(String key, Future<Object?> Function() call) async {
    final l = AppLocalizations.of(context);
    final ok = await runChange(key, () async {
      await call();
      return SelfNotice(l.statusSaved);
    });
    if (ok) await _controller.refresh();
  }

  Future<void> _addComponent(api.StatusPage page) async {
    final manage = widget.manage!;
    final input = await showStatusComponentDialog(
      context,
      components: page.components.toList(),
    );
    if (input == null || !mounted) return;
    await _change(
      'status.addComponent',
      () => manage.createStatusComponent(input),
    );
  }

  Future<void> _editComponent(
    api.StatusPage page,
    api.StatusComponent component,
  ) async {
    final manage = widget.manage!;
    final input = await showStatusComponentDialog(
      context,
      components: page.components.toList(),
      existing: component,
    );
    if (input == null || !mounted) return;
    await _change(
      'status.component.${component.id}.edit',
      () => manage.updateStatusComponent(component.id, input),
    );
  }

  Future<void> _deleteComponent(api.StatusComponent component) async {
    final manage = widget.manage!;
    final l = AppLocalizations.of(context);
    final confirmed = await ConfirmChange.show(
      context,
      title: l.statusDeleteTitle(component.name),
      body: l.statusDeleteBody,
      confirmLabel: l.statusDelete,
      destructive: true,
    );
    if (confirmed == null || !mounted) return;
    await _change(
      'status.component.${component.id}.delete',
      () => manage.deleteStatusComponent(component.id),
    );
  }

  Future<void> _openIncident(api.StatusPage page) async {
    final manage = widget.manage!;
    final input = await showStatusIncidentDialog(
      context,
      components: page.components.toList(),
      clock: widget.clock,
    );
    if (input == null || !mounted) return;
    await _change(
      'status.openIncident',
      () => manage.createStatusIncident(input),
    );
  }

  Future<void> _addUpdate(api.StatusIncident incident) async {
    final manage = widget.manage!;
    final input = await showStatusUpdateDialog(context, incident: incident);
    if (input == null || !mounted) return;
    await _change(
      'status.incident.${incident.id}.update',
      () => manage.addStatusIncidentUpdate(incident.id, input),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    return ListenableBuilder(
      listenable: _controller,
      builder: (context, _) {
        final c = _controller;
        final page = c.page;
        final children = <Widget>[
          Text(
            widget.title ?? l.statusTitle,
            key: const ValueKey('status.title'),
            style: dsTextStyle(DsType.heading)
                .copyWith(color: colors.textPrimary),
          ),
          const SizedBox(height: DsSpace.s16),
        ];
        if (c.unreachable) {
          children.add(const StatusUnreachable());
        } else if (c.problem case final problem?
            when problem.status == 404 && widget.notFoundText != null) {
          children.add(
            DsBanner(
              key: const ValueKey('status.notFound'),
              tone: DsTone.info,
              text: widget.notFoundText!,
            ),
          );
        } else if (c.problem case final problem?) {
          children.add(
            ProblemBanner(
              problem: problem,
              onRetry: () => unawaited(c.refresh()),
            ),
          );
        } else if (page == null) {
          children.add(
            const Align(
              alignment: Alignment.centerLeft,
              child: CircularProgressIndicator(key: ValueKey('status.loading')),
            ),
          );
        } else {
          children.addAll(_page(context, page));
        }
        return Column(
          key: const ValueKey('status.screen'),
          crossAxisAlignment: CrossAxisAlignment.stretch,
          mainAxisSize: MainAxisSize.min,
          children: children,
        );
      },
    );
  }

  List<Widget> _page(BuildContext context, api.StatusPage page) {
    final l = AppLocalizations.of(context);
    final owner = _owner && widget.manage != null;
    final names = {for (final c in page.components) c.id: c.name};
    final incidents = page.incidents.toList();
    bool open(api.StatusIncident i) =>
        i.state != api.StatusIncidentState.resolved;
    final openIncidents = [
      for (final i in incidents)
        if (open(i) && i.kind != api.StatusIncidentKind.maintenance) i,
    ];
    final windows = [
      for (final i in incidents)
        if (open(i) && i.kind == api.StatusIncidentKind.maintenance) i,
    ];
    final history = [
      for (final i in incidents)
        if (!open(i)) i,
    ];
    Widget card(api.StatusIncident i) => StatusIncidentCard(
      incident: i,
      componentNames: names,
      timeZone: widget.timeZone,
      now: widget.clock(),
      onOpenLink: widget.onOpenLink,
      onAddUpdate: owner && !busy ? () => unawaited(_addUpdate(i)) : null,
    );
    final overall = statusStateLook(context, page.overallState);
    return [
      noticeBanner(),
      Semantics(
        key: const ValueKey('status.overall'),
        container: true,
        label: l.statusOverall(overall.label),
        excludeSemantics: true,
        child: StatusStateChip(look: overall, large: true),
      ),
      if (page.overallState == api.StatusComponentState.maintenance) ...[
        const SizedBox(height: DsSpace.s12),
        DsBanner(
          key: const ValueKey('status.maintenanceMode'),
          tone: DsTone.info,
          text: l.statusMaintenanceMode,
        ),
      ],
      if (owner) ...[
        const SizedBox(height: DsSpace.s16),
        Wrap(
          spacing: DsSpace.s8,
          runSpacing: DsSpace.s8,
          children: [
            OutlinedButton(
              key: const ValueKey('status.addComponent'),
              onPressed: busy ? null : () => unawaited(_addComponent(page)),
              child: Text(l.statusAddComponent),
            ),
            OutlinedButton(
              key: const ValueKey('status.openIncident'),
              onPressed: busy ? null : () => unawaited(_openIncident(page)),
              child: Text(l.statusNewIncident),
            ),
          ],
        ),
      ],
      _Section(
        title: l.statusComponents,
        children: page.components.isEmpty
            ? [
                _Empty(
                  key: const ValueKey('status.empty'),
                  text: l.statusNoComponents,
                ),
              ]
            : [
                for (final component in page.components)
                  StatusComponentRow(
                    component: component,
                    onEdit: owner && !busy
                        ? () => unawaited(_editComponent(page, component))
                        : null,
                    onDelete: owner && !busy
                        ? () => unawaited(_deleteComponent(component))
                        : null,
                  ),
              ],
      ),
      _Section(
        key: const ValueKey('status.incidents.open'),
        title: l.statusOpenIncidents,
        children: openIncidents.isEmpty
            ? [
                _Empty(
                  key: const ValueKey('status.incidents.none'),
                  text: l.statusNoIncidents,
                ),
              ]
            : [for (final i in openIncidents) card(i)],
      ),
      if (windows.isNotEmpty)
        _Section(
          key: const ValueKey('status.maintenanceWindows'),
          title: l.statusMaintenanceWindows,
          children: [for (final i in windows) card(i)],
        ),
      if (history.isNotEmpty)
        _Section(
          key: const ValueKey('status.history'),
          title: l.statusHistory,
          children: [for (final i in history) card(i)],
        ),
    ];
  }
}

/// "Server unreachable" with the reason why nothing else shows (16 12 "Limit").
class StatusUnreachable extends StatelessWidget {
  /// Creates the notice.
  const StatusUnreachable({super.key});

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return Column(
      key: const ValueKey('status.unreachable'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        DsBanner(
          tone: DsTone.danger,
          text: l.statusUnreachable,
          liveRegion: true,
        ),
        const SizedBox(height: DsSpace.s8),
        Text(
          l.statusUnreachableHint,
          style: dsTextStyle(DsType.body)
              .copyWith(color: context.dsColors.textSecondary),
        ),
      ],
    );
  }
}

class _Section extends StatelessWidget {
  const _Section({super.key, required this.title, required this.children});

  final String title;
  final List<Widget> children;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(top: DsSpace.s24),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Semantics(
          header: true,
          child: Text(
            title,
            style: dsTextStyle(DsType.title)
                .copyWith(color: context.dsColors.textPrimary),
          ),
        ),
        const SizedBox(height: DsSpace.s8),
        ...children,
      ],
    ),
  );
}

class _Empty extends StatelessWidget {
  const _Empty({super.key, required this.text});

  final String text;

  @override
  Widget build(BuildContext context) => Text(
    text,
    style: dsTextStyle(DsType.body)
        .copyWith(color: context.dsColors.textSecondary),
  );
}

/// One component: its name (with "Agent" for the component of an agent), its description and
/// its state; for owners "Edit" and "Delete".
class StatusComponentRow extends StatelessWidget {
  /// Creates the row.
  const StatusComponentRow({
    super.key,
    required this.component,
    this.onEdit,
    this.onDelete,
  });

  /// The component.
  final api.StatusComponent component;

  /// Opens the form, or null for no button.
  final VoidCallback? onEdit;

  /// Asks to delete it, or null for no button.
  final VoidCallback? onDelete;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final id = component.id;
    final look = statusStateLook(context, component.state);
    return Container(
      key: ValueKey('status.component.$id'),
      padding: const EdgeInsets.symmetric(vertical: DsSpace.s8),
      decoration: BoxDecoration(
        border: Border(
          bottom: BorderSide(
            color: colors.borderSubtle,
            width: DsBorderWidth.hairline,
          ),
        ),
      ),
      child: Wrap(
        crossAxisAlignment: WrapCrossAlignment.center,
        spacing: DsSpace.s12,
        runSpacing: DsSpace.s4,
        children: [
          Semantics(
            container: true,
            label: l.statusComponentSemantics(component.name, look.label),
            excludeSemantics: true,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  component.agentId == null
                      ? dsIsolate(component.name)
                      : l.statusAgentComponent(dsIsolate(component.name)),
                  style: dsTextStyle(DsType.bodyStrong)
                      .copyWith(color: colors.textPrimary),
                ),
                if (component.description.isNotEmpty)
                  Text(
                    dsIsolate(component.description),
                    style: dsTextStyle(DsType.bodySm)
                        .copyWith(color: colors.textSecondary),
                  ),
                StatusStateChip(
                  key: ValueKey('status.component.$id.state'),
                  look: look,
                ),
              ],
            ),
          ),
          if (onEdit != null)
            TextButton(
              key: ValueKey('status.component.$id.edit'),
              onPressed: onEdit,
              child: Text(l.statusEdit),
            ),
          if (onDelete != null)
            TextButton(
              key: ValueKey('status.component.$id.delete'),
              onPressed: onDelete,
              child: Text(l.statusDelete),
            ),
        ],
      ),
    );
  }
}

/// One incident or maintenance window with its updates, newest first; the owner texts are
/// Markdown in the safe subset of the message renderer ([DsMarkdown]: no HTML, no images,
/// links only by its link policy; brief M7-21 requirement 15).
class StatusIncidentCard extends StatelessWidget {
  /// Creates the card.
  const StatusIncidentCard({
    super.key,
    required this.incident,
    required this.componentNames,
    required this.timeZone,
    required this.now,
    this.onOpenLink,
    this.onAddUpdate,
  });

  /// The incident.
  final api.StatusIncident incident;

  /// The names of the visible components by ID.
  final Map<String, String> componentNames;

  /// The time zone of the person.
  final String timeZone;

  /// The current time.
  final DateTime now;

  /// Opens an active link, or null.
  final ValueChanged<Uri>? onOpenLink;

  /// Opens the update form, or null for no button.
  final VoidCallback? onAddUpdate;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final locale = Localizations.localeOf(context);
    final i = incident;
    final maintenance = i.kind == api.StatusIncidentKind.maintenance;
    String at(DateTime t) =>
        statusTime(locale, t, now: now, timeZone: timeZone);
    final affected = [
      for (final id in i.componentIds)
        if (componentNames[id] case final name?) dsIsolate(name),
    ];
    final roleColor = maintenance
        ? colors.infoFg
        : switch (i.impact) {
            api.StatusIncidentImpact.critical ||
            api.StatusIncidentImpact.major => colors.dangerFg,
            api.StatusIncidentImpact.minor => colors.warningFg,
            _ => colors.textSecondary,
          };
    final updates = i.updates.toList()
      ..sort((a, b) => b.createdAt.compareTo(a.createdAt));
    final meta = [
      statusKindLabel(l, i.kind),
      if (!maintenance) statusImpactLabel(l, i.impact),
      statusIncidentStateLabel(l, i.state),
    ].join(' · ');
    return Container(
      key: ValueKey('status.incident.${i.id}'),
      margin: const EdgeInsets.only(bottom: DsSpace.s8),
      padding: const EdgeInsets.all(DsSpace.s12),
      decoration: BoxDecoration(
        color: colors.surfaceRaised,
        border: Border(
          left: BorderSide(color: roleColor, width: DsBorderWidth.spine),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              DsIcon(
                maintenance ? DsIcons.stateInfo : DsIcons.stateWarning,
                size: DsIconSizeRole.sm,
                color: roleColor,
                decorative: true,
              ),
              const SizedBox(width: DsSpace.s4),
              Expanded(
                child: Text(
                  dsIsolate(i.title),
                  style: dsTextStyle(DsType.bodyStrong)
                      .copyWith(color: colors.textPrimary),
                ),
              ),
            ],
          ),
          Text(
            meta,
            key: ValueKey('status.incident.${i.id}.meta'),
            style: dsTextStyle(DsType.caption).copyWith(color: roleColor),
          ),
          Text(
            [
              l.statusStarted(at(i.startedAt)),
              if (i.resolvedAt case final r?) l.statusResolvedAt(at(r)),
              if (affected.isNotEmpty) l.statusAffects(affected.join(', ')),
            ].join(' · '),
            style: dsTextStyle(DsType.caption)
                .copyWith(color: colors.textSecondary),
          ),
          for (final u in updates)
            Padding(
              key: ValueKey('status.update.${u.id}'),
              padding: const EdgeInsets.only(top: DsSpace.s8),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    '${statusIncidentStateLabel(l, u.state)} · ${at(u.createdAt)}',
                    style: dsTextStyle(DsType.caption)
                        .copyWith(color: colors.textSecondary),
                  ),
                  DsMarkdown(u.bodyMd, onOpenLink: onOpenLink ?? (_) {}),
                ],
              ),
            ),
          if (onAddUpdate != null)
            Align(
              alignment: Alignment.centerLeft,
              child: TextButton(
                key: ValueKey('status.incident.${i.id}.update'),
                onPressed: onAddUpdate,
                child: Text(l.statusAddUpdate),
              ),
            ),
        ],
      ),
    );
  }
}
