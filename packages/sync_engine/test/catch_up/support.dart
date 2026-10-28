// SPDX-License-Identifier: Apache-2.0

// The world of the catch-up tests (brief M4-14, Tests): the gateway connection with the fake
// transport and clock of M4-13, the catch-up listener on the in-memory local store, and
// `ApiFetcher` over `api_client` with a fake REST adapter that answers recorded responses
// (the shapes of `docs/api/openapi.yaml`). People: Alex (this device), Sam, Robin (agent).

import 'package:api_client/api_client.dart';
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:sync_engine/sync_engine.dart';

import '../support/fakes.dart';

export '../support/fakes.dart';

/// A UUID whose last group is [n] in hex.
String id(int n) =>
    '0192f4a0-7b1c-78f9-8074-${n.toRadixString(16).padLeft(12, '0')}';

/// Alex, the person on this device (the `userId` of the gateway fakes).
const String alexId = userId;

/// Sam, another person.
final String samId = id(0xa2);

/// Robin, an agent.
final String robinId = id(0xa3);

/// This device.
DeviceId get thisDevice => DeviceId.parse(deviceId);

/// Another device of Alex.
final String otherDevice = id(0xd2);

/// Space [n].
String spaceId(int n) => id(0x100 + n);

/// Channel [n].
String channelId(int n) => id(0x200 + n);

/// Thread [n].
String threadId(int n) => id(0x300 + n);

/// Message [seq] of thread [threadN].
String messageId(int threadN, int seq) => id(0x10000 * threadN + seq);

/// Draft [n].
String draftId(int n) => id(0x500 + n);

/// A fixed time of the tests.
const String at = '2026-10-15T10:00:00.000Z';

// ---------------------------------------------------------------------------------------------
// Recorded objects (docs/api/openapi.yaml)

/// An API `PrincipalRef`.
Map<String, Object?> principalRef(String principal, {String kind = 'user'}) => {
  'id': principal,
  'kind': kind,
  'handle': principal == alexId ? 'alex' : 'sam',
  'display_name': principal == alexId ? 'Alex' : 'Sam',
};

/// An API `Space`.
Map<String, Object?> spaceJson(int n, {int version = 1}) => {
  'id': spaceId(n),
  'name': 'Space $n',
  'sort_key': 'a$n',
  'version': version,
  'created_at': at,
};

/// An API `Channel` of space [spaceN].
Map<String, Object?> channelJson(
  int n, {
  int spaceN = 1,
  int version = 1,
  bool readOnly = false,
}) => {
  'id': channelId(n),
  'space_id': spaceId(spaceN),
  'kind': 'text',
  'name': 'channel-$n',
  'topic': '',
  'sort_key': 'a$n',
  'operator': {'mode': 'inherit'},
  'read_only': readOnly,
  'project_bound': false,
  'version': version,
  'settings_version': 1,
  'created_at': at,
};

/// An API `SpaceView`.
Map<String, Object?> spaceView(int n, {String level = 'write'}) => {
  'space': spaceJson(n),
  'level': level,
  'capabilities': <Object?>[],
};

/// An API `ChannelView`.
Map<String, Object?> channelView(
  int n, {
  int spaceN = 1,
  String level = 'write',
  int version = 1,
}) => {
  'channel': channelJson(n, spaceN: spaceN, version: version),
  'level': level,
  'capabilities': <Object?>[],
};

/// An API `Thread` of channel [channelN].
Map<String, Object?> threadJson(
  int n, {
  int channelN = 1,
  int version = 1,
  int lastSeq = 0,
  String title = 'Release notes',
}) => {
  'id': threadId(n),
  'channel_id': channelId(channelN),
  'kind': 'thread',
  'title': title,
  'title_source': 'manual',
  'state': 'open',
  'last_message_seq': lastSeq,
  'last_activity_at': at,
  'message_count': lastSeq,
  'created_by': alexId,
  'created_at': at,
  'version': version,
  'tag_ids': <Object?>[],
};

/// An API `Message` at position [seq] of thread [threadN].
Map<String, Object?> messageJson(
  int threadN,
  int seq, {
  int channelN = 1,
  int version = 1,
  String? author,
  String? content,
  String? clientMsgId,
  bool deleted = false,
  List<String> mentions = const [],
}) => {
  'id': messageId(threadN, seq),
  'thread_id': threadId(threadN),
  'channel_id': channelId(channelN),
  'thread_seq': seq,
  'author': principalRef(author ?? samId),
  'client_msg_id': clientMsgId ?? id(0x900000 + 0x100 * threadN + seq),
  'origin': 'human',
  'kind': 'text',
  'content_md': deleted ? '' : (content ?? 'deploy note $seq'),
  'content_format': 1,
  'silent': false,
  'attachments': <Object?>[],
  'mention_ids': deleted ? <Object?>[] : mentions,
  'reactions': <Object?>[],
  'pinned': false,
  'components': <Object?>[],
  'version': version,
  'created_at': at,
  if (deleted) 'deleted_at': at,
};

/// An API `ReadState`.
Map<String, Object?> readStateJson(
  int threadN, {
  int channelN = 1,
  required int lastRead,
  required int version,
  int mentions = 0,
  String? device,
}) => {
  'thread_id': threadId(threadN),
  'channel_id': channelId(channelN),
  'last_read_seq': lastRead,
  'mention_count': mentions,
  'version': version,
  'updated_by_device_id': ?device,
};

/// An API `Draft` of thread [threadN].
Map<String, Object?> draftJson(
  int n, {
  int threadN = 1,
  int channelN = 1,
  required String text,
  required int version,
  String? device,
}) => {
  'id': draftId(n),
  'thread_id': threadId(threadN),
  'channel_id': channelId(channelN),
  'content_md': text,
  'file_ids': <Object?>[],
  'version': version,
  'updated_by_device_id': ?device,
};

/// An API `UserSetting`.
Map<String, Object?> settingJson(
  String key,
  Object value, {
  required int version,
  String? device,
}) => {
  'key': key,
  'value': value,
  'version': version,
  'updated_by_device_id': ?device,
};

/// An API `Snapshot`.
Map<String, Object?> snapshotJson({
  int headSeq = 40,
  String epoch = 'epoch-1',
  List<Map<String, Object?>>? spaces,
  List<Map<String, Object?>>? channels,
  List<Map<String, Object?>> unread = const [],
  List<Map<String, Object?>> agents = const [],
  List<Map<String, Object?>> drafts = const [],
  List<Map<String, Object?>> settings = const [],
}) => {
  'head_seq': headSeq,
  'permission_version': 3,
  'sync_epoch': epoch,
  'principal': principalRef(alexId),
  'is_owner': false,
  'spaces': spaces ?? [spaceView(1)],
  'channels': channels ?? [channelView(1), channelView(2)],
  'sidebar': <Object?>[],
  'unread_threads': unread,
  'active_runs': <Object?>[],
  'agents': agents,
  'settings': settings,
  'drafts': drafts,
  'features': [
    {'key': 'dm.enabled', 'enabled': true},
  ],
};

/// An API `InstanceInfo`.
Map<String, Object?> instanceJson() => {
  'instance_id': id(0xf1),
  'branding': {'product_name': 'Chat', 'version': 1},
  'locale_default': 'en',
  'spec_version': '1.0',
  'features': <Object?>[],
  'limits': {
    'message_max_content_bytes': 65536,
    'message_max_attachments': 10,
    'file_max_upload_bytes': 1048576,
    'thread_title_max_chars': 200,
    'voice_max_duration_ms': 600000,
  },
  'app_lock': {'required': false, 'after_ms': 300000},
};

// ---------------------------------------------------------------------------------------------
// Envelopes (04 2.2)

int _eventCounter = 0;

/// A durable or ephemeral event envelope of [type].
Map<String, Object?> envelope(
  String type, {
  int? seq,
  required Map<String, Object?> payload,
  required String resourceKind,
  String? resourceId,
  int? version,
  Map<String, Object?> scope = const {},
  String? clientMsgId,
  bool truncated = false,
  bool redacted = false,
  bool personal = false,
}) => {
  'id': id(0x700000 + ++_eventCounter),
  'seq': ?seq,
  'type': type,
  'schema_version': 1,
  'occurred_at': at,
  'scope': scope,
  'resource': {'kind': resourceKind, 'id': ?resourceId, 'version': ?version},
  'audience': ?(personal ? 'self' : null),
  'payload': payload,
  'permission_version': ?(seq == null ? null : 3),
  'client_msg_id': ?clientMsgId,
  'truncated': ?(truncated ? true : null),
  'redacted': ?(redacted ? true : null),
};

/// `message.created` of message [seq] in thread [threadN].
Map<String, Object?> messageCreated(
  int seq,
  int threadSeq, {
  int threadN = 1,
  int channelN = 1,
  int version = 1,
  String? author,
  String? clientMsgId,
  String? content,
  List<String> mentions = const [],
}) => envelope(
  'message.created',
  seq: seq,
  payload: {
    'message': messageJson(
      threadN,
      threadSeq,
      channelN: channelN,
      version: version,
      author: author,
      clientMsgId: clientMsgId,
      content: content,
      mentions: mentions,
    ),
  },
  resourceKind: 'message',
  resourceId: messageId(threadN, threadSeq),
  version: version,
  scope: {'channel_id': channelId(channelN), 'thread_id': threadId(threadN)},
  clientMsgId: clientMsgId,
);

/// `message.updated` of message [threadSeq] in thread [threadN].
Map<String, Object?> messageUpdated(
  int seq,
  int threadSeq, {
  int threadN = 1,
  required int version,
  String? content,
  bool redacted = false,
}) => envelope(
  'message.updated',
  seq: seq,
  payload: {
    'message': messageJson(
      threadN,
      threadSeq,
      version: version,
      content: content,
      deleted: redacted,
    ),
  },
  resourceKind: 'message',
  resourceId: messageId(threadN, threadSeq),
  version: version,
  scope: {'channel_id': channelId(1), 'thread_id': threadId(threadN)},
  redacted: redacted,
);

/// `thread.created` of thread [n].
Map<String, Object?> threadCreated(int seq, int n, {int channelN = 1}) =>
    envelope(
      'thread.created',
      seq: seq,
      payload: {'thread': threadJson(n, channelN: channelN)},
      resourceKind: 'thread',
      resourceId: threadId(n),
      version: 1,
      scope: {'channel_id': channelId(channelN), 'thread_id': threadId(n)},
    );

/// `access.changed` removing channel [channelN].
Map<String, Object?> accessRemoved(int seq, int channelN) => envelope(
  'access.changed',
  seq: seq,
  payload: {
    'permission_version': 4,
    'added': <Object?>[],
    'removed': [
      {'kind': 'channel', 'id': channelId(channelN)},
    ],
    'changed': <Object?>[],
  },
  resourceKind: 'principal',
  resourceId: alexId,
  personal: true,
);

// ---------------------------------------------------------------------------------------------
// World

/// A [DraftEditing] the test switches.
class SwitchableEditing implements DraftEditing {
  /// The thread being typed in, if any.
  String? editingThread;

  @override
  bool isEditing({required ChannelId channelId, ThreadId? threadId}) =>
      threadId != null && threadId.value == editingThread;
}

/// A [FileCacheEraser] that records the paths.
class RecordingEraser implements FileCacheEraser {
  /// Every path erased.
  final List<String> erased = [];

  @override
  Future<void> erase(List<String> paths) async => erased.addAll(paths);
}

/// The connection, the catch-up listener and the fake REST server.
class World {
  /// Builds the world; [sessionId] is the `sid` of this device.
  World({
    Map<String, String> overrides = const {},
    String? sessionId = 'sid-alex-laptop',
  }) {
    h = Harness(
      overrides: overrides,
      listenerFor: (h) => catchUp = CatchUpListener(
        db: h.db,
        fetcher: ApiFetcher(
          createApiClient(
            baseUrl: serverUrl,
            traceContext: () => TraceContext(traceId: '2' * 32, sampled: false),
            readRetry: ReadRetryPolicy(
              base: Duration.zero,
              cap: Duration.zero,
              maxAttempts: 1,
            ),
            adapter: rest,
          ),
        ),
        config: h.config,
        clock: h.clock,
        identity: FixedIdentity(deviceId: thisDevice, sessionId: sessionId),
        eraser: eraser,
        draftEditing: editing,
        openThreads: () => openThreads,
        onEcho: (clientMsgId, message) => echoes.add(clientMsgId.value),
        onRunUpdated: runUpdates.add,
        onLive: () => liveCalls++,
        logHook: log.add,
      ),
    );
    h.connection; // builds the listener
    catchUp.watchConnection(h.connection.states);
    catchUp.notices.listen(notices.add);
    rest.routes['GET /instance'] = (_) => FakeReply(200, instanceJson());
    rest.routes['GET /sync/snapshot'] = (_) => FakeReply(200, snapshotJson());
  }

  /// The connection harness of M4-13.
  late final Harness h;

  /// The listener under test.
  late final CatchUpListener catchUp;

  /// The fake REST server of the fetcher.
  final FakeApi rest = FakeApi();

  /// The draft editing switch.
  final SwitchableEditing editing = SwitchableEditing();

  /// The file eraser.
  final RecordingEraser eraser = RecordingEraser();

  /// Threads in focus.
  final List<ThreadId> openThreads = [];

  /// Echoed client message IDs.
  final List<String> echoes = [];

  /// Raw `run.updated` payloads.
  final List<Map<String, Object?>> runUpdates = [];

  /// Notices.
  final List<SyncNotice> notices = [];

  /// Log lines of the listener.
  final List<String> log = [];

  /// Calls of the live hook.
  int liveCalls = 0;

  /// The local database.
  LocalDatabase get db => h.db;

  /// The open channel.
  FakeChannel get ch => h.transport.last;

  /// The stored cursor.
  Future<int?> cursor() async => (await db.syncStateDao.read()).cursor?.value;

  /// Connects: stores [cursor] (with `epoch-1`) when given, answers `hello` and sends `ready`
  /// with [snapshot] (the default snapshot when null).
  Future<void> connect({
    int? cursor,
    Map<String, Object?>? snapshot,
    int headSeq = 40,
  }) async {
    if (cursor != null) {
      // A device with a cursor holds the structure of its earlier sessions.
      await h.storeCursor(cursor);
      final s = snapshot ?? snapshotJson(headSeq: headSeq);
      for (final v in s['spaces']! as List) {
        final view = spaceViewFromJson((v as Map).cast<String, Object?>());
        await db.structureDao.upsertSpaceIfNewer(view.space);
      }
      for (final v in s['channels']! as List) {
        final view = channelViewFromJson((v as Map).cast<String, Object?>());
        await db.structureDao.upsertChannelIfNewer(view.channel);
      }
    }
    await h.open();
    ch.push('hello', helloData());
    await idle();
    final ready = readyData(headSeq: headSeq);
    ready['snapshot'] = snapshot ?? snapshotJson(headSeq: headSeq);
    ch.push('ready', ready);
    await idle();
  }

  /// Pushes a server frame and waits until the listener and its background reads are done.
  Future<void> push(String op, Map<String, Object?> d) async {
    ch.push(op, d);
    await idle();
  }

  /// Pushes `dispatch` frames for [envelopes].
  Future<void> dispatch(List<Map<String, Object?>> envelopes) async {
    for (final e in envelopes) {
      ch.push('dispatch', e);
    }
    await idle();
  }

  /// Lets the frame queue and the background reads finish.
  Future<void> idle() async {
    for (var i = 0; i < 3; i++) {
      await settle();
      await catchUp.idle();
    }
  }

  /// The thread positions of the stored messages of thread [n].
  Future<List<int>> storedSeqs(int n) async => [
    for (final m in await db.messagesDao.pageBefore(
      ThreadId.parse(threadId(n)),
      null,
      1000,
    ))
      m.threadSeq.value,
  ];

  /// The thread positions [watchContiguous] shows for thread [n].
  Future<List<int>> visibleSeqs(int n) async => [
    for (final m
        in (await db.messagesDao
                .watchContiguous(ThreadId.parse(threadId(n)), limit: 1000)
                .first)
            .messages)
      m.threadSeq.value,
  ];

  /// Stops everything.
  Future<void> dispose() async {
    await catchUp.dispose();
    await h.dispose();
  }
}
