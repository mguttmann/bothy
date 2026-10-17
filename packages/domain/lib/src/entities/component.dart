// SPDX-License-Identifier: Apache-2.0

/// Message components: buttons, selects and forms (03 `message_components`; 15 6.1).
library;

import '../coded.dart';
import '../ids.dart';
import '../seq.dart';

/// Type of a component (03 `message_components.type`, API `ComponentType`).
enum ComponentType implements Coded {
  /// Button.
  button('button'),

  /// Select.
  select('select'),

  /// Form.
  form('form'),

  /// A value of a newer server.
  unknown('');

  const ComponentType(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static ComponentType fromCode(String code) =>
      decodeCode(values, code, unknown);
}

/// Who may use a component (03 `message_components.restricted_to`, API `ComponentRestriction`).
enum ComponentRestriction implements Coded {
  /// Anyone who may write.
  any('any'),

  /// The person who triggered the run, or the owner.
  runStarter('run_starter'),

  /// Only the owner.
  owner('owner'),

  /// A value of a newer server.
  unknown('');

  const ComponentRestriction(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static ComponentRestriction fromCode(String code) =>
      decodeCode(values, code, unknown);
}

/// State of a component (03 `message_components.state`).
enum ComponentState implements Coded {
  /// Active.
  active('active'),

  /// Used.
  used('used'),

  /// Expired.
  expired('expired'),

  /// Disabled.
  disabled('disabled'),

  /// A value of a newer server.
  unknown('');

  const ComponentState(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static ComponentState fromCode(String code) =>
      decodeCode(values, code, unknown);
}

/// A capability of a grant (API `Capability`).
enum Capability implements Coded {
  /// Approve actions.
  approveActions('approve_actions'),

  /// Upload.
  upload('upload'),

  /// Export.
  export('export'),

  /// Manage.
  manage('manage'),

  /// Terminal.
  terminal('terminal'),

  /// A value of a newer server.
  unknown('');

  const Capability(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static Capability fromCode(String code) => decodeCode(values, code, unknown);
}

/// A component of a message (03 `message_components`, API `Component`).
final class MessageComponent {
  /// Creates a [MessageComponent].
  const MessageComponent({
    required this.id,
    required this.messageId,
    required this.customId,
    required this.type,
    required this.definition,
    required this.position,
    required this.singleUse,
    required this.restrictedTo,
    required this.state,
    required this.version,
    this.requiredCapability,
    this.runId,
    this.expiresAt,
    this.usedBy,
    this.usedAt,
  });

  /// ID.
  final ComponentId id;

  /// Message.
  final MessageId messageId;

  /// ID the agent assigned.
  final String customId;

  /// Type.
  final ComponentType type;

  /// Definition (schema `component.v1`).
  final Map<String, Object?> definition;

  /// Order.
  final int position;

  /// Whether the single-use group allows one decision.
  final bool singleUse;

  /// Capability needed to use it.
  final Capability? requiredCapability;

  /// Who may use it.
  final ComponentRestriction restrictedTo;

  /// Run the question belongs to.
  final RunId? runId;

  /// State.
  final ComponentState state;

  /// Expiry.
  final DateTime? expiresAt;

  /// Who used it.
  final PrincipalId? usedBy;

  /// When it was used.
  final DateTime? usedAt;

  /// Version (03 `version`; 04 9 item 3).
  final ResourceVersion version;

  /// Returns a copy with the given fields replaced; a nullable field cannot be cleared
  /// through it (build a new object for that).
  MessageComponent copyWith({
    MessageId? messageId,
    String? customId,
    ComponentType? type,
    Map<String, Object?>? definition,
    int? position,
    bool? singleUse,
    Capability? requiredCapability,
    ComponentRestriction? restrictedTo,
    RunId? runId,
    ComponentState? state,
    DateTime? expiresAt,
    PrincipalId? usedBy,
    DateTime? usedAt,
    ResourceVersion? version,
  }) => MessageComponent(
    id: id,
    messageId: messageId ?? this.messageId,
    customId: customId ?? this.customId,
    type: type ?? this.type,
    definition: definition ?? this.definition,
    position: position ?? this.position,
    singleUse: singleUse ?? this.singleUse,
    requiredCapability: requiredCapability ?? this.requiredCapability,
    restrictedTo: restrictedTo ?? this.restrictedTo,
    runId: runId ?? this.runId,
    state: state ?? this.state,
    expiresAt: expiresAt ?? this.expiresAt,
    usedBy: usedBy ?? this.usedBy,
    usedAt: usedAt ?? this.usedAt,
    version: version ?? this.version,
  );

  @override
  bool operator ==(Object other) =>
      other is MessageComponent &&
      other.id == id &&
      other.messageId == messageId &&
      other.customId == customId &&
      other.type == type &&
      jsonEquals(other.definition, definition) &&
      other.position == position &&
      other.singleUse == singleUse &&
      other.requiredCapability == requiredCapability &&
      other.restrictedTo == restrictedTo &&
      other.runId == runId &&
      other.state == state &&
      other.expiresAt == expiresAt &&
      other.usedBy == usedBy &&
      other.usedAt == usedAt &&
      other.version == version;

  @override
  int get hashCode => Object.hash(
    id,
    messageId,
    customId,
    type,
    jsonHash(definition),
    position,
    singleUse,
    requiredCapability,
    restrictedTo,
    runId,
    state,
    expiresAt,
    usedBy,
    usedAt,
    version,
  );
}
