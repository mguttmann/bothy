// SPDX-License-Identifier: Apache-2.0

// The grant calls of the fake repository (brief M4-27 Tests): the space Marketing with the
// channels of the example of 07 9.3, the agent that reads both, grants that change the access
// list, and a preview that answers the example request with the text of 07 9.3 read at run
// time (Design note 3). Everything else of the preview a test sets through [previewAnswer].

import 'dart:io' as io;

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';

/// The example text of 07 9.3, read from the specification at run time (D-W-94: no German
/// sentence in a tracked test file): the fenced block between the `language-check` markers
/// that follow the heading of 9.3.
String previewExample() {
  var dir = io.Directory.current.absolute;
  while (true) {
    for (final name in ['07-permissions.md', '07-rechte.md']) {
      final file = io.File('${dir.path}/docs/spec/$name');
      if (file.existsSync()) return exampleOf(file.readAsStringSync());
    }
    if (dir.parent.path == dir.path) {
      throw StateError('no docs/spec/07 above the working directory');
    }
    dir = dir.parent;
  }
}

/// The first fenced block between `language-check: off` and `language-check: on` after the
/// heading `### 9.3` of [spec].
String exampleOf(String spec) {
  final start = spec.indexOf('\n### 9.3');
  final off = spec.indexOf('language-check: off', start);
  final on = spec.indexOf('language-check: on', off);
  if (start < 0 || off < 0 || on < 0) {
    throw StateError('07 9.3 has no example between language-check markers');
  }
  final block = spec.substring(off, on);
  final open = block.indexOf('```text\n') + '```text\n'.length;
  final close = block.indexOf('\n```', open);
  return block.substring(open, close);
}

/// The targets of the fake: the space Marketing and its channels.
const AdminTarget marketing = AdminTarget(
  AdminTargetKind.space,
  id: 'marketing',
  name: 'Marketing',
);

/// The channel of the example request.
const AdminTarget campaigns = AdminTarget(
  AdminTargetKind.channel,
  id: 'campaigns',
  name: 'Kampagnen',
  spaceName: 'Marketing',
);

/// The channel only the agent reads.
const AdminTarget internal = AdminTarget(
  AdminTargetKind.channel,
  id: 'internal',
  name: 'Intern',
  spaceName: 'Marketing',
);

/// The agent of the example.
const AdminAgent texter = AdminAgent(
  id: 'texter',
  handle: 'texter',
  displayName: 'Texter',
);

AccessPrincipal _p(String id, String name, {String kind = 'user'}) =>
    AccessPrincipal(id: id, kind: kind, handle: id, displayName: name);

/// One entry per source kind of 07 9.1 on the channel of the example.
List<AccessEntryInfo> everySource() => [
  AccessEntryInfo(
    principal: _p('alex', 'Alex'),
    level: AccessLevel.trigger,
    capabilities: const [AccessCapability.approveActions],
    sources: const [AccessSourceInfo(kind: AccessSourceKind.owner)],
  ),
  AccessEntryInfo(
    principal: _p('sam', 'Sam'),
    level: AccessLevel.trigger,
    capabilities: const [AccessCapability.upload],
    sources: [
      AccessSourceInfo(
        kind: AccessSourceKind.direct,
        grantId: 'g-sam',
        expiresAt: DateTime.utc(2026, 12, 31, 23),
        reason: 'Stand-in',
      ),
      AccessSourceInfo(
        kind: AccessSourceKind.group,
        groupId: 'support',
        membershipExpiresAt: DateTime.utc(2026, 11, 30, 23),
      ),
    ],
  ),
  AccessEntryInfo(
    principal: _p('robin', 'Robin'),
    level: AccessLevel.read,
    capabilities: const [],
    inactive: true,
    sources: const [
      AccessSourceInfo(
        kind: AccessSourceKind.inherited,
        fromTargetKind: AdminTargetKind.space,
        fromTargetId: 'marketing',
      ),
    ],
  ),
  AccessEntryInfo(
    principal: _p('texter', 'Texter', kind: 'agent'),
    level: AccessLevel.write,
    capabilities: const [],
    sources: const [
      AccessSourceInfo(kind: AccessSourceKind.operator, agentId: 'texter'),
    ],
  ),
  AccessEntryInfo(
    principal: _p('kim', 'Kim'),
    level: AccessLevel.write,
    capabilities: const [],
    sources: const [AccessSourceInfo(kind: AccessSourceKind.dmParticipant)],
  ),
  AccessEntryInfo(
    principal: _p('jo', 'Jo'),
    level: AccessLevel.none,
    capabilities: const [],
    sources: const [
      AccessSourceInfo(kind: AccessSourceKind.viaAgent, agentId: 'texter'),
    ],
  ),
  AccessEntryInfo(
    principal: _p('lee', 'Lee'),
    level: AccessLevel.trigger,
    capabilities: const [],
    sources: const [
      AccessSourceInfo(
        kind: AccessSourceKind.guardBlocked,
        agentId: 'texter',
        rule: 'sr3',
      ),
    ],
  ),
];

/// The changing grant calls and whether 05 gives their action step-up.
const Map<String, bool> accessStepUpOf = {
  'createGrant': true,
  'revokeGrant': true,
  'viewAs': true,
};

/// The grant calls of the fake; [change] is the step-up of the host fake.
mixin FakeAccess {
  /// Records the change [op] and asks for the step-up as the server would.
  Future<void> change(String op, String args, String purpose);

  /// Throws the refusal of [op], if any.
  void refused(String op);

  /// The display name of a person.
  String nameOf(String userId);

  /// The I1 guard of the agent fake (10 5.4, 11).
  void i1Guard(String op, I1Resolution? resolve);

  /// The targets the caller sees.
  final List<AdminTarget> targetList = [marketing, campaigns, internal];

  /// The agents.
  final List<AdminAgent> agentList = [texter];

  /// The valid grants.
  final List<AdminGrant> grantList = [];

  /// The requests of [previewGrant], in order.
  final List<GrantRequest> previews = [];

  /// Answers a preview instead of the default, or null.
  GrantPreviewResult? Function(GrantRequest request)? previewAnswer;

  /// The entries of "Who has access?" per target ID beyond those the grants give.
  final Map<String, List<AccessEntryInfo>> extraAccess = {};

  /// The snapshot per person of "View as person".
  final Map<String, ViewAsResult> snapshots = {};

  int _grants = 0;

  AccessPrincipal _principal(String id) => AccessPrincipal(
    id: id,
    kind: 'user',
    handle: id,
    displayName: nameOf(id),
  );

  bool _isExample(GrantRequest r) =>
      r.subjectId == 'sam' &&
      r.target == campaigns &&
      r.level == AccessLevel.trigger &&
      r.expiresAt != null;

  GrantPreviewResult _default(GrantRequest r) => GrantPreviewResult(
    allowed: true,
    effects: [
      GrantEffect(
        principal: _principal(r.subjectId),
        before: AccessLevel.none,
        after: r.level,
        capabilitiesAdded: r.capabilities.toList(),
        targetKind: r.target.kind,
        targetId: r.target.id,
      ),
    ],
    violations: const [],
    hints: [
      if (_isExample(r))
        const GrantHint(
          code: 'read_horizon',
          agentId: 'texter',
          principalId: 'sam',
          channelId: 'internal',
        ),
      if (r.subjectKind == AdminSubjectKind.user && r.expiresAt == null)
        GrantHint(code: 'expiry_missing', principalId: r.subjectId),
    ],
    text: _isExample(r) ? previewExample() : 'Preview of ${r.subjectId}',
    textLocale: _isExample(r) ? 'de' : 'en',
  );

  /// The targets the caller sees.
  Future<List<AdminTarget>> targets() async {
    refused('targets');
    return List.of(targetList);
  }

  /// The agents.
  Future<List<AdminAgent>> agents() async {
    refused('agents');
    return List.of(agentList);
  }

  /// The preview: [previewAnswer] or the default.
  Future<GrantPreviewResult> previewGrant(GrantRequest request) async {
    previews.add(request);
    refused('previewGrant');
    return previewAnswer?.call(request) ?? _default(request);
  }

  /// Creates the grant.
  Future<AdminGrant> createGrant(
    GrantRequest request, {
    required String purpose,
    I1Resolution? resolveI1,
  }) async {
    await change(
      'createGrant',
      '${request.subjectId} ${request.target.key} ${request.level.wire} '
          '${request.reason}',
      purpose,
    );
    i1Guard('createGrant', resolveI1);
    final grant = AdminGrant(
      id: 'grant-${_grants++}',
      subjectKind: request.subjectKind,
      subjectId: request.subjectId,
      targetKind: request.target.kind,
      targetId: request.target.id,
      level: request.level,
      capabilities: request.capabilities.toList(),
      expiresAt: request.expiresAt,
      reason: request.reason,
      grantedAt: DateTime.utc(2026, 10, 17, 9),
    );
    grantList.add(grant);
    return grant;
  }

  /// The grants on [target].
  Future<List<AdminGrant>> grantsOn(AdminTarget target) async {
    refused('grantsOn');
    return [
      for (final g in grantList)
        if (g.targetKind == target.kind && g.targetId == target.id) g,
    ];
  }

  /// Revokes [grantId].
  Future<AdminGrant> revokeGrant(
    String grantId, {
    required String reason,
    required String purpose,
  }) async {
    await change('revokeGrant', '$grantId $reason', purpose);
    final grant = grantList.firstWhere((g) => g.id == grantId);
    grantList.remove(grant);
    return grant;
  }

  /// Who has access to [target]: the direct grants and [extraAccess].
  Future<AccessExplanationResult> explain(AdminTarget target) async {
    refused('explain');
    return AccessExplanationResult([
      for (final g in grantList)
        if (g.targetKind == target.kind && g.targetId == target.id)
          AccessEntryInfo(
            principal: _principal(g.subjectId),
            level: g.level ?? AccessLevel.none,
            capabilities: g.capabilities,
            sources: [
              AccessSourceInfo(
                kind: AccessSourceKind.direct,
                grantId: g.id,
                expiresAt: g.expiresAt,
                reason: g.reason,
              ),
            ],
          ),
      ...?extraAccess[target.id],
    ]);
  }

  /// The snapshot of [userId].
  Future<ViewAsResult> viewAs(String userId, {required String purpose}) async {
    await change('viewAs', userId, purpose);
    return snapshots[userId] ??
        (throw const SelfActionFailed(
          ApiProblem(status: 404, code: 'not_found'),
        ));
  }

  /// The permission hints of the fake (empty until [seedHints]), in the server's order.
  List<AccessHint> hintList = [];

  /// The access that expires soon (empty until [seedHints]), ordered by expiry.
  List<ExpiringEntry> expiringList = [];

  /// The names the pages of hints and of expiring access carry.
  AccessPageLabels hintLabels = const AccessPageLabels();

  /// Hints or expiring entries per page (an offset cursor).
  int hintPageSize = 50;

  /// The requests of [permissionHints] and [expiringAccess]: `<op> <space> <kind> <cursor>`.
  final List<String> hintRequests = [];

  /// A grant of the hint world, also stored as a valid grant so revoke and extend work.
  AdminGrant _seedGrant(
    String id,
    AdminSubjectKind kind,
    String subject,
    AdminTarget target,
    AccessLevel level, {
    List<AccessCapability> capabilities = const [],
    DateTime? expiresAt,
  }) {
    final grant = AdminGrant(
      id: id,
      subjectKind: kind,
      subjectId: subject,
      targetKind: target.kind,
      targetId: target.id,
      level: level,
      capabilities: capabilities,
      expiresAt: expiresAt,
      reason: 'Seed $id',
      grantedAt: DateTime.utc(2026, 10, 1),
    );
    grantList.add(grant);
    return grant;
  }

  /// The world of 07 9.4 and 9.5 around the example of 07 9.3: Sam can learn the content of
  /// Marketing/Intern through Texter (H1); Robin is disabled and still reads
  /// Marketing/Kampagnen; the group Support triggers on Marketing, which SR2 blocks; Texter
  /// runs on Sam's subscription while Sam and Robin may trigger it. Expiring soon: Sam's grant
  /// on Marketing/Kampagnen, Robin's membership of Support, a token of Texter and one of the
  /// app Board. The test clock is 2026-10-17 09:00 UTC.
  void seedHints() {
    final disabled = _seedGrant(
      'g-robin',
      AdminSubjectKind.user,
      'robin',
      campaigns,
      AccessLevel.read,
    );
    final blocked = _seedGrant(
      'g-support',
      AdminSubjectKind.group,
      'support',
      marketing,
      AccessLevel.trigger,
    );
    final expiring = _seedGrant(
      'g-sam',
      AdminSubjectKind.user,
      'sam',
      campaigns,
      AccessLevel.trigger,
      capabilities: const [AccessCapability.upload],
      expiresAt: DateTime.utc(2026, 10, 21, 22),
    );
    hintList = [
      const AccessHint(
        kind: AccessHintKind.h1ReadHorizon,
        agentId: 'texter',
        personId: 'sam',
        spaceId: 'marketing',
        channelId: 'internal',
      ),
      AccessHint(kind: AccessHintKind.grantToDisabled, grant: disabled),
      AccessHint(
        kind: AccessHintKind.triggerBlocked,
        grant: blocked,
        rules: const ['sr2'],
      ),
      const AccessHint(
        kind: AccessHintKind.agentPersonal,
        agentId: 'texter',
        credentialHolderId: 'sam',
        triggerSet: ['sam', 'robin'],
      ),
    ];
    expiringList = [
      ExpiringEntry(
        kind: ExpiringKind.grant,
        expiresAt: expiring.expiresAt!,
        grant: expiring,
      ),
      ExpiringEntry(
        kind: ExpiringKind.membership,
        expiresAt: DateTime.utc(2026, 10, 22, 22),
        membershipId: 'm-robin',
        groupId: 'support',
        userId: 'robin',
      ),
      ExpiringEntry(
        kind: ExpiringKind.token,
        expiresAt: DateTime.utc(2026, 10, 23, 8),
        tokenId: 't-texter',
        principalId: 'texter',
      ),
      ExpiringEntry(
        kind: ExpiringKind.token,
        expiresAt: DateTime.utc(2026, 10, 24, 8),
        tokenId: 't-board',
        principalId: 'board',
      ),
    ];
    AccessPrincipal p(String id, String name, String kind) =>
        AccessPrincipal(id: id, kind: kind, handle: id, displayName: name);
    hintLabels = AccessPageLabels(
      principals: {
        'sam': p('sam', 'Sam', 'user'),
        'robin': p('robin', 'Robin', 'user'),
        'texter': p('texter', 'Texter', 'agent'),
        'board': p('board', 'Board', 'app'),
      },
      groups: const {'support': 'Support'},
      spaces: const {'marketing': 'Marketing'},
      channels: const {
        'campaigns': (spaceId: 'marketing', name: 'Kampagnen'),
        'internal': (spaceId: 'marketing', name: 'Intern'),
      },
    );
  }

  /// Whether [grant] is still valid in the fake (a revoked one leaves [grantList]).
  bool _valid(AdminGrant? grant) =>
      grant == null || grantList.any((g) => g.id == grant.id);

  /// Whether [spaceId] covers a hint or entry with [space] or the target of [grant].
  bool _inSpace(String? spaceId, {String? space, AdminGrant? grant}) {
    if (spaceId == null) return true;
    if (space != null) return space == spaceId;
    final target = grant?.targetId;
    if (target == null) return false;
    return target == spaceId || hintLabels.channels[target]?.spaceId == spaceId;
  }

  (List<T>, String?) _page<T>(List<T> all, String? cursor) {
    final start = int.parse(cursor ?? '0');
    final end = (start + hintPageSize).clamp(0, all.length);
    return (all.sublist(start, end), end < all.length ? '$end' : null);
  }

  /// One page of the hints: of [spaceId] (its H1 hints and grants on it; `agent_personal`
  /// only for the instance), of [kind], after [cursor].
  Future<AccessHintPage> permissionHints({
    String? spaceId,
    AccessHintKind? kind,
    String? cursor,
  }) async {
    hintRequests.add(
      'hints ${spaceId ?? '-'} ${kind?.wire ?? '-'} ${cursor ?? '-'}',
    );
    refused('permissionHints');
    final all = [
      for (final h in hintList)
        if ((kind == null || h.kind == kind) &&
            _valid(h.grant) &&
            (h.kind == AccessHintKind.agentPersonal
                ? spaceId == null
                : _inSpace(spaceId, space: h.spaceId, grant: h.grant)))
          h,
    ];
    final (items, next) = _page(all, cursor);
    return AccessHintPage(items, hintLabels, nextCursor: next);
  }

  /// One page of the access that expires soon: of [spaceId] (grants on it; memberships and
  /// tokens only for the instance), after [cursor].
  Future<ExpiringPage> expiringAccess({String? spaceId, String? cursor}) async {
    hintRequests.add('expiring ${spaceId ?? '-'} - ${cursor ?? '-'}');
    refused('expiringAccess');
    final all = [
      for (final e in expiringList)
        if (_valid(e.grant) &&
            (e.kind == ExpiringKind.grant
                ? _inSpace(spaceId, grant: e.grant)
                : spaceId == null))
          e,
    ];
    final (items, next) = _page(all, cursor);
    return ExpiringPage(items, hintLabels, nextCursor: next);
  }
}
