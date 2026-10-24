// SPDX-License-Identifier: Apache-2.0

// The golden of the quick switcher D6 (brief M4-23 Tests): mixed rows for the input "r" (a
// recently visited thread first, then channels, threads, a server title of a thread that is
// not stored, a person, an agent with a DM and commands of the current channel) with the
// filter row. Light and dark, two densities, text scaling 1 and 2, German and English.

import 'dart:io';

import 'package:alchemist/alchemist.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/switcher/providers.dart';
import 'package:client/switcher/quick_switcher.dart';
import 'package:client/switcher/recent_targets.dart';
import 'package:client/switcher/switcher_sources.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../support/fakes.dart';
import '../../support/structure.dart';

const String query = 'r';

final Principal robinPerson = Principal(
  id: PrincipalId.parse(uuid(0xa8)),
  kind: PrincipalKind.user,
  handle: 'robin',
  displayName: 'Robin',
);

final PrincipalId researchBot = PrincipalId.parse(uuid(0xa9));

final SwitcherLocalData local = SwitcherLocalData(
  spaces: [
    space(1, name: 'Support'),
    space(2, name: 'Engineering'),
  ],
  channels: [
    channel(1, name: 'general'),
    channel(2, name: 'server', spaceN: 2),
    channel(3, name: 'release-notes', spaceN: 2),
    Channel(
      id: channelId(9),
      kind: ChannelKind.dm,
      name: '',
      topic: '',
      sortKey: 'a9',
      operator: const ChannelOperator(mode: OperatorMode.inherit),
      readOnly: false,
      dmParticipantIds: [sam, researchBot],
      createdAt: t0,
      version: const ResourceVersion(1),
      settingsVersion: const ResourceVersion(1),
    ),
  ],
  threads: [
    thread(1, channelN: 2, title: 'Gateway: resume after network change'),
    thread(
      2,
      channelN: 1,
      title: 'Rate limits of the export',
      activeMinutes: 5,
    ),
  ],
  principals: [
    Principal(
      id: sam,
      kind: PrincipalKind.user,
      handle: 'sam',
      displayName: 'Sam',
    ),
    robinPerson,
  ],
  agents: [
    AgentCard(
      id: researchBot,
      handle: 'research-bot',
      displayName: 'research-bot',
      description: '',
      billingMode: BillingMode.organization,
      toolClass: ToolClass.chat,
      presence: const AgentPresence(state: AgentRuntimeState.online),
      version: const ResourceVersion(1),
    ),
  ],
  commands: [
    Command(
      id: CommandId.parse(uuid(0x901)),
      ownerPrincipalId: researchBot,
      name: 'review',
      description: '',
      scopeKind: TargetKind.instance,
      options: const [],
      requiredLevel: CommandLevel.write,
      control: false,
      version: const ResourceVersion(1),
    ),
  ],
  me: sam,
);

Widget framed() => ProviderScope(
  overrides: [
    appPortsProvider.overrideWithValue(
      testPorts(dataRoot: Directory.systemTemp, clock: TestClock(t0)),
    ),
    switcherCurrentChannelProvider('/').overrideWith((ref) async => null),
    switcherLocalDataProvider(null).overrideWithValue(local),
    recentTargetListProvider.overrideWith(
      (ref) => Stream.value([
        RecentTarget(
          kind: RecentTargetKind.thread,
          targetId: threadId(1).value,
          visitedAt: t0,
        ),
      ]),
    ),
    serverThreadTitlesProvider(query).overrideWith(
      (ref) async => [
        ThreadTitleHit(
          threadId: threadId(40),
          channelId: channelId(2),
          title: 'Rollout plan for the gateway',
          lastActivityAt: t0,
        ),
      ],
    ),
  ],
  child: const Material(
    type: MaterialType.transparency,
    child: SizedBox(
      width: 640,
      height: 760,
      child: QuickSwitcher(location: '/', initialQuery: query),
    ),
  ),
);

void main() {
  goldenTest(
    'switcher in light and dark, two densities, text scale 1 and 2, de and en',
    fileName: 'switcher_d6',
    pumpBeforeTest: dsPumpIcons,
    builder: () => dsGoldenMatrix(
      name: 'switcher_d6',
      builder: framed,
      densities: const [DsDensityMode.comfortable, DsDensityMode.compact],
      textScales: const [1.0, 2.0],
      locales: const [Locale('de'), Locale('en')],
    ),
  );
}
