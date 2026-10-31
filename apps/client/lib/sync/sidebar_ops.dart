// SPDX-License-Identifier: Apache-2.0

/// The write of the personal sidebar: the order of a space and the own channel order of a space
/// (`putMySidebarSpace`) and the own thread order of a channel (`putMySidebarChannel`, M4-51)
/// through the outbox (components.md 7; 04 14; brief M4-19 Design note 2). The collapse of a
/// space stays on its device (01 6.7, R-PR-24) and is never written here.
library;

import 'package:api_client/api_client.dart';
import 'package:built_collection/built_collection.dart';
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../sidebar/sidebar_order.dart';

/// The outbox operations of the sidebar and their payloads.
///
/// | Kind | Payload | Lane |
/// |---|---|---|
/// | `putMySidebarSpace` | `space_id`, `collapsed`, optional `sort_key`, optional `channel_order` | `sidebar:<space_id>` |
/// | `putMySidebarChannel` | `channel_id`, `thread_order` | `sidebar-channel:<channel_id>` |
///
/// Writes of one space or one channel run in order (the last one wins, 04 14), different ones
/// in parallel. The `Idempotency-Key` is the entry's ID (06 7).
///
/// Example: `SidebarOps.register(executors, api, db);`
abstract final class SidebarOps {
  /// `PUT /me/sidebar/spaces/{space_id}`.
  static const String putMySidebarSpace = 'putMySidebarSpace';

  /// `PUT /me/sidebar/channels/{channel_id}` (M4-51).
  static const String putMySidebarChannel = 'putMySidebarChannel';

  /// Registers the executors on [registry]; they call [api] and store the server's state in
  /// [db].
  ///
  /// The answer (API `SidebarState` or `SidebarChannelState`, with the new version) is stored
  /// by the version rule, so a later change of another device wins and an older one does not
  /// (04 14); the echo of this device is ignored by `updated_by_device_id` (M4-14). While a
  /// later write of the same space or channel waits in the outbox, the answer is not stored:
  /// that write's answer follows, and the row keeps the newer local state meanwhile.
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
      final channelOrder = body['channel_order'];
      final response = await prefs.putMySidebarSpace(
        spaceId: space,
        sidebarStatePut: SidebarStatePut((b) {
          b.collapsed = body['collapsed'] == true;
          if (sortKey is String) b.sortKey = sortKey;
          if (channelOrder is List) {
            b.channelOrder = ListBuilder(channelOrder.cast<String>());
          }
        }),
        extra: ctx.extra,
      );
      final state = response.data;
      if (state != null &&
          !await _laterWriteWaits(db, e, putMySidebarSpace, _spaceOf)) {
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
    registry.register(putMySidebarChannel, (e, ctx) async {
      final body = decodePayload(e.payload);
      final order = body['thread_order'];
      final response = await prefs.putMySidebarChannel(
        channelId: _channelOf(body),
        sidebarChannelPut: SidebarChannelPut(
          (b) => b.threadOrder = ListBuilder(
            order is List ? order.cast<String>() : const <String>[],
          ),
        ),
        extra: ctx.extra,
      );
      final state = response.data;
      if (state != null &&
          !await _laterWriteWaits(db, e, putMySidebarChannel, _channelOf)) {
        final json = api.serializers.serializeWith(
          SidebarChannelState.serializer,
          state,
        );
        if (json is Map) {
          await db.structureDao.upsertUserChannelStateIfNewer(
            sidebarChannelFromJson(json.cast<String, Object?>()),
          );
        }
      }
      return OutboxResponse(status: response.statusCode, body: state);
    }, lane: (e) => 'sidebar-channel:${_channelOf(decodePayload(e.payload))}');
  }

  /// Whether a write of [kind] for the same target ([targetOf]) as [e] waits behind it.
  static Future<bool> _laterWriteWaits(
    LocalDatabase db,
    OutboxEntry e,
    String kind,
    String Function(Map<String, Object?>) targetOf,
  ) async {
    final target = targetOf(decodePayload(e.payload));
    for (final other in await db.outboxDao.allEntries()) {
      if (other.id == e.id ||
          other.kind != kind ||
          other.state == OutboxState.confirmed ||
          other.createdAt.isBefore(e.createdAt)) {
        continue;
      }
      if (targetOf(decodePayload(other.payload)) == target) return true;
    }
    return false;
  }

  static String _channelOf(Map<String, Object?> body) {
    final channel = body['channel_id'];
    if (channel is! String) throw StateError('member channel_id is missing');
    return channel;
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

  /// Stores the own [order] of the channels and forums of [space] (M4-51): the local row at
  /// once, then `putMySidebarSpace` with [collapsed] (the value the server holds) and
  /// `channel_order`. An empty [order] returns to the default order.
  Future<void> orderChannels(
    SpaceId space,
    List<ChannelId> order, {
    required bool collapsed,
  }) => _write(space, collapsed: collapsed, channelOrder: order);

  /// Stores the own [order] of the threads of [channel] (M4-51): the local row at once, then
  /// `putMySidebarChannel` with `thread_order`. An empty [order] returns to the default order.
  Future<void> orderThreads(ChannelId channel, List<ThreadId> order) async {
    await _db.structureDao.putUserChannelStateLocally(channel, order);
    await _engine.outbox.enqueue(
      OutboxRequest(
        kind: SidebarOps.putMySidebarChannel,
        payload: {
          'channel_id': channel.value,
          'thread_order': [for (final t in order) t.value],
        },
      ),
    );
  }

  Future<void> _write(
    SpaceId space, {
    required bool collapsed,
    String? sortKey,
    List<ChannelId>? channelOrder,
  }) async {
    await _db.structureDao.putUserSpaceStateLocally(
      space,
      collapsed: collapsed,
      sortKey: sortKey,
      channelOrder: channelOrder,
    );
    await _engine.outbox.enqueue(
      OutboxRequest(
        kind: SidebarOps.putMySidebarSpace,
        payload: {
          'space_id': space.value,
          'collapsed': collapsed,
          'sort_key': ?sortKey,
          if (channelOrder != null)
            'channel_order': [for (final c in channelOrder) c.value],
        },
      ),
    );
  }
}
