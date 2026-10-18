// SPDX-License-Identifier: Apache-2.0

/// Grants, their preview, "Who has access?" and "View as person" over REST
/// (docs/spec/01-product.md 5.2 A2 to A4; 07 2, 9.1 to 9.5; brief M4-27 Design notes 1 and 2).
///
/// The types here carry what the server answers, unchanged: the client never derives a
/// permission from them (15 1 principle 1) and never composes the preview sentence itself
/// (07 9.3: the text comes from templates in `authz`).
library;

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:dio/dio.dart';

import 'self_repository.dart';

/// A level of a grant (07 2.2), or [none] where the API says `LevelOrNone`.
enum AccessLevel {
  /// No level (an effect before or after the change, or an entry only via an agent).
  none('none'),

  /// Read.
  read('read'),

  /// Read and write.
  write('write'),

  /// Read, write and trigger agents.
  trigger('trigger');

  const AccessLevel(this.wire);

  /// The name on the wire, for example `trigger`.
  final String wire;
}

/// A capability next to the level (07 2.3). `terminal` belongs to hosts (M4-37) and is not
/// offered by the wizard of this package.
enum AccessCapability {
  /// Approve actions of agents.
  approveActions('approve_actions'),

  /// Upload files.
  upload('upload'),

  /// Export.
  export('export'),

  /// Manage the target (grants, settings).
  manage('manage'),

  /// Open a terminal on a host.
  terminal('terminal');

  const AccessCapability(this.wire);

  /// The name on the wire, for example `approve_actions`.
  final String wire;
}

/// Who a grant is for (`AdminSubjectKind`).
enum AdminSubjectKind {
  /// A person.
  user,

  /// A group.
  group,

  /// An agent.
  agent,

  /// An app.
  app,
}

/// Where a grant applies (`TargetKind` without `host`, which M4-37 adds).
enum AdminTargetKind {
  /// The whole instance.
  instance,

  /// A space and every channel in it.
  space,

  /// One channel.
  channel,
}

/// A target of a grant as the server lists it: the instance, a space or a channel.
final class AdminTarget {
  /// Creates the target; [id] is null exactly for the instance.
  const AdminTarget(this.kind, {this.id, this.name = '', this.spaceName});

  /// The instance.
  static const AdminTarget instance = AdminTarget(AdminTargetKind.instance);

  /// The kind.
  final AdminTargetKind kind;

  /// The space or channel ID, null for the instance.
  final String? id;

  /// The name of the space or channel (empty for the instance).
  final String name;

  /// The name of the channel's space, or null.
  final String? spaceName;

  /// The path a person reads: `Marketing/Kampagnen` for a channel, the name for a space.
  String get path => spaceName == null ? name : '$spaceName/$name';

  /// A key unique among targets, for example `channel.<id>`.
  String get key => id == null ? kind.name : '${kind.name}.$id';

  @override
  bool operator ==(Object other) =>
      other is AdminTarget && other.kind == kind && other.id == id;

  @override
  int get hashCode => Object.hash(kind, id);
}

/// A principal as the server embeds it (`PrincipalRef`).
final class AccessPrincipal {
  /// Creates the principal.
  const AccessPrincipal({
    required this.id,
    required this.kind,
    required this.handle,
    required this.displayName,
  });

  /// The principal ID.
  final String id;

  /// `user`, `agent` or `app`.
  final String kind;

  /// The handle.
  final String handle;

  /// The display name.
  final String displayName;
}

/// An agent of the directory (`Agent`, reduced to what the wizard shows).
final class AdminAgent {
  /// Creates the agent.
  const AdminAgent({
    required this.id,
    required this.handle,
    required this.displayName,
  });

  /// The agent ID.
  final String id;

  /// The handle.
  final String handle;

  /// The display name.
  final String displayName;
}

/// The request of `previewGrant` and `createGrant` (`GrantCreate`).
final class GrantRequest {
  /// Creates the request.
  const GrantRequest({
    required this.subjectKind,
    required this.subjectId,
    required this.target,
    required this.level,
    this.capabilities = const {},
    this.expiresAt,
    required this.reason,
  });

  /// Who.
  final AdminSubjectKind subjectKind;

  /// The principal or group ID.
  final String subjectId;

  /// Where.
  final AdminTarget target;

  /// The level (never [AccessLevel.none]).
  final AccessLevel level;

  /// The capabilities.
  final Set<AccessCapability> capabilities;

  /// The end of the validity, or null.
  final DateTime? expiresAt;

  /// Why (stored in the audit log; required, 07 9.5).
  final String reason;
}

/// The change of one principal on one target (`AccessEffect`).
final class GrantEffect {
  /// Creates the effect.
  const GrantEffect({
    required this.principal,
    required this.before,
    required this.after,
    required this.capabilitiesAdded,
    this.targetKind,
    this.targetId,
  });

  /// Whose access changes.
  final AccessPrincipal principal;

  /// The level before.
  final AccessLevel before;

  /// The level after.
  final AccessLevel after;

  /// The capabilities that are added.
  final List<AccessCapability> capabilitiesAdded;

  /// The kind of the affected target, or null when the server sent one the client does not
  /// know.
  final AdminTargetKind? targetKind;

  /// The affected target, or null for the instance.
  final String? targetId;
}

/// An invariant the change would violate (`InvariantViolation`, 07 SR1 and SR8).
final class InvariantFinding {
  /// Creates the finding.
  const InvariantFinding({
    required this.code,
    required this.principalIds,
    this.agentId,
    this.organizationReady,
  });

  /// For example `i1_violation`.
  final String code;

  /// The principals the violation names.
  final List<String> principalIds;

  /// The agent of an I1 violation, or null.
  final String? agentId;

  /// Whether the agent supports the organization mode, or null when not sent.
  final bool? organizationReady;
}

/// A hint of the preview (`GrantHint`).
final class GrantHint {
  /// Creates the hint.
  const GrantHint({
    required this.code,
    this.agentId,
    this.principalId,
    this.groupId,
    this.spaceId,
    this.channelId,
    this.count,
  });

  /// `read_horizon`, `trigger_blocked_tool_class`, `group_reach`, `expiry_missing`,
  /// `terminal_not_effective`, or a code this build does not know.
  final String code;

  /// The agent, or null.
  final String? agentId;

  /// The person, or null.
  final String? principalId;

  /// The group, or null.
  final String? groupId;

  /// The space, or null.
  final String? spaceId;

  /// The channel, or null.
  final String? channelId;

  /// A count (people a group reaches), or null.
  final int? count;
}

/// The answer of `previewGrant` (`GrantPreview`, 07 9.3).
final class GrantPreviewResult {
  /// Creates the preview.
  const GrantPreviewResult({
    required this.allowed,
    required this.effects,
    required this.violations,
    required this.hints,
    required this.text,
    required this.textLocale,
    this.denialReason,
  });

  /// Whether the grant would be accepted.
  final bool allowed;

  /// The reason code of 07 4.1 when a delegation limit rejects the grant.
  final String? denialReason;

  /// The changes per principal.
  final List<GrantEffect> effects;

  /// The invariants it would violate.
  final List<InvariantFinding> violations;

  /// The hints.
  final List<GrantHint> hints;

  /// The server's plain text, lines separated by a line feed.
  final String text;

  /// The language of [text], `de` or `en`.
  final String textLocale;
}

/// A grant (`Grant`).
final class AdminGrant {
  /// Creates the grant.
  const AdminGrant({
    required this.id,
    required this.subjectKind,
    required this.subjectId,
    required this.targetKind,
    required this.capabilities,
    required this.reason,
    required this.grantedAt,
    this.targetId,
    this.level,
    this.expiresAt,
    this.revokedAt,
  });

  /// The grant ID.
  final String id;

  /// Who.
  final AdminSubjectKind subjectKind;

  /// The principal or group ID.
  final String subjectId;

  /// The target kind, or null for a kind this package does not show (a host).
  final AdminTargetKind? targetKind;

  /// The target ID, null for the instance.
  final String? targetId;

  /// The level, null on a host.
  final AccessLevel? level;

  /// The capabilities.
  final List<AccessCapability> capabilities;

  /// The end of the validity, or null.
  final DateTime? expiresAt;

  /// Why it was given.
  final String reason;

  /// When it was given.
  final DateTime grantedAt;

  /// When it was revoked, or null.
  final DateTime? revokedAt;
}

/// The kind of a source of access (`AccessSource.kind`, 07 9.1).
enum AccessSourceKind {
  /// An owner.
  owner,

  /// A grant to the principal on this target.
  direct,

  /// A grant to a group the principal is a member of.
  group,

  /// A grant on the space or the instance above.
  inherited,

  /// A participant of a direct message.
  dmParticipant,

  /// The operator agent of the channel.
  operator,

  /// Content reachable only through an agent (hint H1, 07 6).
  viaAgent,

  /// Triggering blocked by a guard rule (SR2, SR3).
  guardBlocked,

  /// A kind this build does not know.
  unknown,
}

/// One source of an entry of "Who has access?" (`AccessSource`).
final class AccessSourceInfo {
  /// Creates the source.
  const AccessSourceInfo({
    required this.kind,
    this.grantId,
    this.groupId,
    this.fromTargetKind,
    this.fromTargetId,
    this.agentId,
    this.rule,
    this.expiresAt,
    this.membershipExpiresAt,
    this.reason,
  });

  /// The kind.
  final AccessSourceKind kind;

  /// The grant, or null.
  final String? grantId;

  /// The group, or null.
  final String? groupId;

  /// Where an inherited grant comes from, or null.
  final AdminTargetKind? fromTargetKind;

  /// The space of an inherited grant, or null.
  final String? fromTargetId;

  /// The agent of an `operator`, `via_agent` or `guard_blocked` source, or null.
  final String? agentId;

  /// `sr2` or `sr3` for `guard_blocked`, or null.
  final String? rule;

  /// The end of the grant, or null.
  final DateTime? expiresAt;

  /// The end of the group membership, or null.
  final DateTime? membershipExpiresAt;

  /// The reason of the grant, or null.
  final String? reason;
}

/// One principal of "Who has access?" (`AccessEntry`).
final class AccessEntryInfo {
  /// Creates the entry.
  const AccessEntryInfo({
    required this.principal,
    required this.level,
    required this.capabilities,
    required this.sources,
    this.inactive = false,
  });

  /// The principal.
  final AccessPrincipal principal;

  /// The effective level; [AccessLevel.none] only for an entry via agents alone.
  final AccessLevel level;

  /// Whether the principal is not active (shown struck through).
  final bool inactive;

  /// The capabilities.
  final List<AccessCapability> capabilities;

  /// The chain of sources.
  final List<AccessSourceInfo> sources;
}

/// The answer of `explainSpaceAccess` and `explainChannelAccess`.
final class AccessExplanationResult {
  /// Creates the answer.
  const AccessExplanationResult(this.entries);

  /// The principals with access.
  final List<AccessEntryInfo> entries;
}

/// A space or channel of a [ViewAsResult].
final class ViewAsItem {
  /// Creates the item.
  const ViewAsItem({
    required this.id,
    required this.name,
    required this.level,
    required this.capabilities,
    this.spaceId,
  });

  /// The space or channel ID.
  final String id;

  /// Its name.
  final String name;

  /// The level of the person.
  final AccessLevel level;

  /// The capabilities of the person.
  final List<AccessCapability> capabilities;

  /// The space of a channel, or null.
  final String? spaceId;
}

/// The answer of `viewAsUser` (`ViewAsSnapshot`, 07 9.2): what the person sees, until
/// [expiresAt].
final class ViewAsResult {
  /// Creates the snapshot.
  const ViewAsResult({
    required this.user,
    required this.expiresAt,
    required this.spaces,
    required this.channels,
  });

  /// The person.
  final AccessPrincipal user;

  /// The end of the view.
  final DateTime expiresAt;

  /// The spaces the person sees.
  final List<ViewAsItem> spaces;

  /// The channels the person sees (never a direct message, SR6).
  final List<ViewAsItem> channels;
}

/// The calls of the grant screens; a changing call carries the [purpose] text the step-up
/// dialog shows when the server answers `401 step_up_required` (05: `grant.create`,
/// `grant.revoke` and `access.view_as` have step-up; `access.preview` has none).
///
/// Errors: [SelfActionFailed] with the server's problem, [StepUpCancelled] when the person
/// cancels the step-up.
abstract interface class AccessRepository {
  /// The spaces and their channels the caller sees (`listSpaces`, `listSpaceChannels`), in
  /// the server's order: each space followed by its channels.
  Future<List<AdminTarget>> targets();

  /// The agents the caller sees (`listAgents`, all pages).
  Future<List<AdminAgent>> agents();

  /// The dry run of [request] (`previewGrant`): nothing is stored, no step-up.
  Future<GrantPreviewResult> previewGrant(GrantRequest request);

  /// Creates the grant (`createGrant`).
  Future<AdminGrant> createGrant(
    GrantRequest request, {
    required String purpose,
  });

  /// The valid grants on [target] (`listGrants`, all pages).
  Future<List<AdminGrant>> grantsOn(AdminTarget target);

  /// Revokes [grantId] with [reason] (`revokeGrant`); it takes effect immediately.
  Future<AdminGrant> revokeGrant(
    String grantId, {
    required String reason,
    required String purpose,
  });

  /// Who has access to the space or channel [target] (`explainSpaceAccess`,
  /// `explainChannelAccess`).
  Future<AccessExplanationResult> explain(AdminTarget target);

  /// The read-only snapshot of what [userId] sees (`viewAsUser`, owners only, step-up).
  Future<ViewAsResult> viewAs(String userId, {required String purpose});
}

/// [AccessRepository] over the generated client of `api_client`.
///
/// Example:
/// ```dart
/// final access = ApiAccessRepository(session.api, newIdempotencyKey: ids.next);
/// final preview = await access.previewGrant(request);
/// ```
final class ApiAccessRepository implements AccessRepository {
  /// Creates the repository over [api] (the client of the signed-in session, so DPoP, refresh
  /// and the step-up interceptor apply); [newIdempotencyKey] returns a fresh UUIDv7.
  ApiAccessRepository(this._api, {required String Function() newIdempotencyKey})
    : _newKey = newIdempotencyKey;

  final ApiClient _api;
  final String Function() _newKey;

  AccessApi get _access => _api.getAccessApi();

  Map<String, Object?> _purpose(String text) => {stepUpPurposeExtra: text};

  @override
  Future<List<AdminTarget>> targets() => _call(() async {
    final spaces = (await _api.getSpacesApi().listSpaces()).data!.items;
    final out = <AdminTarget>[];
    for (final s in spaces) {
      out.add(
        AdminTarget(AdminTargetKind.space, id: s.space.id, name: s.space.name),
      );
      final channels = (await _api.getChannelsApi().listSpaceChannels(
        spaceId: s.space.id,
      )).data!.items;
      for (final c in channels) {
        out.add(
          AdminTarget(
            AdminTargetKind.channel,
            id: c.channel.id,
            name: c.channel.name,
            spaceName: s.space.name,
          ),
        );
      }
    }
    return out;
  });

  @override
  Future<List<AdminAgent>> agents() => _call(() async {
    final out = <AdminAgent>[];
    String? cursor;
    do {
      final page = (await _api.getAgentsApi().listAgents(cursor: cursor)).data!;
      out.addAll([
        for (final a in page.items)
          AdminAgent(id: a.id, handle: a.handle, displayName: a.displayName),
      ]);
      cursor = page.nextCursor;
    } while (cursor != null);
    return out;
  });

  @override
  Future<GrantPreviewResult> previewGrant(GrantRequest request) => _call(
    () async {
      final p = (await _access.previewGrant(grantCreate: _create(request)))
          .data!;
      return GrantPreviewResult(
        allowed: p.allowed,
        denialReason: p.denialReason,
        effects: [
          for (final e in p.effects)
            GrantEffect(
              principal: _principal(e.principal),
              before: _levelOrNone(e.beforeLevel),
              after: _levelOrNone(e.afterLevel),
              capabilitiesAdded: _capabilities(e.capabilitiesAdded),
              targetKind: _targetKind(e.targetKind),
              targetId: e.targetId,
            ),
        ],
        violations: [
          for (final v in p.invariantViolations)
            InvariantFinding(
              code: v.code,
              principalIds: v.principalIds.toList(),
              agentId: v.agentId,
              organizationReady: v.organizationReady,
            ),
        ],
        hints: [
          for (final h in p.hints)
            GrantHint(
              code: switch (h.code) {
                PreviewHintCodeEnum.readHorizon => 'read_horizon',
                PreviewHintCodeEnum.triggerBlockedToolClass =>
                  'trigger_blocked_tool_class',
                PreviewHintCodeEnum.groupReach => 'group_reach',
                PreviewHintCodeEnum.expiryMissing => 'expiry_missing',
                PreviewHintCodeEnum.terminalNotEffective =>
                  'terminal_not_effective',
                _ => 'unknown',
              },
              agentId: h.agentId,
              principalId: h.principalId,
              groupId: h.groupId,
              spaceId: h.spaceId,
              channelId: h.channelId,
              count: h.count,
            ),
        ],
        text: p.text,
        textLocale: p.textLocale == GrantPreviewTextLocaleEnum.en ? 'en' : 'de',
      );
    },
  );

  @override
  Future<AdminGrant> createGrant(
    GrantRequest request, {
    required String purpose,
  }) => _call(
    () async => _grant(
      (await _access.createGrant(
        grantCreate: _create(request),
        idempotencyKey: _newKey(),
        extra: _purpose(purpose),
      )).data!,
    ),
  );

  @override
  Future<List<AdminGrant>> grantsOn(AdminTarget target) => _call(() async {
    final out = <AdminGrant>[];
    String? cursor;
    do {
      final page = (await _access.listGrants(
        cursor: cursor,
        targetKind: _wireTarget(target.kind),
        targetId: target.id,
      )).data!;
      out.addAll(page.items.map(_grant));
      cursor = page.nextCursor;
    } while (cursor != null);
    return out;
  });

  @override
  Future<AdminGrant> revokeGrant(
    String grantId, {
    required String reason,
    required String purpose,
  }) => _call(
    () async => _grant(
      (await _access.revokeGrant(
        grantId: grantId,
        reasonBody: ReasonBody((b) => b..reason = reason),
        idempotencyKey: _newKey(),
        extra: _purpose(purpose),
      )).data!,
    ),
  );

  @override
  Future<AccessExplanationResult> explain(AdminTarget target) =>
      _call(() async {
        final id = target.id!;
        final e = target.kind == AdminTargetKind.channel
            ? (await _access.explainChannelAccess(channelId: id)).data!
            : (await _access.explainSpaceAccess(spaceId: id)).data!;
        return AccessExplanationResult([
          for (final entry in e.entries)
            AccessEntryInfo(
              principal: _principal(entry.principal),
              level: _levelOrNone(entry.level),
              inactive: entry.inactive ?? false,
              capabilities: _capabilities(entry.capabilities),
              sources: [for (final s in entry.sources) _source(s)],
            ),
        ]);
      });

  @override
  Future<ViewAsResult> viewAs(String userId, {required String purpose}) =>
      _call(() async {
        final v = (await _access.viewAsUser(
          userId: userId,
          extra: _purpose(purpose),
        )).data!;
        return ViewAsResult(
          user: _principal(v.user),
          expiresAt: v.expiresAt,
          spaces: [
            for (final s in v.spaces)
              ViewAsItem(
                id: s.space.id,
                name: s.space.name,
                level: _level(s.level),
                capabilities: _capabilities(s.capabilities),
              ),
          ],
          channels: [
            for (final c in v.channels)
              ViewAsItem(
                id: c.channel.id,
                name: c.channel.name,
                level: _level(c.level),
                capabilities: _capabilities(c.capabilities),
                spaceId: c.channel.spaceId,
              ),
          ],
        );
      });

  static GrantCreate _create(GrantRequest r) => GrantCreate(
    (b) => b
      ..subjectKind = switch (r.subjectKind) {
        AdminSubjectKind.user => GrantSubjectKind.user,
        AdminSubjectKind.group => GrantSubjectKind.group,
        AdminSubjectKind.agent => GrantSubjectKind.agent,
        AdminSubjectKind.app => GrantSubjectKind.app,
      }
      ..subjectId = r.subjectId
      ..targetKind = _wireTarget(r.target.kind)
      ..targetId = r.target.id
      ..level = switch (r.level) {
        AccessLevel.read || AccessLevel.none => Level.read,
        AccessLevel.write => Level.write,
        AccessLevel.trigger => Level.trigger,
      }
      ..capabilities.replace([
        for (final c in AccessCapability.values)
          if (r.capabilities.contains(c)) _wireCapability(c),
      ])
      ..expiresAt = r.expiresAt?.toUtc()
      ..reason = r.reason,
  );

  static TargetKind _wireTarget(AdminTargetKind kind) => switch (kind) {
    AdminTargetKind.instance => TargetKind.instance,
    AdminTargetKind.space => TargetKind.space,
    AdminTargetKind.channel => TargetKind.channel,
  };

  static AdminTargetKind? _targetKind(TargetKind? kind) => switch (kind) {
    TargetKind.instance => AdminTargetKind.instance,
    TargetKind.space => AdminTargetKind.space,
    TargetKind.channel => AdminTargetKind.channel,
    _ => null,
  };

  static Capability _wireCapability(AccessCapability c) => switch (c) {
    AccessCapability.approveActions => Capability.approveActions,
    AccessCapability.upload => Capability.upload,
    AccessCapability.export => Capability.export_,
    AccessCapability.manage => Capability.manage,
    AccessCapability.terminal => Capability.terminal,
  };

  static List<AccessCapability> _capabilities(Iterable<Capability> list) => [
    for (final c in list)
      ?switch (c) {
        Capability.approveActions => AccessCapability.approveActions,
        Capability.upload => AccessCapability.upload,
        Capability.export_ => AccessCapability.export,
        Capability.manage => AccessCapability.manage,
        Capability.terminal => AccessCapability.terminal,
        _ => null,
      },
  ];

  static AccessLevel _level(Level? level) => switch (level) {
    Level.read => AccessLevel.read,
    Level.write => AccessLevel.write,
    Level.trigger => AccessLevel.trigger,
    _ => AccessLevel.none,
  };

  static AccessLevel _levelOrNone(LevelOrNone level) => switch (level) {
    LevelOrNone.read => AccessLevel.read,
    LevelOrNone.write => AccessLevel.write,
    LevelOrNone.trigger => AccessLevel.trigger,
    _ => AccessLevel.none,
  };

  static AccessPrincipal _principal(PrincipalRef p) => AccessPrincipal(
    id: p.id,
    kind: switch (p.kind) {
      PrincipalKind.agent => 'agent',
      PrincipalKind.app => 'app',
      _ => 'user',
    },
    handle: p.handle,
    displayName: p.displayName,
  );

  static AdminGrant _grant(Grant g) => AdminGrant(
    id: g.id,
    subjectKind: switch (g.subjectKind) {
      GrantSubjectKind.group => AdminSubjectKind.group,
      GrantSubjectKind.agent => AdminSubjectKind.agent,
      GrantSubjectKind.app => AdminSubjectKind.app,
      _ => AdminSubjectKind.user,
    },
    subjectId: g.subjectId,
    targetKind: _targetKind(g.targetKind),
    targetId: g.targetId,
    level: g.level == null ? null : _level(g.level),
    capabilities: _capabilities(g.capabilities),
    expiresAt: g.expiresAt,
    reason: g.reason,
    grantedAt: g.grantedAt,
    revokedAt: g.revokedAt,
  );

  static AccessSourceInfo _source(AccessSource s) => AccessSourceInfo(
    kind: switch (s.kind) {
      AccessSourceKindEnum.owner => AccessSourceKind.owner,
      AccessSourceKindEnum.direct => AccessSourceKind.direct,
      AccessSourceKindEnum.group => AccessSourceKind.group,
      AccessSourceKindEnum.inherited => AccessSourceKind.inherited,
      AccessSourceKindEnum.dmParticipant => AccessSourceKind.dmParticipant,
      AccessSourceKindEnum.operator_ => AccessSourceKind.operator,
      AccessSourceKindEnum.viaAgent => AccessSourceKind.viaAgent,
      AccessSourceKindEnum.guardBlocked => AccessSourceKind.guardBlocked,
      _ => AccessSourceKind.unknown,
    },
    grantId: s.grantId,
    groupId: s.groupId,
    fromTargetKind: _targetKind(s.fromTargetKind),
    fromTargetId: s.fromTargetId,
    agentId: s.agentId,
    rule: switch (s.rule) {
      AccessSourceRuleEnum.sr2 => 'sr2',
      AccessSourceRuleEnum.sr3 => 'sr3',
      _ => null,
    },
    expiresAt: s.expiresAt,
    membershipExpiresAt: s.membershipExpiresAt,
    reason: s.reason,
  );

  /// Runs [call] and turns its errors into [SelfActionFailed]; an error of the login layer
  /// ([StepUpCancelled], a lost session) passes as it is.
  static Future<T> _call<T>(Future<T> Function() call) async {
    try {
      return await call();
    } on DioException catch (e) {
      final cause = e.error;
      if (cause is AuthException) throw cause;
      final response = e.response;
      throw SelfActionFailed(
        ApiProblem.of(e) ??
            (response == null ? null : ApiProblem.fromResponse(response)),
      );
    }
  }
}
