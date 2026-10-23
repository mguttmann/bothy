// SPDX-License-Identifier: Apache-2.0

// The REST source of apps and webhooks (brief M6-08 Design notes 1 to 5): a manifest file is read
// as JSON or YAML and sent as JSON text unchanged, so an unknown member reaches the server's
// closed schema (11 3.1) and a JSON file keeps its bytes for the hash (D-M2-24-17); the answers
// are mapped to the types of admin_ui unchanged; every changing call sends the step-up purpose
// and, where the operation takes one, an Idempotency-Key. A routing adapter answers by method and
// path; no socket is opened.

import 'dart:convert';
import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart' show stepUpPurposeExtra;
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

/// One request as the adapter saw it: its options and its body as text.
typedef Seen = ({RequestOptions options, String body});

/// Answers each request with the JSON of [routes] by `METHOD path` and records every request.
class RoutingAdapter implements HttpClientAdapter {
  RoutingAdapter(this.routes);

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
    seen.add((options: options, body: utf8.decode(bytes)));
    final key = '${options.method} ${options.uri.path}';
    final (status, answer) =
        routes[key] ?? (throw StateError('no route for $key'));
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

const String _app = '0192f4a0-7b1c-7a00-8000-000000000a01';
const String _inst = '0192f4a0-7b1c-7a00-8000-000000000a02';
const String _alex = '0192f4a0-7b1c-7f89-8d6b-6960453241bc';
const String _sub = '0192f4a0-7b1c-7a00-8000-000000000b01';
const String _delivery = '0192f4a0-7b1c-7a00-8000-000000000c01';
const String _event = '0192f4a0-7b1c-7a00-8000-000000000c02';
const String _channel = '0192f4a0-7b1c-72d4-8bae-7661d2b2f203';
const String _hook = '0192f4a0-7b1c-7a00-8000-000000000d01';
const String _at = '2026-10-18T09:00:00.000Z';

/// The manifest of 11 3.3, shortened, as a JSON object.
Map<String, Object?> _manifest() => {
  'schema_version': 1,
  'slug': 'repo-bridge',
  'name': 'Repo Bridge',
  'description': 'Brings events from a code hosting service into a forum.',
  'publisher': 'Example Org',
  'version': '1.2.0',
  'min_api_version': '0.2',
  'scopes': {
    'required': ['messages:write'],
    'optional': ['messages:read', 'events:subscribe'],
  },
  'events': ['message.created'],
  'delivery': {
    'mode': 'webhook',
    'events_url': 'https://bridge.example.com/hooks/events',
    'interactions_url': 'https://interact.example.com/hooks',
  },
  'commands': [
    {'name': 'issue', 'description': 'Create an issue'},
  ],
  'components': ['button', 'form'],
  'requested_grants': [
    {
      'target_kind': 'channel',
      'level': 'write',
      'capabilities': ['upload'],
      'purpose': 'Create posts per ticket',
    },
  ],
  'incoming_webhooks': [
    {'name': 'alerts', 'purpose': 'Alerts of the help desk'},
  ],
  'data_egress': 'Messages of the granted channels go to bridge.example.com.',
};

Map<String, Object?> _appJson({String state = 'active'}) => {
  'id': _app,
  'slug': 'repo-bridge',
  'display_name': 'Repo Bridge',
  'manifest': _manifest(),
  'manifest_version': '1.2.0',
  'installation': {
    'id': _inst,
    'granted_scopes': ['messages:write', 'events:subscribe'],
    'event_subscriptions': ['message.created'],
    'state': state,
    'installed_by': _alex,
    'installed_at': _at,
  },
};

Map<String, Object?> _subJson({String state = 'active'}) => {
  'id': _sub,
  'app_id': _app,
  'url': 'https://bridge.example.com/hooks/events',
  'event_types': ['message.created'],
  'state': state,
  'consecutive_failures': 3,
  'signature_schemes': ['v1a', 'v1'],
  'signing_key_kid': 'whk-2026-10',
  'created_at': _at,
  'disabled_at': ?(state == 'disabled' ? _at : null),
};

Map<String, Object?> _deliveryJson({String status = 'dead'}) => {
  'id': _delivery,
  'subscription_id': _sub,
  'event_id': _event,
  'event_seq': 42,
  'event_type': 'message.created',
  'attempt': 4,
  'status': status,
  'response_code': 503,
  'last_error': 'http_status',
  'next_attempt_at': _at,
  'created_at': _at,
};

Map<String, Object?> _hookJson() => {
  'id': _hook,
  'app_id': _app,
  'channel_id': _channel,
  'name': 'Monitoring alerts',
  'created_at': _at,
};

Map<String, (int, Object?)> _routes() => {
  'POST /api/v1/app-manifests/validate': (
    200,
    {
      'valid': false,
      'errors': [
        {
          'pointer': '/unknown_member',
          'code': 'unknown_member',
          'detail': 'The member is not part of app_manifest.v1.',
        },
      ],
    },
  ),
  'POST /api/v1/apps': (201, _appJson()),
  'PUT /api/v1/apps/$_app/manifest': (200, _appJson()),
  'PUT /api/v1/apps/$_app/scopes': (200, _appJson()),
  'GET /api/v1/apps': (
    200,
    {
      'items': [_appJson(state: 'suspended')],
    },
  ),
  'GET /api/v1/apps/$_app': (200, _appJson(state: 'uninstalled')),
  'POST /api/v1/apps/$_app/suspend': (204, null),
  'POST /api/v1/apps/$_app/resume': (204, null),
  'POST /api/v1/apps/$_app/uninstall': (204, null),
  'GET /api/v1/apps/$_app/webhooks': (
    200,
    {
      'items': [_subJson(state: 'disabled')],
    },
  ),
  'POST /api/v1/apps/$_app/webhooks': (
    201,
    {'subscription': _subJson(), 'hmac_secret': 'whsec_EXAMPLEOnce'},
  ),
  'PATCH /api/v1/webhooks/$_sub': (
    200,
    {'subscription': _subJson(state: 'paused')},
  ),
  'DELETE /api/v1/webhooks/$_sub': (204, null),
  'GET /api/v1/webhooks/$_sub/deliveries': (
    200,
    {
      'items': [_deliveryJson()],
      'next_cursor': 'c2',
    },
  ),
  'POST /api/v1/webhook-deliveries/$_delivery/redeliver': (
    202,
    _deliveryJson(status: 'pending'),
  ),
  'GET /api/v1/instance/webhook-keys': (
    200,
    {
      'keys': [
        {
          'kid': 'whk-2026-10',
          'public_key': 'whpk_MCowBQYDK2VwAyEAEXAMPLE=',
          'state': 'active',
        },
      ],
    },
  ),
  'GET /api/v1/channels/$_channel/incoming-webhooks': (
    200,
    {
      'items': [_hookJson()],
    },
  ),
  'POST /api/v1/channels/$_channel/incoming-webhooks': (
    201,
    {
      'webhook': _hookJson(),
      'url': 'https://chat.example.com/api/v1/hooks/$_hook/bothy_whi_EXAMPLE',
    },
  ),
  'DELETE /api/v1/incoming-webhooks/$_hook': (204, null),
};

void main() {
  late RoutingAdapter adapter;
  late AdminRepository repository;
  var keys = 0;

  setUp(() {
    adapter = RoutingAdapter(_routes());
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
    repository = ApiAdminRepository(
      client,
      newIdempotencyKey: () => '0192f4a0-0000-7000-8000-00000000000${keys++}',
    );
  });

  Seen last(String method, String path) => adapter.seen.lastWhere(
    (s) => s.options.method == method && s.options.uri.path == path,
  );

  group('manifest files', () {
    test('a JSON file keeps its exact text, unknown members included', () {
      const text =
          '{ "slug": "repo-bridge",\n  "typo_scopes": ["messages:read"] }';
      final upload = ManifestUpload.read(
        'bridge.json',
        Uint8List.fromList(utf8.encode(text)),
      );
      expect(upload.json, text);
      expect(upload.data['typo_scopes'], ['messages:read']);
    });

    test('a YAML file becomes the same JSON object', () {
      const yaml = '''
schema_version: 1
slug: repo-bridge
scopes:
  required: [messages:write]
  optional: []
requested_grants:
  - {target_kind: channel, level: write, capabilities: [], purpose: "Posts"}
''';
      final upload = ManifestUpload.read(
        'bridge.yaml',
        Uint8List.fromList(utf8.encode(yaml)),
      );
      expect(jsonDecode(upload.json), {
        'schema_version': 1,
        'slug': 'repo-bridge',
        'scopes': {
          'required': ['messages:write'],
          'optional': <Object?>[],
        },
        'requested_grants': [
          {
            'target_kind': 'channel',
            'level': 'write',
            'capabilities': <Object?>[],
            'purpose': 'Posts',
          },
        ],
      });
      expect(
        upload.info.requestedGrants.single.targetKind,
        AdminTargetKind.channel,
      );
    });

    test('a YAML text in a file without the YAML name is read as YAML', () {
      final upload = ManifestUpload.read(
        'manifest.txt',
        Uint8List.fromList(utf8.encode('slug: repo-bridge\n')),
      );
      expect(upload.data, {'slug': 'repo-bridge'});
    });

    test(
      'no text, no object and broken YAML are refused before the server',
      () {
        Matcher fails(ManifestFileProblem p) => throwsA(
          isA<ManifestFileError>().having((e) => e.problem, 'problem', p),
        );
        expect(
          () => ManifestUpload.read('a.json', Uint8List.fromList([0xff, 0xfe])),
          fails(ManifestFileProblem.notText),
        );
        expect(
          () => ManifestUpload.read(
            'a.json',
            Uint8List.fromList(utf8.encode('[1, 2]')),
          ),
          fails(ManifestFileProblem.notObject),
        );
        expect(
          () => ManifestUpload.read(
            'a.yaml',
            Uint8List.fromList(utf8.encode('a: [1, 2\n')),
          ),
          fails(ManifestFileProblem.notJsonOrYaml),
        );
        // YAML values JSON cannot carry and keys that are no strings are refused, never
        // changed.
        for (final yaml in [
          'limit: .nan\n',
          'limit: .inf\n',
          'scopes: {1: x}\n',
        ]) {
          expect(
            () => ManifestUpload.read(
              'a.yaml',
              Uint8List.fromList(utf8.encode(yaml)),
            ),
            fails(ManifestFileProblem.notJsonOrYaml),
            reason: yaml,
          );
        }
      },
    );

    test('the consent reads every member of the manifest', () {
      final info = AppManifestInfo.fromJson(_manifest());
      expect(info.slug, 'repo-bridge');
      expect(info.requiredScopes, ['messages:write']);
      expect(info.optionalScopes, ['messages:read', 'events:subscribe']);
      expect(info.events, ['message.created']);
      expect(info.targetHosts, ['bridge.example.com', 'interact.example.com']);
      expect(info.commands.single.name, 'issue');
      expect(info.components, ['button', 'form']);
      final grant = info.requestedGrants.single;
      expect(grant.level, AccessLevel.write);
      expect(grant.capabilities, {AccessCapability.upload});
      expect(grant.purpose, 'Create posts per ticket');
      expect(info.incomingWebhooks.single.name, 'alerts');
      expect(info.dataEgress, contains('bridge.example.com'));
    });
  });

  group('apps', () {
    test(
      'validateManifest sends the text unchanged and maps the rules',
      () async {
        const text = '{"slug": "repo-bridge", "unknown_member": true}';
        final check = await repository.validateManifest(
          ManifestUpload.read('a.json', Uint8List.fromList(utf8.encode(text))),
        );
        expect(last('POST', '/api/v1/app-manifests/validate').body, text);
        expect(check.valid, isFalse);
        expect(check.errors.single.pointer, '/unknown_member');
        expect(check.errors.single.code, 'unknown_member');
      },
    );

    test(
      'installApp inserts the manifest text and sends key and purpose',
      () async {
        final text = jsonEncode(_manifest());
        final app = await repository.installApp(
          ManifestUpload.read('a.json', Uint8List.fromList(utf8.encode(text))),
          grantedScopes: const ['messages:write'],
          eventSubscriptions: const ['message.created'],
          reason: 'Bridge for tickets',
          purpose: 'Install Repo Bridge',
        );
        final seen = last('POST', '/api/v1/apps');
        expect(seen.body, startsWith('{"manifest":$text,'));
        expect(jsonDecode(seen.body), {
          'manifest': _manifest(),
          'granted_scopes': ['messages:write'],
          'event_subscriptions': ['message.created'],
          'reason': 'Bridge for tickets',
        });
        expect(seen.options.headers['Idempotency-Key'], isNotNull);
        expect(seen.options.extra[stepUpPurposeExtra], 'Install Repo Bridge');
        expect(app.id, _app);
        expect(app.grantedScopes, ['messages:write', 'events:subscribe']);
        expect(app.state, AppInstallState.active);
        expect(
          app.manifest.requestedGrants.single.purpose,
          'Create posts per ticket',
        );
      },
    );

    test('upgradeManifest and setScopes carry the consent', () async {
      await repository.upgradeManifest(
        _app,
        ManifestUpload.ofObject(_manifest()),
        grantedScopes: const ['messages:write'],
        eventSubscriptions: const [],
        reason: 'Version 1.3',
        purpose: 'Update',
      );
      final up = last('PUT', '/api/v1/apps/$_app/manifest');
      expect((jsonDecode(up.body) as Map)['reason'], 'Version 1.3');
      expect(up.options.extra[stepUpPurposeExtra], 'Update');
      await repository.setScopes(
        _app,
        grantedScopes: const ['messages:write', 'messages:read'],
        eventSubscriptions: const ['message.created'],
        reason: 'Consent',
        purpose: 'Grant scopes',
      );
      final put = last('PUT', '/api/v1/apps/$_app/scopes');
      expect(jsonDecode(put.body), {
        'granted_scopes': ['messages:write', 'messages:read'],
        'event_subscriptions': ['message.created'],
        'reason': 'Consent',
      });
    });

    test('listApps and getApp map the state of the installation', () async {
      expect((await repository.apps()).single.state, AppInstallState.suspended);
      expect((await repository.app(_app)).state, AppInstallState.uninstalled);
    });

    test('suspend, resume and uninstall send the purpose', () async {
      await repository.suspendApp(_app, purpose: 'Suspend');
      await repository.resumeApp(_app, purpose: 'Resume');
      await repository.uninstallApp(
        _app,
        reason: 'Not used',
        purpose: 'Remove',
      );
      expect(
        last(
          'POST',
          '/api/v1/apps/$_app/suspend',
        ).options.extra[stepUpPurposeExtra],
        'Suspend',
      );
      final un = last('POST', '/api/v1/apps/$_app/uninstall');
      expect(jsonDecode(un.body), {'reason': 'Not used'});
      expect(un.options.extra[stepUpPurposeExtra], 'Remove');
    });

    test('a refused install carries the failed rules', () async {
      adapter.routes['POST /api/v1/apps'] = (
        400,
        {
          'type': '/api/v1/problems/validation_failed',
          'title': 'validation_failed',
          'status': 400,
          'code': 'validation_failed',
          'errors': [
            {'pointer': '/manifest/scopes/required/0', 'code': 'invalid'},
          ],
        },
      );
      await expectLater(
        repository.installApp(
          ManifestUpload.ofObject(_manifest()),
          grantedScopes: const [],
          eventSubscriptions: const [],
          reason: 'r',
          purpose: 'p',
        ),
        throwsA(
          isA<SelfActionFailed>().having(
            (e) => e.problem?.errors.single.pointer,
            'pointer',
            '/manifest/scopes/required/0',
          ),
        ),
      );
    });
  });

  group('outgoing webhooks', () {
    test('subscriptions, create with HMAC, update and delete', () async {
      final subs = await repository.subscriptions(_app);
      expect(subs.single.state, SubscriptionState.disabled);
      expect(subs.single.consecutiveFailures, 3);
      expect(subs.single.signatureSchemes, ['v1a', 'v1']);
      expect(subs.single.disabledAt, DateTime.parse(_at));
      final created = await repository.createSubscription(
        _app,
        url: 'https://bridge.example.com/hooks/events',
        eventTypes: const ['message.created'],
        enableHmac: true,
        purpose: 'Subscribe',
      );
      expect(created.hmacSecret, 'whsec_EXAMPLEOnce');
      final post = last('POST', '/api/v1/apps/$_app/webhooks');
      expect(jsonDecode(post.body), {
        'url': 'https://bridge.example.com/hooks/events',
        'event_types': ['message.created'],
        'enable_hmac': true,
      });
      final updated = await repository.updateSubscription(
        _sub,
        state: SubscriptionState.paused,
        purpose: 'Pause',
      );
      expect(updated.subscription.state, SubscriptionState.paused);
      expect(jsonDecode(last('PATCH', '/api/v1/webhooks/$_sub').body), {
        'state': 'paused',
      });
      await repository.deleteSubscription(_sub, purpose: 'Delete');
      expect(
        last(
          'DELETE',
          '/api/v1/webhooks/$_sub',
        ).options.extra[stepUpPurposeExtra],
        'Delete',
      );
    });

    test('deliveries, redelivery and the keys', () async {
      final page = await repository.deliveries(_sub);
      final d = page.items.single;
      expect(page.nextCursor, 'c2');
      expect(d.status, DeliveryStatus.dead);
      expect(d.attempt, 4);
      expect(d.responseCode, 503);
      expect(d.lastError, 'http_status');
      expect(d.eventType, 'message.created');
      final again = await repository.redeliver(_delivery, purpose: 'Again');
      expect(again.status, DeliveryStatus.pending);
      final keys = await repository.webhookKeys();
      expect(keys.single.kid, 'whk-2026-10');
      expect(keys.single.state, 'active');
    });
  });

  group('incoming webhooks', () {
    test('list, create with the address once and revoke', () async {
      expect(
        (await repository.incomingWebhooks(_channel)).single.name,
        'Monitoring alerts',
      );
      final created = await repository.createIncomingWebhook(
        _channel,
        appId: _app,
        name: 'Monitoring alerts',
        purpose: 'Create webhook',
      );
      expect(created.url, endsWith('/bothy_whi_EXAMPLE'));
      expect(
        jsonDecode(
          last('POST', '/api/v1/channels/$_channel/incoming-webhooks').body,
        ),
        {'app_id': _app, 'name': 'Monitoring alerts'},
      );
      await repository.revokeIncomingWebhook(_hook, purpose: 'Revoke');
      expect(
        last(
          'DELETE',
          '/api/v1/incoming-webhooks/$_hook',
        ).options.extra[stepUpPurposeExtra],
        'Revoke',
      );
    });
  });
}
