// SPDX-License-Identifier: Apache-2.0

/// The write of the personal sidebar: the order of a space through the outbox
/// (`putMySidebarSpace`; components.md 7; 04 14; brief M4-19 Design note 2). The collapse of a
/// space stays on its device (01 6.7, R-PR-24) and is never written here.
library;

import 'package:api_client/api_client.dart';
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../sidebar/sidebar_order.dart';

/// The outbox operation `putMySidebarSpace` and its payload.
///
/// | Kind | Payload | Lane |
/// |---|---|---|
/// | `putMySidebarSpace` | `space_id`, `collapsed`, optional `sort_key` | `sidebar:<space_id>` |
///
/// Writes of one space run in order (the last one wins, 04 14), different spaces in parallel.
/// The `Idempotency-Key` is the entry's ID (06 7).
///
/// Example: `SidebarOps.register(executors, api, db);`
abstract final class SidebarOps {
  /// `PUT /me/sidebar/spaces/{space_id}`.
  static const String putMySidebarSpace = 'putMySidebarSpace';

  /// Registers the executor on [registry]; it calls [api] and stores the server's state in
  /// [db].
  ///
  /// The answer (API `SidebarState`, with the new version) is stored by the version rule, so
  /// a later change of another device wins and an older one does not (04 14); the echo of
  /// this device is ignored by `updated_by_device_id` (M4-14). While a later write of the
  /// same space waits in the outbox, the answer is not stored: that write's answer follows,
  /// and the row keeps the newer local state meanwhile.
  static void register(
    OutboxExecutors registry,
    ApiClient api,
    LocalDatabase db,
  ) {
    final prefs = api.getPreferencesApi();
    registry.register(putMySidebarSpace, (e, ctx) async {
      final body = decodePayload(e.payload);
      final space = _spaceOf(body);
      final sortKey = body['sort_key'];
      final response = await prefs.putMySidebarSpace(
        spaceId: space,
        sidebarStatePut: SidebarStatePut((b) {
          b.collapsed = body['collapsed'] == true;
          if (sortKey is String) b.sortKey = sortKey;
        }),
        extra: ctx.extra,
      );
      final state = response.data;
      if (state != null && !await _laterWriteWaits(db, e)) {
        final json = api.serializers.serializeWith(
          SidebarState.serializer,
          state,
        );
        if (json is Map) {
          await db.structureDao.upsertUserSpaceStateIfNewer(
            sidebarFromJson(json.cast<String, Object?>()),
          );
        }
      }
      return OutboxResponse(status: response.statusCode, body: state);
    }, lane: (e) => 'sidebar:${_spaceOf(decodePayload(e.payload))}');
  }

  static Future<bool> _laterWriteWaits(LocalDatabase db, OutboxEntry e) async {
    final space = _spaceOf(decodePayload(e.payload));
    for (final other in await db.outboxDao.allEntries()) {
      if (other.id == e.id ||
          other.kind != putMySidebarSpace ||
          other.state == OutboxState.confirmed ||
          other.createdAt.isBefore(e.createdAt)) {
        continue;
      }
      if (_spaceOf(decodePayload(other.payload)) == space) return true;
    }
    return false;
  }

  static String _spaceOf(Map<String, Object?> body) {
    final space = body['space_id'];
    if (space is! String) throw StateError('member space_id is missing');
    return space;
  }
}

/// Writes the person's sidebar order: the local row changes at once, the write goes through the
/// outbox (brief M4-19 Design note 2). Each write sends the `collapsed` the server holds, so an
/// order write never changes the collapse of another device (01 6.7, R-PR-24).
///
/// Example:
/// ```dart
/// final writer = SidebarWriter(engine, db);
/// await writer.move(moveSpace(order, entry.space.id, 0));
/// ```
final class SidebarWriter {
  /// Creates the writer over the outbox of [engine] and the local database [db].
  SidebarWriter(this._engine, this._db);

  final SyncEngine _engine;
  final LocalDatabase _db;

  /// Stores the personal keys of [writes] (from `moveSpace`).
  Future<void> move(List<SpaceOrderWrite> writes) async {
    for (final w in writes) {
      await _write(w.spaceId, collapsed: w.collapsed, sortKey: w.sortKey);
    }
  }

  Future<void> _write(
    SpaceId space, {
    required bool collapsed,
    String? sortKey,
  }) async {
    await _db.structureDao.putUserSpaceStateLocally(
      space,
      collapsed: collapsed,
      sortKey: sortKey,
    );
    await _engine.outbox.enqueue(
      OutboxRequest(
        kind: SidebarOps.putMySidebarSpace,
        payload: {
          'space_id': space.value,
          'collapsed': collapsed,
          'sort_key': ?sortKey,
        },
      ),
    );
  }
}
