// SPDX-License-Identifier: Apache-2.0

/// Principals and agent cards (03 `principals`, `agents`; 15 6.1 `principals`, `agents`).
library;

import '../coded.dart';
import '../ids.dart';
import '../seq.dart';

/// Kind of a principal (03 `principals.kind`, API `PrincipalKind`).
enum PrincipalKind implements Coded {
  /// A person.
  user('user'),

  /// An agent.
  agent('agent'),

  /// An app.
  app('app'),

  /// A value of a newer server.
  unknown('');

  const PrincipalKind(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static PrincipalKind fromCode(String code) =>
      decodeCode(values, code, unknown);
}

/// Status of a principal (03 `principals.status`, API `UserStatus`).
enum PrincipalStatus implements Coded {
  /// Not yet active.
  pending('pending'),

  /// Active.
  active('active'),

  /// Disabled.
  disabled('disabled'),

  /// Deleted and pseudonymized.
  deleted('deleted'),

  /// A value of a newer server.
  unknown('');

  const PrincipalStatus(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static PrincipalStatus fromCode(String code) =>
      decodeCode(values, code, unknown);
}

/// Who pays an agent's model calls (03 `agents.billing_mode`, API `BillingMode`, 01 6.7).
enum BillingMode implements Coded {
  /// The person's own account; costs are information only.
  personal('personal'),

  /// The organization's account; costs are estimates until reconciled.
  organization('organization'),

  /// A value of a newer server.
  unknown('');

  const BillingMode(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static BillingMode fromCode(String code) => decodeCode(values, code, unknown);
}

/// Tool class of an agent (03 `agents.tool_class`, API `ToolClass`).
enum ToolClass implements Coded {
  /// Chat only.
  chat('chat'),

  /// Scoped tools.
  scoped('scoped'),

  /// Tools on a host.
  host('host'),

  /// A value of a newer server.
  unknown('');

  const ToolClass(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static ToolClass fromCode(String code) => decodeCode(values, code, unknown);
}

/// Reported runtime state of an agent (03 `agents.reported_runtime_state`, API
/// `AgentRuntimeState`). `unknown` is a value of the spec itself and also the fallback.
enum AgentRuntimeState implements Coded {
  /// No heartbeat yet, or a value of a newer server.
  unknown('unknown'),

  /// Online.
  online('online'),

  /// Online with problems.
  degraded('degraded'),

  /// No heartbeat within `agent.presence_timeout`.
  offline('offline');

  const AgentRuntimeState(this.code);

  @override
  final String code;

  /// Decodes [code]; a value this version does not know is [unknown].
  static AgentRuntimeState fromCode(String code) =>
      decodeCode(values, code, unknown);
}

/// A principal as the client shows it (API `PrincipalRef`): display only.
final class Principal {
  /// Creates a principal.
  const Principal({
    required this.id,
    required this.kind,
    required this.handle,
    required this.displayName,
    this.avatarFileId,
  });

  /// ID.
  final PrincipalId id;

  /// Person, agent or app.
  final PrincipalKind kind;

  /// Handle for mentions.
  final String handle;

  /// Display name.
  final String displayName;

  /// Avatar file, if any.
  final FileId? avatarFileId;

  /// Returns a copy with the given fields replaced.
  Principal copyWith({
    PrincipalKind? kind,
    String? handle,
    String? displayName,
    FileId? avatarFileId,
  }) => Principal(
    id: id,
    kind: kind ?? this.kind,
    handle: handle ?? this.handle,
    displayName: displayName ?? this.displayName,
    avatarFileId: avatarFileId ?? this.avatarFileId,
  );

  @override
  bool operator ==(Object other) =>
      other is Principal &&
      other.id == id &&
      other.kind == kind &&
      other.handle == handle &&
      other.displayName == displayName &&
      other.avatarFileId == avatarFileId;

  @override
  int get hashCode => Object.hash(id, kind, handle, displayName, avatarFileId);
}

/// Reported presence of an agent (04 12 item 2, API `AgentPresence`).
final class AgentPresence {
  /// Creates a presence.
  const AgentPresence({required this.state, this.reportedBillingMode});

  /// Derived state.
  final AgentRuntimeState state;

  /// Last reported billing mode, if the adapter reports one.
  final BillingMode? reportedBillingMode;

  /// Returns a copy with the given fields replaced.
  AgentPresence copyWith({
    AgentRuntimeState? state,
    BillingMode? reportedBillingMode,
  }) => AgentPresence(
    state: state ?? this.state,
    reportedBillingMode: reportedBillingMode ?? this.reportedBillingMode,
  );

  @override
  bool operator ==(Object other) =>
      other is AgentPresence &&
      other.state == state &&
      other.reportedBillingMode == reportedBillingMode;

  @override
  int get hashCode => Object.hash(state, reportedBillingMode);
}

/// The card of an agent: display, mode badge and reported presence (15 6.1 `agents`). It holds
/// no operator configuration and no credential holder.
final class AgentCard {
  /// Creates a card.
  const AgentCard({
    required this.id,
    required this.handle,
    required this.displayName,
    required this.description,
    required this.billingMode,
    required this.toolClass,
    required this.presence,
    required this.version,
    this.avatarFileId,
  });

  /// Principal ID of the agent.
  final PrincipalId id;

  /// Handle for mentions.
  final String handle;

  /// Display name.
  final String displayName;

  /// Avatar file, if any.
  final FileId? avatarFileId;

  /// Description.
  final String description;

  /// Configured billing mode (mode badge).
  final BillingMode billingMode;

  /// Configured tool class.
  final ToolClass toolClass;

  /// Reported presence.
  final AgentPresence presence;

  /// Version (03 `agents.version`).
  final ResourceVersion version;

  /// Returns a copy with the given fields replaced.
  AgentCard copyWith({
    String? handle,
    String? displayName,
    FileId? avatarFileId,
    String? description,
    BillingMode? billingMode,
    ToolClass? toolClass,
    AgentPresence? presence,
    ResourceVersion? version,
  }) => AgentCard(
    id: id,
    handle: handle ?? this.handle,
    displayName: displayName ?? this.displayName,
    avatarFileId: avatarFileId ?? this.avatarFileId,
    description: description ?? this.description,
    billingMode: billingMode ?? this.billingMode,
    toolClass: toolClass ?? this.toolClass,
    presence: presence ?? this.presence,
    version: version ?? this.version,
  );

  @override
  bool operator ==(Object other) =>
      other is AgentCard &&
      other.id == id &&
      other.handle == handle &&
      other.displayName == displayName &&
      other.avatarFileId == avatarFileId &&
      other.description == description &&
      other.billingMode == billingMode &&
      other.toolClass == toolClass &&
      other.presence == presence &&
      other.version == version;

  @override
  int get hashCode => Object.hash(
    id,
    handle,
    displayName,
    avatarFileId,
    description,
    billingMode,
    toolClass,
    presence,
    version,
  );
}
