// SPDX-License-Identifier: Apache-2.0

import 'package:domain/domain.dart';
import 'package:test/test.dart';

import 'support.dart';

/// Checks value equality and copyWith of one entity: [a] equals an identical copy and its
/// `copyWith()` result, and [changed] (a copyWith with one field changed) differs.
void _value(String name, Object a, Object same, Object copy, Object changed) {
  test(name, () {
    expect(a, same);
    expect(a.hashCode, same.hashCode);
    expect(copy, a);
    expect(changed, isNot(a));
  });
}

void main() {
  final file = FileId.parse(uuid(30));
  final space = SpaceId.parse(uuid(40));
  final device = DeviceId.parse(uuid(50));
  const v1 = ResourceVersion(1);

  Space mkSpace() => Space(
    id: space,
    name: 'Support',
    sortKey: 'a',
    createdAt: t0,
    version: v1,
    icon: 'chat',
    color: 'blue',
    defaultOperatorAgentId: sam,
    archivedAt: t0,
  );
  final s = mkSpace();
  _value('Space', s, mkSpace(), s.copyWith(), s.copyWith(name: 'Ops'));

  UserSpaceState mkUss() => UserSpaceState(
    spaceId: space,
    collapsed: false,
    version: v1,
    sortKey: 'b',
    updatedByDeviceId: device,
  );
  final uss = mkUss();
  _value(
    'UserSpaceState',
    uss,
    mkUss(),
    uss.copyWith(),
    uss.copyWith(collapsed: true),
  );

  ChannelOperator mkOp() => ChannelOperator(
    mode: OperatorMode.agent,
    agentId: sam,
    effectiveAgentId: sam,
  );
  final op = mkOp();
  _value(
    'ChannelOperator',
    op,
    mkOp(),
    op.copyWith(),
    op.copyWith(mode: OperatorMode.none),
  );

  Channel mkChannel() => Channel(
    id: channel1,
    kind: ChannelKind.forum,
    name: 'help',
    topic: '',
    sortKey: 'a',
    operator: mkOp(),
    readOnly: false,
    dmParticipantIds: [alex, sam],
    createdAt: t0,
    version: v1,
    settingsVersion: v1,
    spaceId: space,
    forumDefaultSort: ForumSort.created,
    defaultNotificationLevel: NotificationLevel.mentions,
    mainThreadId: thread1,
    archivedAt: t0,
  );
  final c = mkChannel();
  _value(
    'Channel',
    c,
    mkChannel(),
    c.copyWith(),
    c.copyWith(dmParticipantIds: [alex]),
  );

  ForumTag mkTag() => ForumTag(
    id: ForumTagId.parse(uuid(60)),
    channelId: channel1,
    name: 'bug',
    sortKey: 'a',
    restricted: false,
    color: 'red',
  );
  final tag = mkTag();
  _value(
    'ForumTag',
    tag,
    mkTag(),
    tag.copyWith(),
    tag.copyWith(restricted: true),
  );

  final th = thread(1);
  _value(
    'Thread',
    th,
    thread(1),
    th.copyWith(),
    th.copyWith(state: ThreadState.closed),
  );

  ThreadTag mkTt() =>
      ThreadTag(threadId: thread1, tagId: ForumTagId.parse(uuid(60)));
  final tt = mkTt();
  _value(
    'ThreadTag',
    tt,
    mkTt(),
    tt.copyWith(),
    tt.copyWith(tagId: ForumTagId.parse(uuid(61))),
  );

  final m = message(1, mentions: [alex]);
  _value(
    'Message',
    m,
    message(1, mentions: [alex]),
    m.copyWith(),
    m.copyWith(mentionIds: [sam]),
  );

  Thumbnail mkThumb() => const Thumbnail(
    variant: ThumbnailVariant.small,
    mimeType: 'image/png',
    width: 1,
    height: 2,
  );
  final thumb = mkThumb();
  _value(
    'Thumbnail',
    thumb,
    mkThumb(),
    thumb.copyWith(),
    thumb.copyWith(width: 3),
  );

  FileMeta mkFile() => FileMeta(
    id: file,
    sizeBytes: 3,
    mimeType: 'image/png',
    originalName: 'a.png',
    uploadedBy: alex,
    purpose: FilePurpose.attachment,
    uploadState: UploadState.complete,
    scanStatus: ScanStatus.clean,
    transcriptStatus: TranscriptStatus.none,
    thumbnails: [mkThumb()],
    createdAt: t0,
    sha256: 'ab',
    scopeChannelId: channel1,
    width: 1,
    height: 1,
    durationMs: 5,
    transcriptLanguage: 'en',
    transcriptSource: TranscriptSource.service,
  );
  final f = mkFile();
  _value(
    'FileMeta',
    f,
    mkFile(),
    f.copyWith(),
    f.copyWith(thumbnails: const []),
  );

  MessageAttachment mkAtt() => MessageAttachment(
    messageId: m.id,
    fileId: file,
    position: 0,
    threadId: thread1,
    channelId: channel1,
  );
  final att = mkAtt();
  _value(
    'MessageAttachment',
    att,
    mkAtt(),
    att.copyWith(),
    att.copyWith(position: 1),
  );

  ReactionSummary mkReaction() =>
      ReactionSummary(messageId: m.id, emoji: '+', count: 2, me: true);
  final r = mkReaction();
  _value(
    'ReactionSummary',
    r,
    mkReaction(),
    r.copyWith(),
    r.copyWith(count: 3),
  );

  Mention mkMention() =>
      Mention(messageId: m.id, principalId: alex, threadId: thread1);
  final mention = mkMention();
  _value(
    'Mention',
    mention,
    mkMention(),
    mention.copyWith(),
    mention.copyWith(threadId: ThreadId.parse(uuid(12))),
  );

  MessageComponent mkComp() => MessageComponent(
    id: ComponentId.parse(uuid(70)),
    messageId: m.id,
    customId: 'ok',
    type: ComponentType.button,
    definition: {
      'label': 'OK',
      'options': [1, 2],
    },
    position: 0,
    singleUse: true,
    restrictedTo: ComponentRestriction.any,
    state: ComponentState.active,
    version: v1,
    requiredCapability: Capability.approveActions,
    runId: RunId.parse(uuid(80)),
    expiresAt: t0,
    usedBy: alex,
    usedAt: t0,
  );
  final comp = mkComp();
  _value(
    'MessageComponent',
    comp,
    mkComp(),
    comp.copyWith(),
    comp.copyWith(definition: {'label': 'No'}),
  );

  Money mkMoney() =>
      const Money(amountMicros: 5, currency: 'USD', isEstimate: true);
  final money = mkMoney();
  _value(
    'Money',
    money,
    mkMoney(),
    money.copyWith(),
    money.copyWith(isEstimate: false),
  );

  RunSummary mkRun() => RunSummary(
    id: RunId.parse(uuid(80)),
    threadId: thread1,
    channelId: channel1,
    agentId: sam,
    triggeredBy: alex,
    state: RunState.running,
    version: v1,
    billingModeExpected: BillingMode.organization,
    cost: mkMoney(),
    stepCount: 2,
    queuedAt: t0,
    stateReason: 'x',
    billingModeUsed: BillingMode.organization,
    statusLine: 'busy',
    summary: 's',
    durationMs: 4,
    startedAt: t0,
    finishedAt: t0,
  );
  final run = mkRun();
  _value(
    'RunSummary',
    run,
    mkRun(),
    run.copyWith(),
    run.copyWith(state: RunState.completed),
  );

  Principal mkPrincipal() => Principal(
    id: alex,
    kind: PrincipalKind.user,
    handle: 'alex',
    displayName: 'Alex',
    avatarFileId: file,
  );
  final p = mkPrincipal();
  _value(
    'Principal',
    p,
    mkPrincipal(),
    p.copyWith(),
    p.copyWith(displayName: 'Sam'),
  );

  AgentPresence mkPresence() => const AgentPresence(
    state: AgentRuntimeState.online,
    reportedBillingMode: BillingMode.personal,
  );
  final presence = mkPresence();
  _value(
    'AgentPresence',
    presence,
    mkPresence(),
    presence.copyWith(),
    presence.copyWith(state: AgentRuntimeState.offline),
  );

  AgentCard mkCard() => AgentCard(
    id: sam,
    handle: 'helper',
    displayName: 'Helper',
    description: 'd',
    billingMode: BillingMode.organization,
    toolClass: ToolClass.chat,
    presence: mkPresence(),
    version: v1,
    avatarFileId: file,
  );
  final card = mkCard();
  _value(
    'AgentCard',
    card,
    mkCard(),
    card.copyWith(),
    card.copyWith(toolClass: ToolClass.host),
  );

  CommandChoice mkChoice() => const CommandChoice(name: 'A', value: 'a');
  final choice = mkChoice();
  _value(
    'CommandChoice',
    choice,
    mkChoice(),
    choice.copyWith(),
    choice.copyWith(value: 'b'),
  );

  CommandOption mkOption() => CommandOption(
    name: 'n',
    description: 'd',
    type: CommandOptionType.choice,
    required: true,
    choices: [mkChoice()],
    autocomplete: false,
  );
  final option = mkOption();
  _value(
    'CommandOption',
    option,
    mkOption(),
    option.copyWith(),
    option.copyWith(choices: const []),
  );

  Command mkCommand() => Command(
    id: CommandId.parse(uuid(90)),
    ownerPrincipalId: sam,
    name: 'stop',
    description: 'd',
    scopeKind: TargetKind.channel,
    options: [mkOption()],
    requiredLevel: CommandLevel.trigger,
    control: true,
    version: v1,
    scopeId: channel1.value,
  );
  final cmd = mkCommand();
  _value(
    'Command',
    cmd,
    mkCommand(),
    cmd.copyWith(),
    cmd.copyWith(control: false),
  );

  final rs = readState(4, mentions: 1, unread: 2);
  _value(
    'ReadState',
    rs,
    readState(4, mentions: 1, unread: 2),
    rs.copyWith(),
    rs.copyWith(mentionCount: 0),
  );

  Draft mkDraft() => Draft(
    id: DraftId.parse(uuid(91)),
    channelId: channel1,
    contentMd: 'hi',
    fileIds: [file],
    version: v1,
    threadId: thread1,
    replyToMessageId: m.id,
    updatedByDeviceId: device,
    updatedAt: t0,
  );
  final draft = mkDraft();
  _value(
    'Draft',
    draft,
    mkDraft(),
    draft.copyWith(),
    draft.copyWith(contentMd: 'bye'),
  );

  UserSetting mkSetting() => UserSetting(
    key: 'appearance.theme',
    value: {
      'v': ['dark'],
    },
    version: v1,
    updatedByDeviceId: device,
    updatedAt: t0,
  );
  final setting = mkSetting();
  _value(
    'UserSetting',
    setting,
    mkSetting(),
    setting.copyWith(),
    setting.copyWith(value: 'light'),
  );

  NotificationPreference mkPref() => NotificationPreference(
    targetKind: NotificationTargetKind.thread,
    targetId: thread1.value,
    level: NotificationLevel.mute,
    version: v1,
    mutedUntil: t0,
  );
  final pref = mkPref();
  _value(
    'NotificationPreference',
    pref,
    mkPref(),
    pref.copyWith(),
    pref.copyWith(level: NotificationLevel.all),
  );

  QuietHours mkQuiet() => QuietHours(
    id: QuietHoursId.parse(uuid(92)),
    weekdays: 31,
    startLocal: '22:00',
    endLocal: '07:00',
    enabled: true,
    version: v1,
    timeZone: 'Europe/Berlin',
  );
  final quiet = mkQuiet();
  _value(
    'QuietHours',
    quiet,
    mkQuiet(),
    quiet.copyWith(),
    quiet.copyWith(enabled: false),
  );

  Branding mkBranding() => Branding(
    productName: 'Chat',
    version: v1,
    accentColor: 'blue',
    logoFileId: file,
  );
  final branding = mkBranding();
  _value(
    'Branding',
    branding,
    mkBranding(),
    branding.copyWith(),
    branding.copyWith(productName: 'Other'),
  );

  InstanceLimits mkLimits() => const InstanceLimits(
    messageMaxContentBytes: 1,
    messageMaxAttachments: 2,
    fileMaxUploadBytes: 3,
    threadTitleMaxChars: 4,
    voiceMaxDurationMs: 5,
  );
  final limits = mkLimits();
  _value(
    'InstanceLimits',
    limits,
    mkLimits(),
    limits.copyWith(),
    limits.copyWith(voiceMaxDurationMs: 6),
  );

  InstanceInfo mkInfo() => InstanceInfo(
    instanceId: uuid(93),
    branding: mkBranding(),
    localeDefault: 'en',
    specVersion: '1',
    features: {'a': true},
    limits: mkLimits(),
    appLock: const AppLockPolicy(required: false, afterMs: 300000),
  );
  final info = mkInfo();
  _value(
    'InstanceInfo',
    info,
    mkInfo(),
    info.copyWith(),
    info.copyWith(features: {'a': false}),
  );
  _value(
    'InstanceInfo.appLock',
    info,
    mkInfo(),
    info.copyWith(),
    info.copyWith(
      appLock: const AppLockPolicy(required: true, afterMs: 300000),
    ),
  );
  const policy = AppLockPolicy(required: true, afterMs: 60000);
  _value(
    'AppLockPolicy',
    policy,
    const AppLockPolicy(required: true, afterMs: 60000),
    policy,
    const AppLockPolicy(required: true, afterMs: 60001),
  );
}
