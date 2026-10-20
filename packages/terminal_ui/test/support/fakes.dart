// SPDX-License-Identifier: Apache-2.0

// Fakes of the ports for the tests of terminal_ui: API, pins, step-up, clock, links and the
// prompts, plus a harness that wires them with FakeHost and FakeTerminalTransport.

import 'dart:async';

import 'package:api_client/api_client.dart'
    show
        ApiProblem,
        Host,
        HostProfile,
        HostProfileKindEnum,
        HostState,
        LlmCredentials,
        TerminalSession,
        TerminalSessionAttach,
        TerminalSessionGrant,
        TerminalSessionGrantNoiseProtocolEnum,
        TerminalSessionGrantPrologueVersionEnum,
        TerminalSessionOpen;
import 'package:api_client/api_client.dart' as api show TerminalSessionState;
import 'package:auth/auth.dart' show Clock, StepUpCancelled;
import 'package:built_collection/built_collection.dart';
import 'package:domain/domain.dart' show AppConfig, HostId;
import 'package:terminal_ui/terminal_ui.dart';
import 'package:terminal_ui/testing.dart';

const String hostIdText = '0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e0a';
const String sessionIdText = '0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e0f';
const String alexId = '0192f4a0-7b1c-7b22-9d4f-5a6b7c8d0001';
const String samId = '0192f4a0-7b1c-7b22-9d4f-5a6b7c8d0002';
const String oldFingerprint =
    '7KQ2-9XTA-M4PB-1C8D-0000-0000-0000-0000-0000-0000-0000-0000-0000';
const String newFingerprint =
    'Q0ZJ-55RW-2HNE-K7TS-0000-0000-0000-0000-0000-0000-0000-0000-0000';

/// A fixed clock.
final class FakeClock implements Clock {
  DateTime current = DateTime.utc(2026, 10, 20, 10);

  @override
  DateTime now() => current;

  @override
  Future<void> delay(Duration duration) async {}
}

/// A pin store in memory.
final class FakePins implements HostKeyPinStore {
  final Map<String, HostKeyPin> pins = {};
  final List<HostKeyPin> writes = [];

  @override
  Future<HostKeyPin?> read(String hostId) async => pins[hostId];

  @override
  Future<void> write(HostKeyPin pin) async {
    writes.add(pin);
    pins[pin.hostId] = pin;
  }
}

/// A link opener that records what it opened.
final class FakeLinks implements LinkOpener {
  final List<Uri> opened = [];

  @override
  Future<void> open(Uri target) async => opened.add(target);
}

/// Prompts with fixed answers that record every question.
final class FakePrompts implements TerminalPrompts {
  bool acceptKeyChange = true;
  bool acceptForeign = true;
  bool acceptOpen = true;
  final List<String> asked = [];
  PinChanged? lastChange;

  @override
  Future<bool> confirmKeyChange(Host host, PinChanged change) async {
    asked.add('key');
    lastChange = change;
    return acceptKeyChange;
  }

  @override
  Future<bool> confirmForeignHost(Host host) async {
    asked.add('foreign');
    return acceptForeign;
  }

  @override
  Future<OpenChoice?> confirmOpen(Host host, OpenChoice initial) async {
    asked.add('open');
    return acceptOpen ? initial : null;
  }

  @override
  String stepUpPurpose(Host host, StepUpPurpose purpose) =>
      'purpose ${purpose.name}';
}

/// The API of the server in memory, issuing grants for [host].
final class FakeApi implements TerminalServerApi {
  FakeApi(this.host, {required this.log});

  final FakeHost host;
  final List<String> log;
  String? assignedUserId;
  String fingerprint = newFingerprint;
  int keyGeneration = 1;
  String? keyOverride;

  /// The key the grant returns; null means the host's key.
  String? grantKey;
  int? grantGeneration;
  final List<TerminalSessionOpen> opens = [];
  final List<TerminalSessionAttach> attaches = [];
  final List<String> closes = [];

  /// Answers to attach in order before the grant: an error to throw, or null for the grant.
  final List<Object?> attachErrors = [];
  Object? openError;

  /// When set, openSession waits for it before it answers.
  Completer<void>? openGate;
  int tokens = 0;

  Host currentHost() => Host(
    (b) => b
      ..id = hostIdText
      ..name = 'Build host'
      ..state = HostState.active
      ..assignedUserId = assignedUserId
      ..llmCredentials = LlmCredentials.organization
      ..declaredServerHost = false
      ..noisePublicKey = keyOverride ?? host.noisePublicKey
      ..keyFingerprint = fingerprint
      ..keyGeneration = keyGeneration
      ..profiles = ListBuilder<HostProfile>([
        HostProfile(
          (p) => p
            ..name = 'shell'
            ..displayName = 'Shell'
            ..kind = HostProfileKindEnum.shell,
        ),
      ])
      ..connected = true
      ..createdAt = DateTime.utc(2026, 10)
      ..version = 1,
  );

  TerminalSessionGrant grant() {
    tokens++;
    final token = 'token-$tokens';
    host.expectToken(sessionIdText, token);
    return TerminalSessionGrant(
      (b) => b
        ..session.replace(
          TerminalSession(
            (s) => s
              ..id = sessionIdText
              ..hostId = hostIdText
              ..userId = alexId
              ..deviceId = '0192f4a0-7b1c-7b22-9d4f-5a6b7c8d9e01'
              ..profile = 'shell'
              ..state = api.TerminalSessionState.opening
              ..openedAt = DateTime.utc(2026, 10, 20, 9, 30)
              ..bytesIn = 0
              ..bytesOut = 0
              ..version = 1,
          ),
        )
        ..sessionToken = token
        ..sessionTokenExpiresAt = DateTime.utc(2026, 10, 20, 10, 1)
        ..streamPath = '/api/v1/terminal/sessions/$sessionIdText/stream'
        ..noisePublicKey = grantKey ?? keyOverride ?? host.noisePublicKey
        ..keyFingerprint = fingerprint
        ..keyGeneration = grantGeneration ?? keyGeneration
        ..noiseProtocol =
            TerminalSessionGrantNoiseProtocolEnum.noiseNK25519ChaChaPolySHA256
        ..prologueVersion =
            TerminalSessionGrantPrologueVersionEnum.bothyTerminalSlash1,
    );
  }

  @override
  Future<Host> getHost(String hostId) async {
    log.add('getHost');
    return currentHost();
  }

  @override
  Future<TerminalSessionGrant> openSession(
    String hostId,
    TerminalSessionOpen request,
  ) async {
    log.add('open');
    opens.add(request);
    await openGate?.future;
    final error = openError;
    if (error != null) throw error;
    return grant();
  }

  @override
  Future<TerminalSessionGrant> attachSession(
    String sessionId,
    TerminalSessionAttach request,
  ) async {
    log.add('attach');
    attaches.add(request);
    if (attachErrors.isNotEmpty) {
      final error = attachErrors.removeAt(0);
      if (error != null) throw error;
    }
    return grant();
  }

  @override
  Future<TerminalSession> closeSession(String sessionId) async {
    log.add('close');
    closes.add(sessionId);
    return grant().session;
  }
}

/// The step-up problem of the server.
const ApiProblem stepUpRequired = ApiProblem(
  status: 401,
  code: 'step_up_required',
);

/// Everything wired: host, transport, API, pins, prompts, step-up and the controller.
final class Harness {
  Harness._(this.host, this.transport, this.api, this.log);

  static Future<Harness> create({
    String selfUserId = alexId,
    bool cancelStepUp = false,
    FakePins? pins,
    TerminalPrompts? prompts,
    HostEvents? events,
    EphemeralKeyFactory? newEphemeralKey,
  }) async {
    final log = <String>[];
    final host = await FakeHost.create(hostId: hostIdText);
    final transport = FakeTerminalTransport(host);
    final api = FakeApi(host, log: log);
    final h = Harness._(host, transport, api, log);
    h.pins = pins ?? FakePins();
    h.prompts = prompts ?? FakePrompts();
    h.controller = TerminalController(
      api: api,
      transport: transport,
      pins: h.pins,
      stepUp: ({required purposeText}) async {
        log.add('stepUp');
        h.stepUps.add(purposeText);
        await h.stepUpGate?.future;
        if (cancelStepUp) throw const StepUpCancelled();
      },
      links: h.links,
      clock: h.clock,
      prompts: h.prompts,
      selfUserId: selfUserId,
      config: AppConfig.defaults(),
      events: events,
      scheduleTick: h.ticks.add,
      newEphemeralKey: newEphemeralKey,
    );
    return h;
  }

  final FakeHost host;
  final FakeTerminalTransport transport;
  final FakeApi api;
  late TerminalController controller;
  final List<String> log;
  late FakePins pins;
  late TerminalPrompts prompts;

  /// The prompts as [FakePrompts], when the test did not pass its own.
  FakePrompts get fakePrompts => prompts as FakePrompts;
  final FakeLinks links = FakeLinks();
  final FakeClock clock = FakeClock();
  final List<String> stepUps = [];

  /// When set, the step-up waits for it.
  Completer<void>? stepUpGate;
  final List<void Function()> ticks = [];

  /// Runs every scheduled frame tick.
  void tick() {
    final pending = List.of(ticks);
    ticks.clear();
    for (final t in pending) {
      t();
    }
  }

  /// Opens a shell of 80 by 24.
  Future<void> open() => controller.open(
    host: HostId.parse(hostIdText),
    profile: 'shell',
    cols: 80,
    rows: 24,
  );

  /// Waits until [test] holds for the controller state.
  Future<void> until(bool Function(TerminalTabState s) test) async {
    for (var i = 0; i < 2000; i++) {
      if (test(controller.state)) return;
      await Future<void>.delayed(Duration.zero);
    }
    throw StateError('state never reached: ${controller.state}');
  }

  /// Waits until the controller is connected.
  Future<void> connected() => until((s) => s.phase == TerminalPhase.connected);

  /// Waits until the host recorded [count] frames.
  Future<void> frames(int count) async {
    for (var i = 0; i < 2000; i++) {
      if (host.frames.length >= count) return;
      await Future<void>.delayed(Duration.zero);
    }
    throw StateError('host has ${host.frames.length} frames, want $count');
  }

  /// Lets pending microtasks and async crypto finish.
  Future<void> settle([int rounds = 200]) async {
    for (var i = 0; i < rounds; i++) {
      await Future<void>.delayed(Duration.zero);
    }
  }
}

/// A host event source the test drives.
final class FakeHostEvents implements HostEvents {
  final StreamController<HostChange> changes = StreamController.broadcast();

  @override
  Stream<HostChange> watch(String hostId) => changes.stream;
}

/// Another static host key, for key changes.
abstract final class FakeHostKey {
  /// A base64url X25519 key other than the default of [FakeHost].
  static Future<String> other() async => (await FakeHost.create(
    hostId: hostIdText,
    seed: List<int>.generate(32, (i) => 200 - i),
  )).noisePublicKey;
}
