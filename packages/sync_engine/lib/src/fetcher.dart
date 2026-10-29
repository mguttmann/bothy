// SPDX-License-Identifier: Apache-2.0

/// REST reads of the sync engine (15 7.1 `Fetcher`, brief M4-14 Design note 4): the snapshot,
/// thread pages backward from the newest `thread_seq`, ranges that fill gaps, single objects
/// after a truncated event, the structure of new targets and the personal lists.
library;

import 'package:api_client/api_client.dart' as api show ForumSort, ThreadState;
import 'package:api_client/api_client.dart' show ApiClient, standardSerializers;
import 'package:built_value/serializer.dart';
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';

import 'wire.dart';

/// One page of messages of a thread, ascending by `thread_seq` (API `MessagePage`).
final class MessagePageData {
  /// Creates the page.
  const MessagePageData({required this.messages, required this.hasMoreBefore});

  /// The messages, tombstones included.
  final List<MessageBundle> messages;

  /// Whether older messages exist.
  final bool hasMoreBefore;
}

/// The structure of targets that `access.changed` added (04 8.4): spaces and channels with the
/// caller's access and the first page of threads of each channel.
final class StructureData {
  /// Creates the structure.
  const StructureData({
    required this.spaces,
    required this.channels,
    required this.threads,
  });

  /// Spaces.
  final List<SpaceWithAccess> spaces;

  /// Channels.
  final List<ChannelWithAccess> channels;

  /// Threads.
  final List<Thread> threads;
}

/// One page of `listChannelThreads` (API `ThreadPage`).
final class ThreadListPage {
  /// Creates a page.
  const ThreadListPage({required this.threads, this.nextCursor});

  /// The threads of the page, in the server's order.
  final List<Thread> threads;

  /// The cursor of the next page; null on the last page.
  final String? nextCursor;
}

/// The REST reads of the sync engine. Errors are those of `api_client` (a `DioException`
/// carrying an `ApiProblem`) or a [FormatException] for a body that does not fit its schema;
/// reads are retried by the read retry of M4-02 before they fail.
abstract interface class Fetcher {
  /// `getSnapshot` (04 7.3).
  Future<SnapshotData> snapshot();

  /// `listThreadMessages` backward: the newest page of [threadId], or the page before
  /// [before]; [limit] null leaves the page size to the server.
  Future<MessagePageData> threadPage(
    ThreadId threadId, {
    ThreadSeq? before,
    int? limit,
  });

  /// The messages of [threadId] from [from] to [to] (inclusive, ascending), loaded with
  /// `listThreadMessages` `after_seq` page by page (04 4).
  Future<List<MessageBundle>> range(
    ThreadId threadId,
    ThreadSeq from,
    ThreadSeq to,
  );

  /// `getMessage`.
  Future<MessageBundle> message(MessageId id);

  /// `getThread`.
  Future<Thread> thread(ThreadId id);

  /// `getChannel` with the caller's access.
  Future<ChannelWithAccess> channel(ChannelId id);

  /// `getSpace` with the caller's access.
  Future<SpaceWithAccess> space(SpaceId id);

  /// The structure of [targets] (spaces and channels; other kinds are ignored): `getSpace`
  /// and `listSpaceChannels` per space, `getChannel` per channel, and the first page of
  /// `listChannelThreads` of every channel found.
  Future<StructureData> loadStructure(List<TargetRef> targets);

  /// One page of `listChannelThreads` of [channelId] sorted by [sort] (null: the server's
  /// default), from [cursor] (null: the first page), at most [limit] threads (null: the
  /// server's page size); [openOnly] lists only open threads (`state=open`).
  Future<ThreadListPage> channelThreads(
    ChannelId channelId, {
    String? cursor,
    ForumSort? sort,
    int? limit,
    bool openOnly = false,
  });

  /// `listMyDrafts`.
  Future<List<Draft>> myDrafts();

  /// `listMyReadStates` of [channelId].
  Future<List<ReadState>> myReadStates(ChannelId channelId);

  /// `getInstance`.
  Future<InstanceInfo> instance();

  /// `getAgent`: the agent's card with its reported presence.
  Future<AgentCard> agent(PrincipalId id);
}

/// The [Fetcher] over the generated API of `api_client`.
///
/// Example:
/// ```dart
/// final fetcher = ApiFetcher(apiClient);
/// final snapshot = await fetcher.snapshot();
/// ```
final class ApiFetcher implements Fetcher {
  /// Reads through the [ApiClient], whose interceptors add DPoP, tracing and the read retry.
  ApiFetcher(this._client);

  final ApiClient _client;

  /// The generated model [model] of type [T] in its wire form.
  static Json _json<T extends Object>(T? model) {
    if (model == null) throw const FormatException('an empty response body');
    final j = standardSerializers.serialize(model, specifiedType: FullType(T));
    if (j is Map) return j.cast<String, Object?>();
    throw const FormatException('a response body that is not an object');
  }

  @override
  Future<SnapshotData> snapshot() async =>
      snapshotFromJson(_json((await _client.getSyncApi().getSnapshot()).data));

  @override
  Future<MessagePageData> threadPage(
    ThreadId threadId, {
    ThreadSeq? before,
    int? limit,
  }) async {
    final page = _json(
      (await _client.getMessagesApi().listThreadMessages(
        threadId: threadId.value,
        beforeSeq: before?.value,
        limit: limit,
      )).data,
    );
    return MessagePageData(
      messages: [for (final m in objList(page, 'items')) messageFromJson(m)],
      hasMoreBefore: flag(page, 'has_more_before'),
    );
  }

  @override
  Future<List<MessageBundle>> range(
    ThreadId threadId,
    ThreadSeq from,
    ThreadSeq to,
  ) async {
    final result = <MessageBundle>[];
    var after = from.value - 1;
    while (after < to.value) {
      final page = _json(
        (await _client.getMessagesApi().listThreadMessages(
          threadId: threadId.value,
          afterSeq: after,
          limit: to.value - after,
        )).data,
      );
      final items = [
        for (final m in objList(page, 'items')) messageFromJson(m),
      ];
      for (final m in items) {
        if (m.message.threadSeq.value > after && m.message.threadSeq <= to) {
          result.add(m);
        }
      }
      final last = items.isEmpty ? after : items.last.message.threadSeq.value;
      // A page without progress or the end of the thread ends the loop; positions the server
      // does not return stay a gap.
      if (last <= after || !flag(page, 'has_more_after')) break;
      after = last;
    }
    return result;
  }

  @override
  Future<MessageBundle> message(MessageId id) async => messageFromJson(
    _json(
      (await _client.getMessagesApi().getMessage(messageId: id.value)).data,
    ),
  );

  @override
  Future<Thread> thread(ThreadId id) async => threadFromJson(
    _json((await _client.getThreadsApi().getThread(threadId: id.value)).data),
  );

  @override
  Future<ChannelWithAccess> channel(ChannelId id) async => channelViewFromJson(
    _json(
      (await _client.getChannelsApi().getChannel(channelId: id.value)).data,
    ),
  );

  @override
  Future<SpaceWithAccess> space(SpaceId id) async => spaceViewFromJson(
    _json((await _client.getSpacesApi().getSpace(spaceId: id.value)).data),
  );

  @override
  Future<StructureData> loadStructure(List<TargetRef> targets) async {
    final spaces = <SpaceWithAccess>[];
    final channels = <ChannelWithAccess>[];
    final threads = <Thread>[];
    for (final t in targets) {
      switch (t.kind) {
        case RemovedTargetKind.space:
          final id = SpaceId.parse(t.id);
          spaces.add(await space(id));
          final list = _json(
            (await _client.getChannelsApi().listSpaceChannels(
              spaceId: id.value,
            )).data,
          );
          channels.addAll([
            for (final c in objList(list, 'items')) channelViewFromJson(c),
          ]);
        case RemovedTargetKind.channel:
          channels.add(await channel(ChannelId.parse(t.id)));
        case RemovedTargetKind.thread || RemovedTargetKind.host:
          break;
      }
    }
    for (final c in channels) {
      final page = _json(
        (await _client.getThreadsApi().listChannelThreads(
          channelId: c.channel.id.value,
        )).data,
      );
      threads.addAll([
        for (final t in objList(page, 'items')) threadFromJson(t),
      ]);
    }
    return StructureData(spaces: spaces, channels: channels, threads: threads);
  }

  @override
  Future<ThreadListPage> channelThreads(
    ChannelId channelId, {
    String? cursor,
    ForumSort? sort,
    int? limit,
    bool openOnly = false,
  }) async {
    final page = _json(
      (await _client.getThreadsApi().listChannelThreads(
        channelId: channelId.value,
        cursor: cursor,
        limit: limit,
        state: openOnly ? api.ThreadState.open : null,
        sort: sort == null || sort == ForumSort.unknown
            ? null
            : sort == ForumSort.created
            ? api.ForumSort.created
            : api.ForumSort.lastActivity,
      )).data,
    );
    return ThreadListPage(
      threads: [for (final t in objList(page, 'items')) threadFromJson(t)],
      nextCursor: optStr(page, 'next_cursor'),
    );
  }

  @override
  Future<List<Draft>> myDrafts() async {
    final list = _json((await _client.getPreferencesApi().listMyDrafts()).data);
    return [for (final d in objList(list, 'items')) draftFromJson(d)];
  }

  @override
  Future<List<ReadState>> myReadStates(ChannelId channelId) async {
    final list = _json(
      (await _client.getPreferencesApi().listMyReadStates(
        channelId: channelId.value,
      )).data,
    );
    return [for (final r in objList(list, 'items')) readStateFromJson(r)];
  }

  @override
  Future<InstanceInfo> instance() async =>
      instanceFromJson(_json((await _client.getAdminApi().getInstance()).data));

  @override
  Future<AgentCard> agent(PrincipalId id) async => agentFromJson(
    _json((await _client.getAgentsApi().getAgent(agentId: id.value)).data),
  );
}
