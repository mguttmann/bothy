// SPDX-License-Identifier: Apache-2.0

// The REST source of the instance administration (brief M6-09 Design notes 1 to 4): every write
// names the version in `If-Match` and carries the step-up purpose, a 412 becomes an
// InstanceVersionConflict with the problem's `current` (or a fresh read without one), the flag
// write sends the rollout unchanged, provider writes send the configuration as plain JSON (an
// update only its changed members) with an Idempotency-Key, the logo goes as multipart with the
// MIME type of its name and a 404 of the preview is "no preview yet". A routing adapter answers
// by method and path; no socket is opened.

import 'dart:convert';
import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart' show stepUpPurposeExtra;
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

/// One request as the adapter saw it: its options and its body as bytes.
typedef Seen = ({RequestOptions options, List<int> body});

/// Answers each request by `METHOD path` with [routes]: a status and a JSON body, or raw bytes.
class _Adapter implements HttpClientAdapter {
  _Adapter(this.routes);

  final Map<String, (int, Object?)> routes;
  final List<Seen> seen = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final bytes = <int>[];
    if (requestStream != null) {
      await for (final chunk in requestStream) {
        bytes.addAll(chunk);
      }
    }
    seen.add((options: options, body: bytes));
    final key = '${options.method} ${options.uri.path}';
    final (status, answer) =
        routes[key] ?? (throw StateError('no route for $key'));
    if (answer is Uint8List) {
      return ResponseBody.fromBytes(
        answer,
        status,
        headers: {
          Headers.contentTypeHeader: ['image/png'],
        },
      );
    }
    return ResponseBody.fromString(
      answer == null ? '' : jsonEncode(answer),
      status,
      headers: {
        if (answer != null)
          Headers.contentTypeHeader: [
            status >= 400 ? 'application/problem+json' : 'application/json',
          ],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

const String _alex = '0192f4a0-7b1c-7f89-8d6b-6960453241bc';
const String _robin = '0192f4a0-7b1c-7f89-8d6b-6960453241bd';
const String _idp = '0192f4a0-7b1c-7e26-89e1-d4e246452fb8';
const String _logo = '0192f4a0-7b1c-7e26-89e1-d4e246452fb9';
const String _at = '2026-10-18T09:00:00.000Z';

Map<String, Object?> _setting(String value, int version, {String? by}) => {
  'key': 'locale.default',
  'value': value,
  'version': version,
  'updated_by': ?by,
  'updated_at': ?(by == null ? null : _at),
};

Map<String, Object?> _branding({
  String name = 'Team Chat',
  String accent = 'loch',
  String? logo,
  int version = 3,
}) => {
  'product_name': name,
  'accent_color': accent,
  'logo_file_id': ?logo,
  'version': version,
};

Map<String, Object?> _flag({bool enabled = false, int version = 2}) => {
  'key': 'dm.enabled',
  'kind': 'option',
  'description': 'Direct messages between people',
  'default': false,
  'module': 'chat',
  'orphaned': false,
  'enabled': enabled,
  'rollout': {
    'user_ids': [_alex],
    'group_ids': <String>[],
  },
  'version': version,
  'updated_by': _robin,
  'updated_at': _at,
};

Map<String, Object?> _provider({bool enabled = true}) => {
  'id': _idp,
  'kind': 'oidc',
  'name': 'Company login',
  'config': {
    'issuer': 'https://login.example.com',
    'client_id': 'bothy-example',
    'client_auth': 'client_secret_basic',
    'client_secret_ref': 'file:/run/bothy/secrets/idp/EXAMPLE',
    'allowed_tenant_ids': ['a1'],
  },
  'enabled': enabled,
  'allow_provisioning': false,
};

Map<String, Object?> _instance() => {
  'instance_id': '0192f4a0-7b1c-7e26-89e1-d4e246452f00',
  'branding': _branding(logo: _logo),
  'locale_default': 'de',
  'spec_version': '0.2',
  'features': [
    {'key': 'dm.enabled', 'enabled': true},
  ],
  'limits': {
    'message_max_content_bytes': 40000,
    'message_max_attachments': 10,
    'file_max_upload_bytes': 104857600,
    'thread_title_max_chars': 120,
    'voice_max_duration_ms': 300000,
  },
};

void main() {
  late _Adapter adapter;
  late ApiInstanceRepository repository;
  var keys = 0;

  void setUpWith(Map<String, (int, Object?)> routes) {
    adapter = _Adapter(routes);
    keys = 0;
    final client = createApiClient(
      baseUrl: Uri.parse('https://chat.example.com'),
      traceContext: () => TraceContext(
        traceId: '4bf92f3577b34da6a3ce929d0e0e4736',
        sampled: true,
      ),
      readRetry: ReadRetryPolicy.fromBackoff(
        base: const Duration(milliseconds: 1),
        cap: const Duration(milliseconds: 1),
        sleep: (_) async {},
      ),
      adapter: adapter,
    );
    repository = ApiInstanceRepository(
      client,
      newIdempotencyKey: () => 'key-${++keys}',
    );
  }

  Map<String, Object?> jsonBody(Seen s) =>
      jsonDecode(utf8.decode(s.body)) as Map<String, Object?>;

  test('settings: list, and a write with If-Match and the purpose', () async {
    setUpWith({
      'GET /api/v1/instance/settings': (
        200,
        {
          'items': [_setting('de', 0)],
        },
      ),
      'PUT /api/v1/instance/settings/locale.default': (
        200,
        _setting('en', 1, by: _alex),
      ),
    });
    final list = await repository.instanceSettings();
    expect(list.single.key, 'locale.default');
    expect(list.single.value, 'de');
    expect(list.single.version, 0);

    final saved = await repository.putInstanceSetting(
      'locale.default',
      'en',
      version: 0,
      purpose: 'Change locale',
    );
    expect(saved.value, 'en');
    expect(saved.updatedBy, _alex);
    final put = adapter.seen.last;
    expect(put.options.headers['If-Match'], '"0"');
    expect(put.options.extra[stepUpPurposeExtra], 'Change locale');
    expect(jsonBody(put), {'value': 'en'});
  });

  test(
    'a 412 with current becomes a conflict with the newer setting',
    () async {
      setUpWith({
        'PUT /api/v1/instance/settings/locale.default': (
          412,
          {
            'code': 'version_conflict',
            'current': _setting('en', 4, by: _robin),
          },
        ),
      });
      await expectLater(
        repository.putInstanceSetting(
          'locale.default',
          'de',
          version: 2,
          purpose: 'p',
        ),
        throwsA(
          isA<InstanceVersionConflict<InstanceSettingEntry>>()
              .having((c) => c.current.value, 'value', 'en')
              .having((c) => c.current.version, 'version', 4)
              .having((c) => c.current.updatedBy, 'by', _robin),
        ),
      );
    },
  );

  test('a 412 without current reads the setting again', () async {
    setUpWith({
      'PUT /api/v1/instance/settings/locale.default': (
        412,
        {'code': 'version_conflict'},
      ),
      'GET /api/v1/instance/settings': (
        200,
        {
          'items': [_setting('en', 5, by: _robin)],
        },
      ),
    });
    await expectLater(
      repository.putInstanceSetting(
        'locale.default',
        'de',
        version: 2,
        purpose: 'p',
      ),
      throwsA(
        isA<InstanceVersionConflict<InstanceSettingEntry>>().having(
          (c) => c.current.version,
          'version',
          5,
        ),
      ),
    );
  });

  test('a 400 keeps the field errors', () async {
    setUpWith({
      'PUT /api/v1/instance/branding': (
        400,
        {
          'code': 'validation_failed',
          'errors': [
            {'pointer': '/product_name', 'code': 'too_long'},
            {'pointer': '/accent_color', 'code': 'unknown_color_key'},
          ],
        },
      ),
    });
    try {
      await repository.updateBranding(
        productName: 'x' * 99,
        accentColor: 'granite',
        version: 3,
        purpose: 'p',
      );
      fail('expected a refusal');
    } on SelfActionFailed catch (e) {
      expect(fieldErrorsOf(e).keys, ['/product_name', '/accent_color']);
    }
  });

  test('branding: overview, update with If-Match, conflict', () async {
    setUpWith({
      'GET /api/v1/instance': (200, _instance()),
      'PUT /api/v1/instance/branding': (200, _branding(name: 'Forum')),
    });
    final overview = await repository.instanceOverview();
    expect(overview.branding.productName, 'Team Chat');
    expect(overview.branding.logoFileId, _logo);
    expect(overview.features, {'dm.enabled': true});

    await repository.updateBranding(
      productName: 'Forum',
      accentColor: 'loch',
      version: 3,
      purpose: 'Brand',
    );
    final put = adapter.seen.last;
    expect(put.options.headers['If-Match'], '"3"');
    expect(put.options.extra[stepUpPurposeExtra], 'Brand');
    expect(jsonBody(put), {'product_name': 'Forum', 'accent_color': 'loch'});

    setUpWith({
      'PUT /api/v1/instance/branding': (
        412,
        {
          'code': 'version_conflict',
          'current': _branding(name: 'Robin chat', version: 5),
        },
      ),
    });
    await expectLater(
      repository.updateBranding(
        productName: 'Forum',
        accentColor: 'loch',
        version: 3,
        purpose: 'p',
      ),
      throwsA(
        isA<InstanceVersionConflict<BrandingState>>().having(
          (c) => c.current.productName,
          'name',
          'Robin chat',
        ),
      ),
    );
  });

  test('logo: multipart upload, preview, 404 is no preview, remove', () async {
    final png = Uint8List.fromList([0x89, 0x50, 0x4E, 0x47]);
    setUpWith({
      'PUT /api/v1/instance/branding/logo': (200, _branding(logo: _logo)),
      'GET /api/v1/instance/branding/logo': (200, png),
      'DELETE /api/v1/instance/branding/logo': (204, null),
    });
    final saved = await repository.uploadLogo(
      PickedFile('Logo.PNG', png),
      purpose: 'Logo',
    );
    expect(saved.logoFileId, _logo);
    final upload = adapter.seen.last;
    expect(upload.options.extra[stepUpPurposeExtra], 'Logo');
    final text = latin1.decode(upload.body);
    expect(text, contains('name="file"; filename="Logo.PNG"'));
    expect(text, contains('content-type: image/png'));

    expect(await repository.logo(), png);
    await repository.removeLogo(purpose: 'Remove');
    expect(adapter.seen.last.options.extra[stepUpPurposeExtra], 'Remove');

    setUpWith({
      'GET /api/v1/instance/branding/logo': (404, {'code': 'not_found'}),
    });
    expect(await repository.logo(), isNull);
  });

  test('a removed logo (empty file ID) is no logo', () async {
    setUpWith({'PUT /api/v1/instance/branding': (200, _branding(logo: ''))});
    final b = await repository.updateBranding(
      productName: 'Team Chat',
      accentColor: 'loch',
      version: 3,
      purpose: 'p',
    );
    expect(b.logoFileId, isNull);
  });

  test('flags: list and a write that keeps the rollout', () async {
    setUpWith({
      'GET /api/v1/feature-flags': (
        200,
        {
          'items': [
            _flag(),
            {
              'key': 'old.flag',
              'orphaned': true,
              'enabled': false,
              'rollout': <String, Object?>{},
              'version': 1,
            },
          ],
        },
      ),
      'PUT /api/v1/feature-flags/dm.enabled': (
        200,
        _flag(enabled: true, version: 3),
      ),
    });
    final flags = await repository.featureFlags();
    expect(flags.first.kind, FeatureFlagKind.option);
    expect(flags.first.rolloutUserIds, [_alex]);
    expect(flags.first.defaultValue, isFalse);
    expect(flags.last.orphaned, isTrue);
    expect(flags.last.kind, FeatureFlagKind.unknown);

    final saved = await repository.putFeatureFlag(
      flags.first,
      enabled: true,
      reason: 'Pilot',
      purpose: 'Flag',
    );
    expect(saved.enabled, isTrue);
    final put = adapter.seen.last;
    expect(put.options.headers['If-Match'], '"2"');
    expect(put.options.extra[stepUpPurposeExtra], 'Flag');
    expect(jsonBody(put), {
      'enabled': true,
      'rollout': {
        'user_ids': [_alex],
        'group_ids': <String>[],
      },
      'reason': 'Pilot',
    });
  });

  test(
    'providers: plain JSON config, idempotency, changed members only',
    () async {
      setUpWith({
        'GET /api/v1/identity-providers': (
          200,
          {
            'items': [_provider()],
          },
        ),
        'POST /api/v1/identity-providers': (201, _provider()),
        'PATCH /api/v1/identity-providers/$_idp': (
          200,
          _provider(enabled: false),
        ),
        'DELETE /api/v1/identity-providers/$_idp': (204, null),
      });
      final list = await repository.identityProviders();
      expect(list.single.kind, ProviderKind.oidc);
      expect(list.single.config['allowed_tenant_ids'], ['a1']);

      await repository.createIdentityProvider(
        const ProviderDraft(
          kind: ProviderKind.oidc,
          name: 'Company login',
          config: {
            'issuer': 'https://login.example.com',
            'client_id': 'bothy-example',
            'client_auth': 'client_secret_basic',
            'client_secret_ref': 'file:/run/bothy/secrets/idp/EXAMPLE',
            'allowed_tenant_ids': ['a1'],
          },
          enabled: true,
          allowProvisioning: false,
        ),
        purpose: 'Add',
      );
      final post = adapter.seen.last;
      expect(post.options.headers['Idempotency-Key'], 'key-1');
      expect(post.options.extra[stepUpPurposeExtra], 'Add');
      expect(jsonBody(post), {
        'kind': 'oidc',
        'name': 'Company login',
        'config': _provider()['config'],
        'enabled': true,
        'allow_provisioning': false,
      });

      final disabled = await repository.updateIdentityProvider(
        _idp,
        const ProviderChange(enabled: false),
        purpose: 'Disable',
      );
      expect(disabled.enabled, isFalse);
      final patch = adapter.seen.last;
      expect(patch.options.headers['Idempotency-Key'], 'key-2');
      expect(jsonBody(patch), {'enabled': false});

      await repository.deleteIdentityProvider(_idp, purpose: 'Delete');
      expect(adapter.seen.last.options.extra[stepUpPurposeExtra], 'Delete');
    },
  );

  test('a provider of an unknown driver is never created', () {
    setUpWith({});
    expect(
      () => repository.createIdentityProvider(
        const ProviderDraft(
          kind: ProviderKind.unknown,
          name: 'x',
          config: {},
          enabled: true,
          allowProvisioning: false,
        ),
        purpose: 'p',
      ),
      throwsArgumentError,
    );
  });

  test('the redirect URI is the callback of this server', () {
    setUpWith({});
    expect(
      repository.oidcRedirectUri,
      'https://chat.example.com/api/v1/auth/oidc/callback',
    );
  });

  test('the MIME type follows the file name', () {
    expect(logoMimeOf('a.png'), 'image/png');
    expect(logoMimeOf('a.JPEG'), 'image/jpeg');
    expect(logoMimeOf('a.jpg'), 'image/jpeg');
    expect(logoMimeOf('a.webp'), 'image/webp');
    expect(logoMimeOf('a.gif'), 'application/octet-stream');
  });
}
