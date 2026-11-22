// SPDX-License-Identifier: Apache-2.0

// "Create team channel..." in the space menu (01 6.23 item 6): shown to an owner with `manage`
// on the space, it asks for a name, creates a text channel (`createChannel`, kind text, with an
// Idempotency-Key), switches team mode on (`enableChannelTeam`) and opens the team dialog.

import 'dart:convert';
import 'dart:typed_data';

import 'package:api_client/api_client.dart' as wire;
import 'package:client/menus/space_menu.dart';
import 'package:client/notifications_settings/providers.dart';
import 'package:client/team/providers.dart';
import 'package:client/team/team_port.dart';
import 'package:client/thread/providers.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/team_fakes.dart';

Future<ColumnHost> _open(
  WidgetTester tester,
  FakeTeamPort port, {
  bool owner = true,
  List<Capability> capabilities = const [Capability.manage],
}) async {
  final host = await ColumnHost.open(tester);
  final s = space(1, name: 'Dev');
  await tester.runAsync(() async {
    await host.db.structureDao.upsertSpaceIfNewer(s);
    await host.db.structureDao.putSpaceAccess(
      s.id,
      AccessView(level: AccessLevel.trigger, capabilities: capabilities),
    );
  });
  var keys = 0;
  await host.pump(
    tester,
    SpaceMenu(space: s, child: const Text('the space')),
    overrides: [
      isOwnerProvider.overrideWith((ref) async => owner),
      myPrincipalIdProvider.overrideWithValue(alex),
      teamPortProvider.overrideWithValue(port),
      teamKeysProvider.overrideWithValue(() => 'key-${++keys}'),
    ],
  );
  await tester.tap(find.text('the space'), buttons: kSecondaryMouseButton);
  await tester.pumpAndSettle();
  return host;
}

const ValueKey<String> _entry = ValueKey('menu.createTeamChannel');

Future<void> _create(WidgetTester tester, String name) async {
  await tester.enterText(find.byKey(const ValueKey('teamCreate.name')), name);
  await tester.pump();
  await tester.tap(find.byKey(const ValueKey('teamCreate.submit')));
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('an owner with manage on the space sees the entry', (
    tester,
  ) async {
    final host = await _open(tester, FakeTeamPort());
    expect(find.byKey(_entry), findsOneWidget);
    expect(find.text('Create team channel...'), findsOneWidget);
    await host.dispose(tester);
  });

  testWidgets('no entry for a person who is not an owner', (tester) async {
    final host = await _open(tester, FakeTeamPort(), owner: false);
    expect(find.byKey(_entry), findsNothing);
    await host.dispose(tester);
  });

  testWidgets('no entry without manage on the space (channel.create)', (
    tester,
  ) async {
    final host = await _open(tester, FakeTeamPort(), capabilities: const []);
    expect(find.byKey(_entry), findsNothing);
    await host.dispose(tester);
  });

  testWidgets(
    'creating makes a text channel, switches team mode on and opens the team dialog',
    (tester) async {
      final port = FakeTeamPort();
      final host = await _open(tester, port);
      await tester.tap(find.byKey(_entry));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('teamCreate.dialog')), findsOneWidget);
      await _create(tester, '  team  ');
      expect(port.calls, [
        'create ${spaceId(1).value} team key-1',
        'enable',
        'read',
      ]);
      expect(port.enabled, [port.created]);
      expect(find.byKey(const ValueKey('teamCreate.dialog')), findsNothing);
      expect(find.byKey(const ValueKey('team.dialog')), findsOneWidget);
      expect(tester.takeException(), isNull);
      await host.dispose(tester);
    },
  );

  testWidgets(
    'a refused switch keeps the created channel; the next press only switches it on',
    (tester) async {
      final port = FakeTeamPort()
        ..failures.add(
          const TeamFailure.problem(
            'conflict',
            reason: 'team_readers_not_owners',
          ),
        );
      final host = await _open(tester, port);
      await tester.tap(find.byKey(_entry));
      await tester.pumpAndSettle();
      await _create(tester, 'team');
      expect(find.byKey(const ValueKey('teamCreate.failure')), findsOneWidget);
      expect(find.byKey(const ValueKey('team.dialog')), findsNothing);
      await tester.tap(find.byKey(const ValueKey('teamCreate.submit')));
      await tester.pumpAndSettle();
      expect(port.calls, [
        'create ${spaceId(1).value} team key-1',
        'enable',
        'enable',
        'read',
      ]);
      expect(find.byKey(const ValueKey('team.dialog')), findsOneWidget);
      await host.dispose(tester);
    },
  );

  testWidgets('a lost answer of the create sends the same key again', (
    tester,
  ) async {
    final port = FakeTeamPort()..createFailure = const TeamFailure.offline();
    final host = await _open(tester, port);
    await tester.tap(find.byKey(_entry));
    await tester.pumpAndSettle();
    await _create(tester, 'team');
    expect(find.byKey(const ValueKey('teamCreate.failure')), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('teamCreate.submit')));
    await tester.pumpAndSettle();
    expect(port.calls, [
      'create ${spaceId(1).value} team key-1',
      'create ${spaceId(1).value} team key-1',
      'enable',
      'read',
    ]);
    expect(find.byKey(const ValueKey('team.dialog')), findsOneWidget);
    await host.dispose(tester);
  });

  testWidgets(
    'a refused create drops its key; the next press is a new request',
    (tester) async {
      final port = FakeTeamPort()
        ..createFailure = const TeamFailure.problem('forbidden');
      final host = await _open(tester, port);
      await tester.tap(find.byKey(_entry));
      await tester.pumpAndSettle();
      await _create(tester, 'team');
      expect(find.byKey(const ValueKey('teamCreate.failure')), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('teamCreate.submit')));
      await tester.pumpAndSettle();
      expect(port.calls, [
        'create ${spaceId(1).value} team key-1',
        'create ${spaceId(1).value} team key-2',
        'enable',
        'read',
      ]);
      await host.dispose(tester);
    },
  );

  testWidgets('an empty name cannot be sent', (tester) async {
    final port = FakeTeamPort();
    final host = await _open(tester, port);
    await tester.tap(find.byKey(_entry));
    await tester.pumpAndSettle();
    await _create(tester, '   ');
    expect(port.calls, isEmpty);
    expect(find.byKey(const ValueKey('teamCreate.dialog')), findsOneWidget);
    await host.dispose(tester);
  });

  test(
    'the API port creates a text channel with the Idempotency-Key',
    () async {
      final adapter = _Adapter();
      final port = ApiTeamPort(
        wire.ApiClient(
          dio: Dio(BaseOptions(baseUrl: 'https://chat.example.com/v1'))
            ..httpClientAdapter = adapter,
          interceptors: const [],
        ),
      );
      final id = await port.createTextChannel(
        spaceId(1),
        'team',
        idempotencyKey: 'key-1',
      );
      expect(id, ChannelId.parse(_createdId));
      final (method, path, body, headers) = adapter.seen.single;
      expect('$method $path', 'POST /v1/spaces/${spaceId(1).value}/channels');
      expect(body, {'kind': 'text', 'name': 'team'});
      expect(headers['Idempotency-Key'], 'key-1');
    },
  );
}

const String _createdId = '0192f4a0-7b1c-72d4-8bae-7661d2b2f203';

/// Answers `createChannel` with the example channel of the API and records the request.
final class _Adapter implements HttpClientAdapter {
  final List<(String, String, Object?, Map<String, dynamic>)> seen = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final bytes = requestStream == null
        ? <int>[]
        : [for (final chunk in await requestStream.toList()) ...chunk];
    seen.add((
      options.method,
      options.uri.path,
      bytes.isEmpty ? null : jsonDecode(utf8.decode(bytes)),
      options.headers,
    ));
    return ResponseBody.fromString(
      jsonEncode({
        'id': _createdId,
        'space_id': spaceId(1).value,
        'kind': 'text',
        'name': 'team',
        'topic': '',
        'sort_key': 'a1',
        'read_only': false,
        'project_bound': false,
        'operator': {'mode': 'inherit'},
        'version': 1,
        'settings_version': 1,
        'created_at': '2026-11-22T09:30:00.000Z',
      }),
      201,
      headers: {
        Headers.contentTypeHeader: ['application/json'],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}
