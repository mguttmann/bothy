// SPDX-License-Identifier: Apache-2.0

// The host events of the terminal (brief M4-36 Scope; 04 2.1, 8.2; 15 23.1): `host.updated` and
// `terminal_session.changed` (durable) and `host.presence_changed` (ephemeral) reach the hub
// after their page, with IDs, versions and states only; `access.changed` entries of kind `host`
// signal a removed or a newly granted host. Nothing of them is stored.

import 'dart:async';

import 'package:sync_engine/sync_engine.dart';
import 'package:test/test.dart';

import '../catch_up/support.dart';

/// Host [n].
String hostId(int n) => id(0x600 + n);

/// Terminal session [n].
String sessionId(int n) => id(0x650 + n);

/// An API `Host` named "Build host" with [connected] and [state].
Map<String, Object?> hostJson(
  int n, {
  int version = 2,
  bool connected = true,
  String state = 'active',
}) => {
  'id': hostId(n),
  'name': 'Build host',
  'state': state,
  'disabled_reason': ?(state == 'disabled' ? 'owner' : null),
  'llm_credentials': 'organization',
  'declared_server_host': false,
  'noise_public_key': 'Zm9vYmFyZm9vYmFyZm9vYmFyZm9vYmFyZm9vYmFyMTI',
  'key_fingerprint':
      '7K2Q-M9XD-0000-0000-0000-0000-0000-0000-0000-0000-0000-0000-0000',
  'key_generation': 1,
  'profiles': [
    {'name': 'shell', 'display_name': 'Shell', 'kind': 'shell'},
  ],
  'doctor': <Object?>[],
  'connected': connected,
  'created_at': at,
  'version': version,
};

/// An API `TerminalSession` of Alex on host [hostN].
Map<String, Object?> sessionJson(
  int n, {
  int hostN = 1,
  String state = 'active',
  String? closeReason,
  int version = 3,
}) => {
  'id': sessionId(n),
  'host_id': hostId(hostN),
  'user_id': alexId,
  'device_id': deviceId,
  'profile': 'shell',
  'state': state,
  'opened_at': at,
  'closed_at': ?(state == 'closed' ? at : null),
  'close_reason': ?closeReason,
  'bytes_in': 4096,
  'bytes_out': 512,
  'version': version,
};

void main() {
  late World w;
  late List<HostEvent> events;
  late StreamSubscription<HostEvent> sub;
  setUp(() async {
    w = World();
    events = [];
    sub = w.catchUp.hostEvents.listen(events.add);
    await w.connect(cursor: 30);
    await w.push('catchup.complete', {'head_seq': 30});
  });
  tearDown(() async {
    await sub.cancel();
    await w.dispose();
  });

  test(
    'host.updated reaches the hub with ID, version, state and connection',
    () async {
      await w.dispatch([
        envelope(
          'host.updated',
          seq: 31,
          payload: {'host': hostJson(1, version: 4), 'key_changed': true},
          resourceKind: 'host',
          resourceId: hostId(1),
          version: 4,
        ),
      ]);
      expect(events, [
        HostChanged(
          hostId: hostId(1),
          version: 4,
          state: 'active',
          connected: true,
          keyChanged: true,
        ),
      ]);
      // Only IDs, versions and states: the name of the host is not carried.
      expect(events.single.toString(), isNot(contains('Build host')));
    },
  );

  test('host.presence_changed reaches the hub at once', () async {
    await w.dispatch([
      envelope(
        'host.presence_changed',
        payload: {'host_id': hostId(1), 'connected': false},
        resourceKind: 'host',
        resourceId: hostId(1),
      ),
    ]);
    expect(events, [HostPresenceChanged(hostId: hostId(1), connected: false)]);
  });

  test(
    'terminal_session.changed reaches the hub with state and close reason',
    () async {
      await w.dispatch([
        envelope(
          'terminal_session.changed',
          seq: 31,
          payload: sessionJson(
            1,
            state: 'closed',
            closeReason: 'access_revoked',
          ),
          resourceKind: 'terminal_session',
          resourceId: sessionId(1),
          version: 3,
          personal: true,
        ),
      ]);
      expect(events, [
        TerminalSessionChanged(
          sessionId: sessionId(1),
          hostId: hostId(1),
          userId: alexId,
          deviceId: deviceId,
          state: 'closed',
          version: 3,
          closeReason: 'access_revoked',
        ),
      ]);
    },
  );

  test(
    'access.changed of kind host signals the removed and the added hosts',
    () async {
      await w.dispatch([
        envelope(
          'access.changed',
          seq: 31,
          payload: {
            'permission_version': 5,
            'added': [
              {
                'kind': 'host',
                'id': hostId(2),
                'capabilities': ['terminal'],
              },
            ],
            'removed': [
              {'kind': 'host', 'id': hostId(1)},
            ],
            'changed': <Object?>[],
          },
          resourceKind: 'principal',
          resourceId: alexId,
          personal: true,
        ),
      ]);
      expect(
        events,
        unorderedEquals([
          HostRemoved(hostId: hostId(1)),
          HostAccessGranted(hostId: hostId(2)),
        ]),
      );
    },
  );

  test(
    'a duplicate below the cursor and a malformed payload reach nothing',
    () async {
      await w.dispatch([
        envelope(
          'host.updated',
          seq: 12,
          payload: {'host': hostJson(1)},
          resourceKind: 'host',
          resourceId: hostId(1),
          version: 2,
        ),
        envelope(
          'host.presence_changed',
          payload: {'host_id': 'not-a-uuid'},
          resourceKind: 'host',
        ),
      ]);
      expect(events, isEmpty);
    },
  );

  test(
    'host.updated and terminal_session.changed are applied, not skipped',
    () {
      expect(
        appliedEventTypes,
        containsAll(['host.updated', 'terminal_session.changed']),
      );
    },
  );
}
