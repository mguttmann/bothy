// SPDX-License-Identifier: Apache-2.0

// The fake never drifts from the contract (brief M4-17 Design note 5): every handler's
// response and every frame validate against docs/api/; an invalid body or frame sent to the
// fake fails with the JSON Pointer.

import 'dart:typed_data';

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:built_value/json_object.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart' show EventCatalog;
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late FakeWorldHarness h;

  setUp(() async => h = await FakeWorldHarness.start());
  tearDown(() => h.stop());

  test('discovery lists the local driver like the server does', () async {
    final discovery =
        (await h.device().api.getDiscoveryApi().getDiscovery()).data!;
    expect(
      discovery.authProviders.map((p) => p.kind),
      contains(IdentityProviderKind.local),
    );
    expect(discovery.localPasswordEnabled, isTrue);
  });

  test('every handler answers within the contract', () async {
    final alex = h.device();
    final w = h.seed;
    final api = alex.api;
    expect((await api.getDiscoveryApi().getDiscovery()).statusCode, 200);
    await signedIn(h, alex);

    await api.getMeApi().getMe();
    await api.getAdminApi().getInstance();
    await api.getSyncApi().getSnapshot();
    await api.getSyncApi().listEvents(
      after: 0,
      epoch: h.server.world.syncEpoch,
    );
    await api.getSpacesApi().listSpaces();
    await api.getSpacesApi().getSpace(spaceId: w.space);
    await api.getChannelsApi().listSpaceChannels(spaceId: w.space);
    await api.getChannelsApi().getChannel(channelId: w.general);
    await api.getThreadsApi().listChannelThreads(channelId: w.general);
    await api.getThreadsApi().getThread(threadId: w.releaseNotes);

    final messages = api.getMessagesApi();
    final created = (await messages.createMessage(
      threadId: w.releaseNotes,
      messageCreate: MessageCreate(
        (b) => b
          ..clientMsgId = h.server.world.nextId()
          ..contentMd = 'Hello Sam',
      ),
    )).data!;
    await messages.listThreadMessages(threadId: w.releaseNotes);
    await messages.listThreadMessages(threadId: w.releaseNotes, afterSeq: 0);
    await messages.getMessage(messageId: created.id);
    final edited = (await messages.updateMessage(
      messageId: created.id,
      ifMatch: '"${created.version}"',
      messageUpdate: MessageUpdate((b) => b..contentMd = 'Hello Sam and Robin'),
    )).data!;
    await messages.addReaction(messageId: created.id, emoji: 'thumbsup');
    await messages.removeReaction(messageId: created.id, emoji: 'thumbsup');
    final stale = await messages
        .updateMessage(
          messageId: created.id,
          ifMatch: '"${created.version}"',
          messageUpdate: MessageUpdate((b) => b..contentMd = 'old'),
        )
        .then<DioException?>(
          (_) => null,
          onError: (Object e) => e as DioException,
        );
    expect(stale?.response?.statusCode, 412);
    expect((stale?.response?.data as Map?)?['current'], isNotNull);
    await messages.deleteMessage(messageId: created.id);
    await messages.deleteMessage(messageId: created.id);
    final gone = await messages
        .updateMessage(
          messageId: created.id,
          ifMatch: '"${edited.version}"',
          messageUpdate: MessageUpdate((b) => b..contentMd = 'too late'),
        )
        .then<DioException?>(
          (_) => null,
          onError: (Object e) => e as DioException,
        );
    expect((gone?.response?.data as Map?)?['code'], 'message_deleted');

    final prefs = api.getPreferencesApi();
    await prefs.putReadState(
      threadId: w.releaseNotes,
      readStatePut: ReadStatePut((b) => b..lastReadSeq = 1),
    );
    await prefs.listMyReadStates(channelId: w.general);
    await prefs.putThreadDraft(
      threadId: w.releaseNotes,
      draftPut: DraftPut((b) => b..contentMd = 'A draft'),
    );
    await prefs.listMyDrafts();
    await prefs.deleteThreadDraft(threadId: w.releaseNotes);
    await prefs.putMySetting(
      key: 'locale.default',
      userSettingPut: UserSettingPut((b) => b..value = JsonObject('en')),
    );
    await prefs.listMySettings();
    await prefs.deleteMySetting(key: 'locale.default');

    // The sidebar, a forum post and the logo (M4-19).
    await prefs.putMySidebarSpace(
      spaceId: w.space,
      sidebarStatePut: SidebarStatePut(
        (b) => b
          ..collapsed = true
          ..sortKey = 'a0V',
      ),
    );
    await prefs.getMySidebar();
    final forum = h.server.world.addForum(w.space, 'questions', {
      w.alex.id: 'write',
    });
    h.server.world.addForumTag(forum, 'bug', color: 'rowan');
    await api.getChannelsApi().listForumTags(channelId: forum);
    await api.getThreadsApi().createThread(
      channelId: forum,
      threadCreate: ThreadCreate(
        (b) => b
          ..title = 'Printer jams'
          ..firstMessage.clientMsgId = h.server.world.nextId()
          ..firstMessage.contentMd = 'Since Monday',
      ),
    );
    h.server.world.logo = Uint8List.fromList([137, 80, 78, 71]);
    await api.getDiscoveryApi().getBrandingLogo();

    // A run of Robin, then its document and its cancellation.
    h.server.addAgent(
      w.robin.id,
      const AgentScript([AgentStep.queue(), AgentStep.start()]),
    );
    await messages.createMessage(
      threadId: w.releaseNotes,
      messageCreate: MessageCreate(
        (b) => b
          ..clientMsgId = h.server.world.nextId()
          ..contentMd = 'Robin, please look'
          ..mentionIds.add(w.robin.id),
      ),
    );
    final run = h.server.world.activeRunOf(w.releaseNotes)!;
    await api.getRunsApi().getRun(runId: run['id']! as String);
    await api.getRunsApi().cancelRun(
      runId: run['id']! as String,
      runCancel: RunCancel(),
    );

    // A second device of Alex through a pairing.
    final tablet = h.device(keySeed: 2);
    final ticket = await tablet.auth.login.beginPairing();
    await api.getDevicesApi().approvePairing(
      pairingApprove: PairingApprove((b) => b..pairingCode = ticket.code),
    );
    expect(
      await tablet.auth.login.pollPairing(ticket.pairingId),
      isA<LoginSignedIn>(),
    );

    // A waiting device of Sam, approved from Sam's first device.
    final samPhone = h.device(login: WorldSeed.samLogin, keySeed: 3);
    await signedIn(h, samPhone);
    w.sam.newDevicesPending = true;
    final samLaptop = h.device(login: WorldSeed.samLogin, keySeed: 4);
    final pending = await samLaptop.signIn();
    expect(pending, isA<LoginPending>());
    final waiting = (pending as LoginPending).device;
    final denied = await samLaptop.api
        .getSyncApi()
        .getSnapshot()
        .then<DioException?>(
          (_) => null,
          onError: (Object e) => e as DioException,
        );
    expect((denied?.response?.data as Map?)?['code'], 'device_pending');
    await samPhone.api.getDevicesApi().approveDevice(
      deviceId: waiting.deviceId.value,
      deviceApprove: DeviceApprove(
        (b) => b..verificationCode = waiting.verificationCode,
      ),
    );
    expect(
      await samLaptop.auth.login.pollPendingDevice(waiting.deviceId),
      isA<LoginSignedIn>(),
    );

    // Own devices, sessions and profile (M4-25): end the tablet's session, revoke the tablet.
    final devicesApi = api.getDevicesApi();
    final tabletSession = (await devicesApi.listMySessions()).data!.items
        .firstWhere((s) => !(s.current ?? false));
    await devicesApi.listMyDevices();
    await devicesApi.revokeSession(sessionId: tabletSession.id);
    await devicesApi.revokeDevice(
      deviceId: tabletSession.deviceId,
      reasonBody: ReasonBody((b) => b..reason = 'Not needed'),
    );
    await api.getMeApi().updateMyProfile(
      profileUpdate: ProfileUpdate((b) => b..locale = 'en'),
    );

    // Administration (M4-26): people, owners, credentials, devices and sessions of another
    // person, groups with members.
    final users = api.getUsersApi();
    final why = ReasonBody((b) => b..reason = 'Check');
    final robin = (await users.createUser(
      userCreate: UserCreate(
        (b) => b
          ..handle = 'robin.k'
          ..displayName = 'Robin K'
          ..localAccount = true,
      ),
    )).data!.user;
    await users.listUsers(q: 'rob');
    await users.getUser(userId: robin.id);
    await users.updateUser(
      userId: robin.id,
      userUpdate: UserUpdate((b) => b..email = 'robin@example.com'),
    );
    await users.createEnrollmentCode(userId: robin.id);
    await users.disableUser(userId: robin.id, reasonBody: why);
    await users.enableUser(userId: robin.id, reasonBody: why);
    final world = h.server.world;
    final passkey = world.addPasskey(w.sam.id, name: 'Sam laptop');
    final identity = world.addIdentity(w.sam.id, subject: 'sam');
    await users.getUserCredentials(userId: w.sam.id);
    await users.revokePasskey(userId: w.sam.id, passkeyId: passkey);
    await users.unlinkIdentity(userId: w.sam.id, identityId: identity);
    await devicesApi.listUserDevices(userId: w.sam.id);
    await devicesApi.listUserSessions(userId: w.sam.id);
    await users.listOwners();
    await users.addOwner(
      ownerAdd: OwnerAdd(
        (b) => b
          ..userId = w.sam.id
          ..reason = 'Deputy',
      ),
    );
    await users.revokeOwner(userId: w.sam.id, reasonBody: why);
    final groups = api.getGroupsApi();
    final group = (await groups.createGroup(
      groupCreate: GroupCreate((b) => b..name = 'Support'),
    )).data!;
    await groups.listGroups();
    await groups.getGroup(groupId: group.id);
    await groups.updateGroup(
      groupId: group.id,
      groupUpdate: GroupUpdate((b) => b..name = 'Help desk'),
    );
    await groups.addGroupMember(
      groupId: group.id,
      groupMemberAdd: GroupMemberAdd(
        (b) => b
          ..userId = w.sam.id
          ..expiresAt = DateTime.utc(2026, 12, 31),
      ),
    );
    await groups.listGroupMembers(groupId: group.id);
    await groups.removeGroupMember(groupId: group.id, userId: w.sam.id);
    await groups.deleteGroup(groupId: group.id);

    await alex.auth.refresh.refreshNow();
    await api.getMeApi().getMe();
    await alex.auth.login.logout();

    // 04 9.3: resource.version is present exactly for the types the catalogue marks.
    for (final e in h.server.world.log) {
      final info = EventCatalog.lookup(e.type)!;
      expect(
        (e.envelope['resource']! as Map).containsKey('version'),
        info.carriesResourceVersion,
        reason: e.type,
      );
    }

    final served = {
      for (final r in h.server.requests)
        if (r.status >= 200 && r.status < 300) r.operationId,
    };
    expect(h.server.rest.handled.difference(served), isEmpty);
    expect(h.server.violations, isEmpty);
  });

  test('every frame of a gateway session validates', () async {
    final alex = h.device();
    final s = await signedIn(h, alex);
    final raw = RawClient(h.server.baseUri, alex.key);
    final w = h.seed;
    w.post(w.sam, w.releaseNotes, 'Before the catch-up');
    h.server.world.catchupPageSize = 1;
    final g = await RawGateway.connect(h.server.gatewayUri, raw);
    await g.identify(
      s.token,
      s.deviceId,
      cursor: 0,
      epoch: h.server.world.syncEpoch,
    );
    await g.waitFor('catchup.complete');
    h.server.addAgent(
      w.robin.id,
      const AgentScript([
        AgentStep.queue(),
        AgentStep.start(),
        AgentStep.patch([
          {'op': 'add', 'path': '/status_line', 'value': 'Working'},
        ]),
      ], pause: Duration(milliseconds: 5)),
    );
    g
      ..send('heartbeat', {'last_seq': 0})
      ..send('focus', {
        'thread_ids': [w.releaseNotes],
      })
      ..send('focus', {
        'thread_ids': [for (var i = 0; i < 21; i++) w.releaseNotes],
      })
      ..send('ack', {'seq': 1});
    await g.waitFor('heartbeat_ack');
    await g.waitFor('error', count: 2);
    w.post(w.sam, w.releaseNotes, 'Live');
    h.server.world.createMessage(w.alex.id, w.releaseNotes, {
      'client_msg_id': h.server.world.nextId(),
      'content_md': 'Robin, look',
      'mention_ids': [w.robin.id],
    });
    h.server.world.setAccess(w.alex.id, w.support, 'read');
    await g.waitFor('dispatch', count: 8);
    final types = {for (final d in g.data('dispatch')) d['type']};
    expect(
      types,
      containsAll([
        'message.created',
        'message.updated',
        'run.state_changed',
        'access.changed',
      ]),
    );
    await Future<void>.delayed(const Duration(milliseconds: 400));
    expect(types.length, greaterThan(3));
    expect(
      g.ops,
      containsAll([
        'ready',
        'cursor',
        'catchup.complete',
        'heartbeat_ack',
        'error',
      ]),
    );
    final other = await RawGateway.connect(h.server.gatewayUri, raw);
    h.server.faults.resyncRequired('retention');
    await other.identify(s.token, s.deviceId, cursor: 0);
    await other.waitFor('resync_required');
    await g.waitClosed();
    expect(g.closeCode, 4017, reason: 'one connection per device');
    await other.close();
    raw.close();
    expect(h.server.violations, isEmpty);
  });

  test('the snapshot and read states carry the unread and mention counts', () {
    final w = h.seed;
    final world = h.server.world;
    w.post(w.sam, w.releaseNotes, 'One');
    world.createMessage(w.sam.id, w.releaseNotes, {
      'client_msg_id': world.nextId(),
      'content_md': 'Alex, look',
      'mention_ids': [w.alex.id],
    });
    w.post(w.alex, w.releaseNotes, 'Mine');
    final unread =
        world.snapshot(w.alex.id)['unread_threads']! as List<Object?>;
    expect(unread, [
      {
        'thread_id': w.releaseNotes,
        'channel_id': w.general,
        'unread_count': 2,
        'mention_count': 1,
        'last_message_seq': 3,
        'last_read_seq': 0,
      },
    ]);
    expect(
      world.putReadState(w.alex.id, w.releaseNotes, 1)['mention_count'],
      1,
    );
    world.putReadState(w.alex.id, w.releaseNotes, 3);
    expect(world.snapshot(w.alex.id)['unread_threads'], isEmpty);
    expect(
      ContractSchemas.load().validate(world.snapshot(w.sam.id), 'Snapshot'),
      isEmpty,
    );
  });

  test('a reaction does not raise the message version (04 9.3)', () async {
    final w = h.seed;
    final m = w.post(w.alex, w.releaseNotes, 'Hello Sam');
    h.server.world.react(w.sam.id, m['id']! as String, 'thumbsup', add: true);
    final edited = h.server.world.updateMessage(
      w.alex.id,
      m['id']! as String,
      1,
      {'content_md': 'Hello Sam and Robin'},
    );
    expect(edited['version'], 2);
  });

  test('an invalid body fails with the pointer', () async {
    final alex = h.device();
    final s = await signedIn(h, alex);
    final raw = RawClient(h.server.baseUri, alex.key);
    final path = '/api/v1/threads/${h.seed.releaseNotes}/messages';
    final r = await raw.send(
      'POST',
      path,
      headers: {
        'Authorization': 'DPoP ${s.token}',
        'DPoP': await raw.proof(
          method: 'POST',
          htu: '${h.server.baseUri.origin}$path',
          accessToken: s.token,
          nonce: h.server.verifier.currentNonce,
        ),
      },
      body: {
        'client_msg_id': h.server.world.nextId(),
        'content_md': 'x',
        'colour': 'red',
      },
    );
    raw.close();
    expect(r.status, 400);
    expect(r.code, 'validation_failed');
    expect((r.body! as Map)['errors'], [
      {'pointer': '/colour', 'code': 'unknown_member'},
    ]);
    expect(h.server.violations.single, contains('/colour'));
    expect(h.server.world.messagesOf(h.seed.releaseNotes), isEmpty);
  });

  test('an invalid frame closes with 4002 and names the pointer', () async {
    final alex = h.device();
    final s = await signedIn(h, alex);
    final raw = RawClient(h.server.baseUri, alex.key);
    final g = await RawGateway.connect(h.server.gatewayUri, raw);
    await g.identify(s.token, s.deviceId);
    await g.waitFor('ready');
    g.send('heartbeat', {'last_seq': 'seven'});
    await g.waitClosed();
    raw.close();
    expect(g.closeCode, 4002);
    expect(h.server.violations.single, contains('/d/last_seq'));
  });

  test('the embedded copy equals docs/api (D-M4-18-17)', () {
    final files = ContractSchemas.load();
    final copy = ContractSchemas.embedded();
    List<String> facts(ContractSchemas c) => [
      for (final o in c.operations)
        '${o.operationId} ${o.method} ${o.path} ${o.requestSchema} '
            '${o.responseSchemas}',
    ];
    expect(facts(copy), facts(files));
    expect(copy.validate({'last_read_seq': -1}, 'ReadStatePut'), hasLength(1));
    expect(identical(ContractSchemas.embedded(), copy), isTrue);
  });

  test('the validator reports each rule with its pointer', () {
    final contract = ContractSchemas.load();
    String codes(Object? value, String schema) => [
      for (final v in contract.validate(value, schema))
        '${v.pointer} ${v.code}',
    ].join(', ');
    expect(codes({'kind': 'thread'}, 'EventResource'), isEmpty);
    expect(
      codes({'id': 'x'}, 'EventResource'),
      '/kind required, /id invalid_format',
    );
    expect(
      codes({'last_read_seq': -1}, 'ReadStatePut'),
      '/last_read_seq too_small',
    );
    expect(
      codes({'last_read_seq': 1, 'x': 1}, 'ReadStatePut'),
      '/x unknown_member',
    );
    expect(
      codes({
        'thread_ids': [1],
      }, 'GatewayFocus'),
      '/thread_ids/0 invalid_type',
    );
    expect(codes('2026-13', 'Timestamp'), ' invalid_format');
    expect(codes('queued', 'RunState'), isEmpty);
    expect(codes('lost', 'RunState'), ' invalid_enum');
    expect(
      codes({'type': 'nothing.happened'}, 'Event'),
      '/type invalid_enum',
      reason: 'oneOf with a discriminator',
    );
    expect(contract.response('logout', 204, {'a': 1}).single.code, 'invalid');
    expect(
      contract.response('getMe', 418, null).single.code,
      'invalid_type',
      reason: 'default',
    );
    expect(contract.request('getMe', {'a': 1}).single.code, 'invalid');
    expect(
      contract
          .serverFrame({'op': 'nothing', 'd': <String, Object?>{}})
          .single
          .pointer,
      '/op',
    );
    expect(() => contract.operation('noSuchOperation'), throwsArgumentError);
  });
}
