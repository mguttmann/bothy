// SPDX-License-Identifier: Apache-2.0

/// The one editable preview of "New agent" (docs/spec/01-product.md 6.20 item 3;
/// docs/spec/26-builtin-agents.md 8.2; brief A1-08 Design note 4): identity, instructions,
/// skills, MCP servers with the secret names for the runner host, settings with the tool class
/// and what it does not load, billing, model and cost cap, the hosting runner (read-only once
/// the agent exists) and the channel. A field error of the server shows at its field by JSON
/// Pointer; one without a field shows in a list below.
///
/// The same preview edits an existing built-in agent (A1-09, [NewAgentMode]): the identity
/// and the runner are the agent's, the billing mode stays (the agent card switches it), an
/// edit creates no channel, and "Show" of a stored version is read-only.
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/material.dart';
import 'package:l10n/l10n.dart';

import 'new_agent_draft.dart';
import 'new_agent_flow.dart';
import 'new_agent_port.dart';
import 'new_agent_texts.dart';

/// The preview of [flow]; every edit goes through [NewAgentFlow.edit].
///
/// Example: `NewAgentPreview(flow: flow)`.
class NewAgentPreview extends StatelessWidget {
  /// Creates the preview.
  const NewAgentPreview({super.key, required this.flow});

  /// The flow whose draft it edits.
  final NewAgentFlow flow;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final d = flow.draft!;
    final invalid = flow.invalidFields;
    String? error(String pointer, String clientText) {
      final server = flow.errorsAt(pointer);
      if (server.isNotEmpty) {
        return server
            .map((c) => newAgentFieldErrorText(l, c, pointer: pointer))
            .join(' ');
      }
      return flow.attempted && invalid.contains(pointer) ? clientText : null;
    }

    final cost = flow.costMicros;
    return Column(
      key: const ValueKey('newAgent.preview'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        _Hint(l.newAgentPreviewIntro),
        if (cost != null)
          _Hint(
            l.newAgentInterviewCost(
              dsFormat(context).money(cost / 1000000, flow.currency ?? 'USD'),
            ),
            key: const ValueKey('newAgent.cost'),
          ),
        _Section(l.newAgentSectionIdentity),
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Padding(
              padding: const EdgeInsets.only(
                right: DsSpace.s12,
                top: DsSpace.s8,
              ),
              child: ExcludeSemantics(child: _Initials(d.name)),
            ),
            Expanded(
              child: Column(
                children: [
                  _Field(
                    id: 'newAgent.name',
                    label: l.newAgentName,
                    initial: d.name,
                    enabled: flow.canEditIdentity,
                    error: error(PreviewField.name, l.newAgentNameMissing),
                    onChanged: (v) => flow.edit((x) => x.name = v),
                  ),
                  _Field(
                    id: 'newAgent.handle',
                    label: l.newAgentHandle,
                    helper: l.newAgentHandleHint,
                    initial: d.handle,
                    enabled: flow.canEditIdentity,
                    error: error(PreviewField.handle, l.newAgentHandleInvalid),
                    onChanged: (v) => flow.edit((x) => x.handle = v.trim()),
                  ),
                ],
              ),
            ),
          ],
        ),
        _Field(
          id: 'newAgent.description',
          label: l.newAgentDescription,
          initial: d.description,
          enabled: flow.canEditIdentity,
          error: error(PreviewField.description, ''),
          onChanged: (v) => flow.edit((x) => x.description = v),
        ),
        _Hint(l.newAgentAvatarNote),
        _Section(l.newAgentSectionInstructions),
        _Field(
          id: 'newAgent.instructions',
          label: l.newAgentSectionInstructions,
          helper: l.newAgentInstructionsHint,
          initial: d.instructions,
          lines: 6,
          monospace: true,
          enabled: flow.canEditContent,
          error: error('/instructions', ''),
          onChanged: (v) => flow.edit((x) => x.instructions = v),
        ),
        _Section(l.newAgentSectionSkills),
        for (final (i, s) in d.skills.indexed)
          _SkillEditor(
            key: ObjectKey(s),
            flow: flow,
            index: i,
            skill: s,
            error: error,
          ),
        Align(
          alignment: AlignmentDirectional.centerStart,
          child: TextButton.icon(
            key: const ValueKey('newAgent.skill.add'),
            onPressed: !flow.canEditContent
                ? null
                : () => flow.edit((x) => x.skills.add(DraftSkill())),
            icon: const Icon(Icons.add),
            label: Text(l.newAgentSkillAdd),
          ),
        ),
        _Section(l.newAgentSectionServers),
        for (final (i, s) in d.servers.indexed)
          _ServerEditor(
            key: ObjectKey(s),
            flow: flow,
            index: i,
            server: s,
            error: error,
          ),
        Align(
          alignment: AlignmentDirectional.centerStart,
          child: TextButton.icon(
            key: const ValueKey('newAgent.server.add'),
            onPressed: !flow.canEditContent
                ? null
                : () => flow.edit((x) => x.servers.add(DraftMcpServer())),
            icon: const Icon(Icons.add),
            label: Text(l.newAgentServerAdd),
          ),
        ),
        _Secrets(names: d.secretNames),
        _Section(l.newAgentSectionSettings),
        _ClassChoice(flow: flow),
        _Field(
          id: 'newAgent.allowedTools',
          label: l.newAgentAllowedTools,
          helper: l.newAgentAllowedToolsHint,
          initial: d.allowedTools?.join('\n') ?? '',
          lines: 3,
          monospace: true,
          enabled: flow.canEditContent,
          error: error('/settings/allowed_tools', ''),
          onChanged: (v) => flow.edit((x) {
            final tools = [
              for (final t in v.split('\n'))
                if (t.trim().isNotEmpty) t.trim(),
            ];
            x.allowedTools = tools.isEmpty ? null : tools;
          }),
        ),
        _Field(
          id: 'newAgent.model',
          label: l.newAgentModel,
          helper: l.newAgentModelHint,
          initial: d.model,
          enabled: flow.canEditContent,
          error: error('/settings/model', ''),
          onChanged: (v) => flow.edit((x) => x.model = v),
        ),
        _Field(
          id: 'newAgent.costCap',
          label: l.newAgentCostCap,
          helper: l.newAgentCostCapHint,
          initial: d.costCap,
          enabled: flow.canEditContent,
          keyboard: const TextInputType.numberWithOptions(decimal: true),
          error:
              error(PreviewField.costCap, l.newAgentCostCapInvalid) ??
              (flow.invalidFields.contains(PreviewField.costCap)
                  ? l.newAgentCostCapInvalid
                  : null),
          onChanged: (v) => flow.edit((x) => x.costCap = v),
        ),
        _BillingChoice(flow: flow),
        _Section(l.newAgentSectionHost),
        _HostChoice(
          flow: flow,
          error: error(PreviewField.host, l.newAgentHostMissing),
        ),
        if (flow.canEditChannel || flow.channelId != null) ...[
          _Section(l.newAgentSectionChannel),
          _ChannelChoice(flow: flow, error: error),
        ],
        _OtherErrors(flow: flow),
      ],
    );
  }
}

/// A section heading, announced as a header.
class _Section extends StatelessWidget {
  const _Section(this.text);

  final String text;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(top: DsSpace.s16, bottom: DsSpace.s8),
    child: Semantics(
      header: true,
      child: Text(
        text,
        style: dsTextStyle(DsType.bodyStrong)
            .copyWith(color: context.dsColors.textPrimary),
      ),
    ),
  );
}

/// A line of secondary text.
class _Hint extends StatelessWidget {
  const _Hint(this.text, {super.key});

  final String text;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(bottom: DsSpace.s8),
    child: Text(
      text,
      style: dsTextStyle(DsType.bodySm)
          .copyWith(color: context.dsColors.textSecondary),
    ),
  );
}

/// The initials of the agent's name in a circle (the agent has no picture at creation).
class _Initials extends StatelessWidget {
  const _Initials(this.name);

  final String name;

  @override
  Widget build(BuildContext context) {
    final colors = context.dsColors;
    final words = name.trim().split(RegExp(r'\s+')).where((w) => w.isNotEmpty);
    final text = words
        .take(2)
        .map((w) => w.characters.first.toUpperCase())
        .join();
    return CircleAvatar(
      radius: DsSpace.s24,
      backgroundColor: colors.surfaceRaised,
      child: Text(
        text.isEmpty ? '?' : text,
        style: dsTextStyle(DsType.bodyStrong)
            .copyWith(color: colors.textPrimary),
      ),
    );
  }
}

/// A text field that keeps its own controller; [initial] is read once, edits go to
/// [onChanged]. A new [key] starts it again.
class _Field extends StatefulWidget {
  _Field({
    required this.id,
    required this.label,
    required this.initial,
    required this.onChanged,
    this.helper,
    this.error,
    this.enabled = true,
    this.lines = 1,
    this.monospace = false,
    this.keyboard,
  }) : super(key: ValueKey(id));

  final String id;
  final String label;
  final String initial;
  final ValueChanged<String> onChanged;
  final String? helper;
  final String? error;
  final bool enabled;
  final int lines;
  final bool monospace;
  final TextInputType? keyboard;

  @override
  State<_Field> createState() => _FieldState();
}

class _FieldState extends State<_Field> {
  late final TextEditingController _controller = TextEditingController(
    text: widget.initial,
  );

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final error = widget.error;
    final helper = widget.helper;
    // The helper stands below the field, not in it: a text field with a child is a leaf to
    // VoiceOver on macOS, which would never read the child.
    return Padding(
      padding: const EdgeInsets.only(bottom: DsSpace.s12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          TextField(
            key: ValueKey('${widget.id}.field'),
            controller: _controller,
            enabled: widget.enabled,
            minLines: widget.lines,
            maxLines: widget.lines == 1 ? 1 : null,
            keyboardType: widget.keyboard,
            style: widget.monospace ? dsTextStyle(DsType.code) : null,
            onChanged: widget.onChanged,
            decoration: InputDecoration(
              labelText: widget.label,
              errorText: error == null || error.isEmpty ? null : error,
              errorMaxLines: 4,
            ),
          ),
          if (helper != null)
            Padding(
              padding: const EdgeInsets.only(top: DsSpace.s4),
              child: Text(
                helper,
                style: dsTextStyle(DsType.bodySm)
                    .copyWith(color: context.dsColors.textSecondary),
              ),
            ),
        ],
      ),
    );
  }
}

typedef _ErrorOf = String? Function(String pointer, String clientText);

/// One skill: name, description, body, extra files and "Remove".
class _SkillEditor extends StatelessWidget {
  const _SkillEditor({
    super.key,
    required this.flow,
    required this.index,
    required this.skill,
    required this.error,
  });

  final NewAgentFlow flow;
  final int index;
  final DraftSkill skill;
  final _ErrorOf error;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final base = '/skills/$index';
    final id = 'newAgent.skill.$index';
    return _Card(
      children: [
        Row(
          children: [
            Expanded(
              child: _Field(
                id: '$id.name',
                label: l.newAgentSkillName,
                initial: skill.name,
                enabled: flow.canEditContent,
                error: error('$base/name', ''),
                onChanged: (v) => flow.edit((_) => skill.name = v.trim()),
              ),
            ),
            IconButton(
              key: ValueKey('$id.remove'),
              tooltip: l.newAgentSkillRemove(skill.name),
              onPressed: !flow.canEditContent
                  ? null
                  : () => flow.edit((x) => x.skills.remove(skill)),
              icon: const Icon(Icons.delete_outline),
            ),
          ],
        ),
        _Field(
          id: '$id.description',
          label: l.newAgentSkillDescription,
          initial: skill.description,
          enabled: flow.canEditContent,
          error: error('$base/description', ''),
          onChanged: (v) => flow.edit((_) => skill.description = v),
        ),
        _Field(
          id: '$id.body',
          label: l.newAgentSkillBody,
          initial: skill.body,
          lines: 4,
          monospace: true,
          enabled: flow.canEditContent,
          error: error('$base/body', ''),
          onChanged: (v) => flow.edit((_) => skill.body = v),
        ),
        if (skill.files.isNotEmpty)
          _Hint(l.newAgentSkillFiles(skill.files.length)),
      ],
    );
  }
}

/// One MCP server: name, transport, the transport's fields, the entries and what the chosen
/// class does with it.
class _ServerEditor extends StatelessWidget {
  const _ServerEditor({
    super.key,
    required this.flow,
    required this.index,
    required this.server,
    required this.error,
  });

  final NewAgentFlow flow;
  final int index;
  final DraftMcpServer server;
  final _ErrorOf error;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final colors = context.dsColors;
    final base = '/mcp_servers/$index';
    final id = 'newAgent.server.$index';
    final target = flow.targetClass;
    final loaded = switch (target) {
      AgentToolClass.chat => false,
      AgentToolClass.scoped => !server.isStdio,
      AgentToolClass.host => true,
    };
    final entriesKey = server.isStdio ? 'env' : 'headers';
    return _Card(
      children: [
        Row(
          children: [
            Expanded(
              child: _Field(
                id: '$id.name',
                label: l.newAgentServerName,
                initial: server.name,
                enabled: flow.canEditContent,
                error: error('$base/name', ''),
                onChanged: (v) => flow.edit((_) => server.name = v.trim()),
              ),
            ),
            IconButton(
              key: ValueKey('$id.remove'),
              tooltip: l.newAgentServerRemove(server.name),
              onPressed: !flow.canEditContent
                  ? null
                  : () => flow.edit((x) => x.servers.remove(server)),
              icon: const Icon(Icons.delete_outline),
            ),
          ],
        ),
        Wrap(
          spacing: DsSpace.s8,
          children: [
            for (final (transport, label) in [
              ('http', l.newAgentServerHttp),
              ('stdio', l.newAgentServerStdio),
            ])
              ChoiceChip(
                key: ValueKey('$id.transport.$transport'),
                label: Text(label),
                selected: server.transport == transport,
                onSelected: !flow.canEditContent
                    ? null
                    : (_) => flow.edit((_) => server.transport = transport),
              ),
          ],
        ),
        const SizedBox(height: DsSpace.s8),
        if (error('$base/transport', '') case final e?)
          Text(
            e,
            style: dsTextStyle(DsType.bodySm).copyWith(color: colors.dangerFg),
          ),
        if (!server.isStdio)
          _Field(
            id: '$id.url',
            label: l.newAgentServerUrl,
            initial: server.url,
            enabled: flow.canEditContent,
            error: error('$base/url', ''),
            onChanged: (v) => flow.edit((_) => server.url = v.trim()),
          )
        else ...[
          _Field(
            id: '$id.command',
            label: l.newAgentServerCommand,
            initial: server.command,
            enabled: flow.canEditContent,
            error: error('$base/command', ''),
            onChanged: (v) => flow.edit((_) => server.command = v.trim()),
          ),
          _Field(
            id: '$id.package',
            label: l.newAgentServerPackage,
            initial: server.package,
            enabled: flow.canEditContent,
            error: error('$base/package', ''),
            onChanged: (v) => flow.edit((_) => server.package = v.trim()),
          ),
          _Field(
            id: '$id.args',
            label: l.newAgentServerArgs,
            initial: server.args.join('\n'),
            lines: 2,
            monospace: true,
            enabled: flow.canEditContent,
            error: error('$base/args', ''),
            onChanged: (v) => flow.edit(
              (_) => server.args
                ..clear()
                ..addAll([
                  for (final a in v.split('\n'))
                    if (a.trim().isNotEmpty) a.trim(),
                ]),
            ),
          ),
        ],
        Text(
          server.isStdio ? l.newAgentServerEnv : l.newAgentServerHeaders,
          style: dsTextStyle(DsType.bodySm)
              .copyWith(color: colors.textSecondary),
        ),
        for (final (j, e) in server.entries.indexed)
          _EntryEditor(
            key: ObjectKey(e),
            flow: flow,
            id: '$id.entry.$j',
            pointer: '$base/$entriesKey/$j',
            entry: e,
            onRemove: () => flow.edit((_) => server.entries.remove(e)),
            error: error,
          ),
        Align(
          alignment: AlignmentDirectional.centerStart,
          child: TextButton.icon(
            key: ValueKey('$id.entry.add'),
            onPressed: !flow.canEditContent
                ? null
                : () => flow.edit((_) => server.entries.add(DraftEntry())),
            icon: const Icon(Icons.add),
            label: Text(l.newAgentEntryAdd),
          ),
        ),
        if (!loaded)
          Text(
            l.newAgentServerNotLoaded(toolClassName(l, target)),
            key: ValueKey('$id.notLoaded'),
            style: dsTextStyle(DsType.bodySm).copyWith(color: colors.warningFg),
          ),
        if (server.isStdio && target != AgentToolClass.host)
          Text(
            l.newAgentServerNeedsHost,
            style: dsTextStyle(DsType.bodySm).copyWith(color: colors.warningFg),
          ),
      ],
    );
  }
}

/// A header or an environment entry: name, plain value or secret reference.
class _EntryEditor extends StatelessWidget {
  const _EntryEditor({
    super.key,
    required this.flow,
    required this.id,
    required this.pointer,
    required this.entry,
    required this.onRemove,
    required this.error,
  });

  final NewAgentFlow flow;
  final String id;
  final String pointer;
  final DraftEntry entry;
  final VoidCallback onRemove;
  final _ErrorOf error;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Expanded(
          child: _Field(
            id: '$id.name',
            label: l.newAgentEntryName,
            initial: entry.name,
            enabled: flow.canEditContent,
            error: error('$pointer/name', ''),
            onChanged: (v) => flow.edit((_) => entry.name = v.trim()),
          ),
        ),
        const SizedBox(width: DsSpace.s8),
        Expanded(
          child: _Field(
            // A new field when the kind changes, so it shows the value of that kind.
            id: '$id.${entry.isSecret ? 'secret' : 'value'}',
            label: entry.isSecret
                ? l.newAgentEntrySecret
                : l.newAgentEntryValue,
            initial: entry.isSecret ? entry.secretRef : entry.value,
            enabled: flow.canEditContent,
            error:
                error(
                  '$pointer/${entry.isSecret ? 'secret_ref' : 'value'}',
                  '',
                ) ??
                error(pointer, ''),
            onChanged: (v) => flow.edit((_) {
              if (entry.isSecret) {
                entry.secretRef = v.trim();
              } else {
                entry.value = v;
              }
            }),
          ),
        ),
        MergeSemantics(
          child: Column(
            children: [
              Text(l.newAgentEntryIsSecret, style: dsTextStyle(DsType.bodySm)),
              Switch(
                key: ValueKey('$id.isSecret'),
                value: entry.isSecret,
                onChanged: !flow.canEditContent
                    ? null
                    : (v) => flow.edit((_) => entry.isSecret = v),
              ),
            ],
          ),
        ),
        IconButton(
          key: ValueKey('$id.remove'),
          tooltip: l.newAgentEntryRemove(entry.name),
          onPressed: !flow.canEditContent ? null : onRemove,
          icon: const Icon(Icons.close),
        ),
      ],
    );
  }
}

/// The secret names to add on the runner host (01 6.20 item 3).
class _Secrets extends StatelessWidget {
  const _Secrets({required this.names});

  final List<String> names;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    return Column(
      key: const ValueKey('newAgent.secrets'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _Section(l.newAgentSecretsTitle),
        if (names.isEmpty)
          _Hint(l.newAgentSecretsNone)
        else ...[
          for (final n in names)
            Padding(
              padding: const EdgeInsets.only(bottom: DsSpace.s4),
              child: Text(n, style: dsTextStyle(DsType.code)),
            ),
          _Hint(l.newAgentSecretsHint),
        ],
      ],
    );
  }
}

/// The tool class with its explanation (26 4.4, 5.7); a class above the runner's ceiling cannot
/// be chosen.
class _ClassChoice extends StatelessWidget {
  const _ClassChoice({required this.flow});

  final NewAgentFlow flow;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final ceiling = flow.runner?.ceiling;
    final target = flow.targetClass;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(l.newAgentClass, style: dsTextStyle(DsType.bodyStrong)),
        const SizedBox(height: DsSpace.s8),
        Wrap(
          spacing: DsSpace.s8,
          runSpacing: DsSpace.s8,
          children: [
            for (final c in AgentToolClass.values)
              ChoiceChip(
                key: ValueKey('newAgent.class.${c.wire}'),
                label: Text(toolClassName(l, c)),
                selected: target == c,
                onSelected:
                    !flow.canEditContent ||
                        (ceiling != null && c.isAbove(ceiling))
                    ? null
                    : (_) => flow.edit((x) => x.pickClass(c)),
              ),
          ],
        ),
        const SizedBox(height: DsSpace.s8),
        _Hint(
          toolClassExplanation(l, target),
          key: const ValueKey('newAgent.classExplain'),
        ),
        _Hint(l.newAgentClassHostFiles),
        if (ceiling != null && ceiling != AgentToolClass.host)
          if (flow.attempted &&
              flow.invalidFields.contains(PreviewField.toolClass))
            Text(
              l.newAgentClassCeiling(
                flow.runner!.name,
                toolClassName(l, ceiling),
              ),
              key: const ValueKey('newAgent.classAboveCeiling'),
              style: dsTextStyle(DsType.bodySm)
                  .copyWith(color: context.dsColors.dangerFg),
            )
          else
            _Hint(
              l.newAgentClassCeiling(
                flow.runner!.name,
                toolClassName(l, ceiling),
              ),
            ),
      ],
    );
  }
}

/// Whether the billing choice may change: not while a step runs and not for an edit, which
/// keeps the agent's billing mode.
bool _billingEditable(NewAgentFlow flow) =>
    flow.canEditContent && flow.mode != NewAgentMode.edit;

/// Organization or personal (10 10.3); personal only when offered, otherwise the reason.
class _BillingChoice extends StatelessWidget {
  const _BillingChoice({required this.flow});

  final NewAgentFlow flow;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final offer = flow.personalOffer;
    if (flow.mode == NewAgentMode.edit || flow.mode == NewAgentMode.view) {
      // An edit keeps the agent's mode; "Switch backend" of the agent card changes it.
      final personal = flow.billing == 'personal';
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(l.newAgentBilling, style: dsTextStyle(DsType.bodyStrong)),
          const SizedBox(height: DsSpace.s8),
          Wrap(
            children: [
              ChoiceChip(
                key: ValueKey('newAgent.billing.${flow.billing}'),
                label: Text(
                  personal
                      ? l.newAgentBillingPersonal
                      : l.newAgentBillingOrganization,
                ),
                selected: true,
                onSelected: null,
              ),
            ],
          ),
        ],
      );
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(l.newAgentBilling, style: dsTextStyle(DsType.bodyStrong)),
        const SizedBox(height: DsSpace.s8),
        Wrap(
          spacing: DsSpace.s8,
          children: [
            ChoiceChip(
              key: const ValueKey('newAgent.billing.organization'),
              label: Text(l.newAgentBillingOrganization),
              selected: flow.billing == 'organization',
              onSelected: !_billingEditable(flow)
                  ? null
                  : (_) => flow.billing = 'organization',
            ),
            if (offer == PersonalOffer.offered)
              ChoiceChip(
                key: const ValueKey('newAgent.billing.personal'),
                label: Text(l.newAgentBillingPersonal),
                selected: flow.billing == 'personal',
                onSelected: !_billingEditable(flow)
                    ? null
                    : (_) => flow.billing = 'personal',
              ),
          ],
        ),
        if (offer != PersonalOffer.offered)
          Padding(
            padding: const EdgeInsets.only(top: DsSpace.s4),
            child: _Hint(
              personalOfferText(l, offer),
              key: const ValueKey('newAgent.personalNot'),
            ),
          ),
      ],
    );
  }
}

/// The hosting runner: the runners that report `hosts_agents`; read-only once the agent exists
/// (26 8.3).
class _HostChoice extends StatelessWidget {
  const _HostChoice({required this.flow, required this.error});

  final NewAgentFlow flow;
  final String? error;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final runners = flow.offer.runners;
    final e = error;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        DsDropdownField<HostingRunner>(
          key: ValueKey('newAgent.host.${flow.runner?.id}.${flow.canEditHost}'),
          label: l.newAgentHost,
          enabled: flow.canEditHost,
          initialSelection: flow.runner,
          entries: [
            for (final r in runners) DropdownMenuEntry(value: r, label: r.name),
          ],
          onSelected: (r) => flow.runner = r,
        ),
        if (!flow.canEditHost && flow.agent != null)
          Padding(
            padding: const EdgeInsets.only(top: DsSpace.s4),
            child: _Hint(
              l.newAgentHostFixed,
              key: const ValueKey('newAgent.hostFixed'),
            ),
          ),
        if (e != null)
          Text(
            e,
            style: dsTextStyle(DsType.bodySm)
                .copyWith(color: context.dsColors.dangerFg),
          ),
      ],
    );
  }
}

/// The channel to create, or none.
class _ChannelChoice extends StatelessWidget {
  const _ChannelChoice({required this.flow, required this.error});

  final NewAgentFlow flow;
  final _ErrorOf error;

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final d = flow.draft!;
    final spaces = flow.context?.spaces ?? const <NewAgentSpace>[];
    final spaceError = error(
      PreviewField.channelSpace,
      l.newAgentChannelSpaceMissing,
    );
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        CheckboxListTile(
          key: const ValueKey('newAgent.channel.wanted'),
          contentPadding: EdgeInsets.zero,
          controlAffinity: ListTileControlAffinity.leading,
          value: d.channelWanted,
          onChanged: flow.canEditChannel
              ? (v) => flow.edit((x) {
                  x.channelWanted = v ?? false;
                  // The first space is preselected; the person may pick another.
                  if (x.channelWanted &&
                      x.channelSpaceId == null &&
                      spaces.isNotEmpty) {
                    x.channelSpaceId = spaces.first.id;
                  }
                })
              : null,
          title: Text(l.newAgentChannelWanted),
        ),
        if (d.channelWanted) ...[
          DsDropdownField<String>(
            key: ValueKey(
              'newAgent.channel.space.${spaces.length}.${d.channelSpaceId}',
            ),
            label: l.newAgentChannelSpace,
            enabled: flow.canEditChannel && spaces.isNotEmpty,
            initialSelection: d.channelSpaceId,
            entries: [
              for (final s in spaces)
                DropdownMenuEntry(value: s.id, label: s.name),
            ],
            onSelected: (id) => flow.edit((x) => x.channelSpaceId = id),
          ),
          if (spaceError != null)
            Text(
              spaceError,
              style: dsTextStyle(DsType.bodySm)
                  .copyWith(color: context.dsColors.dangerFg),
            ),
          const SizedBox(height: DsSpace.s8),
          _Field(
            id: 'newAgent.channel.name',
            label: l.newAgentChannelName,
            initial: d.channelName,
            enabled: flow.canEditChannel,
            error: error(
              PreviewField.channelName,
              l.newAgentChannelNameMissing,
            ),
            onChanged: (v) => flow.edit((x) => x.channelName = v),
          ),
        ],
      ],
    );
  }
}

/// The server's field errors that no field of the preview shows.
class _OtherErrors extends StatelessWidget {
  const _OtherErrors({required this.flow});

  final NewAgentFlow flow;

  static final RegExp _shown = RegExp(
    r'^/(name|handle|description|hosted_by|instructions|channel/(space|name)|'
    r'settings/(model|allowed_tools|max_cost_usd_per_run)|'
    r'skills/\d+/(name|description|body)|'
    r'mcp_servers/\d+/(name|transport|url|command|package|args)|'
    r'mcp_servers/\d+/(headers|env)/\d+(/(name|value|secret_ref))?)$',
  );

  @override
  Widget build(BuildContext context) {
    final l = AppLocalizations.of(context);
    final other = [
      for (final MapEntry(:key, :value) in flow.fieldErrors.entries)
        if (!_shown.hasMatch(key))
          for (final code in value) '$key: ${newAgentFieldErrorText(l, code)}',
    ];
    if (other.isEmpty) return const SizedBox.shrink();
    final colors = context.dsColors;
    return Padding(
      key: const ValueKey('newAgent.otherErrors'),
      padding: const EdgeInsets.only(top: DsSpace.s12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(l.newAgentOtherErrors, style: dsTextStyle(DsType.bodyStrong)),
          for (final line in other)
            Text(
              line,
              style: dsTextStyle(DsType.bodySm)
                  .copyWith(color: colors.dangerFg),
            ),
        ],
      ),
    );
  }
}

/// A bordered group of one skill or server.
class _Card extends StatelessWidget {
  const _Card({required this.children});

  final List<Widget> children;

  @override
  Widget build(BuildContext context) => Container(
    margin: const EdgeInsets.only(bottom: DsSpace.s12),
    padding: const EdgeInsets.all(DsSpace.s12),
    decoration: BoxDecoration(
      border: Border.all(color: context.dsColors.borderSubtle),
      borderRadius: BorderRadius.circular(DsSpace.s8),
    ),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: children,
    ),
  );
}
