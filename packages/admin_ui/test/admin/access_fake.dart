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
}
