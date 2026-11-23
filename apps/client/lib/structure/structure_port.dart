// SPDX-License-Identifier: Apache-2.0

/// Creating a space and a plain text channel (01 6.25): `createSpace` (action `space.create`,
/// owners) and `createChannel` with kind `text` (action `channel.create`, `manage` on the
/// space) as the client's port. Both are online operations with an `Idempotency-Key`, never
/// outbox entries: the dialog shows a refusal at once and opens the new channel with its ID
/// (docs/decisions.d/FEAT-SPACE-CHANNEL-CREATE.md). Tests replace the port.
library;

import 'package:api_client/api_client.dart' as wire;
import 'package:dio/dio.dart';
import 'package:domain/domain.dart';

/// A refused or failed create: the server's problem [code] and [reason], or [offline] without
/// an answer.
final class StructureFailure implements Exception {
  /// A problem of the server.
  const StructureFailure.problem(String this.code, {this.reason})
    : offline = false;

  /// No answer (offline, timeout).
  const StructureFailure.offline() : code = null, reason = null, offline = true;

  /// The problem code, null when [offline].
  final String? code;

  /// The problem's `reason`, if any.
  final String? reason;

  /// Whether no answer arrived.
  final bool offline;

  /// Whether the server may still carry out the request under its key: no answer, or
  /// `409 request_in_progress` while the first request with the key still runs (06 4). A
  /// repeated press then sends the same key again.
  bool get keepsKey => offline || code == 'request_in_progress';

  @override
  String toString() => offline
      ? 'StructureFailure(offline)'
      : 'StructureFailure($code, $reason)';
}

/// The creates of the signed-in person. Every method throws [StructureFailure] when the server
/// refuses or does not answer.
abstract interface class StructurePort {
  /// Creates the space [name] (`createSpace`) and answers its ID; [idempotencyKey] makes a
  /// repeated press after a lost answer the same request.
  Future<SpaceId> createSpace(String name, {required String idempotencyKey});

  /// Creates the text channel [name] in [space] (`createChannel`, kind `text`) and answers its
  /// ID; [idempotencyKey] makes a repeated press after a lost answer the same request.
  Future<ChannelId> createTextChannel(
    SpaceId space,
    String name, {
    required String idempotencyKey,
  });
}

/// [StructurePort] over the generated client of the signed-in session.
final class ApiStructurePort implements StructurePort {
  /// Creates the port over [_api].
  ApiStructurePort(this._api);

  final wire.ApiClient _api;

  @override
  Future<SpaceId> createSpace(String name, {required String idempotencyKey}) =>
      _guard(() async {
        final created = (await _api.getSpacesApi().createSpace(
          idempotencyKey: idempotencyKey,
          spaceCreate: wire.SpaceCreate((b) => b..name = name),
        )).data!;
        return SpaceId.parse(created.id);
      });

  @override
  Future<ChannelId> createTextChannel(
    SpaceId space,
    String name, {
    required String idempotencyKey,
  }) => _guard(() async {
    final created = (await _api.getChannelsApi().createChannel(
      spaceId: space.value,
      idempotencyKey: idempotencyKey,
      channelCreate: wire.ChannelCreate(
        (b) => b
          ..kind = wire.ChannelCreateKindEnum.text
          ..name = name,
      ),
    )).data!;
    return ChannelId.parse(created.id);
  });

  static Future<T> _guard<T>(Future<T> Function() call) async {
    try {
      return await call();
    } on DioException catch (e) {
      final problem = e.response == null ? null : wire.ApiProblem.of(e);
      if (problem == null) throw const StructureFailure.offline();
      throw StructureFailure.problem(problem.code, reason: problem.reason);
    }
  }
}
