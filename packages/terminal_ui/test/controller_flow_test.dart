// SPDX-License-Identifier: Apache-2.0

// The flow of 15 23.2 against the fake host of testing.dart (brief M4-35 Tests,
// controller_flow_test): order of the steps, input only after `start`, a mismatch of the
// returned key, `exit`, detach and reattach with the replay, the stream close codes, the
// coalesced `resize`, and the step-up of attach.

import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:api_client/api_client.dart' show ApiProblem;
import 'package:bothy_noise/bothy_noise.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';
import 'package:terminal_ui/testing.dart';

import 'support/fakes.dart';

void main() {
  test('opening runs getHost, the dialog, the step-up, open, identify, the handshake '
      'and start in this order, then pins the first key', () async {
    final h = await Harness.create();
    await h.open();
    await h.connected();
    await h.frames(1);
    expect(h.log, ['getHost', 'stepUp', 'open']);
    expect(h.fakePrompts.asked, ['open']);
    expect(h.stepUps, ['purpose open']);
    expect(h.host.identifies.single['protocol_version'], 1);
    expect(h.host.rejectedTokens, isEmpty);
    expect(h.host.frames.first, const StartFrame(80, 24));
    final open = h.api.opens.single;
    expect(open.profile, 'shell');
    expect(open.cols, 80);
    expect(open.rows, 24);
    expect(open.pinChanged, isNull);
    expect(open.clientEphemeralKeyHash, hasLength(43));
    expect(h.controller.state.sessionId, sessionIdText);
    expect(h.controller.state.openedAt, DateTime.utc(2026, 10, 20, 9, 30));
    final pin = h.pins.writes.single;
    expect(pin.noisePublicKey, h.host.noisePublicKey);
    expect(pin.keyGeneration, 1);
    expect(pin.fingerprint, newFingerprint);
    expect(pin.firstSeen, h.clock.current);
  });

  test(
    'input before start is not sent; after start it goes as data frames',
    () async {
      final h = await Harness.create();
      h.host.handshake = FakeHostHandshake.silent;
      await h.open();
      await h.settle();
      expect(h.controller.state.phase, TerminalPhase.connecting);
      h.controller.input(utf8.encode('early'));
      h.controller.screen.terminal.textInput('typed');
      await h.settle();
      expect(h.host.frames, isEmpty);
      expect(h.controller.state.acceptsInput, isFalse);

      final g = await Harness.create();
      await g.open();
      await g.connected();
      g.controller.input(utf8.encode('ls\r'));
      g.controller.screen.terminal.textInput('x');
      await g.frames(3);
      expect(g.host.frames.first, isA<StartFrame>());
      expect(utf8.decode(g.host.input), 'ls\rx');
    },
  );

  test('a returned key or key generation other than the checked one closes the session', () async {
    final other = await FakeHostKey.other();
    for (final variant in ['key', 'generation']) {
      final h = await Harness.create();
      if (variant == 'key') {
        h.api.grantKey = other;
      } else {
        h.api.grantGeneration = 2;
      }
      await h.open();
      await h.settle();
      expect(h.controller.state.phase, TerminalPhase.blocked, reason: variant);
      expect(h.controller.state.blockCause, BlockCause.keyMismatch);
      expect(h.api.closes, [sessionIdText]);
      expect(h.transport.connected, isEmpty);
      expect(h.pins.writes, isEmpty);
    }
  });

  test('exit shows the code and keeps the tab and the screen', () async {
    final h = await Harness.create();
    await h.open();
    await h.connected();
    await h.host.sendData(utf8.encode('done\r\n'));
    await h.host.sendExit(3);
    await h.until((s) => s.phase == TerminalPhase.exited);
    expect(h.controller.state.exitCode, 3);
    expect(h.controller.state.sessionId, sessionIdText);
    expect(h.controller.screen.visibleText(), contains('done'));
  });

  test('the close after exit keeps the exit code on the tab (the real server closes with '
      '4100 process_exited right after it, brief M4-38)', () async {
    final h = await Harness.create();
    await h.open();
    await h.connected();
    await h.host.sendExit(3);
    await h.until((s) => s.phase == TerminalPhase.exited);
    await h.host.close(4100, 'process_exited');
    await h.settle();
    expect(h.controller.state.phase, TerminalPhase.exited);
    expect(h.controller.state.exitCode, 3);
    expect(h.controller.state.closeReason, 'process_exited');
  });

  test(
    'a close with process_exited without an exit frame shows the exit code the server '
    'stored (the frame can be lost at the end of the session, brief M4-38)',
    () async {
      final h = await Harness.create();
      h.api.sessionExitCode = 3;
      await h.open();
      await h.connected();
      await h.host.close(4100, 'process_exited');
      await h.until((s) => s.phase == TerminalPhase.exited);
      expect(h.controller.state.exitCode, 3);
      expect(h.controller.state.closeReason, 'process_exited');
      expect(h.log.last, 'getSession');
    },
  );

  test('a failed read of the session leaves the tab ended', () async {
    final h = await Harness.create();
    h.api
      ..sessionExitCode = 3
      ..sessionError = const ApiProblem(status: 503, code: 'unavailable');
    await h.open();
    await h.connected();
    await h.host.close(4100, 'process_exited');
    await h.settle();
    expect(h.log.last, 'getSession');
    expect(h.controller.state.phase, TerminalPhase.ended);
    expect(h.controller.state.exitCode, isNull);
  });

  test('without a stored exit code the tab ends with process_exited', () async {
    final h = await Harness.create();
    await h.open();
    await h.connected();
    await h.host.close(4100, 'process_exited');
    await h.settle();
    expect(h.controller.state.phase, TerminalPhase.ended);
    expect(h.controller.state.closeReason, 'process_exited');
  });

  test('detach keeps the session; attach uses a new key and the replay replaces the screen', () async {
    final h = await Harness.create();
    await h.open();
    await h.connected();
    await h.host.sendData(utf8.encode('old screen\r\n'));
    await h.settle();
    expect(h.controller.screen.visibleText(), contains('old screen'));
    await h.controller.detach();
    expect(h.controller.state.phase, TerminalPhase.detached);
    expect(h.controller.state.detachCause, DetachCause.person);
    expect(h.api.closes, isEmpty);

    await h.controller.attach();
    await h.connected();
    expect(h.log, ['getHost', 'stepUp', 'open', 'getHost', 'attach']);
    expect(h.stepUps, hasLength(1), reason: 'no step-up without 401');
    expect(
      h.api.attaches.single.clientEphemeralKeyHash,
      isNot(h.api.opens.single.clientEphemeralKeyHash),
    );
    expect(h.host.connections, 2);
    await h.host.sendReplay(utf8.encode('replayed\r\n'));
    await h.until((s) => !s.replaying);
    await h.settle();
    final text = h.controller.screen.visibleText();
    expect(text, contains('replayed'));
    expect(text, isNot(contains('old screen')));
  });

  test(
    'during the replay the screen keeps the old content until replay_end',
    () async {
      final h = await Harness.create();
      await h.open();
      await h.connected();
      await h.host.sendData(utf8.encode('before'));
      await h.settle();
      await h.host.sendRawEncrypted(encodeFrame(const ReplayBeginFrame()));
      await h.host.sendData(utf8.encode('partial'));
      await h.until((s) => s.replaying);
      await h.settle();
      expect(h.controller.screen.visibleText(), 'before');
      expect(h.controller.state.acceptsInput, isFalse);
      await h.host.sendRawEncrypted(encodeFrame(const ReplayEndFrame()));
      await h.until((s) => !s.replaying);
      expect(h.controller.screen.visibleText(), 'partial');
    },
  );

  final reactions = <(int, String?, TerminalPhase, Object?)>[
    (4100, 'process_exited', TerminalPhase.ended, null),
    (4100, 'access_revoked', TerminalPhase.blocked, BlockCause.revoked),
    (4101, null, TerminalPhase.detached, DetachCause.tokenStale),
    (4102, null, TerminalPhase.detached, DetachCause.hostUnavailable),
    (4103, null, TerminalPhase.detached, DetachCause.replaced),
    (4104, null, TerminalPhase.blocked, BlockCause.handshakeFailed),
    (1000, null, TerminalPhase.ended, null),
    (1001, null, TerminalPhase.detached, DetachCause.serverRestart),
    (4013, null, TerminalPhase.detached, DetachCause.serverRestart),
    (4014, null, TerminalPhase.detached, DetachCause.slowConsumer),
  ];
  for (final (code, reason, phase, cause) in reactions) {
    test(
      'close code $code ${reason ?? ''} shows ${phase.name} and never retries by itself',
      () async {
        final h = await Harness.create();
        await h.open();
        await h.connected();
        await h.host.close(code, reason);
        await h.until((s) => s.phase != TerminalPhase.connected);
        await h.settle();
        final s = h.controller.state;
        expect(s.phase, phase);
        expect(s.detachCause ?? s.blockCause, cause);
        if (code == 4100) expect(s.closeReason, reason);
        expect(h.transport.connected, hasLength(1));
        expect(h.log.where((e) => e == 'attach'), isEmpty);
        expect(
          closeReactionFor(code, reason).offersReattach,
          phase == TerminalPhase.detached,
        );
      },
    );
  }

  test('after 4101 attach asks for a step-up before the request', () async {
    final h = await Harness.create();
    await h.open();
    await h.connected();
    await h.host.close(4101);
    await h.until((s) => s.phase == TerminalPhase.detached);
    await h.controller.attach();
    await h.connected();
    expect(h.log, ['getHost', 'stepUp', 'open', 'getHost', 'stepUp', 'attach']);
    expect(h.stepUps.last, 'purpose attach');
  });

  test(
    'attach follows 401 step_up_required with a step-up and one retry',
    () async {
      final h = await Harness.create();
      await h.open();
      await h.connected();
      await h.host.close(4102);
      await h.until((s) => s.phase == TerminalPhase.detached);
      h.api.attachErrors.add(stepUpRequired);
      await h.controller.attach();
      await h.connected();
      expect(h.log.sublist(3), ['getHost', 'attach', 'stepUp', 'attach']);
    },
  );

  test('a refused attach stays detached with the problem code', () async {
    final h = await Harness.create();
    await h.open();
    await h.connected();
    await h.host.close(4102);
    await h.until((s) => s.phase == TerminalPhase.detached);
    h.api.attachErrors.add(
      const ApiProblem(status: 409, code: 'host_unavailable'),
    );
    await h.controller.attach();
    expect(h.controller.state.phase, TerminalPhase.detached);
    expect(h.controller.state.problemCode, 'host_unavailable');
  });

  test(
    'size changes within one frame tick send one resize with the last size',
    () async {
      final h = await Harness.create();
      await h.open();
      await h.connected();
      h.controller
        ..resize(100, 30)
        ..resize(120, 40)
        ..resize(132, 43);
      expect(h.ticks, hasLength(1));
      h.tick();
      await h.frames(2);
      await h.settle();
      expect(h.host.frames.whereType<ResizeFrame>(), [
        const ResizeFrame(132, 43),
      ]);
    },
  );

  test('a tampered message 2 is a hard warning without retry', () async {
    final h = await Harness.create();
    h.host.handshake = FakeHostHandshake.corruptMessage2;
    await h.open();
    await h.until((s) => s.phase == TerminalPhase.blocked);
    expect(h.controller.state.blockCause, BlockCause.handshakeFailed);
    expect(h.host.frames, isEmpty);
    expect(h.pins.writes, isEmpty);
    expect(h.transport.connected, hasLength(1));
  });

  test('an unknown frame of the host is a hard warning', () async {
    final h = await Harness.create();
    await h.open();
    await h.connected();
    await h.host.sendRawEncrypted(Uint8List.fromList([0x09, 1]));
    await h.until((s) => s.phase == TerminalPhase.blocked);
    expect(h.controller.state.blockCause, BlockCause.handshakeFailed);
  });

  test('a refused opening shows the problem code; a cancelled step-up opens nothing', () async {
    final h = await Harness.create();
    h.api.openError = const ApiProblem(status: 403, code: 'terminal_denied');
    await h.open();
    expect(h.controller.state.phase, TerminalPhase.failed);
    expect(h.controller.state.problemCode, 'terminal_denied');

    final g = await Harness.create(cancelStepUp: true);
    await g.open();
    expect(g.controller.state.phase, TerminalPhase.idle);
    expect(g.api.opens, isEmpty);
  });

  test('end sends close and closes the session', () async {
    final h = await Harness.create();
    await h.open();
    await h.connected();
    await h.controller.end();
    await h.frames(2);
    expect(h.host.frames.last, const CloseFrame());
    expect(h.api.closes, [sessionIdText]);
    expect(h.controller.state.phase, TerminalPhase.ended);
  });

  test('a closed session from the host events ends the tab', () async {
    final events = FakeHostEvents();
    final h = await Harness.create(events: events);
    await h.open();
    await h.connected();
    events.changes.add(
      const TerminalSessionUpdated(
        sessionIdText,
        closed: true,
        closeReason: 'access_revoked',
      ),
    );
    await h.until((s) => s.phase == TerminalPhase.blocked);
    expect(h.controller.state.blockCause, BlockCause.revoked);
    expect(h.controller.state.closeReason, 'access_revoked');
  });

  test('process_exited from the host events before the stream close shows the stored exit '
      'code (the order on the real stack, brief M4-38)', () async {
    final events = FakeHostEvents();
    final h = await Harness.create(events: events);
    h.api.sessionExitCode = 3;
    await h.open();
    await h.connected();
    events.changes.add(
      const TerminalSessionUpdated(
        sessionIdText,
        closed: true,
        closeReason: 'process_exited',
      ),
    );
    await h.until((s) => s.phase == TerminalPhase.exited);
    expect(h.controller.state.exitCode, 3);
    expect(h.controller.state.closeReason, 'process_exited');
  });

  test(
    'an exited tab stays exited when the host events report the close',
    () async {
      final events = FakeHostEvents();
      final h = await Harness.create(events: events);
      await h.open();
      await h.connected();
      await h.host.sendExit(5);
      await h.until((s) => s.phase == TerminalPhase.exited);
      events.changes.add(
        const TerminalSessionUpdated(
          sessionIdText,
          closed: true,
          closeReason: 'process_exited',
        ),
      );
      await h.settle();
      expect(h.controller.state.phase, TerminalPhase.exited);
      expect(h.controller.state.exitCode, 5);
    },
  );

  test('only https and http links open', () async {
    final h = await Harness.create();
    expect(await h.controller.openLink('https://example.com/a'), isTrue);
    expect(await h.controller.openLink('http://example.com/b'), isTrue);
    expect(await h.controller.openLink('file:///etc/passwd'), isFalse);
    expect(await h.controller.openLink('javascript:alert(1)'), isFalse);
    expect(h.links.opened.map((u) => u.toString()), [
      'https://example.com/a',
      'http://example.com/b',
    ]);
  });

  test('dispose with an open stream notifies nothing afterwards', () async {
    final h = await Harness.create();
    await h.open();
    await h.connected();
    var notified = 0;
    h.controller.addListener(() => notified++);
    h.controller.dispose();
    await h.host.sendData(utf8.encode('late'));
    await h.host.close(4100, 'process_exited');
    await h.settle();
    expect(notified, 0);
  });

  test('dispose during the step-up opens nothing', () async {
    final h = await Harness.create();
    h.stepUpGate = Completer<void>();
    final opening = h.open();
    await h.settle();
    expect(h.log, ['getHost', 'stepUp']);
    h.controller.dispose();
    h.stepUpGate!.complete();
    await opening;
    await h.settle();
    expect(h.api.opens, isEmpty);
    expect(h.transport.connected, isEmpty);
  });

  test('dispose while the server opens closes the new session and connects nothing', () async {
    final h = await Harness.create();
    h.api.openGate = Completer<void>();
    final opening = h.open();
    await h.settle();
    expect(h.log, ['getHost', 'stepUp', 'open']);
    h.controller.dispose();
    h.api.openGate!.complete();
    await opening;
    await h.settle();
    expect(h.api.closes, [sessionIdText]);
    expect(h.transport.connected, isEmpty);
    expect(h.host.frames, isEmpty);
  });

  test('End during a pending attach wins: the tab stays ended and nothing connects', () async {
    final h = await Harness.create();
    await h.open();
    await h.connected();
    await h.controller.detach();
    h.api.attachGate = Completer<void>();
    final attaching = h.controller.attach();
    await h.settle();
    expect(h.log.last, 'attach');
    await h.controller.end();
    h.api.attachGate!.complete();
    await attaching;
    await h.settle();
    expect(h.controller.state.phase, TerminalPhase.ended);
    expect(h.host.connections, 1);
  });

  test('a revocation during a pending attach stays blocked', () async {
    final events = FakeHostEvents();
    final h = await Harness.create(events: events);
    await h.open();
    await h.connected();
    await h.controller.detach();
    h.api.attachGate = Completer<void>();
    h.api.attachErrors.add(
      const ApiProblem(status: 409, code: 'host_unavailable'),
    );
    final attaching = h.controller.attach();
    await h.settle();
    events.changes.add(
      const TerminalSessionUpdated(
        sessionIdText,
        closed: true,
        closeReason: 'access_revoked',
      ),
    );
    await h.settle();
    h.api.attachGate!.complete();
    await attaching;
    expect(h.controller.state.phase, TerminalPhase.blocked);
    expect(h.controller.state.blockCause, BlockCause.revoked);
  });

  test(
    'a resize tick due after detach sends nothing and blocks nothing',
    () async {
      final h = await Harness.create();
      await h.open();
      await h.connected();
      h.controller.resize(100, 30);
      await h.controller.detach();
      h.tick();
      await h.settle();
      expect(h.host.frames.whereType<ResizeFrame>(), isEmpty);
      expect(h.controller.state.phase, TerminalPhase.detached);
      expect(h.controller.state.detachCause, DetachCause.person);
    },
  );

  test('a stale tick of the old stream leaves a new attach alone', () async {
    final h = await Harness.create();
    await h.open();
    await h.connected();
    h.controller.resize(100, 30);
    await h.controller.detach();
    await h.controller.attach();
    await h.connected();
    h.tick();
    await h.settle();
    expect(h.controller.state.phase, TerminalPhase.connected);
    expect(h.host.frames.whereType<ResizeFrame>(), isEmpty);
  });
}
