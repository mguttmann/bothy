// SPDX-License-Identifier: Apache-2.0

/// The owner forms of the status page (brief M7-21 Scope and requirement 15): a component
/// (`createStatusComponent`, `updateStatusComponent`), an incident or a maintenance window
/// (`createStatusIncident`) and an update of one (`addStatusIncidentUpdate`). Each form builds
/// the generated request body of its operation, so nothing beyond the operation's fields is
/// sent; a change of a component sends the name (required by the operation) and only what
/// the owner changed, because the server keeps every member the body leaves out.
library;

import 'package:api_client/api_client.dart' as api;
import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import '../common/confirm_change.dart';
import 'status_labels.dart';

/// The longest component name the server accepts (D-M7-09-8).
const int statusNameMaxLength = 200;

/// The `health_source` keys an owner may choose (16 12): the four group keys and the single
/// checks; `agent.presence` belongs to the components of agents, which the server creates.
const List<String> statusHealthSources = [
  'api',
  'push',
  'files',
  'search',
  'api.http',
  'sync.fanout',
  'push.apns',
  'push.fcm',
  'files.storage',
  'files.scan',
  'search.query',
];

/// Shows the component form over [context]: a new component, or [existing] changed. [components]
/// are the page's components for "Place after". Returns the request body, or null for "Cancel".
Future<api.StatusComponentInput?> showStatusComponentDialog(
  BuildContext context, {
  required List<api.StatusComponent> components,
  api.StatusComponent? existing,
}) => showDialog<api.StatusComponentInput>(
  context: context,
  builder: (_) =>
      StatusComponentForm(components: components, existing: existing),
);

/// Shows the form of a new incident or maintenance window over [context]; [clock] gives its
/// `started_at`. Returns the request body, or null for "Cancel".
Future<api.StatusIncidentCreate?> showStatusIncidentDialog(
  BuildContext context, {
  required List<api.StatusComponent> components,
  required DateTime Function() clock,
}) => showDialog<api.StatusIncidentCreate>(
  context: context,
  builder: (_) => StatusIncidentForm(components: components, clock: clock),
);

/// Shows the form of an update of [incident] over [context]. Returns the request body, or null
/// for "Cancel".
Future<api.StatusIncidentUpdateCreate?> showStatusUpdateDialog(
  BuildContext context, {
  required api.StatusIncident incident,
}) => showDialog<api.StatusIncidentUpdateCreate>(
  context: context,
  builder: (_) => StatusUpdateForm(incident: incident),
);

/// The problem of the component name [name], or null when it is valid: empty or longer than
/// [statusNameMaxLength].
String? statusNameProblem(AppLocalizations l, String name) {
  final trimmed = name.trim();
  if (trimmed.isEmpty) return l.statusNameRequired;
  if (trimmed.length > statusNameMaxLength) {
    return l.statusNameTooLong(statusNameMaxLength);
  }
  return null;
}

/// The body of `createStatusComponent` (without [existing]) or `updateStatusComponent` from the
/// form's values: on a change only the name and what differs from [existing].
///
/// Example: renaming "Push" to "Push notifications" sends `{"name": "Push notifications"}`.
api.StatusComponentInput statusComponentInput({
  api.StatusComponent? existing,
  required String name,
  required String description,
  required api.StatusComponentState state,
  required String? healthSource,
  required String? afterComponentId,
}) => api.StatusComponentInput((b) {
  b.name = name.trim();
  final text = description.trim();
  if (existing == null) {
    if (text.isNotEmpty) b.description = text;
    b.state = state;
    if (healthSource != null) b.healthSource = healthSource;
  } else {
    if (text != existing.description) b.description = text;
    if (state != existing.state) b.state = state;
    if (existing.agentId == null &&
        (healthSource ?? '') != (existing.healthSource ?? '')) {
      b.healthSource = healthSource ?? '';
    }
  }
  if (afterComponentId != null) b.afterComponentId = afterComponentId;
});

/// The form of one component.
class StatusComponentForm extends StatefulWidget {
  /// Creates the form.
  const StatusComponentForm({
    super.key,
    required this.components,
    this.existing,
  });

  /// The page's components.
  final List<api.StatusComponent> components;

  /// The component to change, or null for a new one.
  final api.StatusComponent? existing;

  @override
  State<StatusComponentForm> createState() => _StatusComponentFormState();
}

class _StatusComponentFormState extends State<StatusComponentForm> {
  late final TextEditingController _name = TextEditingController(
    text: widget.existing?.name ?? '',
  );
  late final TextEditingController _description = TextEditingController(
    text: widget.existing?.description ?? '',
  );
  late api.StatusComponentState _state =
      widget.existing?.state ?? api.StatusComponentState.operational;
  late String? _source = switch (widget.existing?.healthSource) {
    final h? when h.isNotEmpty => h,
    _ => null,
  };
  String? _after;

  @override
  void dispose() {
    _name.dispose();
    _description.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final existing = widget.existing;
    final problem = statusNameProblem(l, _name.text);
    final sources = {...statusHealthSources, ?_source};
    final others = [
      for (final c in widget.components)
        if (c.id != existing?.id) c,
    ];
    return ConfirmChange(
      title: existing == null
          ? l.statusAddComponent
          : l.statusEditComponentTitle(existing.name),
      body: l.statusFormBody,
      confirmLabel: l.statusSave,
      canConfirm: problem == null,
      onConfirm: (_) => Navigator.of(context).pop(
        statusComponentInput(
          existing: existing,
          name: _name.text,
          description: _description.text,
          state: _state,
          healthSource: _source,
          afterComponentId: _after,
        ),
      ),
      fields: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          TextField(
            key: const ValueKey('status.form.name'),
            controller: _name,
            decoration: InputDecoration(
              labelText: l.statusComponentName,
              errorText: _name.text.isEmpty ? null : problem,
            ),
            onChanged: (_) => setState(() {}),
          ),
          const SizedBox(height: DsSpace.s12),
          TextField(
            key: const ValueKey('status.form.description'),
            controller: _description,
            decoration: InputDecoration(
              labelText: l.statusComponentDescription,
            ),
          ),
          const SizedBox(height: DsSpace.s12),
          DropdownButtonFormField<api.StatusComponentState>(
            key: const ValueKey('status.form.state'),
            initialValue: _state,
            decoration: InputDecoration(labelText: l.statusComponentState),
            items: [
              for (final s in const [
                api.StatusComponentState.operational,
                api.StatusComponentState.degraded,
                api.StatusComponentState.partialOutage,
                api.StatusComponentState.majorOutage,
                api.StatusComponentState.maintenance,
              ])
                DropdownMenuItem(
                  value: s,
                  child: Text(statusStateLook(context, s).label),
                ),
            ],
            onChanged: (s) => setState(() => _state = s ?? _state),
          ),
          if (existing?.agentId == null) ...[
            const SizedBox(height: DsSpace.s12),
            DropdownButtonFormField<String?>(
              key: const ValueKey('status.form.source'),
              initialValue: _source,
              decoration: InputDecoration(labelText: l.statusHealthSource),
              items: [
                DropdownMenuItem(child: Text(l.statusHealthSourceNone)),
                for (final s in sources)
                  DropdownMenuItem(value: s, child: Text(s)),
              ],
              onChanged: (s) => setState(() => _source = s),
            ),
          ],
          if (others.isNotEmpty) ...[
            const SizedBox(height: DsSpace.s12),
            DropdownButtonFormField<String?>(
              key: const ValueKey('status.form.after'),
              initialValue: _after,
              decoration: InputDecoration(labelText: l.statusPlaceAfter),
              items: [
                DropdownMenuItem(
                  child: Text(
                    existing == null ? l.statusPlaceEnd : l.statusPlaceKeep,
                  ),
                ),
                for (final c in others)
                  DropdownMenuItem(value: c.id, child: Text(dsIsolate(c.name))),
              ],
              onChanged: (s) => setState(() => _after = s),
            ),
          ],
        ],
      ),
    );
  }
}

/// The form of a new incident or maintenance window.
class StatusIncidentForm extends StatefulWidget {
  /// Creates the form.
  const StatusIncidentForm({
    super.key,
    required this.components,
    required this.clock,
  });

  /// The page's components, offered as the affected ones.
  final List<api.StatusComponent> components;

  /// Gives `started_at`.
  final DateTime Function() clock;

  @override
  State<StatusIncidentForm> createState() => _StatusIncidentFormState();
}

class _StatusIncidentFormState extends State<StatusIncidentForm> {
  final TextEditingController _title = TextEditingController();
  final TextEditingController _body = TextEditingController();
  api.StatusIncidentKind _kind = api.StatusIncidentKind.incident;
  api.StatusIncidentImpact _impact = api.StatusIncidentImpact.minor;
  api.StatusIncidentState _state = api.StatusIncidentState.investigating;
  final Set<String> _components = {};

  @override
  void dispose() {
    _title.dispose();
    _body.dispose();
    super.dispose();
  }

  void _setKind(api.StatusIncidentKind kind) => setState(() {
    _kind = kind;
    // The server's default state of the kind (D-M7-09-8).
    _state = kind == api.StatusIncidentKind.maintenance
        ? api.StatusIncidentState.scheduled
        : api.StatusIncidentState.investigating;
    if (kind == api.StatusIncidentKind.maintenance) {
      _impact = api.StatusIncidentImpact.none;
    }
  });

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final valid = _title.text.trim().isNotEmpty && _body.text.trim().isNotEmpty;
    return ConfirmChange(
      title: l.statusNewIncident,
      body: l.statusFormBody,
      confirmLabel: l.statusPublish,
      canConfirm: valid,
      onConfirm: (_) => Navigator.of(context).pop(
        api.StatusIncidentCreate(
          (b) => b
            ..title = _title.text.trim()
            ..kind = _kind
            ..impact = _impact
            ..state = _state
            ..startedAt = widget.clock().toUtc()
            ..componentIds.addAll(_components)
            ..bodyMd = _body.text.trim(),
        ),
      ),
      fields: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          SegmentedButton<api.StatusIncidentKind>(
            key: const ValueKey('status.form.kind'),
            showSelectedIcon: false,
            segments: [
              ButtonSegment(
                value: api.StatusIncidentKind.incident,
                label: Text(
                  l.statusKindIncident,
                  key: const ValueKey('status.form.kind.incident'),
                ),
              ),
              ButtonSegment(
                value: api.StatusIncidentKind.maintenance,
                label: Text(
                  l.statusKindMaintenance,
                  key: const ValueKey('status.form.kind.maintenance'),
                ),
              ),
            ],
            selected: {_kind},
            onSelectionChanged: (s) => _setKind(s.first),
          ),
          const SizedBox(height: DsSpace.s12),
          TextField(
            key: const ValueKey('status.form.title'),
            controller: _title,
            decoration: InputDecoration(labelText: l.statusIncidentTitle),
            onChanged: (_) => setState(() {}),
          ),
          const SizedBox(height: DsSpace.s12),
          DropdownButtonFormField<api.StatusIncidentImpact>(
            key: ValueKey('status.form.impact.${_kind.name}'),
            initialValue: _impact,
            decoration: InputDecoration(labelText: l.statusIncidentImpact),
            items: [
              for (final i in const [
                api.StatusIncidentImpact.none,
                api.StatusIncidentImpact.minor,
                api.StatusIncidentImpact.major,
                api.StatusIncidentImpact.critical,
              ])
                DropdownMenuItem(
                  value: i,
                  child: Text(statusImpactLabel(l, i)),
                ),
            ],
            onChanged: (i) => setState(() => _impact = i ?? _impact),
          ),
          const SizedBox(height: DsSpace.s12),
          _StateField(
            key: ValueKey('status.form.state.${_kind.name}'),
            value: _state,
            onChanged: (s) => setState(() => _state = s),
          ),
          if (widget.components.isNotEmpty) ...[
            const SizedBox(height: DsSpace.s12),
            Text(
              l.statusIncidentComponents,
              style: dsTextStyle(DsType.label)
                  .copyWith(color: context.dsColors.textSecondary),
            ),
            for (final c in widget.components)
              CheckboxListTile(
                key: ValueKey('status.form.component.${c.id}'),
                contentPadding: EdgeInsets.zero,
                controlAffinity: ListTileControlAffinity.leading,
                value: _components.contains(c.id),
                title: Text(dsIsolate(c.name)),
                onChanged: (on) => setState(
                  () => on ?? false
                      ? _components.add(c.id)
                      : _components.remove(c.id),
                ),
              ),
          ],
          const SizedBox(height: DsSpace.s12),
          TextField(
            key: const ValueKey('status.form.body'),
            controller: _body,
            minLines: 3,
            maxLines: 8,
            decoration: InputDecoration(
              labelText: l.statusIncidentText,
              helperText: l.statusMarkdownHint,
            ),
            onChanged: (_) => setState(() {}),
          ),
        ],
      ),
    );
  }
}

/// The form of an update of one incident: its new state and the text.
class StatusUpdateForm extends StatefulWidget {
  /// Creates the form.
  const StatusUpdateForm({super.key, required this.incident});

  /// The incident.
  final api.StatusIncident incident;

  @override
  State<StatusUpdateForm> createState() => _StatusUpdateFormState();
}

class _StatusUpdateFormState extends State<StatusUpdateForm> {
  final TextEditingController _body = TextEditingController();
  late api.StatusIncidentState _state = widget.incident.state;

  @override
  void dispose() {
    _body.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return ConfirmChange(
      title: l.statusUpdateTitle(widget.incident.title),
      body: l.statusFormBody,
      confirmLabel: l.statusPublish,
      canConfirm: _body.text.trim().isNotEmpty,
      onConfirm: (_) => Navigator.of(context).pop(
        api.StatusIncidentUpdateCreate(
          (b) => b
            ..state = _state
            ..bodyMd = _body.text.trim(),
        ),
      ),
      fields: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _StateField(
            value: _state,
            onChanged: (s) => setState(() => _state = s),
          ),
          const SizedBox(height: DsSpace.s12),
          TextField(
            key: const ValueKey('status.form.body'),
            controller: _body,
            minLines: 3,
            maxLines: 8,
            decoration: InputDecoration(
              labelText: l.statusIncidentText,
              helperText: l.statusMarkdownHint,
            ),
            onChanged: (_) => setState(() {}),
          ),
        ],
      ),
    );
  }
}

class _StateField extends StatelessWidget {
  const _StateField({super.key, required this.value, required this.onChanged});

  final api.StatusIncidentState value;
  final ValueChanged<api.StatusIncidentState> onChanged;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return DropdownButtonFormField<api.StatusIncidentState>(
      key: const ValueKey('status.form.incidentState'),
      initialValue: value,
      decoration: InputDecoration(labelText: l.statusIncidentStateLabel),
      items: [
        for (final s in const [
          api.StatusIncidentState.scheduled,
          api.StatusIncidentState.investigating,
          api.StatusIncidentState.identified,
          api.StatusIncidentState.monitoring,
          api.StatusIncidentState.resolved,
        ])
          DropdownMenuItem(
            value: s,
            child: Text(statusIncidentStateLabel(l, s)),
          ),
      ],
      onChanged: (s) {
        if (s != null) onChanged(s);
      },
    );
  }
}
