// SPDX-License-Identifier: Apache-2.0

// The REST source of the area "Sign-in" (V1-14): getMyCredentials maps to the types of admin_ui,
// setMyLocalCredential sends exactly the members of each step with the step-up purpose,
// the passkey registration hands the server's creation options to the platform and its
// credential back with the name, a cancel at the platform sends no completion, and a problem
// answer becomes SelfActionFailed with its field errors. A handler adapter answers each request.

import 'dart:convert';
import 'dart:typed_data';

import 'package:admin_ui/admin_ui.dart';
import 'package:api_client/api_client.dart';
import 'package:auth/auth.dart' show stepUpPurposeExtra;
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

typedef Sent = ({RequestOptions options, Object? json});

class HandlerAdapter implements HttpClientAdapter {
  HandlerAdapter(this.answer);

  (int, Object?) Function(RequestOptions options) answer;
  final List<Sent> sent = [];

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
    final body = utf8.decode(bytes);
    sent.add((options: options, json: body.isEmpty ? null : jsonDecode(body)));
    final (status, json) = answer(options);
    return ResponseBody.fromString(
      jsonEncode(json),
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

const String _at = '2026-10-10T09:30:00.000Z';
const String _challenge = '0192f4a0-7b1c-747c-835d-13eca158d3c0';

void main() {
  late HandlerAdapter adapter;
  late ApiSignInRepository repository;

  setUp(() {
    adapter = HandlerAdapter((o) => (500, null));
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
    repository = ApiSignInRepository(client);
  });

  test(
    'getMyCredentials maps identities, passkeys and the password state',
    () async {
      adapter.answer = (o) => (
        200,
        {
          'identities': [
            {
              'id': '0192f4a0-7b1c-7dd2-83b9-528d363b749b',
              'provider_id': '0192f4a0-7b1c-7b3c-87cd-dea30414c03e',
              'provider_kind': 'local',
              'subject': 'alex',
              'linked_at': _at,
            },
            {
              'id': '0192f4a0-7b1c-7dd2-83b9-528d363b749c',
              'provider_id': '0192f4a0-7b1c-7b3c-87cd-dea30414c03f',
              'provider_kind': 'trusted_header',
              'subject': 'alex@example.com',
              'linked_at': _at,
            },
          ],
          'passkeys': [
            {
              'id': '0192f4a0-7b1c-7ad4-8a99-f31c00005918',
              'name': 'Laptop',
              'backup_eligible': true,
              'created_at': _at,
            },
          ],
          'local_password_set': true,
          'totp_enrolled': false,
        },
      );
      final c = await repository.credentials();
      expect(adapter.sent.single.options.uri.path, '/api/v1/me/credentials');
      expect(c.identities.map((i) => i.providerKind), [
        'local',
        'trusted_header',
      ]);
      expect(c.hasLocalIdentity, isTrue);
      expect(c.passkeys.single.name, 'Laptop');
      expect(c.passkeys.single.backupEligible, isTrue);
      expect(c.passwordSet, isTrue);
      expect(c.totpEnrolled, isFalse);
    },
  );

  test(
    'the first step sends password and totp_enroll and returns the URI',
    () async {
      adapter.answer = (o) => (
        200,
        {
          'password_set': true,
          'totp_enabled': false,
          'totp_secret_uri': 'otpauth://totp/x:alex?secret=JBSWY3DPEHPK3PXP',
        },
      );
      final uri = await repository.setPassword(
        'correct horse battery',
        enrollTotp: true,
        purpose: 'Set the password',
      );
      expect(uri, 'otpauth://totp/x:alex?secret=JBSWY3DPEHPK3PXP');
      final sent = adapter.sent.single;
      expect(sent.options.method, 'PUT');
      expect(sent.options.uri.path, '/api/v1/me/local-credential');
      expect(sent.json, {
        'password': 'correct horse battery',
        'totp_enroll': true,
      });
      expect(sent.options.extra[stepUpPurposeExtra], 'Set the password');
    },
  );

  test(
    'a password change sends the password alone, a new code totp_enroll alone',
    () async {
      adapter.answer = (o) =>
          (200, {'password_set': true, 'totp_enabled': true});
      expect(
        await repository.setPassword(
          'new one',
          enrollTotp: false,
          purpose: 'p',
        ),
        isNull,
      );
      adapter.answer = (o) => (
        200,
        {
          'password_set': true,
          'totp_enabled': true,
          'totp_secret_uri': 'otpauth://totp/x:alex?secret=AAAA',
        },
      );
      await repository.setPassword(null, enrollTotp: true, purpose: 'p');
      expect(adapter.sent.map((s) => s.json), [
        {'password': 'new one'},
        {'totp_enroll': true},
      ]);
    },
  );

  test('confirmTotp sends the code without spaces', () async {
    adapter.answer = (o) => (200, {'password_set': true, 'totp_enabled': true});
    await repository.confirmTotp('492 039', purpose: 'p');
    expect(adapter.sent.single.json, {'totp_code': '492039'});
  });

  test('a refused code is SelfActionFailed with its field error', () async {
    adapter.answer = (o) => (
      400,
      {
        'type': 'about:blank',
        'code': 'validation_failed',
        'status': 400,
        'errors': [
          {'pointer': '/totp_code', 'code': 'totp_invalid'},
        ],
      },
    );
    await expectLater(
      repository.confirmTotp('000000', purpose: 'p'),
      throwsA(
        isA<SelfActionFailed>().having(
          (e) => e.problem?.errors.single.code,
          'field code',
          'totp_invalid',
        ),
      ),
    );
  });

  test('a passkey goes from the creation options through the platform to the server', () async {
    adapter.answer = (o) => switch (o.uri.path) {
      '/api/v1/me/passkeys/begin' => (
        200,
        {
          'challenge_id': _challenge,
          'public_key': {
            'challenge': 'Zm9v',
            'rp': {'id': 'chat.example.com', 'name': 'Chat'},
          },
          'expires_at': _at,
        },
      ),
      _ => (
        201,
        {
          'id': '0192f4a0-7b1c-7ad4-8a99-f31c00005918',
          'name': 'Work laptop',
          'backup_eligible': false,
          'created_at': _at,
        },
      ),
    };
    Map<String, Object?>? options;
    final added = await repository.addPasskey(
      'Work laptop',
      create: (o) async {
        options = o;
        return {'id': 'cred', 'type': 'public-key'};
      },
      purpose: 'Add a passkey',
    );
    expect(options, {
      'challenge': 'Zm9v',
      'rp': {'id': 'chat.example.com', 'name': 'Chat'},
    });
    expect(added?.name, 'Work laptop');
    final complete = adapter.sent.last;
    expect(complete.options.uri.path, '/api/v1/me/passkeys/complete');
    expect(complete.json, {
      'challenge_id': _challenge,
      'credential': {'id': 'cred', 'type': 'public-key'},
      'name': 'Work laptop',
    });
    for (final s in adapter.sent) {
      expect(s.options.extra[stepUpPurposeExtra], 'Add a passkey');
    }
  });

  test('a cancel at the platform sends no completion', () async {
    adapter.answer = (o) => (
      200,
      {
        'challenge_id': _challenge,
        'public_key': <String, Object?>{},
        'expires_at': _at,
      },
    );
    final added = await repository.addPasskey(
      'Laptop',
      create: (_) async => null,
      purpose: 'p',
    );
    expect(added, isNull);
    expect(adapter.sent.map((s) => s.options.uri.path), [
      '/api/v1/me/passkeys/begin',
    ]);
  });
}
