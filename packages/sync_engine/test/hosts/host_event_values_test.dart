// SPDX-License-Identifier: Apache-2.0

// The host events are values without content (brief M4-36): equal by their fields, printable
// without a host name, and the hub drops events after it closed.

import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

void main() {
  test('every event is a value of its fields', () {
    final pairs = <(HostEvent, HostEvent, HostEvent)>[
      (
        const HostChanged(
          hostId: 'h',
          version: 2,
          state: 'active',
          connected: true,
        ),
        const HostChanged(
          hostId: 'h',
          version: 2,
          state: 'active',
          connected: true,
        ),
        const HostChanged(
          hostId: 'h',
          version: 3,
          state: 'active',
          connected: true,
        ),
      ),
      (
        const HostPresenceChanged(hostId: 'h', connected: true),
        const HostPresenceChanged(hostId: 'h', connected: true),
        const HostPresenceChanged(hostId: 'h', connected: false),
      ),
      (
        const TerminalSessionChanged(
          sessionId: 's',
          hostId: 'h',
          userId: 'u',
          deviceId: 'd',
          state: 'active',
          version: 1,
        ),
        const TerminalSessionChanged(
          sessionId: 's',
          hostId: 'h',
          userId: 'u',
          deviceId: 'd',
          state: 'active',
          version: 1,
        ),
        const TerminalSessionChanged(
          sessionId: 's',
          hostId: 'h',
          userId: 'u',
          deviceId: 'd',
          state: 'closed',
          version: 2,
          closeReason: 'client_closed',
        ),
      ),
      (
        const HostRemoved(hostId: 'h'),
        const HostRemoved(hostId: 'h'),
        const HostRemoved(hostId: 'g'),
      ),
      (
        const HostAccessGranted(hostId: 'h'),
        const HostAccessGranted(hostId: 'h'),
        const HostAccessGranted(hostId: 'g'),
      ),
    ];
    for (final (a, b, c) in pairs) {
      expect(a, b);
      expect(a.hashCode, b.hashCode);
      expect(a, isNot(c));
      expect(a.toString(), contains(a.hostId));
    }
    final closed = pairs[2].$3 as TerminalSessionChanged;
    expect(closed.closed, isTrue);
    expect((pairs[2].$1 as TerminalSessionChanged).closed, isFalse);
  });

  test('the hub delivers in order and drops events after close', () async {
    final hub = HostEventsHub();
    final seen = <HostEvent>[];
    final sub = hub.events.listen(seen.add);
    hub
      ..add(const HostRemoved(hostId: 'a'))
      ..add(const HostAccessGranted(hostId: 'b'));
    await hub.close();
    hub.add(const HostRemoved(hostId: 'c'));
    await sub.cancel();
    expect(seen, [
      const HostRemoved(hostId: 'a'),
      const HostAccessGranted(hostId: 'b'),
    ]);
  });
}
