// SPDX-License-Identifier: Apache-2.0

/// The mentions view (docs/spec/01-product.md 3.5 "View Mentions", route `/mentions` of 15 11):
/// the pages of `listMyMentions` with the path and author names from the local database.
library;

import 'package:api_client/api_client.dart' as rest;
import 'package:domain/domain.dart';
import 'package:local_store/local_store.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:sync_engine/sync_engine.dart';

import '../app/providers/session.dart';

part 'providers.g.dart';

/// One mention of the person: the message and the names the local database knows.
final class MentionEntry {
  /// Creates the entry.
  const MentionEntry({
    required this.message,
    required this.authorName,
    required this.path,
  });

  /// The message that mentions the person, as the server returned it.
  final Message message;

  /// The display name of the author, or the handle-less ID while it is not stored.
  final String authorName;

  /// Space, channel and thread title, the parts that are stored.
  final List<String> path;
}

/// The loaded pages: the entries in the server's order (newest first) and the cursor of the
/// next page, null on the last page.
final class MentionsPage {
  /// Creates the state.
  const MentionsPage({required this.entries, required this.nextCursor});

  /// The loaded entries.
  final List<MentionEntry> entries;

  /// The opaque cursor of the next page, or null.
  final String? nextCursor;
}

/// One page of mentions from the server; tests replace it.
typedef MentionsSource =
    Future<({List<Message> messages, String? nextCursor})> Function(
      String? cursor,
    );

/// The source of [MentionsController]: `listMyMentions` of the signed-in instance, or null
/// while signed out.
@riverpod
MentionsSource? mentionsSource(Ref ref) {
  final services = ref.watch(signedInServicesProvider);
  if (services == null) return null;
  final api = services.auth.api;
  return (cursor) async {
    final page = (await api.getMeApi().listMyMentions(cursor: cursor)).data!;
    return (
      messages: [
        for (final item in page.items)
          messageFromJson(
            (api.serializers.serializeWith(
                      rest.Message.serializer,
                      item.message,
                    )!
                    as Map<Object?, Object?>)
                .cast<String, Object?>(),
          ).message,
      ],
      nextCursor: page.nextCursor,
    );
  };
}

/// Names the parts of [message] from [db]: the author and the path.
Future<MentionEntry> describeMention(LocalDatabase? db, Message message) async {
  final author = await db?.principalsDao.byId(message.authorId);
  final channel = await db?.structureDao.channelById(message.channelId);
  final space = channel?.spaceId == null
      ? null
      : await db?.structureDao.spaceById(channel!.spaceId!);
  final thread = await db?.threadsDao.byId(message.threadId);
  return MentionEntry(
    message: message,
    authorName: author?.displayName ?? message.authorId.value,
    path: [
      ?space?.name,
      ?channel?.name,
      if (thread != null && thread.title.isNotEmpty) thread.title,
    ],
  );
}

/// The pages of the mentions view; [loadMore] appends the next page.
@riverpod
class MentionsController extends _$MentionsController {
  @override
  Future<MentionsPage> build() async {
    final source = ref.watch(mentionsSourceProvider);
    if (source == null) {
      return const MentionsPage(entries: [], nextCursor: null);
    }
    return _page(source, null, const []);
  }

  Future<MentionsPage> _page(
    MentionsSource source,
    String? cursor,
    List<MentionEntry> before,
  ) async {
    final page = await source(cursor);
    final db = ref.read(localDatabaseProvider);
    return MentionsPage(
      entries: [
        ...before,
        for (final m in page.messages) await describeMention(db, m),
      ],
      nextCursor: page.nextCursor,
    );
  }

  /// Loads the next page; does nothing on the last page or while a page loads.
  Future<void> loadMore() async {
    final current = state.value;
    final source = ref.read(mentionsSourceProvider);
    final cursor = current?.nextCursor;
    if (current == null ||
        source == null ||
        cursor == null ||
        state.isLoading) {
      return;
    }
    state = await AsyncValue.guard(
      () => _page(source, cursor, current.entries),
    );
  }
}
