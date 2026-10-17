// SPDX-License-Identifier: Apache-2.0

// Every mirror table gives back what was stored (15 6.1): instance, principals and agent cards,
// commands and the personal tables, each through its DAO and mapper.

import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:test/test.dart';

import 'support/fixtures.dart';

void main() {
  late LocalDatabase db;
  setUp(() => db = openInMemoryForTests());
  tearDown(() => db.close());

  test('the instance row round-trips and a second put replaces it', () async {
    final info = InstanceInfo(
      instanceId: uuid(0x900),
      branding: Branding(
        productName: 'Example Chat',
        version: const ResourceVersion(3),
        accentColor: '#136F69',
        logoFileId: FileId.parse(uuid(0x901)),
      ),
      localeDefault: 'de',
      specVersion: '1.0.0',
      features: const {'terminal': true, 'webhooks': false},
      limits: const InstanceLimits(
        messageMaxContentBytes: 65536,
        messageMaxAttachments: 10,
        fileMaxUploadBytes: 1073741824,
        threadTitleMaxChars: 120,
        voiceMaxDurationMs: 600000,
      ),
    );
    final dao = db.instanceDao;
    expect(await dao.get(), isNull);
    await dao.put(info);
    expect(await dao.get(), info);
    final next = InstanceInfo(
      instanceId: info.instanceId,
      branding: const Branding(
        productName: 'Example Chat',
        version: ResourceVersion(4),
      ),
      localeDefault: 'en',
      specVersion: '1.1.0',
      features: const {},
      limits: info.limits,
    );
    await dao.put(next);
    expect(await dao.get(), next);
    expect(await dao.watch().first, next);
  });

  test('principals and agent cards round-trip', () async {
    final dao = db.principalsDao;
    final person = Principal(
      id: alex,
      kind: PrincipalKind.user,
      handle: 'alex',
      displayName: 'Alex',
      avatarFileId: FileId.parse(uuid(0x902)),
    );
    final agent = Principal(
      id: robin,
      kind: PrincipalKind.agent,
      handle: 'robin',
      displayName: 'Robin',
    );
    await dao.put(person);
    await dao.put(agent);
    expect(await dao.byId(alex), person);
    expect(await dao.byId(sam), isNull);
    expect(await dao.watchAll().first, [person, agent]);

    final card = AgentCard(
      id: robin,
      handle: 'robin',
      displayName: 'Robin',
      description: 'Reviews pull requests',
      billingMode: BillingMode.organization,
      toolClass: ToolClass.scoped,
      presence: const AgentPresence(
        state: AgentRuntimeState.online,
        reportedBillingMode: BillingMode.personal,
      ),
      version: const ResourceVersion(2),
    );
    expect(await dao.upsertAgentIfNewer(card), isTrue);
    expect(
      await dao.upsertAgentIfNewer(
        card.copyWith(version: const ResourceVersion(1)),
      ),
      isFalse,
    );
    expect(await dao.agentById(robin), card);
    expect(await dao.agentById(alex), isNull);
    expect(await dao.watchAgents().first, [card]);
  });

  test('commands round-trip, filter by prefix and delete', () async {
    final dao = db.commandsDao;
    Command command(int n, String name, {int version = 1}) => Command(
      id: CommandId.parse(uuid(0x910 + n)),
      ownerPrincipalId: robin,
      name: name,
      description: 'Command $n',
      scopeKind: TargetKind.channel,
      scopeId: uuid(0x201),
      options: const [
        CommandOption(
          name: 'target',
          description: 'Where to deploy',
          type: CommandOptionType.choice,
          required: true,
          choices: [CommandChoice(name: 'Staging', value: 'staging')],
          autocomplete: false,
        ),
      ],
      requiredLevel: CommandLevel.trigger,
      control: n.isEven,
      version: ResourceVersion(version),
    );
    final deploy = command(1, 'deploy');
    final describe = command(2, 'describe');
    final other = command(3, 'status');
    for (final c in [deploy, describe, other]) {
      expect(await dao.upsertIfNewer(c), isTrue);
    }
    expect(await dao.upsertIfNewer(command(1, 'deploy')), isFalse);
    expect(await dao.withPrefix('de'), unorderedEquals([deploy, describe]));
    await dao.delete(deploy.id);
    expect(await dao.withPrefix('de'), [describe]);
    expect(await dao.watchAll().first, unorderedEquals([describe, other]));
  });

  test(
    'settings, notification preferences and quiet hours round-trip',
    () async {
      final dao = db.personalDao;
      final setting = UserSetting(
        key: 'appearance.theme',
        version: const ResourceVersion(2),
        value: 'dark',
        updatedAt: at(5),
      );
      expect(await dao.upsertSettingIfNewer(setting), isTrue);
      expect(
        await dao.upsertSettingIfNewer(
          UserSetting(
            key: 'appearance.theme',
            version: const ResourceVersion(1),
          ),
        ),
        isFalse,
      );
      expect(await dao.watchSettings().first, [setting]);

      final preference = NotificationPreference(
        targetKind: NotificationTargetKind.channel,
        targetId: uuid(0x201),
        level: NotificationLevel.mentions,
        version: const ResourceVersion(1),
        mutedUntil: at(60),
      );
      expect(await dao.upsertNotificationPreferenceIfNewer(preference), isTrue);
      expect(await dao.notificationPreferences(), [preference]);

      final rule = QuietHours(
        id: QuietHoursId.parse(uuid(0x920)),
        weekdays: 0x1f,
        startLocal: '22:00',
        endLocal: '07:00',
        enabled: true,
        version: const ResourceVersion(1),
        timeZone: 'Europe/Berlin',
      );
      expect(await dao.upsertQuietHoursIfNewer(rule), isTrue);
      expect(await dao.quietHours(), [rule]);
      await dao.deleteQuietHours(rule.id);
      expect(await dao.quietHours(), isEmpty);
    },
  );
}
