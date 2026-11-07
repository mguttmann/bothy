// SPDX-License-Identifier: Apache-2.0

/// The outbox executors of autonomous runs (01 6.21; 04 13.2; 15 7.5; brief AR-08): the cancel
/// of a pending wake-up (`run.wake.cancel`, 07 2.14 item 5) and the channel switch
/// `allow_autonomous_runs` (`channel.autonomous_runs.set`, 07 2.14 item 1). What the server
/// answers goes into the local tables at once, before `run.wake_changed` or `channel.updated`
/// arrives (04 13.2 item 6).
library;

import 'package:api_client/api_client.dart' as wire;
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

/// The operations of autonomous runs and their payloads.
///
/// | Kind | Payload | Thread | Lane |
/// |---|---|---|---|
/// | `cancelRunWake` | `wake_id` | no | `wake:<id>` |
/// | `setChannelAutonomousRuns` | `channel_id`, `allow_autonomous_runs`; `If-Match` from the expected version | no | `channel:<id>` |
///
/// The entry's ID is the `Idempotency-Key` of the cancel, so a retry cancels once (04 5).
///
/// Example: `WakeOps.register(executors, api, database);`
abstract final class WakeOps {
  /// `POST /wakes/{wake_id}/cancel`.
  static const String cancelRunWake = 'cancelRunWake';

  /// `PUT /channels/{channel_id}/autonomous-runs`.
  static const String setChannelAutonomousRuns = 'setChannelAutonomousRuns';

  /// Registers the executors on [registry]; they call [api] and store the answer in [db].
  static void register(
    OutboxExecutors registry,
    wire.ApiClient api,
    LocalDatabase db,
  ) {
    registry
      ..register(cancelRunWake, (e, ctx) async {
        final body = decodePayload(e.payload);
        final response = await api.getRunsApi().cancelRunWake(
          wakeId: _required<String>(body, 'wake_id'),
          runWakeCancel: wire.RunWakeCancel(),
          extra: ctx.extra,
        );
        final data = response.data;
        if (data != null) {
          final json = api.serializers.serializeWith(
            wire.RunWake.serializer,
            data,
          );
          if (json is Map) {
            await db.runsDao.upsertWakeIfNewer(
              wakeFromJson(json.cast<String, Object?>()),
            );
          }
        }
        return OutboxResponse(status: response.statusCode, body: data);
      }, lane: (e) => 'wake:${decodePayload(e.payload)['wake_id']}')
      ..register(setChannelAutonomousRuns, (e, ctx) async {
        final body = decodePayload(e.payload);
        final version =
            e.expectedVersion?.value ??
            (throw StateError('${e.kind} needs an expected version'));
        final response = await api.getChannelsApi().setChannelAutonomousRuns(
          channelId: _required<String>(body, 'channel_id'),
          ifMatch: '"$version"',
          channelAutonomousRunsSet: wire.ChannelAutonomousRunsSet(
            (b) => b
              ..allowAutonomousRuns = _required<bool>(
                body,
                'allow_autonomous_runs',
              ),
          ),
          extra: ctx.extra,
        );
        final data = response.data;
        if (data != null) {
          final json = api.serializers.serializeWith(
            wire.Channel.serializer,
            data,
          );
          if (json is Map) {
            await db.structureDao.upsertChannelIfNewer(
              channelFromJson(json.cast<String, Object?>()),
            );
          }
        }
        return OutboxResponse(status: response.statusCode, body: data);
      }, lane: (e) => 'channel:${decodePayload(e.payload)['channel_id']}');
  }
}

T _required<T extends Object>(Map<String, Object?> body, String key) {
  final value = body[key];
  if (value is! T) throw StateError('member $key is missing');
  return value;
}

/// Puts the cancel of [wake] into the outbox of [engine] (no thread: the lane of the wake-up).
Future<OutboxEntry> enqueueWakeCancel(SyncEngine engine, WakeId wake) =>
    engine.outbox.enqueue(
      OutboxRequest(
        kind: WakeOps.cancelRunWake,
        payload: {'wake_id': wake.value},
      ),
    );

/// Puts the switch of [channel] to [allowed] into the outbox of [engine], with the stored
/// channel version as `If-Match`.
Future<OutboxEntry> enqueueAutonomousRuns(
  SyncEngine engine,
  ChannelId channel,
  ResourceVersion version, {
  required bool allowed,
}) => engine.outbox.enqueue(
  OutboxRequest(
    kind: WakeOps.setChannelAutonomousRuns,
    payload: {'channel_id': channel.value, 'allow_autonomous_runs': allowed},
    expectedVersion: version,
  ),
);
