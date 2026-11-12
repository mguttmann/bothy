// SPDX-License-Identifier: Apache-2.0

// A grant for an installed app from the screen "Access grants" down to the wire (ruling
// AUD-AM1/1; 11 4.1, 4.2: grants of an app arise only with `grant.create`, per target). The
// install dialog creates one grant per requested entry; a second forum, a target left out at the
// installation and a target after an update go through the grant wizard, which therefore offers
// the installed apps as subjects. The widgets run over `ApiAdminRepository`, the generated client
// and its decoder; the adapter below keeps the grants like the server (docs/api/openapi.yaml:
// `App`, `GrantCreate`, `GrantPreview`, `Grant`). The tests use only the keys of the controls
// and the name of the app, so they read the same on a build without the app subject.

import 'dart:convert';
import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart' show stepUpPurposeExtra;
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../self/support.dart';

typedef _Seen = ({RequestOptions options, Object? json});

const String _alex = '0192f4a0-7b1c-7f89-8d6b-6960453241bc';
const String _app = '0192f4a0-7b1c-7a00-8000-000000000a01';
const String _space = '0192f4a0-7b1c-7fd5-831e-d4370e087874';
const String _issues = '0192f4a0-7b1c-72d4-8bae-7661d2b2f203';
const String _releases = '0192f4a0-7b1c-72d4-8bae-7661d2b2f204';
const String _first = '0192f4a0-7b1c-727f-86bd-90fe7bdb3024';
const String _second = '0192f4a0-7b1c-727f-86bd-90fe7bdb3025';
const String _at = '2026-10-18T09:00:00.000Z';

Map<String, Object?> _appJson(String id, String slug, String name) => {
  'id': id,
  'slug': slug,
  'display_name': name,
  'manifest': {
    'schema_version': 1,
    'slug': slug,
    'name': name,
    'description': 'Brings events from a code hosting service into a forum.',
    'publisher': 'Example Org',
    'version': '1.2.0',
    'min_api_version': '0.2',
    'scopes': {
      'required': ['messages:write'],
      'optional': <String>[],
    },
    'events': <String>[],
    'requested_grants': [
      {
        'target_kind': 'channel',
        'level': 'write',
        'capabilities': <String>[],
        'purpose': 'Create posts per ticket',
      },
    ],
  },
  'manifest_version': '1.2.0',
  'installation': {
    'id': '0192f4a0-7b1c-7a00-8000-000000000a02',
    'granted_scopes': ['messages:write'],
    'event_subscriptions': <String>[],
    'state': 'active',
    'installed_by': _alex,
    'installed_at': _at,
  },
};

Map<String, Object?> _channelJson(String id, String name) => {
  'channel': {
    'id': id,
    'space_id': _space,
    'kind': 'forum',
    'name': name,
    'topic': '',
    'sort_key': 'a1',
    'read_only': false,
    'project_bound': false,
    'operator': {'mode': 'inherit'},
    'version': 1,
    'settings_version': 1,
    'created_at': _at,
  },
  'level': 'trigger',
  'capabilities': <Object?>[],
};

/// The directory and grant routes of the server: one installed app with the grant its
/// installation created on the forum "issues", and a second forum.
class _GrantServer implements HttpClientAdapter {
  final List<Map<String, Object?>> grants = [
    {
      'id': _first,
      'subject_kind': 'app',
      'subject_id': _app,
      'target_kind': 'channel',
      'target_id': _issues,
      'level': 'write',
      'capabilities': <String>[],
      'reason': 'Create posts per ticket',
      'granted_by': _alex,
      'granted_at': _at,
    },
  ];
  final List<_Seen> seen = [];

  _Seen last(String method, String path) => seen.lastWhere(
    (s) => s.options.method == method && s.options.uri.path == path,
  );

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
    final text = utf8.decode(bytes);
    final json = text.isEmpty ? null : jsonDecode(text);
    seen.add((options: options, json: json));
    final (status, answer) = _answer(
      options,
      json is Map ? json.cast<String, Object?>() : const {},
    );
    return ResponseBody.fromString(
      jsonEncode(answer),
      status,
      headers: {
        Headers.contentTypeHeader: [
          status >= 400 ? 'application/problem+json' : 'application/json',
        ],
      },
    );
  }

  (int, Object) _answer(RequestOptions options, Map<String, Object?> body) {
    final method = options.method;
    final path = options.uri.path.replaceFirst('/api/v1', '');
    final query = options.uri.queryParameters;
    const empty = {'items': <Object?>[]};
    switch ((method, path)) {
      case ('GET', '/users'):
        return (
          200,
          {
            'items': [
              {
                'id': _alex,
                'handle': 'alex',
                'display_name': 'Alex',
                'status': 'active',
                'is_owner': true,
                'created_at': _at,
              },
            ],
          },
        );
      case ('GET', '/groups') || ('GET', '/agents') || ('GET', '/hosts'):
        return (200, empty);
      case ('GET', '/apps'):
        return (
          200,
          {
            'items': [_appJson(_app, 'repo-bridge', 'Repo Bridge')],
          },
        );
      case ('GET', '/spaces'):
        return (
          200,
          {
            'items': [
              {
                'space': {
                  'id': _space,
                  'name': 'Bridge',
                  'sort_key': 'a0',
                  'version': 1,
                  'created_at': _at,
                },
                'level': 'trigger',
                'capabilities': <Object?>[],
              },
            ],
          },
        );
      case ('GET', '/spaces/$_space/channels'):
        return (
          200,
          {
            'items': [
              _channelJson(_issues, 'issues'),
              _channelJson(_releases, 'releases'),
            ],
          },
        );
      case ('GET', '/grants'):
        return (
          200,
          {
            'items': [
              for (final g in grants)
                if (g['target_id'] == query['target_id']) g,
            ],
          },
        );
      case ('POST', '/grants/preview'):
        return (
          200,
          {
            'allowed': true,
            'effects': [
              {
                'principal': {
                  'id': body['subject_id'],
                  'kind': 'app',
                  'handle': 'repo-bridge',
                  'display_name': 'Repo Bridge',
                },
                'target_kind': body['target_kind'],
                'target_id': body['target_id'],
                'before_level': 'none',
                'after_level': body['level'],
                'capabilities_added': <String>[],
              },
            ],
            'invariant_violations': <Object?>[],
            'hints': <Object?>[],
            'text': 'Repo Bridge may write in the forum.',
            'text_locale': 'en',
          },
        );
      case ('POST', '/grants'):
        final grant = {
          'id': _second,
          'subject_kind': body['subject_kind'],
          'subject_id': body['subject_id'],
          'target_kind': body['target_kind'],
          'target_id': body['target_id'],
          'level': body['level'],
          'capabilities': body['capabilities'] ?? <String>[],
          'reason': body['reason'],
          'granted_by': _alex,
          'granted_at': _at,
        };
        grants.add(grant);
        return (201, grant);
    }
    return (
      404,
      {
        'type': '/api/v1/problems/not_found',
        'title': 'not_found',
        'status': 404,
        'code': 'not_found',
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  late _GrantServer server;
  late AdminRepository repository;
  var keys = 0;

  setUp(() {
    server = _GrantServer();
    keys = 0;
    repository = ApiAdminRepository(
      createApiClient(
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
        adapter: server,
      ),
      newIdempotencyKey: () => '0192f4a0-0000-7000-8000-00000000000${keys++}',
    );
  });

  Future<void> tap(WidgetTester tester, String key) async {
    await tester.ensureVisible(find.byKey(ValueKey(key)).last);
    await tester.tap(find.byKey(ValueKey(key)).last);
    await tester.pumpAndSettle();
  }

  // The requests of the widgets are real asynchronous work of the client, so each step runs
  // in `runAsync` until the request it waits for was seen; the first pump passes the pause of
  // the wizard before a preview.
  Future<void> settle(WidgetTester tester) async {
    await tester.pump(const Duration(milliseconds: 400));
    for (var i = 0; i < 20; i++) {
      await tester.runAsync(() => Future<void>.delayed(Duration.zero));
      await tester.pump();
    }
    await tester.pumpAndSettle();
  }

  Future<void> grantsOf(WidgetTester tester, String channel) async {
    await pumpInApp(
      tester,
      GrantsList(
        repository: repository,
        targetKey: 'channel.$channel',
        onTarget: (_) {},
        clock: testClock,
      ),
      settle: false,
    );
    await settle(tester);
  }

  testWidgets('the grant of the installation reads with the name of the app', (
    tester,
  ) async {
    await grantsOf(tester, _issues);

    expect(
      find.descendant(
        of: find.byKey(const ValueKey('grant.$_first')),
        matching: find.text('Repo Bridge'),
      ),
      findsOneWidget,
    );
    expect(find.text(_app), findsNothing);
  });

  testWidgets('a second forum after the installation: the wizard offers the '
      'installed app and sends createGrant for it', (tester) async {
    await grantsOf(tester, _releases);
    expect(find.byKey(const ValueKey('grant.$_first')), findsNothing);

    await tap(tester, 'grants.create');
    await settle(tester);
    await tap(tester, 'wizard.kind.app');
    await tap(tester, 'subject.$_app');
    await tap(tester, 'wizard.next');
    await tap(tester, 'wizard.next');
    await tap(tester, 'level.write');
    await tester.enterText(
      find.byKey(const ValueKey('wizard.reason')),
      'Release notes go to their own forum',
    );
    await settle(tester);

    final preview = server.last('POST', '/api/v1/grants/preview');
    expect((preview.json! as Map)['subject_kind'], 'app');
    expect(find.text('Repo Bridge may write in the forum.'), findsOneWidget);

    await tap(tester, 'wizard.confirm');
    await settle(tester);

    final sent = server.last('POST', '/api/v1/grants');
    expect(sent.json, {
      'subject_kind': 'app',
      'subject_id': _app,
      'target_kind': 'channel',
      'target_id': _releases,
      'level': 'write',
      'capabilities': <Object?>[],
      'reason': 'Release notes go to their own forum',
    });
    expect(
      sent.options.headers['Idempotency-Key'],
      '0192f4a0-0000-7000-8000-000000000000',
    );
    expect(
      sent.options.extra[stepUpPurposeExtra],
      'Create the access grant for Repo Bridge on Channel Bridge/releases',
    );
    // The screen reads the grants of the forum again and names the app.
    expect(find.byKey(const ValueKey('wizard.dialog')), findsNothing);
    expect(
      find.descendant(
        of: find.byKey(const ValueKey('grant.$_second')),
        matching: find.text('Repo Bridge'),
      ),
      findsOneWidget,
    );
    expect(server.grants, hasLength(2));
  });
}
