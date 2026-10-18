// SPDX-License-Identifier: Apache-2.0

/// The fake server of the client integration tests (brief M4-17 Design note 1): REST under
/// `/api/v1/` and the gateway at `/api/v1/gateway` on `127.0.0.1` with a free port.
library;

import 'dart:async';
import 'dart:io';

import 'action_catalog.dart';
import 'dpop_verifier.dart';
import 'faults.dart';
import 'gateway_fake.dart';
import 'rest/admin_handlers.dart';
import 'rest/auth_handlers.dart';
import 'rest/message_handlers.dart';
import 'rest/personal_handlers.dart';
import 'rest/run_handlers.dart';
import 'rest/self_handlers.dart';
import 'rest/structure_handlers.dart';
import 'rest/sync_handlers.dart';
import 'rest/thread_view_handlers.dart';
import 'rest_fake.dart';
import 'schema_validator.dart';
import 'scripted_agent.dart';
import 'world.dart';

/// The settings of a [FakeServer]; every value the fake reports to clients has the default
/// of 04 18 or 08 18 and can be shortened by tests.
final class FakeServerOptions {
  /// Creates the settings.
  const FakeServerOptions({
    this.timings = const GatewayTimings(),
    this.accessTokenTtl = const Duration(minutes: 10),
    this.now,
  });

  /// The values the gateway reports and uses.
  final GatewayTimings timings;

  /// `auth.access_token_ttl`.
  final Duration accessTokenTtl;

  /// The time source of the world (UTC); default the system time.
  final DateTime Function()? now;
}

/// A fake of the server for the integration tests of the client (15 20 row "Integration").
///
/// It keeps a [FakeWorld] in memory, serves the REST subset of brief M4-17 Design note 3 and
/// the gateway, checks DPoP proofs, validates every request, response and frame against the
/// API documents ([violations]) and injects the faults of its [faults] plan. One fake per
/// test; [stop] closes every socket. There is no global state, so parallel tests start their
/// own fakes.
///
/// Example:
/// ```dart
/// final server = await FakeServer.start();
/// final world = WorldSeed.standard(server.world);
/// // configure the client with server.baseUri (plain http, test builds only)
/// server.faults.dropResponseAfterCommit('createMessage');
/// // ...
/// expect(server.violations, isEmpty);
/// await server.stop();
/// ```
final class FakeServer {
  FakeServer._(
    this._http,
    this.world,
    this.rest,
    this.gateway,
    this.faults,
    this.verifier,
    this._contract,
    this.actions,
  );

  final HttpServer _http;

  /// The world.
  final FakeWorld world;

  /// The REST side; [handle] adds operations.
  final RestFake rest;

  /// The gateway side.
  final GatewayFake gateway;

  /// The fault plan.
  final FaultPlan faults;

  /// The DPoP check; [DpopVerifier.rotateNonce] makes the current nonce the previous one.
  final DpopVerifier verifier;

  final ContractSchemas _contract;

  /// The action catalogue of `docs/spec/actions.yaml` (brief M4-26 Design note 3).
  final ActionCatalog actions;

  /// The authorization action of [operationId] (`x-authz-action`), or null when it names none.
  String? actionOf(String operationId) =>
      _contract.operation(operationId).authzAction;

  /// The `step_up` value (`true`, `false`, `fresh`, `conditional`) of the action of
  /// [operationId] in `docs/spec/actions.yaml`; `false` for an operation without an action.
  String stepUpOf(String operationId) {
    final action = actionOf(operationId);
    return action == null ? 'false' : actions.stepUp(action);
  }

  final List<ScriptedAgent> _agents = [];
  final Set<Future<void>> _serving = {};
  bool _stopped = false;

  /// Starts a fake on `127.0.0.1` with a free port.
  ///
  /// Throws a [StateError] when `docs/api/openapi.yaml` is not found above the working
  /// directory (the contract checks need it), and the error of `HttpServer.bind`.
  static Future<FakeServer> start({
    FakeServerOptions options = const FakeServerOptions(),
  }) async {
    final contract = ContractSchemas.load();
    final world = FakeWorld(now: options.now);
    final verifier = DpopVerifier(now: world.now);
    final http = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    final base = Uri.parse('http://127.0.0.1:${http.port}');
    final gateway = GatewayFake(
      world: world,
      contract: contract,
      verifier: verifier,
      timings: options.timings,
      baseUri: () => base,
    );
    final faults = FaultPlan(gateway);
    final rest = RestFake(
      world: world,
      contract: contract,
      verifier: verifier,
      faults: faults,
    );
    registerAuthHandlers(rest, accessTokenTtl: options.accessTokenTtl);
    registerSyncHandlers(rest);
    registerStructureHandlers(rest);
    registerMessageHandlers(rest);
    registerPersonalHandlers(rest);
    registerRunHandlers(rest);
    registerSelfHandlers(rest);
    registerThreadViewHandlers(rest);
    final actions = ActionCatalog.load();
    registerAdminHandlers(rest, actions);
    final server = FakeServer._(
      http,
      world,
      rest,
      gateway,
      faults,
      verifier,
      contract,
      actions,
    );
    http.listen(server._onRequest);
    return server;
  }

  /// The address of the fake, `http://127.0.0.1:<port>` (plain HTTP: test builds only).
  Uri get baseUri => Uri.parse('http://127.0.0.1:${_http.port}');

  /// The gateway address, `ws://127.0.0.1:<port>/api/v1/gateway`.
  Uri get gatewayUri =>
      Uri.parse('ws://127.0.0.1:${_http.port}/api/v1/gateway');

  /// Every contract violation of a request, response or frame, with its JSON Pointer; a test
  /// expects this list to be empty.
  List<String> get violations => [...rest.violations, ...gateway.violations];

  /// Every REST request, in order.
  List<FakeRequestRecord> get requests => rest.requests;

  /// The requests of [operationId].
  List<FakeRequestRecord> requestsOf(String operationId) => [
    for (final r in rest.requests)
      if (r.operationId == operationId) r,
  ];

  /// Registers [handler] for [operationId] (the handler registry of Design note 3).
  void handle(String operationId, FakeHandler handler) =>
      rest.handle(operationId, handler);

  /// Adds and attaches a scripted agent for [agentId] that plays [script] on every mention.
  ScriptedAgent addAgent(String agentId, AgentScript script) {
    final agent = ScriptedAgent(world, agentId: agentId, script: script)
      ..attach();
    _agents.add(agent);
    return agent;
  }

  /// Ends the gateway connections of [deviceId] (all when null) without a close code, as a
  /// lost network does.
  void disconnect({String? deviceId}) => gateway.disconnect(deviceId: deviceId);

  void _onRequest(HttpRequest request) {
    if (_stopped) {
      unawaited(request.response.close());
      return;
    }
    final work = _serve(request);
    _serving.add(work);
    unawaited(work.whenComplete(() => _serving.remove(work)));
  }

  Future<void> _serve(HttpRequest request) async {
    try {
      if (request.uri.path == '/api/v1/gateway' &&
          WebSocketTransformer.isUpgradeRequest(request)) {
        gateway.accept(await WebSocketTransformer.upgrade(request));
        return;
      }
      await rest.serve(request, baseUri);
    } on Object {
      // A dropped response or a client that went away; the record is in [requests].
      try {
        await request.response.close();
      } on Object {
        // The socket is gone already.
      }
    }
  }

  /// Stops the agents, closes every socket and the listener.
  Future<void> stop() async {
    if (_stopped) return;
    _stopped = true;
    for (final a in _agents) {
      a.detach();
    }
    faults.cancelTimers();
    await gateway.close();
    await _http.close(force: true);
    await Future.wait(List.of(_serving));
  }
}
