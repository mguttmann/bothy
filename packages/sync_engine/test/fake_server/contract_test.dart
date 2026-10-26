// SPDX-License-Identifier: Apache-2.0

// The fake never drifts from the contract (brief M4-17 Design note 5): every handler's
// response and every frame validate against docs/api/; an invalid body or frame sent to the
// fake fails with the JSON Pointer.

import 'dart:convert';
import 'dart:typed_data';

import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart';
import 'package:built_value/json_object.dart';
import 'package:crypto/crypto.dart';
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

  test(
    'getInstance reports the app lock policy of the world (M5-08)',
    () async {
      final alex = h.device();
      await signedIn(h, alex);
      final admin = alex.api.getAdminApi();
      final defaults = (await admin.getInstance()).data!.appLock;
      expect(defaults.required_, isFalse);
      expect(defaults.afterMs, 300000);
      h.server.world
        ..appLockRequired = true
        ..appLockAfterMs = 1000;
      final required = (await admin.getInstance()).data!.appLock;
      expect(required.required_, isTrue);
      expect(required.afterMs, 1000);
      expect(h.server.violations, isEmpty);
    },
  );

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
    // The thread view (M4-20): revisions, read position back, pins, state, title, move,
    // templates and a thread from a message.
    final world = h.server.world;
    final revised = w.post(w.alex, w.releaseNotes, 'first draft');
    await messages.updateMessage(
      messageId: revised['id']! as String,
      ifMatch: '"1"',
      messageUpdate: MessageUpdate((b) => b..contentMd = 'second draft'),
    );
    await messages.listMessageRevisions(messageId: revised['id']! as String);
    final personal = api.getPreferencesApi();
    final read = (await personal.putReadState(
      threadId: w.releaseNotes,
      readStatePut: ReadStatePut(
        (b) => b..lastReadSeq = revised['thread_seq']! as int,
      ),
    )).data!;
    await personal.markThreadUnread(
      threadId: w.releaseNotes,
      ifMatch: '"${read.version}"',
      readStatePut: ReadStatePut((b) => b..lastReadSeq = 0),
    );
    world
      ..grantCapability(w.alex.id, w.general, 'manage')
      ..grantCapability(w.alex.id, w.support, 'manage');
    await messages.pinMessage(messageId: revised['id']! as String);
    await messages.listPins(threadId: w.releaseNotes);
    await messages.unpinMessage(messageId: revised['id']! as String);
    final threadsApi = api.getThreadsApi();
    await threadsApi.closeThread(threadId: w.releaseNotes);
    await threadsApi.reopenThread(threadId: w.releaseNotes);
    await threadsApi.setThreadTitle(
      threadId: w.releaseNotes,
      ifMatch: '"${world.threads[w.releaseNotes]!['version']}"',
      threadTitlePut: ThreadTitlePut((b) => b..title = 'Release plan'),
    );
    final movable = world.addThread(
      w.general,
      'Wrong place',
      createdBy: w.alex.id,
    );
    await threadsApi.moveThread(
      threadId: movable,
      threadMove: ThreadMove(
        (b) => b
          ..targetChannelId = w.support
          ..reason = 'Belongs to support',
      ),
    );
    world.addTemplate(
      scope: 'channel',
      channelId: w.general,
      name: 'Escalation',
      contentMd: 'Please escalate.',
    );
    await api.getChannelsApi().listTemplates(channelId: w.general);
    await api.getMeApi().createPersonalTemplate(
      templateCreate: TemplateCreate(
        (b) => b
          ..name = 'Greeting'
          ..contentMd = 'Hello Sam',
      ),
    );
    await api.getMeApi().listMyTemplates();
    final main = world.addMainThread(w.general, createdBy: w.alex.id);
    final parent = w.post(w.alex, main, 'A question');
    await threadsApi.createThread(
      channelId: w.general,
      threadCreate: ThreadCreate(
        (b) => b..parentMessageId = parent['id']! as String,
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

    // Grants (M4-27): preview, create, list, read, explain, view as, revoke.
    final access = api.getAccessApi();
    await api.getAgentsApi().listAgents();
    final request = GrantCreate(
      (b) => b
        ..subjectKind = GrantSubjectKind.user
        ..subjectId = w.sam.id
        ..targetKind = TargetKind.channel
        ..targetId = w.general
        ..level = Level.trigger
        ..reason = 'Release',
    );
    await access.previewGrant(grantCreate: request);
    final grant = (await access.createGrant(grantCreate: request)).data!;
    await access.listGrants(
      targetKind: TargetKind.channel,
      targetId: w.general,
    );
    await access.getGrant(grantId: grant.id);
    await access.explainChannelAccess(channelId: w.general);
    await access.explainSpaceAccess(spaceId: w.space);
    await access.viewAsUser(userId: w.sam.id);
    await access.revokeGrant(grantId: grant.id, reasonBody: why);

    // Permission hints and expiring access (M6-07): a personal agent of Alex and a grant that
    // expires within `authz.expiry_reminder_lead`, so the contract check sees real items.
    world.addAgent(
      handle: 'texter',
      displayName: 'Texter',
      billingMode: 'personal',
      credentialHolderId: w.alex.id,
    );
    final expiring = (await access.createGrant(
      grantCreate: GrantCreate(
        (b) => b
          ..subjectKind = GrantSubjectKind.user
          ..subjectId = w.sam.id
          ..targetKind = TargetKind.channel
          ..targetId = w.support
          ..level = Level.read
          ..expiresAt = world.now().add(const Duration(hours: 1))
          ..reason = 'Cover',
      ),
    )).data!;
    final hints = (await access.listPermissionHints()).data!;
    expect(
      hints.items.map((x) => x.kind),
      contains(PermissionHintKind.agentPersonal),
    );
    await access.listPermissionHints(
      spaceId: w.space,
      kind: PermissionHintKind.grantToDisabled,
      limit: 1,
    );
    final soon = (await access.listExpiringAccess()).data!;
    expect([for (final x in soon.items) x.grant?.id], contains(expiring.id));
    await access.listExpiringAccess(spaceId: w.space, limit: 1);

    // Agents (M4-28): the card, the backend policy per agent and for all, the holder, and the
    // agent tokens. Sam holds the personal agent first, so moving it to Alex, the only owner
    // and so the only triggerer, keeps I1 (10 5.4).
    final agentsApi = api.getAgentsApi();
    final helper = world.addAgent(
      handle: 'helper',
      displayName: 'Helper',
      billingMode: 'personal',
      credentialHolderId: w.sam.id,
    );
    await agentsApi.getAgent(agentId: helper.id);
    await agentsApi.setCredentialHolder(
      agentId: helper.id,
      ifMatch: '"${helper.version}"',
      credentialHolderSet: CredentialHolderSet(
        (b) => b
          ..credentialHolderId = w.alex.id
          ..reason = 'Alex pays',
      ),
    );
    await agentsApi.setAgentBillingMode(
      agentId: helper.id,
      ifMatch: '"${helper.version}"',
      billingModeSet: BillingModeSet(
        (b) => b
          ..billingMode = BillingMode.organization
          ..reason = 'Team uses it',
      ),
    );
    await agentsApi.setAllAgentsBillingMode(
      billingModeSetAll: BillingModeSetAll(
        (b) => b
          ..billingMode = BillingMode.organization
          ..reason = 'Company key',
      ),
    );
    final tokensApi = api.getTokensApi();
    final token = (await tokensApi.createToken(
      principalId: helper.id,
      apiTokenCreate: ApiTokenCreate(
        (b) => b
          ..name = 'Adapter production'
          ..dpopJkt = 'NzbLsXh8uDCcd-6MNwXF4W_7noWXFZAfHkxZsRGC9Xs'
          ..expiresAt = DateTime.utc(2027, 10, 10),
      ),
    )).data!;
    await tokensApi.listTokens(principalId: helper.id);
    await tokensApi.revokeToken(tokenId: token.token.id);

    // Audit log (M4-29): list, NDJSON export and chain check.
    h.server.audit.add(world, action: 'grant.create', actor: w.alex.id);
    final audit = api.getAuditApi();
    await audit.listAuditEvents(decision: AuditDecision.allow);
    await api.dio.get<List<int>>(
      '/audit/export',
      queryParameters: {
        'from': '2000-01-01T00:00:00.000Z',
        'to': '2100-01-01T00:00:00.000Z',
      },
      options: Options(
        responseType: ResponseType.bytes,
        extra: {
          'secure': [
            {'type': 'http', 'scheme': 'DPoP', 'name': 'dpopAuth'},
          ],
        },
      ),
    );
    await audit.verifyAuditChain(auditVerifyRequest: AuditVerifyRequest());
    // Notification levels, quiet hours and the channel export (M4-33).
    final notify = api.getPreferencesApi();
    await notify.putNotificationPreference(
      targetKind: 'channel',
      targetId: h.seed.general,
      notificationPreferencePut: NotificationPreferencePut(
        (b) => b..level = NotificationLevel.mute,
      ),
    );
    await notify.listMyNotificationPreferences();
    await notify.deleteNotificationPreference(
      targetKind: 'channel',
      targetId: h.seed.general,
    );
    QuietHoursRuleInput quiet(String end) => QuietHoursRuleInput(
      (b) => b
        ..weekdays = 31
        ..startLocal = '19:00'
        ..endLocal = end
        ..enabled = true,
    );
    final rule = (await notify.createQuietHoursRule(
      quietHoursRuleInput: quiet('07:00'),
    )).data!;
    await notify.listMyQuietHours();
    await notify.putQuietHoursRule(
      ruleId: rule.id,
      quietHoursRuleInput: quiet('06:00'),
    );
    await notify.deleteQuietHoursRule(ruleId: rule.id);
    h.server.world.grantCapability(h.seed.alex.id, h.seed.general, 'export');
    await api.dio.get<List<int>>(
      '/channels/${h.seed.general}/export',
      queryParameters: {
        'from': '2000-01-01T00:00:00.000Z',
        'to': '2100-01-01T00:00:00.000Z',
      },
      options: Options(
        responseType: ResponseType.bytes,
        extra: {
          'secure': [
            {'type': 'http', 'scheme': 'DPoP', 'name': 'dpopAuth'},
          ],
        },
      ),
    );
    // Hosts (M4-37): create, pair, a wrong and a right fingerprint, session users, the
    // declaration, the state, a grant `terminal`, the audit and the sessions.
    final hosts = api.getHostsApi();
    final code = (await hosts.createHost(
      hostCreate: HostCreate(
        (b) => b
          ..name = 'Build host'
          ..reason = 'Builds',
      ),
    )).data!;
    await hosts.listHosts();
    h.server.world.pairHost(code.hostId, ['term-alex', 'term-sam']);
    var host = (await hosts.getHost(hostId: code.hostId)).data!;
    HostConfirm confirm(String prefix) => HostConfirm(
      (b) => b
        ..fingerprintPrefix = prefix
        ..llmCredentials = LlmCredentials.organization
        ..declaredServerHost = false
        ..sessionUsers.add(
          HostSessionUserAssign(
            (u) => u
              ..userId = w.sam.id
              ..osUser = 'term-sam',
          ),
        )
        ..reason = 'Builds',
    );
    final mismatch = await hosts
        .confirmHost(
          hostId: host.id,
          ifMatch: '"${host.version}"',
          hostConfirm: confirm('ZZZZ-ZZZZ'),
        )
        .then<DioException?>(
          (_) => null,
          onError: (Object e) => e as DioException,
        );
    expect(mismatch?.response?.statusCode, 409);
    expect((mismatch?.response?.data as Map)['attempts_remaining'], 2);
    host = (await hosts.confirmHost(
      hostId: host.id,
      ifMatch: '"${host.version}"',
      hostConfirm: confirm('7kq2 9xta'),
    )).data!;
    expect(host.state, HostState.active);
    host = (await hosts.updateHost(
      hostId: host.id,
      ifMatch: '"${host.version}"',
      hostUpdate: HostUpdate(
        (b) => b
          ..assignedUserId = w.sam.id
          ..revokeSessionUsers.add(w.sam.id)
          ..reason = 'Rotate',
      ),
    )).data!;
    host = (await hosts.setHostLlmCredentials(
      hostId: host.id,
      ifMatch: '"${host.version}"',
      hostLlmCredentialsSet: HostLlmCredentialsSet(
        (b) => b
          ..llmCredentials = LlmCredentials.none
          ..reason = 'No login',
      ),
    )).data!;
    host = (await hosts.setHostState(
      hostId: host.id,
      ifMatch: '"${host.version}"',
      hostStateSet: HostStateSet(
        (b) => b
          ..state = HostStateSetStateEnum.disabled
          ..reason = 'Maintenance',
      ),
    )).data!;
    final onHost = GrantCreate(
      (b) => b
        ..subjectKind = GrantSubjectKind.user
        ..subjectId = w.sam.id
        ..targetKind = TargetKind.host
        ..targetId = host.id
        ..capabilities.add(Capability.terminal)
        ..reason = 'Builds',
    );
    final preview = (await access.previewGrant(grantCreate: onHost)).data!;
    expect(preview.text, contains('no session user (SR12)'));
    await access.createGrant(grantCreate: onHost);
    final events = (await api.getAuditApi().listAuditEvents(
      action: 'host.manage',
      resourceId: host.id,
    )).data!;
    expect(
      [for (final e in events.items) e.details['change']?.value],
      ['revoked', 'assigned'],
    );
    await api.getTerminalApi().listTerminalSessions(hostId: host.id);
    await hosts.issueHostPairingCode(hostId: host.id, reasonBody: why);

    // The terminal sessions of M4-36 (24 6): open, read, attach again and close.
    final terminalHost = h.server.world.addTerminalHost(
      name: 'Linux VM',
      noisePublicKey: 'Zm9vYmFyZm9vYmFyZm9vYmFyZm9vYmFyZm9vYmFyMTI',
      sessionUsers: {w.alex.id: 'term-alex'},
      grantees: [w.alex.id],
    );
    final terminal = api.getTerminalApi();
    final opened = (await terminal.openTerminalSession(
      hostId: terminalHost.id,
      terminalSessionOpen: TerminalSessionOpen(
        (b) => b
          ..profile = 'shell'
          ..cols = 80
          ..rows = 24
          ..clientEphemeralKeyHash = 'A' * 43,
      ),
    )).data!;
    await terminal.getTerminalSession(sessionId: opened.session.id);
    await terminal.attachTerminalSession(
      sessionId: opened.session.id,
      terminalSessionAttach: TerminalSessionAttach(
        (b) => b
          ..cols = 100
          ..rows = 30
          ..clientEphemeralKeyHash = 'E' * 43,
      ),
    );
    await terminal.closeTerminalSession(
      sessionId: opened.session.id,
      reasonBody: ReasonBody((b) => b.reason = 'Done'),
    );

    // The file operations of M4-22 (12 3, 8, 9): a PNG in one part, a second session that is
    // aborted, a third file that is deleted before it is attached.
    final fileApi = api.getFilesApi();
    final png = base64.decode(
      'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==',
    );
    Future<UploadSession> startUpload(List<int> bytes) async =>
        (await fileApi.createUpload(
          channelId: w.general,
          idempotencyKey: h.server.world.nextId(),
          uploadCreate: UploadCreate(
            (b) => b
              ..name = 'shot.png'
              ..sizeBytes = bytes.length
              ..sha256 = sha256.convert(bytes).toString()
              ..declaredMimeType = 'image/png',
          ),
        )).data!;
    final upload = await startUpload(png);
    final part = (await fileApi.uploadPart(
      fileId: upload.fileId,
      partNumber: 1,
      contentDigest: 'sha-256=:${base64.encode(sha256.convert(png).bytes)}:',
      body: MultipartFile.fromBytes(png),
    )).data!;
    await fileApi.getUpload(fileId: upload.fileId);
    await fileApi.completeUpload(
      fileId: upload.fileId,
      idempotencyKey: h.server.world.nextId(),
      uploadComplete: UploadComplete((b) => b.parts.replace([part])),
    );
    await fileApi.getFile(fileId: upload.fileId);
    await fileApi.downloadFile(fileId: upload.fileId);
    await fileApi.downloadThumbnail(fileId: upload.fileId, variant: 'small');
    await fileApi.createFileTicket(
      fileId: upload.fileId,
      fileTicketCreate: FileTicketCreate(
        (b) => b..variant = FileTicketCreateVariantEnum.original,
      ),
    );
    h.server.world.fileSettings.transcriptionMode = 'service';
    await fileApi.requestTranscription(
      fileId: upload.fileId,
      idempotencyKey: h.server.world.nextId(),
    );
    await fileApi.getTranscript(fileId: upload.fileId);

    // The cross-cutting views of M4-23 (14 4, 4.9, 01 3.5 and D10).
    final search = api.getSearchApi();
    await search.search(q: 'release', type: 'threads');
    await search.searchThreadTitles(q: 'rel');
    await api.getMeApi().listMyMentions();
    await api.getRunsApi().listActiveRuns();
    await fileApi.abortUpload(fileId: (await startUpload(png)).fileId);
    await fileApi.deleteFile(fileId: (await startUpload(png)).fileId);

    // The agent interface of M4-21 (09 9, 10, 11).
    final agentWorld = h.server.world;
    final ui = FakeAgentInterface.of(agentWorld)
      ..currentSession(w.releaseNotes, w.robin.id);
    agentWorld.setChannelOperator(w.general, w.robin.id);
    final trigger = w.post(w.alex, w.releaseNotes, '@robin check');
    final cardRun = agentWorld.createRun(
      agentId: w.robin.id,
      triggeringMessageId: trigger['id']! as String,
      triggeredBy: w.alex.id,
    );
    final card = ui.postCard(w.robin.id, w.releaseNotes, 'Approve?', [
      {
        'custom_id': 'ok',
        'type': 'button',
        'definition': {'label': 'OK', 'value': 'ok'},
      },
    ], runId: cardRun['id']! as String);
    final component =
        ((agentWorld.messages[card]!['components']! as List<Object?>).single
                as Map)['id']!
            as String;
    await api.getInteractionsApi().createInteraction(
      componentId: component,
      interactionCreate: InteractionCreate(
        (b) => b..clientInteractionId = agentWorld.nextId(),
      ),
    );
    final command = ui.registerCommand(
      w.robin.id,
      'model',
      options: [
        {
          'name': 'name',
          'description': 'Model',
          'type': 'string',
          'required': true,
          'autocomplete': true,
        },
      ],
      autocomplete: (_, _, _) => [
        {'name': 'model-large', 'value': 'model-large'},
      ],
    );
    final commandsApi = api.getCommandsApi();
    await commandsApi.listChannelCommands(channelId: w.general);
    await commandsApi.autocompleteCommand(
      channelId: w.general,
      commandId: command,
      autocompleteQuery: AutocompleteQuery(
        (b) => b
          ..option = 'name'
          ..value = 'm',
      ),
    );
    await commandsApi.invokeCommand(
      channelId: w.general,
      commandId: command,
      commandInvoke: CommandInvoke(),
      idempotencyKey: agentWorld.nextId(),
    );
    await api.getRunsApi().listThreadRuns(threadId: w.releaseNotes);
    await api.getStatsApi().getThreadCosts(threadId: w.releaseNotes);
    final sessionsApi = api.getThreadsApi();
    await sessionsApi.listAgentSessions(threadId: w.releaseNotes);
    await sessionsApi.resetAgentSession(
      threadId: w.releaseNotes,
      idempotencyKey: agentWorld.nextId(),
    );
    await sessionsApi.bindAgentSession(
      threadId: w.releaseNotes,
      agentSessionBind: AgentSessionBind(
        (b) => b..externalSessionRef = 'sess-earlier',
      ),
      idempotencyKey: agentWorld.nextId(),
    );

    // The statistics of M6-06 answer what the world holds, within the contract.
    h.server.world.stats['getUsageStats'] = {
      'buckets': [
        {
          'start': '2026-10-12T00:00:00.000Z',
          'group_key': '0192f4a0-7b1c-7000-8a00-000000000001',
          'metrics': {'messages': 3.0},
          'below_min_group': ['active_people'],
        },
      ],
    };
    h.server.world.stats['getCostStats'] = {
      'currency': 'EUR',
      'buckets': [
        {
          'start': '2026-10-12T00:00:00.000Z',
          'billing_mode': 'personal',
          'amount_micros': 600000,
          'is_estimate': true,
        },
      ],
    };
    final stats = api.getStatsApi();
    final from = DateTime.utc(2026, 10, 10);
    final to = DateTime.utc(2026, 10, 17);
    await stats.getUsageStats(from: from, to: to);
    await stats.getRunStats(from: from, to: to);
    await stats.getToolCallStats(from: from, to: to);
    await stats.getErrorStats(from: from, to: to);
    await stats.getCostStats(from: from, to: to, groupBy: 'agent');
    // The views of M6-16 answer an empty body without a stored one, within the contract.
    await stats.getStatsLive();
    await stats.getStatsTimeline(
      from: to.subtract(const Duration(hours: 1)),
      to: to,
    );
    await stats.getStatsSlo();
    await stats.getStatsBackends(from: from, to: to);

    // The status page of M7-21: the seven operations within the contract.
    final statusApi = api.getStatusApi();
    final statusComponent = (await statusApi.createStatusComponent(
      statusComponentInput: StatusComponentInput(
        (b) => b
          ..name = 'Messages'
          ..description = 'Messages and sync',
      ),
      idempotencyKey: h.server.world.nextId(),
    )).data!;
    await statusApi.updateStatusComponent(
      componentId: statusComponent.id,
      statusComponentInput: StatusComponentInput(
        (b) => b
          ..name = 'Messages'
          ..state = StatusComponentState.degraded,
      ),
      idempotencyKey: h.server.world.nextId(),
    );
    final incident = (await statusApi.createStatusIncident(
      statusIncidentCreate: StatusIncidentCreate(
        (b) => b
          ..title = 'Messages delayed'
          ..kind = StatusIncidentKind.incident
          ..impact = StatusIncidentImpact.minor
          ..startedAt = DateTime.utc(2026, 10, 20, 9)
          ..componentIds.add(statusComponent.id)
          ..bodyMd = 'We are looking into it.',
      ),
      idempotencyKey: h.server.world.nextId(),
    )).data!;
    await statusApi.addStatusIncidentUpdate(
      incidentId: incident.id,
      statusIncidentUpdateCreate: StatusIncidentUpdateCreate(
        (b) => b
          ..state = StatusIncidentState.resolved
          ..bodyMd = 'Delays are resolved.',
      ),
      idempotencyKey: h.server.world.nextId(),
    );
    expect(
      (await statusApi.getStatus()).data!.overallState,
      StatusComponentState.degraded,
    );
    h.server.world.statusPublicEnabled = true;
    await statusApi.getPublicStatus();
    await statusApi.deleteStatusComponent(componentId: statusComponent.id);

    // M5-06: the child session of the Share Extension (08 7.4).
    await api.getDevicesApi().createExtensionSession(
      extensionSessionCreate: ExtensionSessionCreate(
        (b) => b
          ..clientComponent =
              ExtensionSessionCreateClientComponentEnum.shareExt,
      ),
    );

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
    // One walk over every handler: its run time grows with each operation and with the load of
    // the machine, so no fixed wall-clock bound applies (rules for deterministic tests).
  }, timeout: Timeout.none);

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
