// SPDX-License-Identifier: Apache-2.0

/// Fakes of the console tests: an in-memory server behind Dio's adapter slot and the browser
/// ports (flow store, navigation, passkeys). No test opens a socket.
library;

import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:console/console.dart';
import 'package:dio/dio.dart';

/// One request the fake server received.
class SeenRequest {
  /// Creates the record.
  SeenRequest(this.method, this.path, this.headers, this.body);

  /// HTTP method in upper case.
  final String method;

  /// Path without the origin, for example `/api/v1/auth/logout`.
  final String path;

  /// Request headers with lower-case names.
  final Map<String, String> headers;

  /// The decoded JSON body, or null.
  final Object? body;
}

/// A canned answer: status and JSON body.
class Answer {
  /// Creates the answer.
  const Answer(this.status, [this.body]);

  /// HTTP status.
  final int status;

  /// JSON body or null.
  final Object? body;
}

/// An in-memory server: answers by `METHOD path` from [routes] and records every request.
class FakeServer implements HttpClientAdapter {
  /// Creates the server with the version both sides report.
  FakeServer({String specVersion = '0.2.0', String builtVersion = '0.2.0'}) {
    routes['GET /api/v1/instance/discovery'] = (_) =>
        Answer(200, discovery(specVersion: specVersion));
    routes['GET /console/version.json'] = (_) =>
        Answer(200, {'spec_version': builtVersion, 'server_version': '0.0.0'});
    routes['GET /api/v1/console/session'] = (_) =>
        const Answer(401, {'code': 'unauthenticated', 'status': 401});
  }

  /// Handlers by `METHOD path`.
  final Map<String, Answer Function(SeenRequest)> routes = {};

  /// Every request in order.
  final List<SeenRequest> seen = [];

  /// The requests to [path] with [method].
  List<SeenRequest> to(String method, String path) => [
    for (final r in seen)
      if (r.method == method && r.path == path) r,
  ];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    Object? body;
    if (requestStream != null) {
      final bytes = <int>[];
      await for (final chunk in requestStream) {
        bytes.addAll(chunk);
      }
      if (bytes.isNotEmpty) body = jsonDecode(utf8.decode(bytes));
    }
    final req = SeenRequest(options.method.toUpperCase(), options.uri.path, {
      for (final e in options.headers.entries)
        e.key.toLowerCase(): '${e.value}',
    }, body);
    seen.add(req);
    final handler = routes['${req.method} ${req.path}'];
    final answer = handler == null
        ? const Answer(404, {'code': 'not_found', 'status': 404})
        : handler(req);
    final problem = answer.status >= 400;
    return ResponseBody.fromString(
      answer.body == null ? '' : jsonEncode(answer.body),
      answer.status,
      headers: {
        Headers.contentTypeHeader: [
          problem ? 'application/problem+json' : 'application/json',
        ],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

/// A Discovery body with an OIDC provider, a proxy provider and local passkeys and passwords as
/// requested.
Map<String, Object?> discovery({
  String specVersion = '0.2.0',
  bool oidc = true,
  bool trustedHeader = false,
  bool local = false,
  bool password = false,
}) => {
  'instance_id': '0192f4a0-7b1c-7db7-89b9-2fe4c48dcb3e',
  'setup_required': false,
  'api_versions': ['v1'],
  'spec_version': specVersion,
  'gateway': {
    'url': '/api/v1/gateway',
    'protocol_versions': [1],
  },
  'branding': {'product_name': 'Team Chat', 'accent_color': 'loch'},
  'auth_providers': [
    if (oidc) {'id': oidcProviderId, 'kind': 'oidc', 'name': 'Company SSO'},
    if (trustedHeader)
      {'id': proxyProviderId, 'kind': 'trusted_header', 'name': 'Proxy'},
    if (local) {'id': localProviderId, 'kind': 'local', 'name': 'Local'},
  ],
  'local_password_enabled': password,
  'jwks_uri': '/api/v1/instance/jwks',
  'webhook_keys_uri': '/api/v1/instance/webhook-keys',
};

/// IDs of the providers of [discovery].
const oidcProviderId = '0192f4a0-7b1c-7474-8636-4dbb076ee0db';

/// ID of the proxy provider.
const proxyProviderId = '0192f4a0-7b1c-7474-8636-4dbb076ee0dc';

/// ID of the local provider.
const localProviderId = '0192f4a0-7b1c-7474-8636-4dbb076ee0dd';

/// A ConsoleSession body; [stepUp] adds `step_up_at`.
Map<String, Object?> consoleSession({
  bool stepUp = false,
  List<String> methods = const ['passkey', 'oidc_reauth', 'password_totp'],
}) => {
  'user_id': '0192f4a0-7b1c-7f89-8d6b-6960453241bc',
  'display_name': 'Alex',
  'expires_at': '2026-10-10T17:30:00.000Z',
  'idle_timeout_seconds': 1800,
  if (stepUp) 'step_up_at': '2026-10-10T09:30:00.000Z',
  if (stepUp) 'step_up_method': 'passkey',
  if (stepUp) 'step_up_valid_until': '2026-10-10T09:45:00.000Z',
  'step_up_methods_available': methods,
};

/// A StepUpResult body.
const stepUpResult = {
  'step_up_at': '2026-10-10T09:30:00.000Z',
  'valid_until': '2026-10-10T09:45:00.000Z',
};

/// sessionStorage of one tab, kept across a "reload" by passing the same instance.
class MemoryFlowStore implements FlowStore {
  /// The stored entries.
  final Map<String, String> entries = {};

  @override
  String? read(String key) => entries[key];

  @override
  void write(String key, String value) => entries[key] = value;

  @override
  void delete(String key) => entries.remove(key);
}

/// Records the navigation of the tab instead of leaving the app.
class RecordingNavigation implements BrowserNavigation {
  /// URLs the tab was sent to.
  final List<Uri> assigned = [];

  /// How often the page was reloaded.
  int reloads = 0;

  /// The last language tag set on the document.
  String? language;

  @override
  void assign(Uri url) => assigned.add(url);

  @override
  void reload() => reloads++;

  @override
  void setDocumentLanguage(String tag) => language = tag;
}

/// A passkey authenticator that answers with a fixed assertion and records the options.
class FakePasskeys implements PasskeyPort {
  /// The options of every call.
  final List<Map<String, Object?>> calls = [];

  /// When set, the call fails with it.
  Object? failWith;

  @override
  Future<Map<String, Object?>> get(Map<String, Object?> options) async {
    calls.add(options);
    final failure = failWith;
    if (failure != null) throw failure;
    return {
      'id': 'Y3JlZA',
      'rawId': 'Y3JlZA',
      'type': 'public-key',
      'response': {
        'clientDataJSON': 'e30',
        'authenticatorData': 'YXV0aA',
        'signature': 'c2ln',
      },
      'clientExtensionResults': <String, Object?>{},
    };
  }
}

/// Records the files the console offers as downloads.
class FakeDownloads implements FileDownloads {
  /// The saved files: name, media type and bytes.
  final List<({String name, String type, Uint8List bytes})> saved = [];

  @override
  void save(String name, String type, Uint8List bytes) =>
      saved.add((name: name, type: type, bytes: bytes));
}

/// The browser environment of a test: the fake server and the fake ports.
class TestBrowser {
  /// Creates the environment; pass [flows] to keep sessionStorage across a reload.
  TestBrowser({FakeServer? server, MemoryFlowStore? flows})
    : server = server ?? FakeServer(),
      flows = flows ?? MemoryFlowStore();

  /// The fake server.
  final FakeServer server;

  /// The tab's sessionStorage.
  final MemoryFlowStore flows;

  /// The tab's navigation.
  final RecordingNavigation navigation = RecordingNavigation();

  /// The passkey authenticator.
  final FakePasskeys passkeys = FakePasskeys();

  /// The downloads of the tab.
  final FakeDownloads downloads = FakeDownloads();

  /// The console environment over these fakes, at the origin of the harness.
  ConsoleEnvironment get environment => ConsoleEnvironment(
    origin: Uri.parse('https://chat.example.com'),
    flowStore: flows,
    navigation: navigation,
    passkeys: passkeys,
    downloads: downloads,
    adapter: server,
  );
}
