// SPDX-License-Identifier: Apache-2.0

// The goldens of the mentions view and of Running now D10 (brief M4-23 Tests): two mentions
// with path, author and time; three runs (running with context and an organization cost and
// Stop, waiting for the person, queued in Personal mode) with the filters and the footer with
// the organization sum as an estimate and the note on personal runs. Light and dark, two
// densities, text scaling 1 and 2, German and English.

import 'dart:io';

import 'package:alchemist/alchemist.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/mentions/mentions_view.dart';
import 'package:client/mentions/providers.dart';
import 'package:client/running/providers.dart';
import 'package:client/running/running_now_view.dart';
import 'package:client/settings/settings_providers.dart';
import 'package:client/sidebar/providers.dart';
import 'package:client/thread/providers.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;

import '../../support/fakes.dart';
import '../../support/structure.dart';
import '../../support/thread_fixtures.dart';

final DateTime now = t0.add(const Duration(minutes: 4, seconds: 12));

class _Profile extends ProfileController {
  @override
  Future<Profile> build() async =>
      const Profile(locale: 'de', timeZone: 'Europe/Berlin');
}

class _Mentions extends MentionsController {
  @override
  Future<MentionsPage> build() async => MentionsPage(
    entries: [
      MentionEntry(
        message: msg(
          2,
          author: sam,
          text: '@Alex can you check the resume window after the deploy?',
          minutes: 3,
        ),
        authorName: 'Sam',
        path: const [
          'Engineering',
          'server',
          'Gateway: resume after network change',
        ],
      ),
      MentionEntry(
        message: msg(
          1,
          author: robin,
          text: '@Alex the weekly report is ready for review.',
        ),
        authorName: 'Robin',
        path: const ['Research', 'reports', 'Weekly report'],
      ),
    ],
    nextCursor: 'next',
  );
}

RunSummary active(
  int n,
  RunState state, {
  BillingMode mode = BillingMode.organization,
  int micros = 0,
  PrincipalId? triggeredBy,
}) => run(n, state, threadN: n, triggeredBy: triggeredBy).copyWith(
  billingModeExpected: mode,
  cost: Money(amountMicros: micros, currency: 'USD', isEstimate: true),
  startedAt: state == RunState.queued ? null : t0,
);

final List<RunningRow> rows = [
  RunningRow(
    run: active(1, RunState.running, micros: 620000),
    agentName: 'dev-runner',
    path: const [
      'Engineering',
      'server',
      'Gateway: resume after network change',
    ],
    spaceId: spaceId(1),
    canStop: true,
  ),
  RunningRow(
    run: active(2, RunState.waitingInput, micros: 180000, triggeredBy: sam),
    agentName: 'dev-runner',
    path: const ['Engineering', 'server', 'Check migration 0012'],
    spaceId: spaceId(1),
    canStop: false,
  ),
  RunningRow(
    run: active(3, RunState.queued, mode: BillingMode.personal),
    agentName: 'research-bot',
    path: const ['Research', 'reports', 'Weekly report'],
    spaceId: spaceId(2),
    canStop: false,
  ),
];

List<Override> overrides() => [
  appPortsProvider.overrideWithValue(
    testPorts(dataRoot: Directory.systemTemp, clock: TestClock(now)),
  ),
  profileControllerProvider.overrideWith(_Profile.new),
  mentionsControllerProvider.overrideWith(_Mentions.new),
  runningRefreshProvider.overrideWith((ref) async => true),
  runningRowsProvider.overrideWith((ref) => Stream.value(rows)),
  runningClockProvider.overrideWith((ref) => Stream.value(now)),
  runContextProvider(rows.first.run.id.value)
      .overrideWith((ref) => Stream.value(62)),
  storedSpacesProvider.overrideWith(
    (ref) => Stream.value([
      space(1, name: 'Engineering'),
      space(2, name: 'Research'),
    ]),
  ),
  myPrincipalIdProvider.overrideWithValue(alex),
  syncEngineProvider.overrideWithValue(null),
];

Widget framed(Widget child) => ProviderScope(
  overrides: overrides(),
  child: Material(
    type: MaterialType.transparency,
    child: SizedBox(width: 900, height: 520, child: child),
  ),
);

final Map<String, Widget Function()> views = {
  'mentions_view': () => framed(const MentionsView()),
  'running_now_d10': () => framed(const RunningNowView()),
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
