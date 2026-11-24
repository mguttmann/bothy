// SPDX-License-Identifier: Apache-2.0

// `ApiPushRegistrations.state` (docs/spec/13-push-notifications.md 13,
// docs/decisions.d/FIX-PUSH-RENEW.md): it reads `getDevice` of the session's device and maps the
// member `push_registrations` onto active, disabled with its reason, or missing.

import 'dart:convert';
import 'dart:typed_data';

import 'package:api_client/api_client.dart' as rest;
import 'package:client/platform/push_platform.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

const _device = '01890a5d-ac96-774b-bcce-b302099a8057';
const _active = '01890a5d-ac96-774b-bcce-b302099a8001';
const _disabled = '01890a5d-ac96-774b-bcce-b302099a8002';
const _gone = '01890a5d-ac96-774b-bcce-b302099a8003';

/// Answers every request with [body] and records the paths.
class _DeviceAdapter implements HttpClientAdapter {
  _DeviceAdapter(this.body);

  final Map<String, Object?> body;
  final List<String> paths = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    paths.add('${options.method} ${options.uri.path}');
    return ResponseBody.fromString(
      jsonEncode(body),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

Map<String, Object?> _registration(String id, {String? disabledReason}) => {
  'id': id,
  'device_id': _device,
  'provider': 'apns',
  'environment': 'sandbox',
  'app_id': 'com.example.chat',
  'created_at': '2026-11-01T10:00:00.000Z',
  if (disabledReason != null) ...{
    'disabled_at': '2026-11-02T10:00:00.000Z',
    'disabled_reason': disabledReason,
  },
};

void main() {
  late _DeviceAdapter adapter;
  late ApiPushRegistrations registrations;

  setUp(() {
    adapter = _DeviceAdapter({
      'id': _device,
      'user_id': '01890a5d-ac96-774b-bcce-b302099a8100',
      'name': 'Alex phone',
      'platform': 'ios',
      'state': 'active',
      'created_at': '2026-11-01T09:00:00.000Z',
      'push_registrations': [
        _registration(_active),
        _registration(_disabled, disabledReason: 'repeated_failures'),
      ],
    });
    registrations = ApiPushRegistrations(
      rest.ApiClient(
        dio: Dio(BaseOptions(baseUrl: 'https://chat.example.com'))
          ..httpClientAdapter = adapter,
      ),
      () => 'key',
      () async => _device,
    );
  });

  test('an active registration of the device', () async {
    final state = await registrations.state(_active);
    expect(adapter.paths, ['GET /devices/$_device']);
    expect(state.exists, isTrue);
    expect(state.disabledReason, isNull);
    expect(state.renew, isFalse);
  });

  test('a registration disabled after transient failures renews', () async {
    final state = await registrations.state(_disabled);
    expect(state.disabledReason, 'repeated_failures');
    expect(state.renew, isTrue);
  });

  test('a registration the device no longer has renews', () async {
    final state = await registrations.state(_gone);
    expect(state.exists, isFalse);
    expect(state.renew, isTrue);
  });
}
