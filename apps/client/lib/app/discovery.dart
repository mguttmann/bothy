// SPDX-License-Identifier: Apache-2.0

/// Screen E1: what `getDiscovery` says about the address the person typed (docs/spec/
/// 01-product.md 5.6, R-PR-14; brief M4-18 Design note 3).
library;

import 'dart:io';
import 'dart:math';
import 'dart:typed_data';

import 'package:api_client/api_client.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart' show canonicalUuid;

/// A login method of the instance as E2 offers it.
final class LoginProvider {
  /// Creates the provider.
  const LoginProvider({
    required this.id,
    required this.kind,
    required this.name,
  });

  /// The provider ID for `beginOidcLogin`.
  final String id;

  /// `oidc` or `trusted_header`.
  final String kind;

  /// The display name, for example `Company SSO`.
  final String name;

  /// Whether the login runs behind a trusted proxy without a browser (08 2.4).
  bool get isProxy => kind == 'trusted_header';

  @override
  bool operator ==(Object other) =>
      other is LoginProvider &&
      other.id == id &&
      other.kind == kind &&
      other.name == name;

  @override
  int get hashCode => Object.hash(id, kind, name);
}

/// What the app may show before the login: the instance and its public branding (01 5.6:
/// name, logo and accent only) and the login methods for E2.
final class InstanceInfo {
  /// Creates the description.
  const InstanceInfo({
    required this.address,
    required this.instanceId,
    required this.productName,
    this.accentColor,
    this.logo,
    this.providers = const [],
    this.localEnabled = false,
    this.passwordEnabled = false,
  });

  /// The origin of the instance.
  final Uri address;

  /// `instance_id`, the scope of the device keys and the prefix of the secrets.
  final String instanceId;

  /// `branding.product_name`.
  final String productName;

  /// `branding.accent_color`, a category color key of the design system.
  final String? accentColor;

  /// The bytes of the logo, or null for the product's mark.
  final Uint8List? logo;

  /// The OIDC and proxy providers in the order of discovery.
  final List<LoginProvider> providers;

  /// Whether a `local` driver is active (login name, passkey).
  final bool localEnabled;

  /// `local_password_enabled`.
  final bool passwordEnabled;

  /// Whether the instance offers any login method (E2 says so otherwise).
  bool get hasLoginMethod => providers.isNotEmpty || localEnabled;

  /// The JSON the app remembers for a start without network (D-M4-18-13); the logo is not
  /// kept.
  Map<String, Object?> toJson() => {
    'address': address.toString(),
    'instance_id': instanceId,
    'product_name': productName,
    'accent_color': accentColor,
    'providers': [
      for (final p in providers) {'id': p.id, 'kind': p.kind, 'name': p.name},
    ],
    'local_enabled': localEnabled,
    'password_enabled': passwordEnabled,
  };

  /// Reads [toJson]; returns null for anything else.
  static InstanceInfo? fromJson(Object? json) {
    if (json is! Map) return null;
    final address = Uri.tryParse('${json['address']}');
    final id = json['instance_id'];
    final name = json['product_name'];
    final providers = json['providers'];
    if (address == null || id is! String || name is! String) return null;
    // The ID is a storage scope (see ApiDiscoveryClient.discover); a stored one that is no
    // UUID is not trusted either.
    final String instanceId;
    try {
      instanceId = canonicalUuid(id, 'instance ID');
    } on FormatException {
      return null;
    }
    return InstanceInfo(
      address: address,
      instanceId: instanceId,
      productName: name,
      accentColor: json['accent_color'] is String
          ? json['accent_color']! as String
          : null,
      providers: [
        if (providers is List)
          for (final p in providers)
            if (p is Map &&
                p['id'] is String &&
                p['kind'] is String &&
                p['name'] is String)
              LoginProvider(
                id: p['id']! as String,
                kind: p['kind']! as String,
                name: p['name']! as String,
              ),
      ],
      localEnabled: json['local_enabled'] == true,
      passwordEnabled: json['password_enabled'] == true,
    );
  }
}

/// The result of E1.
sealed class DiscoveryOutcome {
  const DiscoveryOutcome();
}

/// The instance answered; go on to E2.
final class DiscoveryFound extends DiscoveryOutcome {
  /// Creates the outcome.
  const DiscoveryFound(this.instance);

  /// The instance.
  final InstanceInfo instance;
}

/// No valid discovery answer at the address.
final class DiscoveryNoInstance extends DiscoveryOutcome {
  /// Creates the outcome.
  const DiscoveryNoInstance();
}

/// No answer at all (no network, the host is down). The start routing treats it like
/// [DiscoveryNoInstance]: a stored session opens its local data, otherwise E1 shows "No
/// instance at this address" (D-M4-18-13, D-M4-18-15).
final class DiscoveryUnreachable extends DiscoveryOutcome {
  /// Creates the outcome.
  const DiscoveryUnreachable();
}

/// The TLS handshake failed; there is no way to continue and no exception (R-PR-14).
final class DiscoveryCertificateInvalid extends DiscoveryOutcome {
  /// Creates the outcome.
  const DiscoveryCertificateInvalid();
}

/// `setup_required`: the instance has no owner yet (16 3.1).
final class DiscoverySetupRequired extends DiscoveryOutcome {
  /// Creates the outcome.
  const DiscoverySetupRequired();
}

/// This build is below the `min_client_versions` entry of its platform (18).
final class DiscoveryAppTooOld extends DiscoveryOutcome {
  /// Creates the outcome.
  const DiscoveryAppTooOld(this.minVersion);

  /// The version the instance requires.
  final String minVersion;
}

/// Reads the public metadata of an instance.
abstract interface class DiscoveryClient {
  /// Calls `getDiscovery` at [address] (an origin) and classifies the answer; never throws.
  Future<DiscoveryOutcome> discover(Uri address);
}

/// [DiscoveryClient] over the generated API client without authentication.
///
/// [platform] is `macos` or `windows`, [appVersion] the version of this build; only
/// [allowPlainHttp] (the integration test entry) accepts an `http` address. [adapter] replaces
/// the HTTP adapter in tests.
final class ApiDiscoveryClient implements DiscoveryClient {
  /// Creates the client.
  ApiDiscoveryClient({
    required this.platform,
    required this.appVersion,
    this.allowPlainHttp = false,
    HttpClientAdapter? adapter,
    Duration? connectTimeout,
  }) : _adapter = adapter, // ignore: prefer_initializing_formals
       _connectTimeout = connectTimeout; // ignore: prefer_initializing_formals

  /// The `DevicePlatform` of this build.
  final String platform;

  /// The version of this build.
  final String appVersion;

  /// Whether `http` addresses are accepted (tests only).
  final bool allowPlainHttp;

  final HttpClientAdapter? _adapter;
  final Duration? _connectTimeout;

  @override
  Future<DiscoveryOutcome> discover(Uri address) async {
    if (address.scheme == 'http' && !allowPlainHttp) {
      return const DiscoveryNoInstance();
    }
    final client = createApiClient(
      baseUrl: address,
      traceContext: newTraceContext,
      readRetry: ReadRetryPolicy(
        base: Duration.zero,
        cap: Duration.zero,
        maxAttempts: 1,
      ),
      adapter: _adapter,
      connectTimeout: _connectTimeout,
    );
    final Discovery discovery;
    try {
      discovery = (await client.getDiscoveryApi().getDiscovery()).data!;
    } on DioException catch (e) {
      return _classify(e);
    } on Object {
      return const DiscoveryNoInstance();
    }
    if (discovery.setupRequired) return const DiscoverySetupRequired();
    // The instance ID names the data folder, the device key scope and the secret prefix, and a
    // wipe deletes that folder recursively; anything but a UUID (`..`, a path) is no instance.
    final instanceId = _instanceId(discovery.instanceId);
    if (instanceId == null) return const DiscoveryNoInstance();
    final required = discovery.minClientVersions
        ?.where((r) => r.platform.name == platform)
        .firstOrNull;
    if (required != null &&
        compareVersions(appVersion, required.minVersion) < 0) {
      return DiscoveryAppTooOld(required.minVersion);
    }
    Uint8List? logo;
    if (discovery.branding.logoUrl != null) {
      try {
        logo = (await client.getDiscoveryApi().getBrandingLogo()).data;
      } on Object {
        // The app falls back to the product's mark while the logo is missing (01 12).
        logo = null;
      }
    }
    final providers = discovery.authProviders;
    return DiscoveryFound(
      InstanceInfo(
        address: address,
        instanceId: instanceId,
        productName: discovery.branding.productName,
        accentColor: discovery.branding.accentColor,
        logo: logo,
        providers: [
          for (final p in providers)
            if (p.kind != IdentityProviderKind.local)
              LoginProvider(
                id: p.id,
                // The wire code; the Dart name of the generated enum is camelCase.
                kind: p.kind == IdentityProviderKind.trustedHeader
                    ? 'trusted_header'
                    : 'oidc',
                name: p.name,
              ),
        ],
        localEnabled: providers.any(
          (p) => p.kind == IdentityProviderKind.local,
        ),
        passwordEnabled: discovery.localPasswordEnabled ?? false,
      ),
    );
  }

  static String? _instanceId(String text) {
    try {
      return canonicalUuid(text, 'instance ID');
    } on FormatException {
      return null;
    }
  }

  static DiscoveryOutcome _classify(DioException e) {
    if (e.type == DioExceptionType.badCertificate ||
        e.error is TlsException ||
        e.error is HandshakeException) {
      return const DiscoveryCertificateInvalid();
    }
    if (e.response == null &&
        (e.type == DioExceptionType.connectionError ||
            e.type == DioExceptionType.connectionTimeout ||
            e.type == DioExceptionType.receiveTimeout ||
            e.type == DioExceptionType.sendTimeout ||
            e.error is SocketException)) {
      return const DiscoveryUnreachable();
    }
    if (ApiProblem.of(e)?.code == 'setup_required' ||
        (e.response != null &&
            ApiProblem.fromResponse(e.response!).code == 'setup_required')) {
      return const DiscoverySetupRequired();
    }
    return const DiscoveryNoInstance();
  }
}

/// A fresh W3C trace context with a random trace ID, unsampled (the client sends no traces).
TraceContext newTraceContext() {
  final random = _random;
  final id = List<String>.generate(
    32,
    (_) => random.nextInt(16).toRadixString(16),
  ).join();
  return TraceContext(traceId: id == '0' * 32 ? '1' * 32 : id, sampled: false);
}

final _random = Random.secure();

/// Compares two semantic versions by their numeric `major.minor.patch` parts; a pre-release or
/// build suffix is ignored, a missing part counts as 0. Returns a negative number, 0 or a
/// positive number.
///
/// Example: `compareVersions('1.2.0', '1.10.0') < 0`.
int compareVersions(String a, String b) {
  List<int> parts(String v) => v
      .split(RegExp('[-+]'))
      .first
      .split('.')
      .map((p) => int.tryParse(p) ?? 0)
      .toList();
  final x = parts(a);
  final y = parts(b);
  for (var i = 0; i < 3; i++) {
    final d = (i < x.length ? x[i] : 0) - (i < y.length ? y[i] : 0);
    if (d != 0) return d;
  }
  return 0;
}

/// Turns what the person typed on E1 into an origin: `https://` is added when no scheme is
/// given; path, query and fragment are dropped. Returns null for something that is no
/// `http` or `https` address with a host.
///
/// Example: `normalizeServerAddress('chat.example.com/')` is `https://chat.example.com`.
Uri? normalizeServerAddress(String typed) {
  final text = typed.trim();
  if (text.isEmpty) return null;
  final withScheme = text.contains('://') ? text : 'https://$text';
  final uri = Uri.tryParse(withScheme);
  if (uri == null ||
      uri.host.isEmpty ||
      !(uri.scheme == 'https' || uri.scheme == 'http')) {
    return null;
  }
  return Uri(
    scheme: uri.scheme,
    host: uri.host,
    port: uri.hasPort ? uri.port : null,
  );
}
