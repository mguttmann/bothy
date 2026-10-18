// SPDX-License-Identifier: Apache-2.0

/// The app's outbox executors of the thread view that M4-15 does not have (brief M4-20 Scope;
/// 04 13.2; 15 7.5): read position back, pins, Done and reopen, title, move, personal
/// templates and the stop of a run, plus the draft write that reports `overwrote_newer`.
library;

import 'package:api_client/api_client.dart' as wire show ReadState, Thread;
import 'package:api_client/api_client.dart' hide ReadState, Thread;
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

/// Where the executors put what a confirmed write returned, so the view changes at once and
/// not only with the event that follows (04 13.2 item 6).
final class AppOpsStore {
  /// Creates the store callbacks.
  const AppOpsStore({
    required this.storeThread,
    required this.storeReadState,
    required this.applyPin,
    required this.storeDraftResult,
    this.readVersion,
  });

  /// Stores an API `Thread` as JSON by the version rule (`storeConfirmedThread`).
  final Future<void> Function(Map<String, Object?> thread) storeThread;

  /// Stores a read state the server returned; [reset] for `markThreadUnread` (04 14).
  final Future<void> Function(ReadState state, {required bool reset})
  storeReadState;

  /// Sets the pin state of a message (`pin.*` follows).
  final Future<void> Function(MessageId message, {required bool pinned})
  applyPin;

  /// Stores an API `DraftPutResult` as JSON and reports `overwrote_newer`
  /// (`CatchUpListener.applyDraftPutResult`).
  final Future<void> Function(Map<String, Object?> result) storeDraftResult;

  /// The version of the stored read state of a thread, or null; `markThreadUnread` takes it
  /// when it is sent, so an own `putReadState` ahead of it in the thread's lane does not make
  /// its `If-Match` stale (04 14).
  final Future<ResourceVersion?> Function(ThreadId thread)? readVersion;

  /// The store of [engine] on [db].
  factory AppOpsStore.of(SyncEngine Function() engine, LocalDatabase db) =>
      AppOpsStore(
        storeThread: (t) => engine().catchUp.storeConfirmedThread(t),
        storeReadState: (r, {required reset}) =>
            db.personalDao.applyReadState(r, reset: reset),
        applyPin: (m, {required pinned}) =>
            db.messagesDao.applyPin(m, pinned: pinned),
        storeDraftResult: (r) => engine().catchUp.applyDraftPutResult(r),
        readVersion: (t) async => (await db.personalDao.readState(t))?.version,
      );
}

/// The operations of the thread view and their payloads.
///
/// | Kind | Payload | Thread | Lane |
/// |---|---|---|---|
/// | `markThreadUnread` | `last_read_seq`; `If-Match` from the expected version | yes | thread |
/// | `pinMessage`, `unpinMessage` | `message_id` | yes | thread |
/// | `closeThread`, `reopenThread` | empty | yes | thread |
/// | `setThreadTitle` | `title`; `If-Match` from the expected version | yes | thread |
/// | `moveThread` | `target_channel_id`, `reason` | yes | thread |
/// | `createPersonalTemplate` | `name`, `content_md` | no | `templates` |
/// | `cancelRun` | `run_id`, optional `reason` | no | `run:<id>` |
/// | `putThreadDraft` | `DraftPut` (replaces the executor of M4-15) | yes | thread |
/// | `putReadState` | `ReadStatePut` (replaces the executor of M4-15) | yes | thread |
///
/// Example: `AppMessageOps.register(executors, api, AppOpsStore.of(() => engine, db));`
abstract final class AppMessageOps {
  /// `POST /threads/{thread_id}/read-state/mark-unread`.
  static const String markThreadUnread = 'markThreadUnread';

  /// `PUT /messages/{message_id}/pin`.
  static const String pinMessage = 'pinMessage';

  /// `DELETE /messages/{message_id}/pin`.
  static const String unpinMessage = 'unpinMessage';

  /// `POST /threads/{thread_id}/close`.
  static const String closeThread = 'closeThread';

  /// `POST /threads/{thread_id}/reopen`.
  static const String reopenThread = 'reopenThread';

  /// `PUT /threads/{thread_id}/title`.
  static const String setThreadTitle = 'setThreadTitle';

  /// `POST /threads/{thread_id}/move`.
  static const String moveThread = 'moveThread';

  /// `POST /me/templates`.
  static const String createPersonalTemplate = 'createPersonalTemplate';

  /// `POST /runs/{run_id}/cancel` (Running now of M4-23 and the live box of M4-21).
  static const String cancelRun = 'cancelRun';

  /// Registers the executors on [registry]; they call [api] and hand what the server returned
  /// to [store].
  static void register(
    OutboxExecutors registry,
    ApiClient api,
    AppOpsStore store,
  ) {
    final threads = api.getThreadsApi();
    final messages = api.getMessagesApi();
    final prefs = api.getPreferencesApi();
    Future<OutboxResponse> thread(Future<dynamic> call) async {
      final response = await call;
      final data = response.data;
      if (data is wire.Thread) {
        final json = api.serializers.serializeWith(
          wire.Thread.serializer,
          data,
        );
        if (json is Map) await store.storeThread(json.cast<String, Object?>());
      }
      return OutboxResponse(status: response.statusCode as int?, body: data);
    }

    registry
      ..register(markThreadUnread, (e, ctx) async {
        final body = decodePayload(e.payload);
        final stored = await store.readVersion?.call(e.threadId!);
        final version = stored ?? ResourceVersion(_version(e));
        final response = await prefs.markThreadUnread(
          threadId: _thread(e),
          ifMatch: '"${version.value}"',
          readStatePut: ReadStatePut(
            (b) => b..lastReadSeq = _required<int>(body, 'last_read_seq'),
          ),
          extra: ctx.extra,
        );
        final data = response.data;
        if (data != null) {
          await store.storeReadState(_readState(data), reset: true);
        }
        return OutboxResponse(status: response.statusCode, body: data);
      })
      ..register(pinMessage, (e, ctx) => _pin(messages, store, e, ctx, true))
      ..register(unpinMessage, (e, ctx) => _pin(messages, store, e, ctx, false))
      ..register(
        closeThread,
        (e, ctx) =>
            thread(threads.closeThread(threadId: _thread(e), extra: ctx.extra)),
      )
      ..register(
        reopenThread,
        (e, ctx) => thread(
          threads.reopenThread(threadId: _thread(e), extra: ctx.extra),
        ),
      )
      ..register(setThreadTitle, (e, ctx) {
        final body = decodePayload(e.payload);
        return thread(
          threads.setThreadTitle(
            threadId: _thread(e),
            ifMatch: '"${_version(e)}"',
            threadTitlePut: ThreadTitlePut(
              (b) => b..title = _required<String>(body, 'title'),
            ),
            extra: ctx.extra,
          ),
        );
      })
      ..register(moveThread, (e, ctx) {
        final body = decodePayload(e.payload);
        return thread(
          threads.moveThread(
            threadId: _thread(e),
            threadMove: ThreadMove(
              (b) => b
                ..targetChannelId = _required<String>(body, 'target_channel_id')
                ..reason = _required<String>(body, 'reason'),
            ),
            extra: ctx.extra,
          ),
        );
      })
      ..register(createPersonalTemplate, (e, ctx) async {
        final body = decodePayload(e.payload);
        final response = await api.getMeApi().createPersonalTemplate(
          templateCreate: TemplateCreate(
            (b) => b
              ..name = _required<String>(body, 'name')
              ..contentMd = _required<String>(body, 'content_md'),
          ),
          extra: ctx.extra,
        );
        return OutboxResponse(status: response.statusCode, body: response.data);
      }, lane: (_) => 'templates')
      ..register(cancelRun, (e, ctx) async {
        final body = decodePayload(e.payload);
        final response = await api.getRunsApi().cancelRun(
          runId: _required<String>(body, 'run_id'),
          runCancel: RunCancel((b) => b..reason = body['reason'] as String?),
          extra: ctx.extra,
        );
        return OutboxResponse(status: response.statusCode, body: response.data);
      }, lane: (e) => 'run:${decodePayload(e.payload)['run_id']}')
      ..register(PersonalOps.putReadState, (e, ctx) async {
        final response = await prefs.putReadState(
          threadId: _thread(e),
          readStatePut: api.serializers.deserializeWith(
            ReadStatePut.serializer,
            decodePayload(e.payload),
          )!,
          extra: ctx.extra,
        );
        final data = response.data;
        // The answer carries the new version at once; the echo may come later (04 14).
        if (data != null) {
          await store.storeReadState(_readState(data), reset: false);
        }
        return OutboxResponse(status: response.statusCode, body: data);
      })
      ..register(PersonalOps.putThreadDraft, (e, ctx) async {
        final response = await prefs.putThreadDraft(
          threadId: _thread(e),
          draftPut: api.serializers.deserializeWith(
            DraftPut.serializer,
            decodePayload(e.payload),
          )!,
          extra: ctx.extra,
        );
        final data = response.data;
        if (data != null) {
          final json = api.serializers.serializeWith(
            DraftPutResult.serializer,
            data,
          );
          // 04 14: the answer reports `overwrote_newer`; the app shows a notice.
          if (json is Map) {
            await store.storeDraftResult(json.cast<String, Object?>());
          }
        }
        return OutboxResponse(status: response.statusCode, body: data);
      });
  }

  static Future<OutboxResponse> _pin(
    MessagesApi messages,
    AppOpsStore store,
    OutboxEntry e,
    ApiCallContext ctx,
    bool pinned,
  ) async {
    final id = _required<String>(decodePayload(e.payload), 'message_id');
    final response = pinned
        ? await messages.pinMessage(messageId: id, extra: ctx.extra)
        : await messages.unpinMessage(messageId: id, extra: ctx.extra);
    await store.applyPin(MessageId.parse(id), pinned: pinned);
    return OutboxResponse(status: response.statusCode);
  }

  static ReadState _readState(wire.ReadState r) => ReadState(
    threadId: ThreadId.parse(r.threadId),
    channelId: ChannelId.parse(r.channelId),
    lastReadSeq: ThreadSeq(r.lastReadSeq),
    mentionCount: r.mentionCount,
    version: ResourceVersion(r.version),
    updatedByDeviceId: r.updatedByDeviceId == null
        ? null
        : DeviceId.parse(r.updatedByDeviceId!),
    updatedAt: r.updatedAt,
  );

  /// Puts a write of [kind] on [thread] with [payload] into the outbox of [engine];
  /// [expectedVersion] is the `If-Match` of `markThreadUnread` and `setThreadTitle`.
  static Future<OutboxEntry> enqueue(
    SyncEngine engine,
    String kind, {
    ThreadId? thread,
    Map<String, Object?> payload = const {},
    ResourceVersion? expectedVersion,
  }) => engine.outbox.enqueue(
    OutboxRequest(
      kind: kind,
      threadId: thread,
      payload: payload,
      expectedVersion: expectedVersion,
    ),
  );
}

String _thread(OutboxEntry e) =>
    e.threadId?.value ?? (throw StateError('${e.kind} needs a thread'));

int _version(OutboxEntry e) =>
    e.expectedVersion?.value ??
    (throw StateError('${e.kind} needs an expected version'));

T _required<T extends Object>(Map<String, Object?> body, String key) {
  final value = body[key];
  if (value is! T) throw StateError('member $key is missing');
  return value;
}
