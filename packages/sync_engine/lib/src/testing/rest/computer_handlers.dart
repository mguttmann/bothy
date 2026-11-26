// SPDX-License-Identifier: Apache-2.0

/// Handlers of computer use for the apps (docs/spec/27-computer-use.md 12.2; brief CU-07
/// Scope): `listComputerSessions`, `stopComputerSession` and `disableComputerDevice` over the
/// computer sessions of the [FakeTerminal].
///
/// A small model of the server: every operation is for owners only (`403 forbidden` with
/// `reason` `owner_only` otherwise, 07 2.15 items 4 and 5), the list holds the caller's own
/// sessions newest first, a stop ends the session with `stopped_by_user` and `client_stop` (a
/// closed one answers unchanged), and the switch-off follows
/// [FakeWorldComputer.disableComputer]. The agent's operations (`listComputerDesktops`,
/// `openComputerSession`) are not served: the tests open a session with
/// [FakeWorldComputer.openComputerSession] as the runner would.
library;

import '../rest_fake.dart';
import '../world.dart';

/// Registers the computer handlers on [rest].
void registerComputerHandlers(RestFake rest) {
  final world = rest.world;
  final terminal = world.terminal;

  void owner(FakeRequest r) {
    if (!r.person.isOwner) {
      throw const FakeProblem(403, 'forbidden', reason: 'owner_only');
    }
  }

  rest
    ..handle('listComputerSessions', (r) {
      owner(r);
      final me = r.person.id;
      final state = r.query('state');
      final items = [
        for (final s in terminal.computerSessions.values.toList().reversed)
          if (s.userId == me && (state == null || s.state == state)) s.json,
      ];
      return FakeResponse(200, {'items': items});
    })
    ..handle('stopComputerSession', (r) {
      owner(r);
      final s =
          terminal.computerSessions[r.param('session_id')] ??
          (throw const FakeProblem(404, 'not_found'));
      world.endComputerSession(
        s,
        'stopped_by_user',
        detail: 'client_stop',
        stoppedBy: r.person.id,
      );
      return FakeResponse(200, s.json);
    })
    ..handle('disableComputerDevice', (r) {
      owner(r);
      final h =
          world.hosts[r.param('host_id')] ??
          (throw const FakeProblem(404, 'not_found'));
      world.disableComputer(h);
      return FakeResponse(200, h.ownerJson(r.person.id));
    });
}
