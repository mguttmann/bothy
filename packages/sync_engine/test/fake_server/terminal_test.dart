// SPDX-License-Identifier: Apache-2.0

// The terminal of the fake server (brief M4-36 Scope): opening with a fresh step-up and the
// rules of 24 8 (own session user, working directories of `claude` only from the own entry),
// the device stream with `hello`, the check of `identify` and the relay to an in-process host,
// detaching, an attach from another device (`4103` on the first) and a revocation (`4100`
// with `access_revoked`), each with its `terminal_session.changed`. Every request, response
// and event matches the contract.

import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

/// A host that echoes every device message with the prefix `echo:`.
final class _EchoHost implements FakeTerminalHostLink {
  final List<FakeTerminalPipe> pipes = [];

  @override
  void connect(FakeTerminalPipe pipe) {
    pipes.add(pipe);
    pipe.fromDevice.listen(
      (bytes) => pipe.toDevice([...utf8.encode('echo:'), ...bytes]),
    );
  }
}

/// One device stream opened by hand: `hello`, then `identify` with the proof of [device].
final class _Stream {
  _Stream._(this.socket, this.messages);

  final WebSocket socket;
  final StreamIterator<Object?> messages;

  static Future<_Stream> open(
    FakeServer server,
    TestDevice device,
    String streamPath,
  ) async {
    final url = server.baseUri.replace(scheme: 'ws', path: streamPath);
    final socket = await WebSocket.connect(url.toString());
    final messages = StreamIterator<Object?>(socket);
    final s = _Stream._(socket, messages);
    final hello = jsonDecode(await s.next() as String) as Map;
    expect(hello['op'], 'hello');
    final nonce = (hello['d'] as Map)['dpop_nonce']! as String;
    final token = await device.auth.refresh.ensureFresh();
    final proof = await device.auth.proofs.forGateway(
      gatewayUrl: url,
      accessToken: token,
      nonce: nonce,
    );
    socket.add(
      jsonEncode({
        'op': 'identify',
        'd': {
          'protocol_version': 1,
          'access_token': token,
          'dpop': proof,
          'device_id': device.auth.session!.deviceId.value,
        },
      }),
    );
    return s;
  }

  Future<Object?> next() async {
    if (!await messages.moveNext()) return null;
    return messages.current;
  }

  /// Waits until the socket is closed and returns its code and reason.
  Future<(int?, String?)> closed() async {
    while (await messages.moveNext()) {}
    return (socket.closeCode, socket.closeReason);
  }
}

void main() {
  late FakeWorldHarness h;
  late _EchoHost host;
  setUp(() async {
    h = await FakeWorldHarness.start();
    host = _EchoHost();
    h.server.world.terminal.hostLink = host;
  });
  tearDown(() async {
    expect(h.server.violations, isEmpty);
    await h.stop();
  });

  FakeHost buildHost() => h.server.world.addTerminalHost(
    name: 'Build host',
    noisePublicKey: 'Zm9vYmFyZm9vYmFyZm9vYmFyZm9vYmFyZm9vYmFyMTI',
    sessionUsers: {h.seed.alex.id: 'term-alex', h.seed.sam.id: 'term-sam'},
    workdirs: {
      'term-alex': ['Project A', 'Project B'],
      'term-sam': ['Sam only'],
    },
    grantees: [h.seed.alex.id],
  );

  TerminalSessionOpen opening(String profile, {int? workdir}) =>
      TerminalSessionOpen(
        (b) => b
          ..profile = profile
          ..workdirIndex = workdir
          ..cols = 80
          ..rows = 24
          ..clientEphemeralKeyHash = 'A' * 43,
      );

  Future<ApiProblem?> problemOf(Future<Object?> call) async {
    try {
      await call;
      return null;
    } on DioException catch (e) {
      return ApiProblem.of(e);
    }
  }

  List<String> sessionEvents() => [
    for (final e in h.server.world.log)
      if (e.type == 'terminal_session.changed')
        '${(e.envelope['payload']! as Map)['state']}'
            '${(e.envelope['payload']! as Map)['close_reason'] == null ? '' : ' ${(e.envelope['payload']! as Map)['close_reason']}'}',
  ];

  test(
    'opening asks a fresh step-up each time and grants the stream',
    () async {
      final alex = h.device();
      await signedIn(h, alex);
      final h1 = buildHost();
      final api = alex.api.getTerminalApi();
      final grant = (await api.openTerminalSession(
        hostId: h1.id,
        terminalSessionOpen: opening('shell'),
      )).data!;
      expect(
        grant.streamPath,
        '/api/v1/terminal/sessions/${grant.session.id}/stream',
      );
      expect(grant.noisePublicKey, h1.noisePublicKey);
      await api.openTerminalSession(
        hostId: h1.id,
        terminalSessionOpen: opening('shell'),
      );
      final steps = [
        for (final r in h.server.requests)
          if (r.operationId != 'loginWithPassword')
            '${r.operationId} ${r.status}',
      ];
      expect(steps, [
        'openTerminalSession 401',
        'createStepUpChallenge 200',
        'completeStepUpWithDeviceKey 200',
        'openTerminalSession 201',
        'openTerminalSession 401',
        'createStepUpChallenge 200',
        'completeStepUpWithDeviceKey 200',
        'openTerminalSession 201',
      ]);
      expect(sessionEvents(), ['opening', 'opening']);
    },
  );

  test('claude takes only an index into the own working directories', () async {
    final alex = h.device();
    await signedIn(h, alex);
    final h1 = buildHost();
    final api = alex.api.getTerminalApi();
    for (final (profile, index) in [
      ('claude', null),
      ('claude', 2),
      ('shell', 0),
      ('nothing', null),
    ]) {
      final p = await problemOf(
        api.openTerminalSession(
          hostId: h1.id,
          terminalSessionOpen: opening(profile, workdir: index),
        ),
      );
      expect(p?.code, 'terminal_denied', reason: '$profile $index');
      expect(p?.reason, 'profile_unknown');
    }
    final grant = (await api.openTerminalSession(
      hostId: h1.id,
      terminalSessionOpen: opening('claude', workdir: 1),
    )).data!;
    expect(grant.session.profile, 'claude');
  });

  test(
    'without an own session user the opening is denied with no_session_user',
    () async {
      final sam = h.device(login: WorldSeed.samLogin, keySeed: 2);
      await signedIn(h, sam);
      final h1 = h.server.world.addTerminalHost(
        name: 'Linux VM',
        noisePublicKey: 'Zm9vYmFyZm9vYmFyZm9vYmFyZm9vYmFyZm9vYmFyMTI',
        grantees: [h.seed.sam.id],
      );
      final p = await problemOf(
        sam.api.getTerminalApi().openTerminalSession(
          hostId: h1.id,
          terminalSessionOpen: opening('shell'),
        ),
      );
      expect(p?.code, 'terminal_denied');
      expect(p?.reason, 'no_session_user');
    },
  );

  test('the stream relays to the host, detaches, and another device closes it with 4103', () async {
    final alex = h.device();
    await signedIn(h, alex);
    final h1 = buildHost();
    final grant = (await alex.api.getTerminalApi().openTerminalSession(
      hostId: h1.id,
      terminalSessionOpen: opening('shell'),
    )).data!;
    final first = await _Stream.open(h.server, alex, grant.streamPath);
    first.socket.add(Uint8List.fromList([1, 2, 3]));
    expect(await first.next(), [...utf8.encode('echo:'), 1, 2, 3]);
    expect(host.pipes.single.sessionToken, grant.sessionToken);
    expect(host.pipes.single.hostId, h1.id);

    await first.socket.close();
    await host.pipes.single.done;

    final laptop = h.device(keySeed: 3);
    await signedIn(h, laptop);
    final again = (await laptop.api.getTerminalApi().attachTerminalSession(
      sessionId: grant.session.id,
      terminalSessionAttach: TerminalSessionAttach(
        (b) => b
          ..cols = 80
          ..rows = 24
          ..clientEphemeralKeyHash = '${'B' * 42}A',
      ),
    )).data!;
    expect(again.session.deviceId, laptop.auth.session!.deviceId.value);
    final second = await _Stream.open(h.server, laptop, again.streamPath);
    second.socket.add(Uint8List.fromList([9]));
    expect(await second.next(), [...utf8.encode('echo:'), 9]);

    // Alex's first device attaches again: the laptop's stream ends with 4103.
    final back = (await alex.api.getTerminalApi().attachTerminalSession(
      sessionId: grant.session.id,
      terminalSessionAttach: TerminalSessionAttach(
        (b) => b
          ..cols = 80
          ..rows = 24
          ..clientEphemeralKeyHash = '${'C' * 42}A',
      ),
    )).data!;
    expect(await second.closed(), (4103, ''));
    expect(back.session.deviceId, alex.auth.session!.deviceId.value);
    expect(
      sessionEvents(),
      containsAllInOrder(['opening', 'active', 'detached']),
    );
  });

  test(
    'a revoked grant ends the session with 4100 and access_revoked',
    () async {
      final alex = h.device();
      await signedIn(h, alex);
      final h1 = buildHost();
      final grant = (await alex.api.getTerminalApi().openTerminalSession(
        hostId: h1.id,
        terminalSessionOpen: opening('shell'),
      )).data!;
      final stream = await _Stream.open(h.server, alex, grant.streamPath);
      stream.socket.add(Uint8List.fromList([1]));
      await stream.next();
      final grantId = h.server.world.terminalGrantOf(h.seed.alex.id, h1.id)!;
      await alex.api.getAccessApi().revokeGrant(
        grantId: grantId,
        reasonBody: ReasonBody((b) => b.reason = 'Leaves the project'),
      );
      expect(await stream.closed(), (4100, 'access_revoked'));
      expect(sessionEvents().last, 'closed access_revoked');
      final removed = h.server.world.log.lastWhere(
        (e) => e.type == 'access.changed',
      );
      expect((removed.envelope['payload']! as Map)['removed'], [
        {'kind': 'host', 'id': h1.id},
      ]);
      // The session is closed for good.
      final p = await problemOf(
        alex.api.getTerminalApi().attachTerminalSession(
          sessionId: grant.session.id,
          terminalSessionAttach: TerminalSessionAttach(
            (b) => b
              ..cols = 80
              ..rows = 24
              ..clientEphemeralKeyHash = '${'D' * 42}A',
          ),
        ),
      );
      expect(p?.code, 'invalid_state_transition');
    },
  );

  test(
    'identify of another device or a text frame after it closes the stream',
    () async {
      final alex = h.device();
      await signedIn(h, alex);
      final sam = h.device(login: WorldSeed.samLogin, keySeed: 2);
      await signedIn(h, sam);
      final h1 = buildHost();
      final grant = (await alex.api.getTerminalApi().openTerminalSession(
        hostId: h1.id,
        terminalSessionOpen: opening('shell'),
      )).data!;
      final foreign = await _Stream.open(h.server, sam, grant.streamPath);
      expect((await foreign.closed()).$1, 4004);
      final own = await _Stream.open(h.server, alex, grant.streamPath);
      own.socket.add('{"op":"identify"}');
      expect((await own.closed()).$1, 4002);
    },
  );
}
