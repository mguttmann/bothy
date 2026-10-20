// SPDX-License-Identifier: Apache-2.0

// The agent administration of the fake (brief M4-28, screen A5): the agent card with the
// badge data and the warnings of 10 8, the backend policy per agent and for all agents with
// I1 (10 5.4, 9.1, BR-08), the guided flow `resolve_i1` on grants, group members and owners
// (10 11, BR-07) and the agent tokens of 08 12 (ADR 0061).

import 'dart:convert';

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late FakeWorldHarness h;
  late FakeWorld world;
  late WorldSeed w;
  late TestDevice alex;

  setUp(() async {
    h = await FakeWorldHarness.start();
    world = h.server.world;
    w = h.seed;
    alex = h.device();
    await signedIn(h, alex);
    h.server.requests.clear();
  });
  tearDown(() => h.stop());

  List<String> ops() => [
    for (final r in h.server.requests) '${r.operationId} ${r.status}',
  ];

  Future<DioException?> failure(Future<Object?> call) async {
    try {
      await call;
      return null;
    } on DioException catch (e) {
      return e;
    }
  }

  JsonMap bodyOf(DioException? e) =>
      Map<String, Object?>.from(e!.response!.data as Map);

  List<JsonMap> rowsOf(String action) => [
    for (final r in h.server.audit.rows)
      if (r['action'] == action) r,
  ];

  /// Alex's personal agent "Support" that operates the space Team.
  FakeAgent personalSupport() {
    final a = world.addAgent(
      handle: 'support',
      displayName: 'Support',
      billingMode: 'personal',
      credentialHolderId: w.alex.id,
    );
    world.setSpaceOperator(w.space, a.id);
    return a;
  }

  test(
    'the agent card carries the badge data and a warning per code',
    () async {
      final mismatch = world.addAgent(
        handle: 'mismatch',
        displayName: 'Mismatch',
        reportedBillingMode: 'personal',
      );
      final unsupported = world.addAgent(
        handle: 'unsupported',
        displayName: 'Unsupported',
        billingModesSupported: const ['personal'],
      );
      final unknown = world.addAgent(
        handle: 'unknown',
        displayName: 'Unknown',
        keyType: 'unknown',
      );
      final offline = world.addAgent(
        handle: 'offline',
        displayName: 'Offline',
        keyType: 'unknown',
        state: 'offline',
      );
      final held = personalSupport();
      final agents = alex.api.getAgentsApi();

      final card = (await agents.getAgent(agentId: mismatch.id)).data!;
      expect(card.handle, 'mismatch');
      expect(card.billingMode, BillingMode.organization);
      expect(card.presence.state, AgentRuntimeState.online);
      expect(card.presence.reportedBillingMode, BillingMode.personal);
      expect(card.presence.billingModeMatches, isFalse);
      expect(card.presence.warnings, ['billing_mode_mismatch']);
      expect(
        card.presence.billingEvidence!.keyType,
        BillingEvidenceKeyTypeEnum.apiKey,
      );
      expect(card.presence.billingEvidence!.provider, 'anthropic');
      expect(card.presence.lastHeartbeatAt, isNotNull);

      final noReport = (await agents.getAgent(agentId: unsupported.id)).data!;
      expect(noReport.presence.reportedBillingMode, isNull);
      expect(noReport.presence.billingModeMatches, isNull);
      expect(noReport.presence.warnings, ['billing_mode_unsupported']);
      expect(
        (await agents.getAgent(agentId: unknown.id)).data!.presence.warnings,
        ['billing_evidence_unknown'],
      );
      expect(
        (await agents.getAgent(agentId: offline.id)).data!.presence.warnings,
        isEmpty,
        reason: 'warnings only while online or degraded (10 8)',
      );
      final personal = (await agents.getAgent(agentId: held.id)).data!;
      expect(personal.billingMode, BillingMode.personal);
      expect(personal.credentialHolderId, w.alex.id);

      final list = (await agents.listAgents()).data!.items;
      expect(list.map((a) => a.handle), [
        'robin',
        'mismatch',
        'unsupported',
        'unknown',
        'offline',
        'support',
      ]);
      expect(
        list.first.billingMode,
        BillingMode.organization,
        reason: 'an agent person without a FakeAgent gets the defaults',
      );
      expect(world.agents[w.robin.id], isNotNull);

      world.agentHeartbeat(mismatch.id, reportedBillingMode: 'organization');
      final healed = (await agents.getAgent(agentId: mismatch.id)).data!;
      expect(healed.presence.billingModeMatches, isTrue);
      expect(healed.presence.warnings, isEmpty);
      world.agentHeartbeat(mismatch.id, clearReported: true, state: 'degraded');
      expect(
        world.agentJson(mismatch)['presence'],
        isNot(contains('reported_billing_mode')),
      );
      expect(
        (world.agentJson(mismatch)['presence']! as Map)['state'],
        'degraded',
      );

      final missing = await failure(agents.getAgent(agentId: world.nextId()));
      expect(missing?.response?.statusCode, 404);
      expect(h.server.violations, isEmpty);
    },
  );

  test('the operators of space and channel follow the space default', () {
    final a = world.addAgent(handle: 'a', displayName: 'A');
    final b = world.addAgent(handle: 'b', displayName: 'B');
    world.setSpaceOperator(w.space, a.id);
    JsonMap op(String channel) =>
        world.channels[channel]!.json['operator']! as JsonMap;
    expect(world.spaces[w.space]!['default_operator_agent_id'], a.id);
    expect(op(w.general), {'mode': 'inherit', 'effective_agent_id': a.id});
    world.setChannelOperator(w.general, b.id);
    expect(op(w.general), {
      'mode': 'agent',
      'agent_id': b.id,
      'effective_agent_id': b.id,
    });
    world.setSpaceOperator(w.space, null);
    expect(world.spaces[w.space], isNot(contains('default_operator_agent_id')));
    expect(op(w.support), {'mode': 'inherit'});
    expect(op(w.general)['effective_agent_id'], b.id);
    world.setChannelOperator(w.general, null);
    expect(op(w.general), {'mode': 'inherit'});
  });

  test('a switch to personal with a second triggerer answers 409 and changes nothing', () async {
    final a = world.addAgent(handle: 'support', displayName: 'Support');
    world.setChannelOperator(w.general, a.id);
    world.channels[w.general]!.access[w.sam.id] = 'trigger';
    final agents = alex.api.getAgentsApi();
    final version = world.permissionVersion;
    BillingModeSet toPersonal([String? holder]) => BillingModeSet(
      (b) => b
        ..billingMode = BillingMode.personal
        ..credentialHolderId = holder
        ..reason = 'Alex pays',
    );

    final blocked = await failure(
      agents.setAgentBillingMode(
        agentId: a.id,
        ifMatch: '"1"',
        billingModeSet: toPersonal(w.alex.id),
      ),
    );
    expect(blocked?.response?.statusCode, 409);
    final problem = bodyOf(blocked);
    expect(problem['code'], 'i1_violation');
    expect(problem['hint'], 'Switch the agent to organization first');
    expect(problem['conflicts'], [
      {
        'code': 'i1_violation',
        'agent_id': a.id,
        'credential_holder_id': w.alex.id,
        'principal_ids': [w.sam.id],
      },
    ]);
    expect(ops(), [
      'setAgentBillingMode 401',
      'createStepUpChallenge 200',
      'completeStepUpWithDeviceKey 200',
      'setAgentBillingMode 409',
    ]);
    expect(a.billingMode, 'organization');
    expect(a.credentialHolderId, isNull);
    expect(a.version, 1);
    expect(world.permissionVersion, version);
    expect(rowsOf('agent.billing_mode.set').single['decision'], 'deny');
    expect(
      rowsOf('agent.billing_mode.set').single['deny_reason'],
      'i1_violation',
    );

    final noHolder = await failure(
      agents.setAgentBillingMode(
        agentId: a.id,
        ifMatch: '"1"',
        billingModeSet: toPersonal(),
      ),
    );
    expect(noHolder?.response?.statusCode, 400);
    expect(bodyOf(noHolder)['errors'], [
      {'pointer': '/credential_holder_id', 'code': 'required'},
    ]);
    final robinHolds = await failure(
      agents.setAgentBillingMode(
        agentId: a.id,
        ifMatch: '"1"',
        billingModeSet: toPersonal(w.robin.id),
      ),
    );
    expect(robinHolds?.response?.statusCode, 400);
    final stale = await failure(
      agents.setAgentBillingMode(
        agentId: a.id,
        ifMatch: '"7"',
        billingModeSet: toPersonal(w.alex.id),
      ),
    );
    expect(stale?.response?.statusCode, 412);
    expect(bodyOf(stale)['code'], 'version_conflict');

    final sam = h.device(login: WorldSeed.samLogin, keySeed: 3);
    await signedIn(h, sam);
    final notOwner = await failure(
      sam.api.getAgentsApi().setAgentBillingMode(
        agentId: a.id,
        ifMatch: '"1"',
        billingModeSet: toPersonal(w.sam.id),
      ),
    );
    expect(notOwner?.response?.statusCode, 403);
    expect(bodyOf(notOwner)['reason'], 'owner_only');

    world.channels[w.general]!.access[w.sam.id] = 'write';
    final switched = (await agents.setAgentBillingMode(
      agentId: a.id,
      ifMatch: '"1"',
      billingModeSet: toPersonal(w.alex.id),
    )).data!;
    expect(switched.billingMode, BillingMode.personal);
    expect(switched.credentialHolderId, w.alex.id);
    expect(switched.version, 2);
    expect(switched.billingModeChangedAt, isNotNull);
    expect(world.permissionVersion, version + 1);
    final row = rowsOf('agent.billing_mode.set').last;
    expect(row['decision'], 'allow');
    expect(row['resource_kind'], 'agent');
    expect(row['resource_id'], a.id);
    expect(row['details'], {
      'from_billing_mode': 'organization',
      'to_billing_mode': 'personal',
      'reason': 'Alex pays',
      'to_credential_holder_id': w.alex.id,
    });
    expect(h.server.violations, isEmpty);
  });

  test('a switch to organization clears the holder; the holder moves only on personal', () async {
    final a = personalSupport();
    final agents = alex.api.getAgentsApi();

    final otherHolder = await failure(
      agents.setCredentialHolder(
        agentId: a.id,
        ifMatch: '"1"',
        credentialHolderSet: CredentialHolderSet(
          (b) => b
            ..credentialHolderId = w.sam.id
            ..reason = 'Handover',
        ),
      ),
    );
    expect(otherHolder?.response?.statusCode, 409);
    expect(bodyOf(otherHolder)['conflicts'], [
      {
        'code': 'i1_violation',
        'agent_id': a.id,
        'credential_holder_id': w.sam.id,
        'principal_ids': [w.alex.id],
      },
    ], reason: 'Alex as owner still triggers the agent');
    expect(a.credentialHolderId, w.alex.id);
    expect(a.version, 1);

    final org = (await agents.setAgentBillingMode(
      agentId: a.id,
      ifMatch: '"1"',
      billingModeSet: BillingModeSet(
        (b) => b
          ..billingMode = BillingMode.organization
          ..reason = 'Team uses it',
      ),
    )).data!;
    expect(org.billingMode, BillingMode.organization);
    expect(org.credentialHolderId, isNull);
    expect(a.credentialHolderId, isNull);
    expect(rowsOf('agent.billing_mode.set').single['details'], {
      'from_billing_mode': 'personal',
      'to_billing_mode': 'organization',
      'reason': 'Team uses it',
      'from_credential_holder_id': w.alex.id,
    });

    final notPersonal = await failure(
      agents.setCredentialHolder(
        agentId: a.id,
        ifMatch: '"2"',
        credentialHolderSet: CredentialHolderSet(
          (b) => b
            ..credentialHolderId = w.alex.id
            ..reason = 'Back',
        ),
      ),
    );
    expect(notPersonal?.response?.statusCode, 409);
    expect(bodyOf(notPersonal)['code'], 'conflict');

    // A holder that is not the only triggerer moves back to the one who is.
    final b = world.addAgent(
      handle: 'sams',
      displayName: 'Sams',
      billingMode: 'personal',
      credentialHolderId: w.sam.id,
    );
    final moved = (await agents.setCredentialHolder(
      agentId: b.id,
      ifMatch: '"1"',
      credentialHolderSet: CredentialHolderSet(
        (x) => x
          ..credentialHolderId = w.alex.id
          ..reason = 'Alex pays',
      ),
    )).data!;
    expect(moved.credentialHolderId, w.alex.id);
    expect(moved.version, 2);
    expect(rowsOf('agent.credential_holder.set').map((r) => r['decision']), [
      'deny',
      'allow',
    ], reason: 'the refused handover leaves its deny row (10 5.4)');
    expect(rowsOf('agent.credential_holder.set').last['details'], {
      'from_credential_holder_id': w.sam.id,
      'to_credential_holder_id': w.alex.id,
      'reason': 'Alex pays',
    });
    expect(h.server.violations, isEmpty);
  });

  test(
    'set_all to personal with one violating agent changes nothing (BR-08)',
    () async {
      final quiet = world.addAgent(handle: 'quiet', displayName: 'Quiet');
      final busy = world.addAgent(handle: 'busy', displayName: 'Busy');
      world.setChannelOperator(w.general, busy.id);
      world.channels[w.general]!.access[w.sam.id] = 'trigger';
      final agents = alex.api.getAgentsApi();
      BillingModeSetAll all(List<String> ids) => BillingModeSetAll(
        (b) => b
          ..billingMode = BillingMode.personal
          ..credentialHolderId = w.alex.id
          ..agentIds.addAll(ids)
          ..reason = 'Alex pays',
      );

      final blocked = await failure(
        agents.setAllAgentsBillingMode(
          billingModeSetAll: all([quiet.id, busy.id]),
        ),
      );
      expect(blocked?.response?.statusCode, 409);
      expect(bodyOf(blocked)['conflicts'], [
        {
          'code': 'i1_violation',
          'agent_id': busy.id,
          'credential_holder_id': w.alex.id,
          'principal_ids': [w.sam.id],
        },
      ]);
      expect(quiet.billingMode, 'organization');
      expect(busy.billingMode, 'organization');
      expect(quiet.version, 1);
      expect(rowsOf('agent.billing_mode.set_all').single['decision'], 'deny');

      final noList = await failure(
        agents.setAllAgentsBillingMode(
          billingModeSetAll: BillingModeSetAll(
            (b) => b
              ..billingMode = BillingMode.personal
              ..credentialHolderId = w.alex.id
              ..reason = 'Alex pays',
          ),
        ),
      );
      expect(noList?.response?.statusCode, 400);
      expect(bodyOf(noList)['errors'], [
        {'pointer': '/agent_ids', 'code': 'required'},
      ]);

      final done = (await agents.setAllAgentsBillingMode(
        billingModeSetAll: all([quiet.id]),
      )).data!;
      expect(done.changedAgentIds, [quiet.id]);
      expect(quiet.billingMode, 'personal');
      expect(quiet.credentialHolderId, w.alex.id);
      expect(h.server.violations, isEmpty);
    },
  );

  test('set_all to organization switches every personal agent with the server audit shape', () async {
    final first = personalSupport();
    final second = world.addAgent(
      handle: 'second',
      displayName: 'Second',
      billingMode: 'personal',
      credentialHolderId: w.alex.id,
    );
    world.addAgent(handle: 'team', displayName: 'Team');
    final agents = alex.api.getAgentsApi();
    final request = BillingModeSetAll(
      (b) => b
        ..billingMode = BillingMode.organization
        ..reason = 'Company key',
    );

    final result = (await agents.setAllAgentsBillingMode(
      billingModeSetAll: request,
    )).data!;
    expect(result.changedAgentIds, [first.id, second.id]);
    expect(first.billingMode, 'organization');
    expect(second.credentialHolderId, isNull);
    final row = rowsOf('agent.billing_mode.set_all').single;
    expect(row['resource_kind'], 'instance');
    expect(row['details'], {
      'to_billing_mode': 'organization',
      'reason': 'Company key',
      'agent_ids': [first.id, second.id],
      'from_billing_modes': ['personal', 'personal'],
      'held_agent_ids': [first.id, second.id],
      'from_credential_holder_ids': [w.alex.id, w.alex.id],
    });

    final again = (await agents.setAllAgentsBillingMode(
      billingModeSetAll: request,
    )).data!;
    expect(again.changedAgentIds, isEmpty);
    expect(rowsOf('agent.billing_mode.set_all').last['details'], {
      'to_billing_mode': 'organization',
      'reason': 'Company key',
      'no_change': true,
    });
    expect(h.server.violations, isEmpty);
  });

  test(
    'a grant with resolve_i1 switches and grants in one request (BR-07)',
    () async {
      final a = personalSupport();
      final access = alex.api.getAccessApi();
      GrantCreate grant({bool resolve = false}) => GrantCreate((b) {
        b
          ..subjectKind = GrantSubjectKind.user
          ..subjectId = w.sam.id
          ..targetKind = TargetKind.channel
          ..targetId = w.general
          ..level = Level.trigger
          ..reason = 'Release';
        if (resolve) {
          b.resolveI1.replace(
            ResolveI1(
              (r) => r
                ..switchToOrganization.add(a.id)
                ..reason = 'Sam triggers the assistant too',
            ),
          );
        }
      });

      final bare = await failure(access.createGrant(grantCreate: grant()));
      expect(bare?.response?.statusCode, 409);
      expect(bodyOf(bare)['conflicts'], [
        {
          'code': 'i1_violation',
          'agent_id': a.id,
          'credential_holder_id': w.alex.id,
          'principal_ids': [w.sam.id],
        },
      ]);
      expect(world.grants, isEmpty);
      expect(world.channels[w.general]!.access[w.sam.id], 'write');
      expect(a.billingMode, 'personal');

      final preview = (await access.previewGrant(grantCreate: grant())).data!;
      expect(preview.allowed, isFalse);
      final violation = preview.invariantViolations.single;
      expect(violation.code, 'i1_violation');
      expect(violation.agentId, a.id);
      expect(violation.credentialHolderId, w.alex.id);
      expect(violation.principalIds, [w.sam.id]);
      expect(violation.organizationReady, isTrue);
      expect(violation.hint, 'Switch the agent to organization first');
      expect(
        preview.text,
        contains('Support runs on the personal subscription'),
      );
      final dryRun = (await access.previewGrant(
        grantCreate: grant(resolve: true),
      )).data!;
      expect(dryRun.allowed, isTrue);
      expect(dryRun.invariantViolations, isEmpty);
      expect(a.billingMode, 'personal', reason: 'a preview changes nothing');
      expect(world.grants, isEmpty);

      final before = h.server.audit.rows.length;
      final created = (await access.createGrant(
        grantCreate: grant(resolve: true),
      )).data!;
      expect(created.level, Level.trigger);
      expect(a.billingMode, 'organization');
      expect(a.credentialHolderId, isNull);
      expect(a.version, 2);
      expect(world.channels[w.general]!.access[w.sam.id], 'trigger');
      final rows = h.server.audit.rows.sublist(before);
      expect(rows.map((r) => r['action']), [
        'agent.billing_mode.set',
        'grant.create',
      ]);
      expect(rows.first['details'], {
        'from_billing_mode': 'personal',
        'to_billing_mode': 'organization',
        'reason': 'Sam triggers the assistant too',
        'from_credential_holder_id': w.alex.id,
      });
      expect(rows.last['resource_id'], created.id);
      expect(rows.first['correlation_id'], rows.last['correlation_id']);
      expect(
        h.server.audit.rows.sublist(0, before).map((r) => r['correlation_id']),
        isNot(contains(rows.first['correlation_id'])),
      );
      expect(h.server.violations, isEmpty);
    },
  );

  test('a grant that would still violate I1 changes nothing', () async {
    final a = personalSupport();
    final b = world.addAgent(
      handle: 'general',
      displayName: 'General',
      billingMode: 'personal',
      credentialHolderId: w.alex.id,
    );
    world.setChannelOperator(w.general, b.id);
    final version = world.permissionVersion;
    final rows = h.server.audit.rows.length;

    final blocked = await failure(
      alex.api.getAccessApi().createGrant(
        grantCreate: GrantCreate(
          (x) => x
            ..subjectKind = GrantSubjectKind.user
            ..subjectId = w.sam.id
            ..targetKind = TargetKind.space
            ..targetId = w.space
            ..level = Level.trigger
            ..reason = 'Release'
            ..resolveI1.replace(
              ResolveI1(
                (r) => r
                  ..switchToOrganization.add(a.id)
                  ..reason = 'Sam triggers',
              ),
            ),
        ),
      ),
    );
    expect(blocked?.response?.statusCode, 409);
    expect(
      [
        for (final c in bodyOf(blocked)['conflicts']! as List)
          (c as Map)['agent_id'],
      ],
      [b.id],
    );
    expect(a.billingMode, 'personal');
    expect(a.credentialHolderId, w.alex.id);
    expect(a.version, 1);
    expect(world.grants, isEmpty);
    expect(world.channels[w.general]!.access[w.sam.id], 'write');
    expect(world.permissionVersion, version);
    expect(
      [
        for (final r in h.server.audit.rows.sublist(rows))
          '${r['action']} ${r['decision']}',
      ],
      ['grant.create deny'],
    );

    final unknown = await failure(
      alex.api.getAccessApi().createGrant(
        grantCreate: GrantCreate(
          (x) => x
            ..subjectKind = GrantSubjectKind.user
            ..subjectId = w.sam.id
            ..targetKind = TargetKind.channel
            ..targetId = w.support
            ..level = Level.read
            ..reason = 'Read'
            ..resolveI1.replace(
              ResolveI1(
                (r) => r
                  ..switchToOrganization.add(w.sam.id)
                  ..reason = 'Not an agent',
              ),
            ),
        ),
      ),
    );
    expect(unknown?.response?.statusCode, 404);
    expect(world.grants, isEmpty);
    expect(h.server.violations, isEmpty);
  });

  test('addGroupMember and addOwner take resolve_i1', () async {
    final a = personalSupport();
    final groups = alex.api.getGroupsApi();
    final group = (await groups.createGroup(
      groupCreate: GroupCreate((b) => b..name = 'Support'),
    )).data!;
    await alex.api.getAccessApi().createGrant(
      grantCreate: GrantCreate(
        (b) => b
          ..subjectKind = GrantSubjectKind.group
          ..subjectId = group.id
          ..targetKind = TargetKind.space
          ..targetId = w.space
          ..level = Level.trigger
          ..reason = 'Support team',
      ),
    );
    ResolveI1 resolve(FakeAgent agent) => ResolveI1(
      (r) => r
        ..switchToOrganization.add(agent.id)
        ..reason = 'Sam joins',
    );

    final bare = await failure(
      groups.addGroupMember(
        groupId: group.id,
        groupMemberAdd: GroupMemberAdd((b) => b..userId = w.sam.id),
      ),
    );
    expect(bare?.response?.statusCode, 409);
    expect(bodyOf(bare)['code'], 'i1_violation');
    expect(world.groupMembers[group.id], isEmpty);
    expect(a.billingMode, 'personal');

    final before = h.server.audit.rows.length;
    await groups.addGroupMember(
      groupId: group.id,
      groupMemberAdd: GroupMemberAdd(
        (b) => b
          ..userId = w.sam.id
          ..resolveI1.replace(resolve(a)),
      ),
    );
    expect(world.groupMembers[group.id], contains(w.sam.id));
    expect(a.billingMode, 'organization');
    var rows = h.server.audit.rows.sublist(before);
    expect(rows.map((r) => r['action']), [
      'agent.billing_mode.set',
      'group.member.add',
    ]);
    expect(rows.first['correlation_id'], rows.last['correlation_id']);

    final b = world.addAgent(
      handle: 'owners',
      displayName: 'Owners',
      billingMode: 'personal',
      credentialHolderId: w.alex.id,
    );
    final users = alex.api.getUsersApi();
    OwnerAdd owner({bool resolveIt = false}) => OwnerAdd((x) {
      x
        ..userId = w.sam.id
        ..reason = 'Deputy';
      if (resolveIt) x.resolveI1.replace(resolve(b));
    });
    final blocked = await failure(users.addOwner(ownerAdd: owner()));
    expect(blocked?.response?.statusCode, 409);
    expect(w.sam.isOwner, isFalse);
    final after = h.server.audit.rows.length;
    await users.addOwner(ownerAdd: owner(resolveIt: true));
    expect(w.sam.isOwner, isTrue);
    expect(b.billingMode, 'organization');
    rows = h.server.audit.rows.sublist(after);
    expect(rows.map((r) => r['action']), [
      'agent.billing_mode.set',
      'owner.add',
    ]);
    expect(rows.first['correlation_id'], rows.last['correlation_id']);
    expect(h.server.violations, isEmpty);
  });

  test(
    'a resolve_i1 agent already on organization gets an entry with no_change',
    () async {
      final a = world.addAgent(handle: 'team', displayName: 'Team');
      final before = h.server.audit.rows.length;
      await alex.api.getUsersApi().addOwner(
        ownerAdd: OwnerAdd(
          (x) => x
            ..userId = w.sam.id
            ..reason = 'Deputy'
            ..resolveI1.replace(
              ResolveI1(
                (r) => r
                  ..switchToOrganization.add(a.id)
                  ..reason = 'Nothing to switch',
              ),
            ),
        ),
      );
      final row = h.server.audit.rows[before];
      expect(row['action'], 'agent.billing_mode.set');
      expect((row['details']! as Map)['no_change'], isTrue);
      expect(a.version, 1);
    },
  );

  test('a token shows its secret once; revoking needs no step-up', () async {
    final tokens = alex.api.getTokensApi();
    final body = ApiTokenCreate(
      (b) => b
        ..name = 'Adapter production'
        ..expiresAt = DateTime.utc(2027, 10, 10),
    );
    const key = '0193f2a7-0000-7000-8000-000000000a01';

    final created = (await tokens.createToken(
      principalId: w.robin.id,
      apiTokenCreate: body,
      idempotencyKey: key,
    )).data!;
    expect(ops(), [
      'createToken 401',
      'createStepUpChallenge 200',
      'completeStepUpWithDeviceKey 200',
      'createToken 201',
    ]);
    final secret = created.secret;
    final shape = RegExp(r'^bothy_agt_([0-9A-Za-z]{43})_([0-9A-Za-z]{6})$');
    final m = shape.firstMatch(secret)!;
    expect(m[2], _base62(_crc32(m[1]!), 6), reason: 'ADR 0061 checksum');
    expect(created.token.tokenPrefix, 'bothy_agt_${m[1]!.substring(0, 6)}');
    expect(created.token.principalId, w.robin.id);
    expect(created.token.scopes, isEmpty);
    expect(jsonEncode(h.server.audit.rows), isNot(contains(m[1]!)));
    expect(rowsOf('token.create').single['resource_id'], created.token.id);

    final replay = await failure(
      tokens.createToken(
        principalId: w.robin.id,
        apiTokenCreate: body,
        idempotencyKey: key,
      ),
    );
    expect(replay?.response?.statusCode, 409);
    expect(bodyOf(replay)['reason'], 'one_time_secret');

    final second = (await tokens.createToken(
      principalId: w.robin.id,
      apiTokenCreate: ApiTokenCreate(
        (b) => b
          ..name = 'Adapter staging'
          ..dpopJkt = '0ZcOCORZNYy-DWpqq30jZyJGHTN0d2HglBV3uiguA4I'
          ..expiresAt = DateTime.utc(2027, 10, 10),
      ),
    )).data!;
    var listed = (await tokens.listTokens(principalId: w.robin.id)).data!.items;
    expect(listed.map((t) => t.name), [
      'Adapter staging',
      'Adapter production',
    ]);
    expect(listed.last.tokenPrefix, created.token.tokenPrefix);
    expect(listed.first.dpopJkt, isNotNull);

    h.server.requests.clear();
    await tokens.revokeToken(tokenId: second.token.id);
    expect(ops(), ['revokeToken 204']);
    listed = (await tokens.listTokens(principalId: w.robin.id)).data!.items;
    expect(listed.first.revokedAt, isNotNull);
    expect(listed.last.revokedAt, isNull);
    expect(rowsOf('token.revoke').single['resource_id'], second.token.id);
    final unknown = await failure(tokens.revokeToken(tokenId: world.nextId()));
    expect(unknown?.response?.statusCode, 404);
    final person = await failure(
      tokens.listTokens(principalId: world.nextId()),
    );
    expect(person?.response?.statusCode, 404);
    expect(h.server.violations, isEmpty);
  });
}

/// CRC-32 (IEEE, reflected, polynomial 0xEDB88320) bit by bit, independent of the fake's table.
int _crc32(String text) {
  var crc = 0xFFFFFFFF;
  for (final byte in utf8.encode(text)) {
    crc ^= byte;
    for (var i = 0; i < 8; i++) {
      crc = (crc & 1) == 1 ? (crc >> 1) ^ 0xEDB88320 : crc >> 1;
    }
  }
  return crc ^ 0xFFFFFFFF;
}

String _base62(int value, int width) {
  const digits =
      '0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz';
  final out = StringBuffer();
  var v = value;
  while (v > 0) {
    out.write(digits[v % 62]);
    v ~/= 62;
  }
  return out.toString().split('').reversed.join().padLeft(width, '0');
}
