// SPDX-License-Identifier: Apache-2.0

// The fake side of the behaviour conformance between the real server and the fake server
// (AUD-pkg-sync-engine-01, finding sync-fake-server-1). Every file of
// `test/fixtures/conformance` names the requests of one rule and, under `expect`, what the real
// server answered and which events it wrote: `TestFakeConformance` of
// `services/server/test/fakeconformance` sends the requests to the server of this tree, records
// that part and fails when the server no longer answers so. This test sends the same requests to
// the fake and compares with the same file, so a rule the fake copies by hand cannot differ from
// the server without a failing test. A new rule is a new file (README, "Conformance with the
// real server").

import 'dart:convert';
import 'dart:io';

import 'package:auth/auth.dart' show LoginSignedIn;
import 'package:dio/dio.dart';
import 'package:sync_engine/testing.dart';
import 'package:test/test.dart';

import 'support.dart';

/// The folder of the scenario files.
const String scenarioDir = 'test/fixtures/conformance';

final RegExp _placeholder = RegExp(r'\{(uuid:)?([a-z0-9_]+)\}');

/// The world of a scenario on one fake: Alex and Sam, both with `write` on the text channel of
/// one space, and a thread of that channel; the same world `newWorld` of the server test builds.
class ConformanceWorld {
  ConformanceWorld._(this.server, this.devices, this.names);

  /// Starts a fake, builds the world and signs both people in.
  static Future<ConformanceWorld> start() async {
    final server = await FakeServer.start();
    final world = server.world;
    final people = {
      for (final handle in ['alex', 'sam'])
        handle: world.addPerson(
          handle: handle,
          displayName: handle[0].toUpperCase() + handle.substring(1),
          loginName: handle,
          password: WorldSeed.password,
          totpCode: WorldSeed.totp,
        ),
    };
    final space = world.addSpace('Team');
    final channel = world.addChannel(space, 'general', {
      for (final p in people.values) p.id: 'write',
    });
    final thread = world.addThread(
      channel,
      'general',
      createdBy: people['alex']!.id,
    );
    final devices = <String, TestDevice>{};
    var seed = 20;
    for (final handle in people.keys) {
      final device = TestDevice(
        server.baseUri,
        SoftwareP256Signer.seeded(seed++),
        handle,
      );
      devices[handle] = device;
      final outcome = await device.signIn();
      if (outcome is! LoginSignedIn) {
        throw StateError('sign-in of $handle failed: $outcome');
      }
    }
    return ConformanceWorld._(server, devices, {
      'space': space,
      'channel': channel,
      'thread': thread,
      for (final MapEntry(:key, :value) in people.entries) key: value.id,
    });
  }

  /// The fake.
  final FakeServer server;

  /// The signed-in device of each person, by handle.
  final Map<String, TestDevice> devices;

  /// The placeholders of the scenario files and the IDs they stand for.
  final Map<String, String> names;

  /// Replaces the placeholders of [text]: `{name}` by the ID of the world, `{uuid:name}` by a
  /// UUID that is new in this run and the same for every use of the name.
  String fill(String text) => text.replaceAllMapped(_placeholder, (m) {
    final key = m[0]!.substring(1, m[0]!.length - 1);
    final known = names[key];
    if (known != null) return known;
    if (!key.startsWith('uuid:')) return m[0]!;
    return names[key] = server.world.nextId();
  });

  /// Replaces every string of [value] that is an ID of the world by its placeholder.
  Object? named(Object? value) => switch (value) {
    final String s =>
      names.entries
              .where((e) => e.value == s)
              .map((e) => '{${e.key}}')
              .firstOrNull ??
          s,
    final List<Object?> l => [for (final v in l) named(v)],
    final Map<Object?, Object?> m => {
      for (final MapEntry(:key, :value) in m.entries) key: named(value),
    },
    _ => value,
  };

  /// Sends the request of [step] as its person and returns the answer, whatever its status.
  Future<Response<Object?>> send(Map<String, Object?> step) async {
    final device = devices[step['as']];
    if (device == null) throw StateError('unknown person ${step['as']}');
    final method = step['method']! as String;
    final body = step['body'];
    try {
      return await device.api.dio.request<Object?>(
        fill(step['path']! as String),
        data: body == null ? null : jsonDecode(fill(jsonEncode(body))),
        options: Options(
          method: method,
          // 401 stays an error, so the auth interceptors answer a nonce or a refresh.
          validateStatus: (status) => status != 401,
          contentType: body == null ? null : Headers.jsonContentType,
          headers: {
            if (method == 'POST' || method == 'PATCH')
              'Idempotency-Key': server.world.nextId(),
          },
          // As the generated operations mark a request that needs the person's token.
          extra: {
            'secure': [
              {'type': 'http', 'scheme': 'DPoP', 'name': 'dpopAuth'},
            ],
          },
        ),
      );
    } on DioException catch (e) {
      final response = e.response;
      if (response == null) rethrow;
      return response;
    }
  }

  /// What [response] and the events appended after the first [eventsBefore] show of the rule
  /// of [scenario], in the form the server test records.
  Map<String, Object?> observed(
    Map<String, Object?> scenario,
    Map<String, Object?> observe,
    Response<Object?> response,
    int eventsBefore,
  ) {
    final status = response.statusCode!;
    final types = (scenario['events']! as List).cast<String>();
    final fields = (scenario['event_fields']! as List).cast<String>();
    final body = _picked(response.data, (observe['body']! as List).cast());
    return {
      'status': status,
      if (status >= 400) 'problem': _problem(response.data),
      if (status < 400 && body.isNotEmpty) 'body': body,
      'events': [
        for (final e in server.world.log.skip(eventsBefore))
          if (types.contains(e.type))
            {..._picked(e.envelope, fields), 'type': e.type},
      ],
    };
  }

  Map<String, Object?> _picked(Object? doc, List<String> pointers) => {
    for (final p in pointers)
      if (_at(doc, p) case (final Object? v,)) p: named(v),
  };

  /// The code and the field errors of a problem body, the errors in the order of their
  /// pointers.
  static Map<String, Object?> _problem(Object? body) {
    final map = body is Map ? body : const <Object?, Object?>{};
    final errors = <Map<String, Object?>>[
      for (final e
          in ((map['errors'] as List?) ?? const <Object?>[])
              .cast<Map<Object?, Object?>>())
        {'pointer': e['pointer'], 'code': e['code']},
    ]..sort((a, b) => _key(a).compareTo(_key(b)));
    return {'code': map['code'], if (errors.isNotEmpty) 'errors': errors};
  }

  static String _key(Map<String, Object?> e) => '${e['pointer']} ${e['code']}';

  /// The value at the JSON Pointer [pointer] (RFC 6901) in [doc] as a record, or null when
  /// the pointer leads nowhere.
  static (Object?,)? _at(Object? doc, String pointer) {
    var current = doc;
    if (pointer.isEmpty) return (current,);
    for (final raw in pointer.substring(1).split('/')) {
      final token = raw.replaceAll('~1', '/').replaceAll('~0', '~');
      if (current is Map) {
        if (!current.containsKey(token)) return null;
        current = current[token];
      } else if (current is List) {
        final i = int.tryParse(token);
        if (i == null || i < 0 || i >= current.length) return null;
        current = current[i];
      } else {
        return null;
      }
    }
    return (current,);
  }

  /// Stops the devices and the fake.
  Future<void> stop() async {
    for (final d in devices.values) {
      await d.auth.dispose();
    }
    await server.stop();
  }
}

/// [value] as JSON with the members of every object in the order of their names, so two equal
/// documents give the same text.
String canonical(Object? value) => jsonEncode(_sorted(value));

Object? _sorted(Object? value) => switch (value) {
  final Map<Object?, Object?> m => {
    for (final k in m.keys.cast<String>().toList()..sort()) k: _sorted(m[k]),
  },
  final List<Object?> l => [for (final v in l) _sorted(v)],
  _ => value,
};

void main() {
  final files =
      Directory(scenarioDir)
          .listSync()
          .whereType<File>()
          .where((f) => f.path.endsWith('.json'))
          .toList()
        ..sort((a, b) => a.path.compareTo(b.path));

  test('the folder holds the scenarios the server test records', () {
    expect(files, isNotEmpty);
    for (final f in files) {
      final scenario = jsonDecode(f.readAsStringSync()) as Map<String, Object?>;
      expect(
        '${scenario['rule']}.json',
        f.uri.pathSegments.last,
        reason: 'a scenario is named after its rule',
      );
      expect(
        (scenario['steps']! as List).where(
          (s) => (s! as Map)['observe'] != null,
        ),
        isNotEmpty,
        reason: '${f.path} observes nothing',
      );
    }
  });

  for (final file in files) {
    final scenario =
        jsonDecode(file.readAsStringSync()) as Map<String, Object?>;
    test('the fake answers ${scenario['rule']} as the real server does: '
        '${scenario['summary']}', () async {
      final world = await ConformanceWorld.start();
      addTearDown(world.stop);
      final differences = <String>[];
      for (final raw in scenario['steps']! as List) {
        final step = (raw! as Map).cast<String, Object?>();
        final name = step['name'];
        final before = world.server.world.log.length;
        final response = await world.send(step);
        final observe = step['observe'];
        if (observe == null) {
          expect(
            response.statusCode,
            inInclusiveRange(200, 299),
            reason: 'the preparing step "$name" failed: ${response.data}',
          );
          continue;
        }
        final expected = step['expect'];
        if (expected == null) {
          fail(
            'the step "$name" has no recorded answer: run TestFakeConformance of '
            'services/server/test/fakeconformance with -update-conformance',
          );
        }
        final got = world.observed(
          scenario,
          (observe as Map).cast<String, Object?>(),
          response,
          before,
        );
        if (canonical(got) != canonical(expected)) {
          differences.add(
            '"$name"\n'
            '  real server: ${canonical(expected)}\n'
            '  fake:        ${canonical(got)}',
          );
        }
      }
      expect(
        differences,
        isEmpty,
        reason:
            'the fake differs from the real server in '
            '${differences.length} step(s):\n${differences.join('\n')}',
      );
      // Every request carries a DPoP proof signed in pure Dart, which a loaded machine makes
      // slow; the limit is far above a normal run.
    }, timeout: const Timeout(Duration(minutes: 3)));
  }
}
