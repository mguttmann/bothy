// SPDX-License-Identifier: Apache-2.0

// The owner's computer sessions in the app (brief CU-07 requirements 13, 16 and 17; 27 11):
// loaded from `listComputerSessions` and kept current by `computer_session.changed`; an older
// version never replaces a newer one; a session this device saw live names its close reason
// once, until dismissed; stop and switch-off go to the API; a person who is no owner loads
// nothing; and the only operations are the list, the stop and the switch-off.

import 'dart:async';

import 'package:api_client/api_client.dart';
import 'package:client/computer/computer_status.dart';
import 'package:client/computer/providers.dart';
import 'package:client/notifications_settings/providers.dart'
    show isOwnerProvider;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:sync_engine/sync_engine.dart' show HostEvent;

import 'support.dart';

void main() {
  late FakeComputerApi api;
  late StreamController<HostEvent> events;
  setUp(() {
    api = FakeComputerApi();
    events = StreamController<HostEvent>.broadcast(sync: true);
  });
  tearDown(() => events.close());

  ProviderContainer container({bool owner = true}) {
    final c = ProviderContainer(
      overrides: [
        computerPortsProvider.overrideWithValue(
          ComputerPorts(api: api, events: events.stream),
        ),
        isOwnerProvider.overrideWith((ref) async => owner),
      ],
    );
    addTearDown(c.dispose);
    return c;
  }

  /// Reads the state once the owner check and the first load settled.
  Future<ComputerSessionsState> settled(ProviderContainer c) async {
    c.listen(computerSessionsProvider, (_, _) {});
    await c.read(isOwnerProvider.future);
    for (var i = 0; i < 5; i++) {
      await Future<void>.delayed(Duration.zero);
    }
    return c.read(computerSessionsProvider);
  }

  test('the live sessions come from the list, closed ones stay out', () async {
    api.sessions = [
      computerSession(1),
      computerSession(
        2,
        state: ComputerSessionState.closed,
        closeReason: ComputerCloseReason.done,
      ),
    ];
    final c = container();
    final s = await settled(c);
    expect(s.live.keys, [computerSessionId(1)]);
    expect(s.ended, isEmpty);
    final live = s.live[computerSessionId(1)]!;
    expect(live.phase, ComputerPhase.active);
    expect(live.desktop, 'Desk');
    expect(live.frontApp, 'com.example.editor');
    expect(live.actionCount, 4);
    expect(s.ofRun(deskRun), live);
    expect(s.liveOn(deskHost), live);
  });

  test('events update the session; a stale version changes nothing; the end shows once', () async {
    api.sessions = [computerSession(1)];
    final c = container();
    await settled(c);
    events.add(computerEvent(1, state: 'paused', actions: 6, version: 3));
    var s = c.read(computerSessionsProvider);
    expect(s.live[computerSessionId(1)]!.phase, ComputerPhase.paused);
    expect(s.live[computerSessionId(1)]!.actionCount, 6);
    // An older version (a replay) never goes back.
    events.add(computerEvent(1, actions: 5, version: 2));
    expect(
      c.read(computerSessionsProvider).live[computerSessionId(1)]!.phase,
      ComputerPhase.paused,
    );
    events.add(
      computerEvent(
        1,
        state: 'closed',
        closeReason: 'device_disabled',
        version: 4,
      ),
    );
    s = c.read(computerSessionsProvider);
    expect(s.live, isEmpty);
    expect(s.ended[computerSessionId(1)]!.closeReason, 'device_disabled');
    expect(s.ofRun(deskRun)!.closed, isTrue);
    c.read(computerSessionsProvider.notifier).dismiss(computerSessionId(1));
    s = c.read(computerSessionsProvider);
    expect(s.ended, isEmpty);
    expect(s.ofRun(deskRun), isNull);
    // The same end again (a duplicate) does not bring the notice back.
    events.add(
      computerEvent(
        1,
        state: 'closed',
        closeReason: 'device_disabled',
        version: 4,
      ),
    );
    expect(c.read(computerSessionsProvider).ended, isEmpty);
  });

  test(
    'a new session from an event shows; an unknown closed one does not',
    () async {
      final c = container();
      await settled(c);
      events.add(computerEvent(3, state: 'opening', actions: 0, version: 1));
      expect(
        c.read(computerSessionsProvider).live[computerSessionId(3)]!.phase,
        ComputerPhase.opening,
      );
      events.add(
        computerEvent(4, state: 'closed', closeReason: 'done', version: 5),
      );
      expect(c.read(computerSessionsProvider).ended, isEmpty);
    },
  );

  test('stop goes to the API and shows the end once', () async {
    api.sessions = [computerSession(1)];
    final c = container();
    await settled(c);
    await c.read(computerSessionsProvider.notifier).stop(computerSessionId(1));
    final s = c.read(computerSessionsProvider);
    expect(api.calls, ['list', 'stop ${computerSessionId(1)}']);
    expect(s.live, isEmpty);
    expect(s.ended[computerSessionId(1)]!.closeReason, 'stopped_by_user');
  });

  test('a refused stop leaves the session and passes the error on', () async {
    api
      ..sessions = [computerSession(1)]
      ..failure = problem(409, 'conflict');
    final c = container();
    await settled(c);
    await expectLater(
      c.read(computerSessionsProvider.notifier).stop(computerSessionId(1)),
      throwsA(isA<Object>()),
    );
    expect(c.read(computerSessionsProvider).live.keys, [computerSessionId(1)]);
  });

  test('switching the enablement off goes to the API', () async {
    final c = container();
    await settled(c);
    api.failure = problem(403, 'forbidden', reason: 'owner_only');
    await expectLater(
      c.read(computerSessionsProvider.notifier).disableDevice(deskHost),
      throwsA(isA<Object>()),
    );
    expect(api.calls, ['list', 'disable $deskHost']);
  });

  test('a person who is no owner loads nothing and hears no event', () async {
    api.sessions = [computerSession(1)];
    final c = container(owner: false);
    final s = await settled(c);
    events.add(computerEvent(1));
    expect(s, ComputerSessionsState.none);
    expect(c.read(computerSessionsProvider), ComputerSessionsState.none);
    expect(api.calls, isEmpty);
  });

  test(
    'the status never names the desktop or the app in a log line (R-CL-18)',
    () {
      final s = ComputerStatus.fromEvent(computerEvent(1));
      expect(s.toString(), isNot(contains('Desk')));
      expect(s.toString(), isNot(contains('com.example.editor')));
    },
  );
}
