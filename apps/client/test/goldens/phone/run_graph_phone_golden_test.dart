// SPDX-License-Identifier: Apache-2.0

// The goldens of the control room M11 and the run graph M12 on the phone (brief M5-14 Tests;
// 01 5.3 M11, M12): the cards in one column (one live with its document, the others paused
// with Stop where allowed) and the run graph of a run with a subagent and a background job,
// with the bars below the labels in portrait and beside them in landscape. The density
// `touch`, light and dark, text scaling 1 and 2, German and English, the motion context
// `reduced` of every cell. One file per screen, orientation and theme keeps each file below
// repo.max_file_kb.

import 'dart:io';

import 'package:alchemist/alchemist.dart';
import 'package:client/agents/providers.dart';
import 'package:client/app/providers/ports.dart';
import 'package:client/app/providers/session.dart';
import 'package:client/app/router.dart';
import 'package:client/phone/phone_navigation.dart';
import 'package:client/running/providers.dart';
import 'package:client/running/run_graph_screen.dart';
import 'package:client/running/running_now_view.dart';
import 'package:client/sidebar/providers.dart';
import 'package:client/thread/providers.dart';
import 'package:design_system/design_system.dart';
import 'package:design_system/testing.dart';
import 'package:domain/domain.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:sync_engine/sync_engine.dart';

import '../../phone/support/phone_app.dart' show phoneSize;
import '../../phone/support/phone_scenes.dart' show phoneFrame;
import '../../support/agent_fakes.dart' show runDoc, runId;
import '../../support/fakes.dart';
import '../../support/structure.dart';
import '../../support/thread_fixtures.dart';

final DateTime now = t0.add(const Duration(minutes: 4, seconds: 12));

/// A landscape phone: the portrait window turned.
final Size landscape = Size(phoneSize.height, phoneSize.width);

String _at(int seconds) => t0.add(Duration(seconds: seconds)).toIso8601String();

Map<String, Object?> _step(
  String id,
  int ordinal,
  String kind,
  String name, {
  String? parent,
  String title = '',
  int start = 0,
  int? end,
  String? status,
}) => {
  'id': id,
  'ordinal': ordinal,
  'kind': kind,
  'name': name,
  'title': title,
  'status': status ?? (end == null ? 'running' : 'completed'),
  'parent_step_id': ?parent,
  'started_at': _at(start),
  'finished_at': ?(end == null ? null : _at(end)),
};

/// The run of the graph and of the live card: a subagent writing tests with a running Bash
/// call, a background job, a failed subagent and finished reads.
final RunDocument document = RunDocument.fromJson({
  ...runDoc(
    1,
    steps: {
      for (final s in [
        _step('r1', 1, 'tool_call', 'Read', start: 3, end: 9),
        _step('r2', 2, 'tool_call', 'Grep', start: 10, end: 14),
        _step('r3', 3, 'tool_call', 'Read', start: 15, end: 21),
        _step('r4', 4, 'tool_call', 'Read', start: 22, end: 30),
        _step(
          'sa',
          5,
          'subagent',
          'tests',
          title: 'Write test cases',
          start: 35,
        ),
        _step('b1', 6, 'tool_call', 'Bash', parent: 'sa', start: 120),
        _step('j1', 7, 'background_job', 'npm test', start: 60),
        _step(
          'sb',
          8,
          'subagent',
          'migration',
          title: 'Check migration',
          start: 40,
          end: 95,
          status: 'failed',
        ),
      ])
        s['id']! as String: s,
    },
  ),
  'thread_id': threadId(1).value,
});

RunSummary _active(int n, RunState state, {PrincipalId? triggeredBy}) =>
    run(n, state, threadN: n, triggeredBy: triggeredBy).copyWith(
      cost: const Money(
        amountMicros: 620000,
        currency: 'USD',
        isEstimate: true,
      ),
      startedAt: state == RunState.queued ? null : t0,
    );

final List<RunningRow> rows = [
  RunningRow(
    run: _active(1, RunState.running),
    agentName: 'dev-runner',
    path: const ['Engineering', 'server', 'Gateway: resume after network'],
    spaceId: spaceId(1),
    canStop: true,
  ),
  RunningRow(
    run: _active(2, RunState.running, triggeredBy: sam),
    agentName: 'research-bot',
    path: const ['Research', 'reports', 'Weekly report'],
    spaceId: spaceId(2),
    canStop: false,
  ),
  RunningRow(
    run: _active(3, RunState.waitingInput),
    agentName: 'dev-runner',
    path: const ['Engineering', 'server', 'Check migration 0012'],
    spaceId: spaceId(1),
    canStop: true,
  ),
];

/// Card 1 is live with [document]; the others show their summary.
final class _Focus extends ControlRoomFocus {
  @override
  Set<RunId> build() => {runId(1)};

  @override
  void show(List<({RunId run, ThreadId thread})> cards) {}
}

List<Override> _overrides() => [
  appPortsProvider.overrideWithValue(
    testPorts(
      dataRoot: Directory.systemTemp,
      clock: TestClock(now),
      platform: 'ios',
    ),
  ),
  runningRefreshProvider.overrideWith((ref) async => true),
  runningRowsProvider.overrideWith((ref) => Stream.value(rows)),
  runningClockProvider.overrideWith((ref) => Stream.value(now)),
  controlRoomFocusProvider.overrideWith(_Focus.new),
  cardDocumentProvider.overrideWith(
    (ref, id) => Stream.value(id == runId(1).value ? document : null),
  ),
  runGraphDocumentProvider.overrideWith((ref, args) => Stream.value(document)),
  threadRunsProvider.overrideWith(
    (ref, thread) => Stream.value([_active(1, RunState.running)]),
  ),
  principalsByIdProvider.overrideWith(
    (ref) => Stream.value({for (final p in people) p.id: p}),
  ),
  storedSpacesProvider.overrideWith(
    (ref) => Stream.value([
      space(1, name: 'Engineering'),
      space(2, name: 'Research'),
    ]),
  ),
  myPrincipalIdProvider.overrideWithValue(alex),
  syncEngineProvider.overrideWithValue(null),
];

Widget _scene(Widget body, {required Size size, required bool graph}) =>
    ProviderScope(
      overrides: _overrides(),
      child: phoneFrame(
        size: size,
        tab: graph ? null : PhoneTab.running,
        running: rows.length,
        back: graph ? 'Running' : null,
        body: body,
      ),
    );

final Map<String, Widget Function()> scenes = {
  for (final (orientation, size) in [
    ('portrait', phoneSize),
    ('landscape', landscape),
  ]) ...{
    'm11_running_$orientation': () =>
        _scene(const RunningNowView(), size: size, graph: false),
    'm12_run_graph_$orientation': () => _scene(
      RunGraphScreen(location: RunGraphLocation(runId(1), threadId(1))),
      size: size,
      graph: true,
    ),
  },
};

void main() {
  for (final MapEntry(key: name, value: scene) in scenes.entries) {
    for (final theme in DsThemeMode.values) {
      goldenTest(
        '$name in ${theme.name}, text scale 1 and 2, de and en',
        fileName: 'phone_${name}_${theme.name}',
        pumpBeforeTest: dsPumpIcons,
        builder: () => dsGoldenMatrix(
          name: name,
          themes: [theme],
          densities: const [DsDensityMode.touch],
          textScales: const [1.0, 2.0],
          locales: const [Locale('de'), Locale('en')],
          builder: scene,
        ),
      );
    }
  }
}
