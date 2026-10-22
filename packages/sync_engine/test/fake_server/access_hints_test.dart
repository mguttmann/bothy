// SPDX-License-Identifier: Apache-2.0

// The lists "Permission hints" and "Expiring soon" of the fake (brief M6-07; 07 6, 9.3 to 9.5,
// SR2): H1 from the access lists and operators of the world, grants to disabled people, trigger
// grants that SR2 blocks, personal agents with their trigger set, the scope of a space, paging,
// and the window of `authz.expiry_reminder_lead` on a clock that stands still.

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart' show SystemClock;
import 'package:dio/dio.dart';
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

/// The IDs a page names, by kind of label.
class _Names {
  final Set<String> principals = {};
  final Set<String> groups = {};
  final Set<String> spaces = {};
  final Set<String> channels = {};
}

void main() {
  late DateTime start;
  late FakeWorldHarness h;
  late FakeWorld world;
  late WorldSeed w;
  late TestDevice alex;
  late AccessApi access;

  setUp(() async {
    // The world's clock stands still, so the window of `authz.expiry_reminder_lead` is exact to
    // the millisecond. It starts at the system time because the auth session signs its DPoP
    // proofs with the system clock and the fake checks their `iat` against the world's clock
    // (08 7.3); whole milliseconds, as the API stamps them.
    final system = const SystemClock().now();
    start = DateTime.fromMillisecondsSinceEpoch(
      system.millisecondsSinceEpoch,
      isUtc: true,
    );
    h = await FakeWorldHarness.start(
      options: FakeServerOptions(now: () => start),
    );
    world = h.server.world;
    w = h.seed;
    alex = h.device();
    await signedIn(h, alex);
    access = alex.api.getAccessApi();
  });
  tearDown(() => h.stop());

  Future<ApiProblem?> problemOf(Future<Object?> call) async {
    try {
      await call;
      return null;
    } on DioException catch (e) {
      return ApiProblem.of(e);
    }
  }

  /// Alex grants [level] on [channel] to [subject] through the API, expiring at [expiresAt].
  Future<Grant> grant(
    String subject,
    String channel,
    Level level, {
    DateTime? expiresAt,
    GrantSubjectKind kind = GrantSubjectKind.user,
  }) async => (await access.createGrant(
    grantCreate: GrantCreate(
      (b) => b
        ..subjectKind = kind
        ..subjectId = subject
        ..targetKind = TargetKind.channel
        ..targetId = channel
        ..level = level
        ..expiresAt = expiresAt
        ..reason = 'Rota',
    ),
  )).data!;

  /// The example of 07 9.3: agent Texter operates the space Marketing and reads its channels
  /// Kampagnen and Intern; Sam has `trigger` on Kampagnen only.
  ({String space, FakeAgent texter, String kampagnen, String intern})
  marketing() {
    final texter = world.addAgent(handle: 'texter', displayName: 'Texter');
    final space = world.addSpace('Marketing');
    world.setSpaceOperator(space, texter.id);
    final kampagnen = world.addChannel(space, 'Kampagnen', {
      texter.id: 'write',
      w.sam.id: 'trigger',
    });
    final intern = world.addChannel(space, 'Intern', {texter.id: 'read'});
    return (space: space, texter: texter, kampagnen: kampagnen, intern: intern);
  }

  /// One hint of every kind, in the order of the list: H1 of [marketing] (space Marketing), a
  /// grant to the disabled Robin K on `support`, Sam's `trigger` on `build` of the host agent
  /// Coder (both space Team), Sam's personal agent Helper that operates `support`, where Sam has
  /// `trigger`, and the personal agent Orphan without holder and without a channel. The
  /// personal agents come last: the I1 guard of the grant writes would refuse their world.
  Future<
    ({
      String marketing,
      Grant disabled,
      Grant blocked,
      FakeAgent helper,
      FakeAgent orphan,
    })
  >
  everyKind() async {
    final m = marketing();
    final robin = world.addPerson(handle: 'robin.k', displayName: 'Robin K');
    final disabled = await grant(robin.id, w.support, Level.read);
    world.disablePerson(robin.id);
    final coder = world.addAgent(handle: 'coder', displayName: 'Coder')
      ..toolClass = 'host';
    final build = world.addChannel(w.space, 'build', {coder.id: 'write'});
    world.setChannelOperator(build, coder.id);
    final blocked = await grant(w.sam.id, build, Level.trigger);
    final helper = world.addAgent(
      handle: 'helper',
      displayName: 'Helper',
      billingMode: 'personal',
      credentialHolderId: w.sam.id,
    );
    world
      ..setChannelOperator(w.support, helper.id)
      ..setAccess(w.sam.id, w.support, 'trigger');
    final orphan = world.addAgent(
      handle: 'orphan',
      displayName: 'Orphan',
      billingMode: 'personal',
    );
    return (
      marketing: m.space,
      disabled: disabled,
      blocked: blocked,
      helper: helper,
      orphan: orphan,
    );
  }

  void nameGrant(Grant g, _Names n) {
    (g.subjectKind == GrantSubjectKind.group ? n.groups : n.principals).add(
      g.subjectId,
    );
    if (g.targetKind == TargetKind.space) n.spaces.add(g.targetId!);
    if (g.targetKind == TargetKind.channel) n.channels.add(g.targetId!);
  }

  /// Expects [labels] to name exactly [n], plus the space of every channel of [n].
  void expectNamed(AccessLabels labels, _Names n) {
    for (final c in n.channels) {
      n.spaces.add(world.channels[c]!.spaceId!);
    }
    expect({for (final p in labels.principals) p.id}, n.principals);
    expect({for (final g in labels.groups) g.id}, n.groups);
    expect({for (final s in labels.spaces) s.id}, n.spaces);
    expect({for (final c in labels.channels) c.id}, n.channels);
    for (final c in labels.channels) {
      expect(c.spaceId, world.channels[c.id]!.spaceId);
      expect(c.name, world.channels[c.id]!.json['name']);
    }
    for (final s in labels.spaces) {
      expect(s.name, world.spaces[s.id]!['name']);
    }
    for (final p in labels.principals) {
      expect(p.displayName, world.people[p.id]!.displayName);
    }
  }

  void expectHintLabels(PermissionHintPage page) {
    final n = _Names();
    for (final x in page.items) {
      n.principals.addAll([
        ?x.agentId,
        ?x.personId,
        ?x.credentialHolderId,
        ...?x.triggerSet,
      ]);
      if (x.spaceId case final s?) n.spaces.add(s);
      if (x.channelId case final c?) n.channels.add(c);
      if (x.grant case final g?) nameGrant(g, n);
    }
    expectNamed(page.labels, n);
  }

  void expectExpiringLabels(ExpiringAccessPage page) {
    final n = _Names();
    for (final x in page.items) {
      if (x.grant case final g?) nameGrant(g, n);
      if (x.groupId case final g?) n.groups.add(g);
      n.principals.addAll([?x.userId, ?x.principalId]);
    }
    expectNamed(page.labels, n);
  }

  Future<List<PermissionHint>> allHints({
    String? spaceId,
    PermissionHintKind? kind,
  }) async {
    final page = (await access.listPermissionHints(
      spaceId: spaceId,
      kind: kind,
      limit: 200,
    )).data!;
    expect(page.nextCursor, isNull);
    expectHintLabels(page);
    return page.items.toList();
  }

  test('H1: Sam learns Intern through Texter until Sam may read it', () async {
    final m = marketing();

    final page = (await access.listPermissionHints()).data!;
    final h1 = [
      for (final x in page.items)
        if (x.kind == PermissionHintKind.h1ReadHorizon) x,
    ];
    expect(h1, hasLength(1));
    expect(h1.single.agentId, m.texter.id);
    expect(h1.single.personId, w.sam.id);
    expect(h1.single.spaceId, m.space);
    expect(h1.single.channelId, m.intern);
    expect(h1.single.grant, isNull);
    final only = (await access.listPermissionHints(
      kind: PermissionHintKind.h1ReadHorizon,
    )).data!;
    expect(only.items, h1);
    expect(only.labels.principals.map((p) => p.displayName), ['Sam', 'Texter']);
    expect(only.labels.spaces.map((s) => s.name), ['Marketing']);
    expect(only.labels.channels.map((c) => c.name), ['Intern']);
    expect(only.labels.groups, isEmpty);
    expectHintLabels(only);

    await grant(w.sam.id, m.intern, Level.read);

    expect(await allHints(kind: PermissionHintKind.h1ReadHorizon), isEmpty);
    expect(h.server.violations, isEmpty);
  });

  test('a grant to a disabled person carries the grant', () async {
    final robin = world.addPerson(handle: 'robin.k', displayName: 'Robin K');
    final given = await grant(robin.id, w.support, Level.read);
    expect(await allHints(kind: PermissionHintKind.grantToDisabled), isEmpty);

    world.disablePerson(robin.id);

    final hints = await allHints(kind: PermissionHintKind.grantToDisabled);
    expect(hints, hasLength(1));
    expect(hints.single.grant!.id, given.id);
    expect(hints.single.agentId, isNull);
    expect(h.server.violations, isEmpty);
  });

  test('SR2 blocks a trigger grant on the channel of a host agent', () async {
    final coder = world.addAgent(handle: 'coder', displayName: 'Coder')
      ..toolClass = 'host';
    final build = world.addChannel(w.space, 'build', {coder.id: 'write'});
    world.setChannelOperator(build, coder.id);
    final helper = world.addAgent(handle: 'helper', displayName: 'Helper');
    world.setChannelOperator(w.support, helper.id);

    final blocked = await grant(w.sam.id, build, Level.trigger);
    // An owner passes SR2; a chat agent and a channel without operator block nothing.
    await grant(w.alex.id, build, Level.trigger);
    await grant(w.sam.id, w.support, Level.trigger);
    await grant(w.sam.id, w.general, Level.trigger);

    final hints = await allHints(kind: PermissionHintKind.triggerBlocked);
    expect(hints, hasLength(1));
    expect(hints.single.grant!.id, blocked.id);
    expect(hints.single.rules, [PermissionHintRulesEnum.sr2]);
    expect(h.server.violations, isEmpty);
  });

  test('a personal agent names its holder and its trigger set', () async {
    final helper = world.addAgent(
      handle: 'helper',
      displayName: 'Helper',
      billingMode: 'personal',
      credentialHolderId: w.sam.id,
    );
    world
      ..setChannelOperator(w.support, helper.id)
      ..setAccess(w.sam.id, w.support, 'trigger');
    final orphan = world.addAgent(
      handle: 'orphan',
      displayName: 'Orphan',
      billingMode: 'personal',
    );
    world.addAgent(handle: 'team', displayName: 'Team');

    final hints = await allHints(kind: PermissionHintKind.agentPersonal);

    expect(hints.map((x) => x.agentId), [helper.id, orphan.id]);
    expect(hints.first.credentialHolderId, w.sam.id);
    expect(hints.first.triggerSet, [w.alex.id, w.sam.id]..sort());
    expect(hints.last.credentialHolderId, isNull);
    expect(hints.last.triggerSet, [w.alex.id]);
    expect(h.server.violations, isEmpty);
  });

  test('the list keeps the order of kinds and the kind filter', () async {
    final x = await everyKind();

    final all = await allHints();

    expect(all.map((e) => e.kind), [
      PermissionHintKind.h1ReadHorizon,
      PermissionHintKind.grantToDisabled,
      PermissionHintKind.triggerBlocked,
      PermissionHintKind.agentPersonal,
      PermissionHintKind.agentPersonal,
    ]);
    expect(all[1].grant!.id, x.disabled.id);
    expect(all[2].grant!.id, x.blocked.id);
    expect(all[3].agentId, x.helper.id);
    expect(all[4].agentId, x.orphan.id);
    for (final kind in PermissionHintKind.values) {
      if (kind == PermissionHintKind.unknownDefaultOpenApi) continue;
      expect(await allHints(kind: kind), [
        for (final e in all)
          if (e.kind == kind) e,
      ], reason: '$kind');
    }
    expect(h.server.violations, isEmpty);
  });

  test('a small limit puts every hint on exactly one page', () async {
    await everyKind();
    final all = await allHints();
    final seen = <PermissionHint>[];
    String? cursor;
    var pages = 0;
    do {
      final page = (await access.listPermissionHints(
        limit: 2,
        cursor: cursor,
      )).data!;
      expect(page.items.length, lessThanOrEqualTo(2));
      expectHintLabels(page);
      seen.addAll(page.items);
      cursor = page.nextCursor;
      pages++;
    } while (cursor != null);

    expect(seen, all);
    expect(pages, 3);
    final first = (await access.listPermissionHints()).data!;
    expect(first.items, all, reason: 'the default page holds them all');
    expect(first.nextCursor, isNull);
    expect(h.server.violations, isEmpty);
  });

  test('a space lists its own agents, channels and grants', () async {
    final x = await everyKind();

    final marketing = await allHints(spaceId: x.marketing);
    expect(marketing.map((e) => e.kind), [PermissionHintKind.h1ReadHorizon]);

    final team = await allHints(spaceId: w.space);
    expect(team.map((e) => e.kind), [
      PermissionHintKind.grantToDisabled,
      PermissionHintKind.triggerBlocked,
      PermissionHintKind.agentPersonal,
    ]);
    expect(team.last.agentId, x.helper.id);
    expect(h.server.violations, isEmpty);
  });

  test('the instance only for owners, a space only with manage', () async {
    final m = marketing();
    final finance = world.addSpace('Finance');
    world.addChannel(finance, 'budget', {w.alex.id: 'write'});
    final sam = h.device(login: WorldSeed.samLogin, keySeed: 3);
    await signedIn(h, sam);
    final samAccess = sam.api.getAccessApi();

    for (final p in [
      await problemOf(samAccess.listPermissionHints()),
      await problemOf(samAccess.listExpiringAccess()),
    ]) {
      expect(p?.status, 403);
      expect(p?.code, 'forbidden');
      expect(p?.reason, 'owner_only');
    }
    for (final p in [
      await problemOf(samAccess.listPermissionHints(spaceId: finance)),
      await problemOf(samAccess.listExpiringAccess(spaceId: finance)),
      await problemOf(access.listPermissionHints(spaceId: world.nextId())),
      await problemOf(access.listExpiringAccess(spaceId: world.nextId())),
    ]) {
      expect(p?.status, 404);
      expect(p?.code, 'not_found');
    }
    // Sam reads Team; `manage` in one of its channels is not `manage` on the space.
    world.grantCapability(w.sam.id, w.general, 'manage');
    for (final p in [
      await problemOf(samAccess.listPermissionHints(spaceId: w.space)),
      await problemOf(samAccess.listExpiringAccess(spaceId: w.space)),
    ]) {
      expect(p?.status, 403);
      expect(p?.code, 'forbidden');
    }

    world
      ..grantCapability(w.sam.id, m.kampagnen, 'manage')
      ..grantCapability(w.sam.id, m.intern, 'manage');
    final hints = (await samAccess.listPermissionHints(spaceId: m.space)).data!;
    expect(hints.items.single.channelId, m.intern);
    expect(
      (await samAccess.listExpiringAccess(spaceId: m.space)).data!.items,
      isEmpty,
    );
    expect(h.server.violations, isEmpty);
  });

  group('expiring soon', () {
    late Duration lead;
    late Grant first;
    late Grant edge;
    late Grant tie;
    late Grant elsewhere;
    late String support;
    late String design;
    late FakeAgent texter;
    late String token;

    /// Grants, memberships and agent tokens in and around the window from [start]: inside are
    /// [first] (1 hour), [tie] with Sam's membership in Support and the [token] (5 hours each),
    /// Sam's membership in Design (6 hours), [elsewhere] on Marketing (7 hours) and [edge]
    /// (exactly the lead). Outside are a grant a millisecond after the lead, an expired grant,
    /// a revoked grant, Alex's membership after the lead, a far and a revoked token.
    setUp(() async {
      lead = world.expiryReminderLead;
      DateTime at(Duration d) => start.add(d);
      first = await grant(
        w.sam.id,
        w.general,
        Level.read,
        expiresAt: at(const Duration(hours: 1)),
      );
      edge = await grant(w.sam.id, w.support, Level.read, expiresAt: at(lead));
      await grant(
        w.sam.id,
        w.private,
        Level.read,
        expiresAt: at(lead + const Duration(milliseconds: 1)),
      );
      await grant(
        w.sam.id,
        w.general,
        Level.write,
        expiresAt: at(const Duration(hours: -1)),
      );
      final revoked = await grant(
        w.sam.id,
        w.general,
        Level.write,
        expiresAt: at(const Duration(hours: 2)),
      );
      await access.revokeGrant(
        grantId: revoked.id,
        reasonBody: ReasonBody((b) => b..reason = 'Not needed'),
      );

      final groups = alex.api.getGroupsApi();
      Future<String> group(String name, Map<String, Duration> members) async {
        final g = (await groups.createGroup(
          groupCreate: GroupCreate((b) => b..name = name),
        )).data!;
        for (final MapEntry(key: user, value: d) in members.entries) {
          await groups.addGroupMember(
            groupId: g.id,
            groupMemberAdd: GroupMemberAdd(
              (b) => b
                ..userId = user
                ..expiresAt = at(d),
            ),
          );
        }
        return g.id;
      }

      support = await group('Support', {
        w.sam.id: const Duration(hours: 5),
        w.alex.id: lead + const Duration(hours: 1),
      });
      design = await group('Design', {w.sam.id: const Duration(hours: 6)});
      tie = await grant(
        support,
        w.support,
        Level.read,
        kind: GrantSubjectKind.group,
        expiresAt: at(const Duration(hours: 5)),
      );
      final m = marketing();
      elsewhere = await grant(
        w.sam.id,
        m.kampagnen,
        Level.read,
        expiresAt: at(const Duration(hours: 7)),
      );

      texter = m.texter;
      final tokens = alex.api.getTokensApi();
      Future<ApiTokenCreated> create(Duration d) async =>
          (await tokens.createToken(
            principalId: texter.id,
            apiTokenCreate: ApiTokenCreate(
              (b) => b
                ..name = 'Adapter'
                ..dpopJkt = 'NzbLsXh8uDCcd-6MNwXF4W_7noWXFZAfHkxZsRGC9Xs'
                ..expiresAt = at(d),
            ),
          )).data!;
      token = (await create(const Duration(hours: 5))).token.id;
      await create(const Duration(days: 30));
      final gone = await create(const Duration(hours: 1));
      await tokens.revokeToken(tokenId: gone.token.id);
    });

    test('lists what expires within the lead, by expiry and kind', () async {
      final page = (await access.listExpiringAccess()).data!;

      expect(page.items.map((x) => (x.kind, x.expiresAt)), [
        (ExpiringAccessItemKindEnum.grant, start.add(const Duration(hours: 1))),
        (ExpiringAccessItemKindEnum.grant, start.add(const Duration(hours: 5))),
        (
          ExpiringAccessItemKindEnum.membership,
          start.add(const Duration(hours: 5)),
        ),
        (ExpiringAccessItemKindEnum.token, start.add(const Duration(hours: 5))),
        (
          ExpiringAccessItemKindEnum.membership,
          start.add(const Duration(hours: 6)),
        ),
        (ExpiringAccessItemKindEnum.grant, start.add(const Duration(hours: 7))),
        (ExpiringAccessItemKindEnum.grant, start.add(lead)),
      ]);
      expect(
        [for (final x in page.items) x.grant?.id],
        [first.id, tie.id, null, null, null, elsewhere.id, edge.id],
      );
      final members = [
        for (final x in page.items)
          if (x.kind == ExpiringAccessItemKindEnum.membership) x,
      ];
      expect(members.map((x) => (x.groupId, x.userId)), [
        (support, w.sam.id),
        (design, w.sam.id),
      ]);
      final uuid = RegExp(
        r'^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$',
      );
      expect(members.map((x) => x.membershipId), everyElement(matches(uuid)));
      expect(members.first.membershipId, isNot(members.last.membershipId));
      final again = (await access.listExpiringAccess()).data!;
      expect(
        [for (final x in again.items) x.membershipId],
        [for (final x in page.items) x.membershipId],
        reason: 'a membership keeps its ID',
      );
      final onToken = page.items[3];
      expect(onToken.tokenId, token);
      expect(onToken.principalId, texter.id);
      expectExpiringLabels(page);
      expect(page.nextCursor, isNull);
      expect(h.server.violations, isEmpty);
    });

    test('a token item names the token, never its value', () async {
      final raw = await alex.api.dio.get<Object?>(
        '/access/expiring',
        options: Options(
          extra: {
            'secure': [
              {'type': 'http', 'scheme': 'DPoP', 'name': 'dpopAuth'},
            ],
          },
        ),
      );
      final items = (raw.data! as Map)['items']! as List;
      final tokens = [
        for (final x in items.cast<Map<Object?, Object?>>())
          if (x['kind'] == 'token') x,
      ];
      expect(tokens, hasLength(1));
      expect(tokens.single.keys.toSet(), {
        'kind',
        'expires_at',
        'token_id',
        'principal_id',
      });
      final prefix = texter.tokens[token]!['token_prefix']! as String;
      expect(raw.data.toString(), isNot(contains(prefix)));
      expect(h.server.violations, isEmpty);
    });

    test('a space lists its grants and the memberships of their groups, '
        'no tokens', () async {
      final page = (await access.listExpiringAccess(spaceId: w.space)).data!;

      expect(page.items.map((x) => x.kind), [
        ExpiringAccessItemKindEnum.grant,
        ExpiringAccessItemKindEnum.grant,
        ExpiringAccessItemKindEnum.membership,
        ExpiringAccessItemKindEnum.grant,
      ]);
      expect(
        [for (final x in page.items) x.grant?.id ?? x.groupId],
        [first.id, tie.id, support, edge.id],
      );
      expectExpiringLabels(page);
      expect(h.server.violations, isEmpty);
    });

    test('a small limit puts every item on exactly one page', () async {
      final all = (await access.listExpiringAccess()).data!.items.toList();
      final seen = <ExpiringAccessItem>[];
      String? cursor;
      do {
        final page = (await access.listExpiringAccess(
          limit: 3,
          cursor: cursor,
        )).data!;
        expect(page.items.length, lessThanOrEqualTo(3));
        expectExpiringLabels(page);
        seen.addAll(page.items);
        cursor = page.nextCursor;
      } while (cursor != null);

      expect(seen, all);
      expect(h.server.violations, isEmpty);
    });
  });
}
