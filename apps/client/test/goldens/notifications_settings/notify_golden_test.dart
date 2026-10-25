// SPDX-License-Identifier: Apache-2.0

// The goldens of D9 and the menus (brief M4-33 Tests): the page with a personal default, the
// preview mode, two quiet hours rules, the switches for an owner and the tree with an own
// level, an inherited one, a temporary mute and a followed thread; the entries of a channel
// menu with "Export history" and of a followed thread. Light and dark, two densities, text
// scaling 1 and 2, German and English. One file per view keeps each file below
// repo.max_file_kb.

import 'dart:io';

import 'package:alchemist/alchemist.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/menus/level_menu.dart';
import 'package:client/notifications_settings/notification_settings_page.dart';
import 'package:client/notifications_settings/providers.dart';
import 'package:client/settings/settings_providers.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:l10n/l10n.dart';

import '../../support/fakes.dart';
import '../../support/structure.dart';

final DateTime now = DateTime.utc(2026, 10, 17, 9);

class _Profile extends ProfileController {
  @override
  Future<Profile> build() async =>
      const Profile(locale: 'de', timeZone: 'Europe/Berlin');
}

NotificationPreference pref(
  NotificationTargetKind kind,
  String id,
  NotificationLevel level, {
  DateTime? until,
}) => NotificationPreference(
  targetKind: kind,
  targetId: id,
  level: level,
  mutedUntil: until,
  version: const ResourceVersion(1),
);

final Channel general = channel(1, name: 'general');
final Channel requests = channel(2, name: 'requests', kind: ChannelKind.forum);
final Channel server = channel(3, spaceN: 2, name: 'server');

final LevelContext levels = LevelContext(
  preferences: [
    pref(NotificationTargetKind.space, spaceId(1).value, NotificationLevel.all),
    pref(
      NotificationTargetKind.channel,
      requests.id.value,
      NotificationLevel.mentions,
    ),
    pref(
      NotificationTargetKind.space,
      spaceId(2).value,
      NotificationLevel.mentions,
    ),
    pref(
      NotificationTargetKind.thread,
      threadId(3).value,
      NotificationLevel.all,
      until: now.add(const Duration(hours: 9)),
    ),
    pref(
      NotificationTargetKind.thread,
      threadId(1).value,
      NotificationLevel.all,
    ),
  ],
  personalDefault: NotificationLevel.mentions,
  now: now,
);

final List<LevelTreeSpace> tree = [
  LevelTreeSpace(
    space: space(1, name: 'Support'),
    channels: [
      LevelTreeChannel(
        channel: general,
        threads: [LevelTreeThread(id: threadId(1), title: 'Release notes')],
      ),
      LevelTreeChannel(channel: requests, threads: const []),
    ],
  ),
  LevelTreeSpace(
    space: space(2, name: 'Engineering'),
    channels: [
      LevelTreeChannel(
        channel: server,
        threads: [LevelTreeThread(id: threadId(3), title: 'Gateway: resume')],
      ),
    ],
  ),
];

final List<QuietHours> rules = [
  QuietHours(
    id: QuietHoursId.parse('0192f4a0-7b1c-7c3e-88ef-5d4efd8f4f24'),
    weekdays: 0x1f,
    startLocal: '19:00',
    endLocal: '07:00',
    enabled: true,
    version: const ResourceVersion(1),
  ),
  QuietHours(
    id: QuietHoursId.parse('0192f4a0-7b1c-7c3e-88ef-5d4efd8f4f25'),
    weekdays: 0x60,
    startLocal: '00:00',
    endLocal: '00:00',
    enabled: true,
    version: const ResourceVersion(1),
  ),
];

List<Override> overrides() => [
  appPortsProvider.overrideWithValue(
    testPorts(dataRoot: Directory.systemTemp, clock: TestClock(now)),
  ),
  profileControllerProvider.overrideWith(_Profile.new),
  settingsRepositoryProvider.overrideWithValue(null),
  notifyWriterProvider.overrideWithValue(null),
  notifyRefreshProvider.overrideWith((ref) async {}),
  isOwnerProvider.overrideWith((ref) async => true),
  notifySettingsProvider.overrideWithValue(
    NotifySettings.resolve(const {
      'notify.default_level': 'mentions',
      'notify.preview_mode': 'names',
      'notify.input_needed_breaks_quiet_hours': true,
    }),
  ),
  levelContextProvider.overrideWithValue(levels),
  levelTreeProvider.overrideWith((ref) async => tree),
  quietHoursRulesProvider.overrideWith((ref) => Stream.value(rules)),
];

Widget framed(Widget child, {double width = 760}) => ProviderScope(
  overrides: overrides(),
  child: Material(
    type: MaterialType.transparency,
    child: SizedBox(
      width: width,
      child: Padding(padding: const EdgeInsets.all(24), child: child),
    ),
  ),
);

/// The entries of one menu as the open menu shows them: the top level and the two submenus
/// side by side.
class MenuEntries extends ConsumerWidget {
  const MenuEntries({super.key, required this.target, this.export = false});

  final LevelMenuTarget target;
  final bool export;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = AppLocalizations.of(context);
    final sections = levelMenuSections(context, ref, target);
    Widget panel(List<Widget> children) => Material(
      elevation: 3,
      color: context.dsColors.surfaceRaised,
      child: IntrinsicWidth(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          mainAxisSize: MainAxisSize.min,
          children: children,
        ),
      ),
    );
    return Wrap(
      spacing: 12,
      runSpacing: 12,
      crossAxisAlignment: WrapCrossAlignment.start,
      children: [
        panel([
          DsMenuItem(
            onPressed: null,
            trailingIcon: const Icon(Icons.chevron_right, size: 16),
            child: Text(l.notifyMenuNotifications),
          ),
          DsMenuItem(
            onPressed: null,
            trailingIcon: const Icon(Icons.chevron_right, size: 16),
            child: Text(l.notifyMenuMuteUntil),
          ),
          if (export)
            DsMenuItem(onPressed: () {}, child: Text(l.channelMenuExport)),
        ]),
        panel(sections.levels),
        panel(sections.mute),
      ],
    );
  }
}

final Map<String, Widget Function()> views = {
  'notify_d9': () => framed(const NotificationSettingsPage()),
  'notify_menus': () => framed(
    width: 820,
    Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        MenuEntries(
          export: true,
          target: LevelMenuTarget(
            kind: NotificationTargetKind.channel,
            id: general.id.value,
            own: null,
            inherited: levels.inherited(
              NotificationTargetKind.channel,
              channel: general,
            ),
            now: now,
          ),
        ),
        const SizedBox(height: 16),
        MenuEntries(
          target: LevelMenuTarget(
            kind: NotificationTargetKind.thread,
            id: threadId(1).value,
            own: levels.own(NotificationTargetKind.thread, threadId(1).value),
            inherited: levels.inherited(
              NotificationTargetKind.thread,
              channel: general,
            ),
            now: now,
          ),
        ),
      ],
    ),
  ),
};

void main() {
  for (final MapEntry(key: name, value: view) in views.entries) {
    goldenTest(
      '$name in light and dark, two densities, text scale 1 and 2, de and en',
      fileName: name,
      pumpBeforeTest: dsPumpIcons,
      builder: () => dsGoldenMatrix(
        name: name,
        builder: view,
        densities: const [DsDensityMode.comfortable, DsDensityMode.compact],
        textScales: const [1.0, 2.0],
        locales: const [Locale('de'), Locale('en')],
      ),
    );
  }
}
