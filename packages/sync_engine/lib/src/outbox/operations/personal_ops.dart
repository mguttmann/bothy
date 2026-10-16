// SPDX-License-Identifier: Apache-2.0

/// Executors of the personal writes: read state, thread drafts and own settings (06 7: each
/// with the entry's ID as `Idempotency-Key`).
library;

import 'package:api_client/api_client.dart';
import 'package:local_store/local_store.dart';

import '../executors.dart';

/// The personal operations of the outbox and their payloads.
///
/// | Kind | Payload | Lane |
/// |---|---|---|
/// | `putReadState` | `ReadStatePut` | thread |
/// | `putThreadDraft` | `DraftPut` | thread |
/// | `deleteThreadDraft` | empty | thread |
/// | `putMySetting` | `key`, `value` | `setting:<key>` |
/// | `deleteMySetting` | `key` | `setting:<key>` |
///
/// Example: `PersonalOps.register(executors, api);`
abstract final class PersonalOps {
  /// `PUT /threads/{thread_id}/read-state`.
  static const String putReadState = 'putReadState';

  /// `PUT /threads/{thread_id}/draft`.
  static const String putThreadDraft = 'putThreadDraft';

  /// `DELETE /threads/{thread_id}/draft`.
  static const String deleteThreadDraft = 'deleteThreadDraft';

  /// `PUT /me/settings/{key}`.
  static const String putMySetting = 'putMySetting';

  /// `DELETE /me/settings/{key}`.
  static const String deleteMySetting = 'deleteMySetting';

  /// Registers the five executors on [registry]; they call [api].
  static void register(OutboxExecutors registry, ApiClient api) {
    final prefs = api.getPreferencesApi();
    registry
      ..register(putReadState, (e, ctx) async {
        final response = await prefs.putReadState(
          threadId: _thread(e),
          readStatePut: api.serializers.deserializeWith(
            ReadStatePut.serializer,
            decodePayload(e.payload),
          )!,
          extra: ctx.extra,
        );
        return OutboxResponse(status: response.statusCode, body: response.data);
      })
      ..register(putThreadDraft, (e, ctx) async {
        final response = await prefs.putThreadDraft(
          threadId: _thread(e),
          draftPut: api.serializers.deserializeWith(
            DraftPut.serializer,
            decodePayload(e.payload),
          )!,
          extra: ctx.extra,
        );
        return OutboxResponse(status: response.statusCode, body: response.data);
      })
      ..register(deleteThreadDraft, (e, ctx) async {
        final response = await prefs.deleteThreadDraft(
          threadId: _thread(e),
          extra: ctx.extra,
        );
        return OutboxResponse(status: response.statusCode);
      })
      ..register(putMySetting, (e, ctx) async {
        final body = decodePayload(e.payload);
        final response = await prefs.putMySetting(
          key: _key(body),
          userSettingPut: api.serializers.deserializeWith(
            UserSettingPut.serializer,
            {'value': body['value']},
          )!,
          extra: ctx.extra,
        );
        return OutboxResponse(status: response.statusCode, body: response.data);
      }, lane: _settingLane)
      ..register(deleteMySetting, (e, ctx) async {
        final response = await prefs.deleteMySetting(
          key: _key(decodePayload(e.payload)),
          extra: ctx.extra,
        );
        return OutboxResponse(status: response.statusCode);
      }, lane: _settingLane);
  }

  /// Writes of one setting run in order; different settings in parallel (Design note 3).
  static String _settingLane(OutboxEntry e) =>
      'setting:${decodePayload(e.payload)['key']}';
}

String _thread(OutboxEntry e) =>
    e.threadId?.value ?? (throw StateError('${e.kind} needs a thread'));

String _key(Map<String, Object?> body) {
  final key = body['key'];
  if (key is! String) throw StateError('member key is missing');
  return key;
}
