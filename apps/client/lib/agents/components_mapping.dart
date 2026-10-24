// SPDX-License-Identifier: Apache-2.0

/// Components on messages (09 11; 01 6.5; components.md 8; SEC-OUT-4, SEC-T14; brief M4-21
/// Design note 3): the card data and the header of `DsMessageComponents` built only from the
/// server's fields, and whether the reader may use the card.
///
/// The header never reads the agent's text: its inputs are the principal of the message, the
/// run of the component, `required_capability` and `restricted_to`. An agent that writes
/// "Approval needed" or "Only the owner can approve this" into its text gets exactly that text
/// below the divider "Text from `agent`" and nothing else (T-AI-05).
library;

import 'package:design_system/design_system.dart';
import 'package:domain/domain.dart';
import 'package:l10n/l10n.dart';
import 'package:local_store/local_store.dart';

/// What the server reported about the reader, for [cardPermission]: the access of the
/// channel ([access], from the snapshot and `access.changed`), the signed-in person [me]
/// and whether that person is an owner ([owner], `getMe`).
typedef ReaderFacts = ({AccessView? access, PrincipalId? me, bool owner});

/// Whether the reader may use [components] and, if not, the translated reason (09 11.2;
/// components.md 8 "active, not permitted").
///
/// No own derivation from grants (15 1 principle 1): the check reads only the level and the
/// capabilities the server reported for the channel, the component's `required_capability`
/// and `restricted_to`, the `triggered_by` of its [run] and the owner flag of `getMe`.
/// Effective owners hold every capability and meet every restriction (09 11.2). The server
/// checks again; its refusal shows as the card's error (the outbox entry's problem).
({bool permitted, String? reason}) cardPermission(
  AppLocalizations l,
  List<MessageComponent> components,
  ReaderFacts reader, {
  RunSummary? run,
  String? runStarterName,
}) {
  final access = reader.access;
  if (access == null || access.level != AccessLevel.trigger) {
    return (permitted: false, reason: l.componentsDeniedLevel);
  }
  if (reader.owner) return (permitted: true, reason: null);
  final first = components.isEmpty ? null : components.first;
  final capability = first?.requiredCapability;
  if (capability != null && !access.capabilities.contains(capability)) {
    return (
      permitted: false,
      reason: l.componentsDeniedCapability(capabilityLabel(l, capability)),
    );
  }
  switch (first?.restrictedTo) {
    case ComponentRestriction.runStarter:
      if (run == null || run.triggeredBy != reader.me) {
        return (
          permitted: false,
          reason: runStarterName == null
              ? l.componentsApproverOwner
              : l.componentsApproverRunStarter(dsIsolate(runStarterName)),
        );
      }
    case ComponentRestriction.owner:
      return (permitted: false, reason: l.componentsApproverOwner);
    case ComponentRestriction.any || ComponentRestriction.unknown || null:
      break;
  }
  return (permitted: true, reason: null);
}

/// The translated name of [capability] for the header and the lock note.
String capabilityLabel(AppLocalizations l, Capability capability) =>
    switch (capability) {
      Capability.approveActions => l.capabilityApproveActions,
      Capability.upload => l.adminCapabilityUpload,
      Capability.export => l.adminCapabilityExport,
      Capability.manage => l.adminCapabilityManage,
      Capability.terminal => l.adminCapabilityTerminal,
      Capability.unknown => capability.code,
    };

/// The header of the card of [message] (SEC-OUT-4): the agent from the message's principal in
/// [principals], the run label from [runLabel] (the run of the components, never a text of
/// the agent), the capability and the approver rule of the first component, and the name of
/// the run's triggering person for `run_starter`.
DsComponentHeader componentHeader(
  Message message,
  List<MessageComponent> components, {
  required Map<PrincipalId, Principal> principals,
  String? runLabel,
  String? runStarterName,
}) {
  final first = components.isEmpty ? null : components.first;
  return DsComponentHeader(
    agentName:
        principals[message.authorId]?.displayName ?? message.authorId.value,
    runLabel: runLabel,
    requiredCapability: first?.requiredCapability?.code,
    approver: DsApprover.fromCode(first?.restrictedTo.code),
    runStarterName: runStarterName,
  );
}

/// The card data of [message] with [components] in `position` order.
///
/// The common state follows 09 11.3: one `used` component makes the card `used` (with who,
/// the label of the choice and [timeLabel] of `used_at`); else an `active` one that has not
/// passed `expires_at` at [now] keeps it `active`; else every component `expired` (or past
/// `expires_at`) makes it `expired`; else it is `disabled`. [pendingCustomId] is the
/// component whose interaction waits in the outbox, [errorText] the translated refusal of the
/// last one.
DsComponentData componentData(
  Message message,
  List<MessageComponent> components, {
  required Map<PrincipalId, Principal> principals,
  required DateTime now,
  required String Function(DateTime at) timeLabel,
  String? pendingCustomId,
  String? errorText,
}) {
  final sorted = [...components]
    ..sort((a, b) => a.position.compareTo(b.position));
  bool expired(MessageComponent c) =>
      c.state == ComponentState.expired ||
      (c.state == ComponentState.active &&
          c.expiresAt != null &&
          !c.expiresAt!.isAfter(now));
  MessageComponent? used;
  for (final c in sorted) {
    if (c.state == ComponentState.used) used = c;
  }
  final DsComponentState state;
  if (used != null) {
    state = DsComponentState.used;
  } else if (sorted.any(
    (c) => c.state == ComponentState.active && !expired(c),
  )) {
    state = DsComponentState.active;
  } else if (sorted.isNotEmpty && sorted.every(expired)) {
    state = DsComponentState.expired;
  } else {
    state = DsComponentState.disabled;
  }
  final usedBy = used?.usedBy;
  final usedAt = used?.usedAt;
  return DsComponentData(
    messageId: message.id.value,
    items: [
      for (final c in sorted)
        if (state != DsComponentState.active ||
            (c.state == ComponentState.active && !expired(c)))
          componentItem(c),
    ],
    state: state,
    pendingCustomId: state == DsComponentState.active ? pendingCustomId : null,
    usedByName: usedBy == null
        ? null
        : principals[usedBy]?.displayName ?? usedBy.value,
    usedChoice: used == null ? null : _choiceLabel(used),
    usedAtLabel: usedAt == null ? null : timeLabel(usedAt),
    errorText: state == DsComponentState.active ? errorText : null,
  );
}

/// One component of `component.v1` (09 11.1) as `DsComponentItem`; members of the wrong type
/// read as absent, an unknown type renders as "needs a newer app".
DsComponentItem componentItem(MessageComponent c) {
  final d = c.definition;
  String? text(String key) => d[key] is String ? d[key]! as String : null;
  int? number(String key) => d[key] is int ? d[key]! as int : null;
  List<Map<String, Object?>> list(String key) => [
    if (d[key] case final List<Object?> items)
      for (final item in items)
        if (item is Map) item.cast<String, Object?>(),
  ];
  return DsComponentItem(
    customId: c.customId,
    type: DsComponentType.fromCode(c.type.code),
    label: text('label') ?? '',
    style: d['style'] == null
        ? DsButtonStyle.secondary
        : DsButtonStyle.fromCode(text('style')),
    placeholder: text('placeholder'),
    options: [
      for (final o in list('options'))
        DsSelectOption(
          label: o['label'] is String ? o['label']! as String : '',
          value: o['value'] is String ? o['value']! as String : '',
          description: o['description'] is String
              ? o['description']! as String
              : null,
        ),
    ],
    minValues: number('min_values') ?? 1,
    maxValues: number('max_values') ?? 1,
    title: text('title'),
    submitLabel: text('submit_label'),
    fields: [
      for (final f in list('fields'))
        DsFormField(
          name: f['name'] is String ? f['name']! as String : '',
          label: f['label'] is String ? f['label']! as String : '',
          type: DsFieldType.fromCode(
            f['type'] is String ? f['type']! as String : null,
          ),
          required: f['required'] == true,
          maxLength: f['max_length'] is int ? f['max_length']! as int : null,
          choices: [
            if (f['choices'] case final List<Object?> choices)
              for (final choice in choices)
                if (choice is String) choice,
          ],
        ),
    ],
  );
}

/// The values of `InteractionCreate` for an activation of [component] with [value] as
/// `DsMessageComponents` hands it over (09 11.1; API `InteractionCreate.values`): a button
/// sends `{}`, a select `{"selected": [...]}`, a form its field map.
Map<String, Object?> interactionValues(
  MessageComponent component,
  Object? value,
) => switch (component.type) {
  ComponentType.select => {
    'selected': [
      if (value is List)
        for (final v in value)
          if (v is String) v,
    ],
  },
  ComponentType.form when value is Map => {
    for (final e in value.entries)
      if (e.key is String && e.value != null) e.key as String: e.value,
  },
  _ => const {},
};

String _choiceLabel(MessageComponent c) {
  final d = c.definition;
  for (final key in const ['label', 'title', 'placeholder']) {
    if (d[key] case final String text when text.isNotEmpty) return text;
  }
  return c.customId;
}
