// SPDX-License-Identifier: Apache-2.0

// "Create channel..." in the space menu and "Create space..." in the space menu and the empty
// sidebar (01 6.25; docs/decisions.d/FEAT-SPACE-CHANNEL-CREATE.md): shown by the right the
// operation needs, one Idempotency-Key per name, a refusal drops the key, a lost answer and
// `409 request_in_progress` keep it.

import 'dart:convert';
import 'dart:typed_data';

import 'package:api_client/api_client.dart' as wire;
import 'package:client/menus/space_menu.dart';
import 'package:client/notifications_settings/providers.dart';
import 'package:client/sidebar/sidebar_column.dart';
import 'package:client/structure/providers.dart';
import 'package:client/structure/structure_port.dart';
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:local_store/local_store.dart';

import '../support/column_host.dart';
import '../support/structure.dart';
import '../support/structure_fakes.dart';

Future<ColumnHost> _menu(
  WidgetTester tester,
  FakeStructurePort port, {
  bool owner = false,
  List<Capability> capabilities = const [Capability.manage],
  Locale locale = const Locale('en'),
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
    locale: locale,
    overrides: [
      isOwnerProvider.overrideWith((ref) async => owner),
      structurePortProvider.overrideWithValue(port),
      structureKeysProvider.overrideWithValue(() => 'key-${++keys}'),
    ],
  );
  await tester.tap(find.text('the space'), buttons: kSecondaryMouseButton);
  await tester.pumpAndSettle();
  return host;
}

Future<ColumnHost> _emptySidebar(
  WidgetTester tester,
  FakeStructurePort port, {
  bool owner = true,
  bool phone = false,
}) async {
  final host = await ColumnHost.open(tester);
  var keys = 0;
  await host.pump(
    tester,
    SidebarColumn(phone: phone),
    platform: phone ? 'ios' : 'macos',
    overrides: [
      isOwnerProvider.overrideWith((ref) async => owner),
      structurePortProvider.overrideWithValue(port),
      structureKeysProvider.overrideWithValue(() => 'key-${++keys}'),
    ],
  );
  return host;
}

const ValueKey<String> _channelEntry = ValueKey('menu.createChannel');
const ValueKey<String> _spaceEntry = ValueKey('menu.createSpace');
const ValueKey<String> _emptyButton = ValueKey('sidebar.createSpace');
const ValueKey<String> _dialog = ValueKey('structureCreate.dialog');
const ValueKey<String> _failure = ValueKey('structureCreate.failure');

Future<void> _create(WidgetTester tester, String name) async {
  await tester.enterText(
    find.byKey(const ValueKey('structureCreate.name')),
    name,
  );
  await tester.pump();
  await _submit(tester);
}

Future<void> _submit(WidgetTester tester) async {
  await tester.tap(find.byKey(const ValueKey('structureCreate.submit')));
  await tester.pumpAndSettle();
}

void main() {
  group('Create channel...', () {
    testWidgets('a person with manage on the space sees the entry', (
      tester,
    ) async {
      final host = await _menu(tester, FakeStructurePort());
      expect(find.byKey(_channelEntry), findsOneWidget);
      expect(find.text('Create channel...'), findsOneWidget);
      await host.dispose(tester);
    });

    testWidgets('German label', (tester) async {
      final host = await _menu(
        tester,
        FakeStructurePort(),
        locale: const Locale('de'),
      );
      expect(find.text('Kanal anlegen...'), findsOneWidget);
      await host.dispose(tester);
    });

    testWidgets('no entry without manage on the space (channel.create)', (
      tester,
    ) async {
      final host = await _menu(
        tester,
        FakeStructurePort(),
        owner: true,
        capabilities: const [],
      );
      expect(find.byKey(_channelEntry), findsNothing);
      await host.dispose(tester);
    });

    testWidgets('creating makes a text channel and opens it', (tester) async {
      final port = FakeStructurePort();
      final host = await _menu(tester, port);
      await tester.tap(find.byKey(_channelEntry));
      await tester.pumpAndSettle();
      expect(find.textContaining('New channel in'), findsOneWidget);
      await _create(tester, '  general  ');
      expect(port.calls, ['channel ${spaceId(1).value} general key-1']);
      expect(find.byKey(_dialog), findsNothing);
      expect(host.visited.last, '/c/${port.channel.value}');
      expect(tester.takeException(), isNull);
      await host.dispose(tester);
    });

    testWidgets(
      'a refused create shows the reason and drops its key; the next press is a new request',
      (tester) async {
        final port = FakeStructurePort()
          ..failures.add(const StructureFailure.problem('forbidden'));
        final host = await _menu(tester, port);
        await tester.tap(find.byKey(_channelEntry));
        await tester.pumpAndSettle();
        await _create(tester, 'general');
        expect(find.byKey(_failure), findsOneWidget);
        expect(find.byKey(_dialog), findsOneWidget);
        await _submit(tester);
        expect(port.calls, [
          'channel ${spaceId(1).value} general key-1',
          'channel ${spaceId(1).value} general key-2',
        ]);
        expect(find.byKey(_dialog), findsNothing);
        await host.dispose(tester);
      },
    );

    testWidgets('a lost answer sends the same key again', (tester) async {
      final port = FakeStructurePort()
        ..failures.add(const StructureFailure.offline());
      final host = await _menu(tester, port);
      await tester.tap(find.byKey(_channelEntry));
      await tester.pumpAndSettle();
      await _create(tester, 'general');
      expect(find.text('No connection. Try again.'), findsOneWidget);
      await _submit(tester);
      expect(port.calls, [
        'channel ${spaceId(1).value} general key-1',
        'channel ${spaceId(1).value} general key-1',
      ]);
      await host.dispose(tester);
    });

    testWidgets('409 request_in_progress keeps the key', (tester) async {
      final port = FakeStructurePort()
        ..failures.add(const StructureFailure.problem('request_in_progress'));
      final host = await _menu(tester, port);
      await tester.tap(find.byKey(_channelEntry));
      await tester.pumpAndSettle();
      await _create(tester, 'general');
      expect(find.byKey(_failure), findsOneWidget);
      await _submit(tester);
      expect(port.calls, [
        'channel ${spaceId(1).value} general key-1',
        'channel ${spaceId(1).value} general key-1',
      ]);
      await host.dispose(tester);
    });

    testWidgets('a changed name takes a new key', (tester) async {
      final port = FakeStructurePort()
        ..failures.add(const StructureFailure.offline());
      final host = await _menu(tester, port);
      await tester.tap(find.byKey(_channelEntry));
      await tester.pumpAndSettle();
      await _create(tester, 'general');
      await _create(tester, 'random');
      expect(port.calls, [
        'channel ${spaceId(1).value} general key-1',
        'channel ${spaceId(1).value} random key-2',
      ]);
      await host.dispose(tester);
    });

    testWidgets('an empty name cannot be sent', (tester) async {
      final port = FakeStructurePort();
      final host = await _menu(tester, port);
      await tester.tap(find.byKey(_channelEntry));
      await tester.pumpAndSettle();
      await _create(tester, '   ');
      expect(port.calls, isEmpty);
      expect(find.byKey(_dialog), findsOneWidget);
      await host.dispose(tester);
    });
  });

  group('Create space...', () {
    testWidgets('an owner sees the entry in the space menu', (tester) async {
      final host = await _menu(tester, FakeStructurePort(), owner: true);
      expect(find.byKey(_spaceEntry), findsOneWidget);
      expect(find.text('Create space...'), findsOneWidget);
      await host.dispose(tester);
    });

    testWidgets('no entry in the space menu for a person who is no owner', (
      tester,
    ) async {
      final host = await _menu(tester, FakeStructurePort());
      expect(find.byKey(_spaceEntry), findsNothing);
      await host.dispose(tester);
    });

    testWidgets('the empty sidebar offers it to an owner', (tester) async {
      final host = await _emptySidebar(tester, FakeStructurePort());
      expect(find.text('No spaces yet'), findsOneWidget);
      expect(find.byKey(_emptyButton), findsOneWidget);
      await host.dispose(tester);
    });

    testWidgets('the empty sidebar of the phone offers it to an owner', (
      tester,
    ) async {
      final host = await _emptySidebar(
        tester,
        FakeStructurePort(),
        phone: true,
      );
      expect(find.byKey(_emptyButton), findsOneWidget);
      await host.dispose(tester);
    });

    testWidgets(
      'the empty sidebar offers nothing to a person who is no owner',
      (tester) async {
        final host = await _emptySidebar(
          tester,
          FakeStructurePort(),
          owner: false,
        );
        expect(find.text('No spaces yet'), findsOneWidget);
        expect(find.byKey(_emptyButton), findsNothing);
        await host.dispose(tester);
      },
    );

    testWidgets('creating from the empty sidebar makes a space', (
      tester,
    ) async {
      final port = FakeStructurePort();
      final host = await _emptySidebar(tester, port);
      await tester.tap(find.byKey(_emptyButton));
      await tester.pumpAndSettle();
      expect(find.text('New space'), findsOneWidget);
      await _create(tester, ' Research ');
      expect(port.calls, ['space Research key-1']);
      expect(find.byKey(_dialog), findsNothing);
      expect(tester.takeException(), isNull);
      await host.dispose(tester);
    });

    testWidgets('a refused create drops its key, a lost answer keeps it', (
      tester,
    ) async {
      final port = FakeStructurePort()
        ..failures.addAll(const [
          StructureFailure.problem('forbidden'),
          StructureFailure.offline(),
        ]);
      final host = await _menu(tester, port, owner: true);
      await tester.tap(find.byKey(_spaceEntry));
      await tester.pumpAndSettle();
      await _create(tester, 'Research');
      expect(find.byKey(_failure), findsOneWidget);
      await _submit(tester);
      expect(find.byKey(_failure), findsOneWidget);
      await _submit(tester);
      expect(port.calls, [
        'space Research key-1',
        'space Research key-2',
        'space Research key-2',
      ]);
      expect(find.byKey(_dialog), findsNothing);
      await host.dispose(tester);
    });
  });

  group('API port', () {
    test('createSpace sends the name with the Idempotency-Key', () async {
      final adapter = _Adapter({
        'id': spaceId(7).value,
        'name': 'Research',
        'sort_key': 'a1',
        'version': 1,
        'created_at': '2026-11-23T09:30:00.000Z',
      });
      final id = await _port(adapter)
          .createSpace('Research', idempotencyKey: 'key-1');
      expect(id, spaceId(7));
      final (method, path, body, headers) = adapter.seen.single;
      expect('$method $path', 'POST /v1/spaces');
      expect(body, {'name': 'Research'});
      expect(headers['Idempotency-Key'], 'key-1');
    });

    test(
      'createChannel sends a text channel with the Idempotency-Key',
      () async {
        final adapter = _Adapter({
          'id': channelId(7).value,
          'space_id': spaceId(1).value,
          'kind': 'text',
          'name': 'general',
          'topic': '',
          'sort_key': 'a1',
          'read_only': false,
          'project_bound': false,
          'operator': {'mode': 'inherit'},
          'version': 1,
          'settings_version': 1,
          'created_at': '2026-11-23T09:30:00.000Z',
        });
        final id = await _port(adapter)
            .createTextChannel(spaceId(1), 'general', idempotencyKey: 'key-1');
        expect(id, channelId(7));
        final (method, path, body, headers) = adapter.seen.single;
        expect('$method $path', 'POST /v1/spaces/${spaceId(1).value}/channels');
        expect(body, {'kind': 'text', 'name': 'general'});
        expect(headers['Idempotency-Key'], 'key-1');
      },
    );

    test('a problem answer is a refusal, no answer is offline', () async {
      final refused = _Adapter({
        'code': 'request_in_progress',
        'title': 'In progress',
        'status': 409,
      }, status: 409);
      await expectLater(
        _port(refused).createSpace('Research', idempotencyKey: 'key-1'),
        throwsA(
          isA<StructureFailure>()
              .having((f) => f.code, 'code', 'request_in_progress')
              .having((f) => f.keepsKey, 'keepsKey', isTrue),
        ),
      );
      await expectLater(
        _port(_Adapter(null)).createSpace('Research', idempotencyKey: 'key-1'),
        throwsA(
          isA<StructureFailure>().having((f) => f.offline, 'offline', isTrue),
        ),
      );
    });
  });
}

ApiStructurePort _port(HttpClientAdapter adapter) => ApiStructurePort(
  wire.ApiClient(
    dio: Dio(BaseOptions(baseUrl: 'https://chat.example.com/v1'))
      ..httpClientAdapter = adapter,
    interceptors: [wire.ProblemInterceptor()],
  ),
);

/// Answers every request with [answer] and [status] (null: the connection fails) and records
/// the request.
final class _Adapter implements HttpClientAdapter {
  _Adapter(this.answer, {this.status = 201});

  final Map<String, Object?>? answer;
  final int status;
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
    final body = answer;
    if (body == null) {
      throw DioException.connectionError(
        requestOptions: options,
        reason: 'offline',
      );
    }
    return ResponseBody.fromString(
      jsonEncode(body),
      status,
      headers: {
        Headers.contentTypeHeader: [
          status >= 400 ? 'application/problem+json' : 'application/json',
        ],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}
