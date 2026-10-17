// SPDX-License-Identifier: Apache-2.0

// E1's classification of getDiscovery (01 5.6, R-PR-14; brief M4-18 Design note 3).

import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:client/app/discovery.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

/// Answers every request with [respond]; records the paths.
class StubAdapter implements HttpClientAdapter {
  StubAdapter(this.respond);

  final Future<ResponseBody> Function(RequestOptions options) respond;
  final List<String> paths = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) {
    paths.add(options.uri.path);
    return respond(options);
  }

  @override
  void close({bool force = false}) {}
}

ResponseBody json(int status, Object body) => ResponseBody.fromString(
  jsonEncode(body),
  status,
  headers: {
    Headers.contentTypeHeader: ['application/json'],
  },
);

Map<String, Object?> discovery({
  bool setupRequired = false,
  List<Map<String, Object?>> providers = const [],
  bool? password,
  List<Map<String, Object?>>? minVersions,
  String? logoUrl,
}) => {
  'instance_id': '0193f2a7-0000-7000-8000-0000000000aa',
  'setup_required': setupRequired,
  'api_versions': ['v1'],
  'spec_version': '1.0.0',
  'gateway': {
    'url': 'wss://chat.example.com/api/v1/gateway',
    'protocol_versions': [1],
  },
  'branding': {
    'product_name': 'Team Chat',
    'accent_color': 'loch',
    'logo_url': ?logoUrl,
  },
  'auth_providers': providers,
  'local_password_enabled': ?password,
  'jwks_uri': '/api/v1/instance/jwks',
  'webhook_keys_uri': '/api/v1/instance/webhook-keys',
  'min_client_versions': ?minVersions,
};

Future<DiscoveryOutcome> run(
  Future<ResponseBody> Function(RequestOptions) respond, {
  String version = '1.2.0',
  bool allowPlainHttp = false,
  String address = 'https://chat.example.com',
}) => ApiDiscoveryClient(
  platform: 'macos',
  appVersion: version,
  allowPlainHttp: allowPlainHttp,
  adapter: StubAdapter(respond),
).discover(Uri.parse(address));

void main() {
  test(
    'a valid answer gives the instance with its methods and branding',
    () async {
      final outcome = await run(
        (o) async => o.path.endsWith('/branding/logo')
            ? ResponseBody.fromBytes([1, 2, 3], 200)
            : json(
                200,
                discovery(
                  providers: [
                    {
                      'id': '0193f2a7-0000-7000-8000-0000000000b1',
                      'kind': 'oidc',
                      'name': 'Company SSO',
                    },
                    {
                      'id': '0193f2a7-0000-7000-8000-0000000000b2',
                      'kind': 'trusted_header',
                      'name': 'Proxy',
                    },
                    {
                      'id': '0193f2a7-0000-7000-8000-0000000000b3',
                      'kind': 'local',
                      'name': 'Local',
                    },
                  ],
                  password: true,
                  logoUrl: '/api/v1/instance/branding/logo',
                ),
              ),
      );
      final instance = (outcome as DiscoveryFound).instance;
      expect(instance.productName, 'Team Chat');
      expect(instance.accentColor, 'loch');
      expect(instance.logo, [1, 2, 3]);
      expect(instance.providers.map((p) => p.name), ['Company SSO', 'Proxy']);
      expect(instance.providers.last.isProxy, isTrue);
      expect(instance.localEnabled, isTrue);
      expect(instance.passwordEnabled, isTrue);
      expect(instance.hasLoginMethod, isTrue);
    },
  );

  test(
    'a missing logo falls back to the mark; no method means no login',
    () async {
      final outcome = await run(
        (o) async => o.path.endsWith('/branding/logo')
            ? json(404, {'code': 'not_found', 'status': 404})
            : json(200, discovery(logoUrl: '/api/v1/instance/branding/logo')),
      );
      final instance = (outcome as DiscoveryFound).instance;
      expect(instance.logo, isNull);
      expect(instance.hasLoginMethod, isFalse);
    },
  );

  test('setup_required gives "not set up", by flag and by problem', () async {
    expect(
      await run((_) async => json(200, discovery(setupRequired: true))),
      isA<DiscoverySetupRequired>(),
    );
    expect(
      await run(
        (_) async => json(503, {
          'type': 'https://chat.example.com/problems/setup_required',
          'title': 'Setup required',
          'status': 503,
          'code': 'setup_required',
        }),
      ),
      isA<DiscoverySetupRequired>(),
    );
  });

  test(
    'below min_client_versions of the platform the app is too old',
    () async {
      final outcome = await run(
        (_) async => json(
          200,
          discovery(
            minVersions: [
              {'platform': 'windows', 'min_version': '9.0.0'},
              {'platform': 'macos', 'min_version': '1.10.0'},
            ],
          ),
        ),
        version: '1.9.3',
      );
      expect((outcome as DiscoveryAppTooOld).minVersion, '1.10.0');
      expect(
        await run(
          (_) async => json(
            200,
            discovery(
              minVersions: [
                {'platform': 'macos', 'min_version': '1.10.0'},
              ],
            ),
          ),
          version: '1.10.0',
        ),
        isA<DiscoveryFound>(),
      );
    },
  );

  test(
    'a TLS error is "certificate invalid", never anything to continue with',
    () async {
      expect(
        await run(
          (_) async => throw const HandshakeException('bad certificate'),
        ),
        isA<DiscoveryCertificateInvalid>(),
      );
      expect(
        await run((_) async => throw const TlsException('expired')),
        isA<DiscoveryCertificateInvalid>(),
      );
      expect(
        await run(
          (o) async => throw DioException.badCertificate(requestOptions: o),
        ),
        isA<DiscoveryCertificateInvalid>(),
      );
    },
  );

  test('no valid answer is "no instance"', () async {
    expect(
      await run((_) async => throw const SocketException('refused')),
      isA<DiscoveryUnreachable>(),
    );
    expect(
      await run((_) async => json(200, {'hello': 1})),
      isA<DiscoveryNoInstance>(),
    );
    expect(
      await run((_) async => ResponseBody.fromString('<html>', 200)),
      isA<DiscoveryNoInstance>(),
    );
    expect(
      await run(
        (_) async => json(200, discovery()),
        address: 'http://chat.example.com',
      ),
      isA<DiscoveryNoInstance>(),
    );
    expect(
      await run(
        (_) async => json(200, discovery()),
        address: 'http://127.0.0.1:9',
        allowPlainHttp: true,
      ),
      isA<DiscoveryFound>(),
    );
  });

  test('compareVersions compares numeric parts and ignores suffixes', () {
    expect(compareVersions('1.2.0', '1.10.0'), lessThan(0));
    expect(compareVersions('2.0', '1.9.9'), greaterThan(0));
    expect(compareVersions('1.0.0-beta.1', '1.0.0'), 0);
    expect(compareVersions('1.0.0+7', '1.0'), 0);
  });

  test('normalizeServerAddress adds https and keeps only the origin', () {
    expect(
      normalizeServerAddress(' chat.example.com/x?y '),
      Uri.parse('https://chat.example.com'),
    );
    expect(
      normalizeServerAddress('http://127.0.0.1:8443/'),
      Uri.parse('http://127.0.0.1:8443'),
    );
    expect(normalizeServerAddress(''), isNull);
    expect(normalizeServerAddress('ftp://chat.example.com'), isNull);
    expect(normalizeServerAddress('https://'), isNull);
  });

  test('newTraceContext gives a valid random trace ID', () {
    final a = newTraceContext();
    expect(a.traceId, matches(RegExp(r'^[0-9a-f]{32}$')));
    expect(a.sampled, isFalse);
    expect(newTraceContext().traceId, isNot(a.traceId));
  });

  test('LoginProvider compares by value', () {
    const a = LoginProvider(id: '1', kind: 'oidc', name: 'A');
    expect(a, const LoginProvider(id: '1', kind: 'oidc', name: 'A'));
    expect(
      a.hashCode,
      const LoginProvider(id: '1', kind: 'oidc', name: 'A').hashCode,
    );
    expect(a == const LoginProvider(id: '2', kind: 'oidc', name: 'A'), isFalse);
  });
}
