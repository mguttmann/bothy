// SPDX-License-Identifier: Apache-2.0

/// The REST side of the fake server (brief M4-17 Design note 3): routing by the paths of
/// `docs/api/openapi.yaml`, token and DPoP checks, `Idempotency-Key`, faults and the contract
/// check of every request and response. The handlers live in `rest/`, one file per operation
/// group, and register through [RestFake.handle].
library;

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:crypto/crypto.dart';

import 'dpop_verifier.dart';
import 'faults.dart';
import 'schema_validator.dart';
import 'world.dart';

/// One REST request as a handler sees it, after routing, token and proof checks.
final class FakeRequest {
  /// Creates the request.
  FakeRequest({
    required this.operationId,
    required this.method,
    required this.uri,
    required this.pathParams,
    required this.headers,
    required this.body,
    required this.world,
    required this.baseUri,
    this.session,
    this.proofJkt,
  });

  /// The `operationId` of the route.
  final String operationId;

  /// The HTTP method in upper case.
  final String method;

  /// The request URI as received.
  final Uri uri;

  /// The path parameters by name (`thread_id`, ...), decoded.
  final Map<String, String> pathParams;

  /// The headers, names in lower case.
  final Map<String, String> headers;

  /// The decoded JSON body, or null.
  final Object? body;

  /// The world.
  final FakeWorld world;

  /// The address of the fake (`http://127.0.0.1:<port>`).
  final Uri baseUri;

  /// The session of the access token of a secured operation.
  final FakeSession? session;

  /// The thumbprint of the DPoP proof's key, when the request carried a valid proof.
  final String? proofJkt;

  /// The person of [session]; throws a [StateError] for an unsecured operation.
  FakePerson get person =>
      world.people[session?.userId] ?? (throw StateError('no session'));

  /// The body as a JSON object (empty without a body).
  JsonMap get json => body is JsonMap ? body! as JsonMap : const {};

  /// The query parameter [name], or null.
  String? query(String name) => uri.queryParameters[name];

  /// The path parameter [name]; throws when the route has none.
  String param(String name) => pathParams[name]!;
}

/// The answer of a handler.
final class FakeResponse {
  /// A response with [status], a JSON [body] (none when null) and extra [headers].
  const FakeResponse(this.status, [this.body, this.headers = const {}]);

  /// `204 No Content`.
  const FakeResponse.noContent() : this(204);

  /// The HTTP status.
  final int status;

  /// The JSON body, or null.
  final Object? body;

  /// Extra headers.
  final Map<String, String> headers;
}

/// Handles one operation; throw [FakeProblem] for a problem answer.
typedef FakeHandler = FutureOr<FakeResponse> Function(FakeRequest request);

/// One request as the fake answered it, for assertions.
final class FakeRequestRecord {
  /// Creates the record.
  const FakeRequestRecord({
    required this.operationId,
    required this.method,
    required this.path,
    required this.status,
    required this.body,
    required this.headers,
  });

  /// The operation, or `unknown`.
  final String operationId;

  /// The HTTP method.
  final String method;

  /// The path below the origin.
  final String path;

  /// The status sent; `0` when the response was dropped.
  final int status;

  /// The decoded request body.
  final Object? body;

  /// The request headers, names in lower case.
  final Map<String, String> headers;
}

class _Route {
  _Route(this.op, this.pattern, this.names);

  final ContractOperation op;
  final RegExp pattern;
  final List<String> names;
}

/// Operations a waiting device may use (08 5.2): everything else ends with
/// `403 device_pending`.
const Set<String> _pendingAllowed = {'getMe', 'getDevice', 'logout'};

/// Serves `/api/v1/` on the fake server.
///
/// Every secured operation needs `Authorization: DPoP <token>` of a known session and a valid
/// proof bound to its key (08 7.3); proofs without the current or the previous nonce get
/// `401 use_dpop_nonce` with a `DPoP-Nonce` header. A waiting device gets
/// `403 device_pending` outside the allowed operations. Writes with `Idempotency-Key` repeat
/// their stored answer (04 5.2). Faults of the [FaultPlan] apply after the checks.
final class RestFake {
  /// Creates the REST side.
  RestFake({
    required this.world,
    required this.contract,
    required this.verifier,
    required this.faults,
  }) {
    for (final op in contract.operations) {
      final names = <String>[];
      final source = op.path.replaceAllMapped(RegExp(r'\{([a-z_]+)\}'), (m) {
        names.add(m[1]!);
        return '([^/]+)';
      });
      _routes.add(_Route(op, RegExp('^/api/v1$source\$'), names));
    }
    // A literal segment wins over a parameter (`/me/devices/pairings` before
    // `/me/devices/{device_id}`).
    _routes.sort((a, b) => a.names.length.compareTo(b.names.length));
  }

  /// The world.
  final FakeWorld world;

  /// The API documents.
  final ContractSchemas contract;

  /// The DPoP check.
  final DpopVerifier verifier;

  /// The faults.
  final FaultPlan faults;

  final List<_Route> _routes = [];
  final Map<String, FakeHandler> _handlers = {};

  /// Every request answered, in order.
  final List<FakeRequestRecord> requests = [];

  /// Contract violations of requests and responses, as text with the pointer.
  final List<String> violations = [];

  /// Registers [handler] for [operationId], replacing an earlier one (Design note 3: later
  /// tickets add operations this way). Throws an [ArgumentError] for an operation that is not
  /// in `openapi.yaml`.
  void handle(String operationId, FakeHandler handler) {
    contract.operation(operationId);
    _handlers[operationId] = handler;
  }

  /// The operations with a handler.
  Set<String> get handled => Set.unmodifiable(_handlers.keys);

  /// Serves [request]; [baseUri] is the address of the fake.
  Future<void> serve(HttpRequest request, Uri baseUri) async {
    final method = request.method.toUpperCase();
    final path = request.uri.path;
    final headers = <String, String>{};
    request.headers.forEach((name, values) => headers[name.toLowerCase()] = values.join(','));
    final raw = await utf8.decoder.bind(request).join();
    Object? body;
    if (raw.isNotEmpty) {
      try {
        body = jsonDecode(raw);
      } on FormatException {
        await _send(request, 'unknown', method, path, null, headers, _problem(400, 'malformed_request'));
        return;
      }
    }
    _Route? route;
    final params = <String, String>{};
    for (final r in _routes) {
      if (r.op.method != method) continue;
      final m = r.pattern.firstMatch(path);
      if (m == null) continue;
      route = r;
      for (var i = 0; i < r.names.length; i++) {
        params[r.names[i]] = Uri.decodeComponent(m[i + 1]!);
      }
      break;
    }
    if (route == null) {
      await _send(request, 'unknown', method, path, body, headers, _problem(404, 'not_found'));
      return;
    }
    final op = route.op.operationId;
    final handler = _handlers[op];
    if (handler == null) {
      await _send(request, op, method, path, body, headers, _problem(501, 'internal_error', detail: 'not implemented by the fake'));
      return;
    }
    final htu = '${baseUri.origin}$path';
    FakeResponse response;
    FakeSession? session;
    String? jkt;
    try {
      (session, jkt) = _authenticate(route.op, method, htu, headers);
      final requestViolations = contract.request(op, body);
      if (requestViolations.isNotEmpty) {
        violations.add('request $op: ${requestViolations.join('; ')}');
        throw FakeProblem(
          400,
          'validation_failed',
          errors: [
            for (final v in requestViolations) {'pointer': v.pointer, 'code': v.code},
          ],
        );
      }
      final fake = FakeRequest(
        operationId: op,
        method: method,
        uri: request.uri,
        pathParams: params,
        headers: headers,
        body: body,
        world: world,
        baseUri: baseUri,
        session: session,
        proofJkt: jkt,
      );
      final fault = faults.take(op);
      if (fault is AnswerFault) {
        response = _problem(fault.status, fault.code, retryAfter: fault.retryAfter);
      } else {
        response = await _idempotent(fake, handler);
        if (fault is DelayFault) await Future<void>.delayed(fault.duration);
        if (fault is DropResponseFault) {
          requests.add(FakeRequestRecord(operationId: op, method: method, path: path, status: 0, body: body, headers: headers));
          final socket = await request.response.detachSocket(writeHeaders: false);
          socket.destroy();
          return;
        }
      }
    } on FakeProblem catch (p) {
      response = _problem(p.status, p.code, current: p.current, reason: p.reason, retryAfter: p.retryAfter, errors: p.errors);
    } on _Unauthorized catch (u) {
      response = u.response;
    }
    await _send(request, op, method, path, body, headers, response);
  }

  (FakeSession?, String?) _authenticate(
    ContractOperation op,
    String method,
    String htu,
    Map<String, String> headers,
  ) {
    final proof = headers['dpop'];
    if (!op.secured) {
      if (proof == null) return (null, null);
      final result = verifier.verify(proof, method: method, htu: htu);
      return (null, _accept(result));
    }
    final authorization = headers['authorization'] ?? '';
    if (authorization.startsWith('Bearer ')) {
      // A DPoP-bound token sent as Bearer (RFC 9449 7.2).
      throw _Unauthorized(_dpopProblem('invalid_dpop_proof'));
    }
    if (!authorization.startsWith('DPoP ')) {
      throw _Unauthorized(_problem(401, 'unauthenticated'));
    }
    final token = authorization.substring(5);
    final session = _sessionOf(token);
    if (session == null) throw _Unauthorized(_problem(401, 'unauthenticated'));
    if (session.revoked) throw _Unauthorized(_problem(401, 'session_revoked'));
    if (tokenExpired(token)) throw _Unauthorized(_problem(401, 'token_expired'));
    final result = verifier.verify(
      proof,
      method: method,
      htu: htu,
      accessToken: token,
      expectedJkt: session.jkt,
    );
    final jkt = _accept(result);
    final device = world.devices[session.deviceId]!;
    if (device.state == 'revoked') throw _Unauthorized(_problem(401, 'session_revoked'));
    if (device.state == 'pending' && !_pendingAllowed.contains(op.operationId)) {
      throw const FakeProblem(403, 'device_pending');
    }
    return (session, jkt);
  }

  String _accept(DpopResult result) => switch (result) {
    DpopAccepted(:final jkt) => jkt,
    DpopRejected(:final code) => throw _Unauthorized(_dpopProblem(code)),
  };

  FakeSession? _sessionOf(String token) {
    for (final s in world.sessions.values) {
      if (s.accessToken == token || s.olderAccessTokens.contains(token)) return s;
    }
    return null;
  }

  /// Whether the `exp` of a token issued by the fake lies in the past.
  bool tokenExpired(String token) {
    final parts = token.split('.');
    if (parts.length != 3) return true;
    try {
      final payload =
          jsonDecode(utf8.decode(base64Url.decode(base64Url.normalize(parts[1])))) as JsonMap;
      final exp = payload['exp']! as int;
      return world.now().millisecondsSinceEpoch ~/ 1000 >= exp;
    } on Object {
      return true;
    }
  }

  Future<FakeResponse> _idempotent(FakeRequest r, FakeHandler handler) async {
    final key = r.headers['idempotency-key'];
    if (key == null || r.session == null || r.operationId == 'createMessage') {
      return handler(r);
    }
    final hash = sha256.convert(utf8.encode('${r.method} ${r.uri.path} ${jsonEncode(r.body)}')).toString();
    final slot = '${r.session!.userId}|$key';
    final stored = world.idempotency[slot];
    if (stored != null) {
      if (stored.hash != hash) throw const FakeProblem(422, 'idempotency_key_reuse');
      return FakeResponse(stored.status, stored.body);
    }
    FakeResponse response;
    try {
      response = await handler(r);
    } on FakeProblem catch (p) {
      if (p.status < 500) {
        world.idempotency[slot] = StoredResponse(hash, p.status, _problemBody(p.status, p.code, current: p.current, reason: p.reason));
      }
      rethrow;
    }
    if (response.status < 500) {
      world.idempotency[slot] = StoredResponse(hash, response.status, response.body);
    }
    return response;
  }

  FakeResponse _dpopProblem(String code) => _problem(
    401,
    code,
    headers: {
      'DPoP-Nonce': verifier.currentNonce,
      'WWW-Authenticate': 'DPoP error="$code"',
    },
  );

  FakeResponse _problem(
    int status,
    String code, {
    JsonMap? current,
    String? reason,
    String? detail,
    Duration? retryAfter,
    List<JsonMap>? errors,
    Map<String, String> headers = const {},
  }) => FakeResponse(
    status,
    _problemBody(status, code, current: current, reason: reason, detail: detail, errors: errors),
    {
      ...headers,
      if (retryAfter != null)
        'Retry-After': '${(retryAfter.inMilliseconds + 999) ~/ 1000}',
    },
  );

  JsonMap _problemBody(
    int status,
    String code, {
    JsonMap? current,
    String? reason,
    String? detail,
    List<JsonMap>? errors,
  }) => {
    'type': '/api/v1/problems/$code',
    'title': code,
    'status': status,
    'code': code,
    'correlation_id': world.nextId(),
    'detail': ?detail,
    'reason': ?reason,
    'errors': ?errors,
    'current': ?current,
  };

  Future<void> _send(
    HttpRequest request,
    String op,
    String method,
    String path,
    Object? requestBody,
    Map<String, String> requestHeaders,
    FakeResponse response,
  ) async {
    var out = response;
    if (op != 'unknown' && out.status != 501) {
      final problems = contract.response(op, out.status, out.body);
      if (problems.isNotEmpty) {
        violations.add('response $op ${out.status}: ${problems.join('; ')}');
        out = _problem(500, 'internal_error', detail: 'the fake broke the contract');
      }
    }
    requests.add(
      FakeRequestRecord(
        operationId: op,
        method: method,
        path: path,
        status: out.status,
        body: requestBody,
        headers: requestHeaders,
      ),
    );
    final res = request.response..statusCode = out.status;
    res.headers.set('Correlation-Id', world.nextId());
    out.headers.forEach(res.headers.set);
    if (out.body != null) {
      final problem = out.body is JsonMap && (out.body! as JsonMap)['correlation_id'] != null && out.status >= 400;
      res.headers.contentType = problem
          ? ContentType('application', 'problem+json', charset: 'utf-8')
          : ContentType.json;
      res.write(jsonEncode(out.body));
    }
    await res.close();
  }
}

final class _Unauthorized implements Exception {
  const _Unauthorized(this.response);

  final FakeResponse response;
}
