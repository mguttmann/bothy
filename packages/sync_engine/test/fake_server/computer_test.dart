// SPDX-License-Identifier: Apache-2.0

// Computer use in the fake server (brief CU-07 Scope; docs/spec/27-computer-use.md 4, 8, 12):
// an owner lists the own computer sessions, stops one and switches the enablement of a host off;
// each change appends `computer_session.changed`, the switch-off ends the host's session with
// `device_disabled` and raises the host's version; the host carries the reported `computer`
// object and `computer_listed` only for owners, never in `host.updated`; a person who is no
// owner is refused. Every request, response and event matches the contract.

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late FakeWorldHarness h;
  setUp(() async => h = await FakeWorldHarness.start());
  tearDown(() async {
    expect(h.server.violations, isEmpty);
    await h.stop();
  });

  FakeHost desk() {
    final host = h.server.world.addTerminalHost(
      name: 'Desk',
      noisePublicKey: 'Zm9vYmFyZm9vYmFyZm9vYmFyZm9vYmFyZm9vYmFyMTI',
      grantees: [h.seed.sam.id],
    );
    h.server.world.reportComputer(host.id, enabled: true, generation: 3);
    return host;
  }

  Future<ApiProblem?> problemOf(Future<Object?> call) async {
    try {
      await call;
      return null;
    } on DioException catch (e) {
      return ApiProblem.of(e);
    }
  }

  List<String> computerEvents() => [
    for (final e in h.server.world.log)
      if (e.type == 'computer_session.changed')
        [
          (e.envelope['payload']! as Map)['state'],
          ?(e.envelope['payload']! as Map)['close_reason'],
        ].join(' '),
  ];

  test('an owner lists the own sessions and stops one', () async {
    final alex = h.device();
    await signedIn(h, alex);
    final host = desk();
    final s = h.server.world.openComputerSession(
      hostId: host.id,
      userId: h.seed.alex.id,
    );
    h.server.world.setComputerState(
      s,
      state: 'active',
      actionCount: 4,
      frontApp: 'com.example.editor',
    );
    final api = alex.api.getTerminalApi();
    final listed = (await api.listComputerSessions()).data!.items;
    expect(listed.single.id, s.id);
    expect(listed.single.desktop, 'Desk');
    expect(listed.single.state, ComputerSessionState.active);
    expect(listed.single.actionCount, 4);
    expect(listed.single.frontApp, 'com.example.editor');

    final stopped = (await api.stopComputerSession(sessionId: s.id)).data!;
    expect(stopped.state, ComputerSessionState.closed);
    expect(stopped.closeReason, ComputerCloseReason.stoppedByUser);
    expect(stopped.closeDetail, ComputerCloseDetail.clientStop);
    // A second stop answers the closed session unchanged.
    final again = (await api.stopComputerSession(sessionId: s.id)).data!;
    expect(again.version, stopped.version);
    expect(computerEvents(), ['opening', 'active', 'closed stopped_by_user']);
  });

  test(
    'switching the enablement off ends the session and unlists the host',
    () async {
      final alex = h.device();
      await signedIn(h, alex);
      final host = desk();
      final s = h.server.world.openComputerSession(
        hostId: host.id,
        userId: h.seed.alex.id,
      );
      final hosts = alex.api.getHostsApi();
      final before = (await hosts.getHost(hostId: host.id)).data!;
      expect(before.computer?.enabled, isTrue);
      expect(before.computerListed, isTrue);

      final after = (await alex.api.getTerminalApi().disableComputerDevice(
        hostId: host.id,
      )).data!;
      expect(after.computerDisabledGeneration, 3);
      expect(after.computerListed, isFalse);
      expect(after.version, greaterThan(before.version));
      expect(
        h.server.world.terminal.computerSessions[s.id]!.closeReason,
        'device_disabled',
      );
      expect(computerEvents(), ['opening', 'closed device_disabled']);
      // 27 12.4: host.updated carries none of the computer members.
      final updated = h.server.world.log.where((e) => e.type == 'host.updated');
      for (final e in updated) {
        final payload = (e.envelope['payload']! as Map)['host']! as Map;
        expect(payload.containsKey('computer'), isFalse);
        expect(payload.containsKey('computer_listed'), isFalse);
      }
    },
  );

  test(
    'a person who is no owner sees no computer state and is refused',
    () async {
      final sam = h.device(login: WorldSeed.samLogin, keySeed: 2);
      await signedIn(h, sam);
      final host = desk();
      final s = h.server.world.openComputerSession(
        hostId: host.id,
        userId: h.seed.alex.id,
      );
      final seen = (await sam.api.getHostsApi().getHost(hostId: host.id)).data!;
      expect(seen.computer, isNull);
      expect(seen.computerListed, isNull);
      final terminal = sam.api.getTerminalApi();
      for (final call in <Future<Object?>>[
        terminal.listComputerSessions(),
        terminal.stopComputerSession(sessionId: s.id),
        terminal.disableComputerDevice(hostId: host.id),
      ]) {
        final p = await problemOf(call);
        expect(p?.status, 403);
        expect(p?.reason, 'owner_only');
      }
      expect(h.server.world.terminal.computerSessions[s.id]!.closed, isFalse);
    },
  );
}
