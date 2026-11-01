// SPDX-License-Identifier: Apache-2.0

/// Handlers of the structure: spaces, channels, forum tags and threads, filtered by visibility,
/// and the creation of forum posts.
library;

import '../rest_fake.dart';
import '../world.dart';

/// Registers the structure handlers on [rest]; a target the person does not see answers
/// `404 not_found`.
void registerStructureHandlers(RestFake rest) {
  final world = rest.world;

  String visibleSpace(FakeRequest r) {
    final id = r.param('space_id');
    if (!world.spaces.containsKey(id) || !world.seesSpace(r.person.id, id)) {
      throw const FakeProblem(404, 'not_found');
    }
    return id;
  }

  String visibleChannel(FakeRequest r) {
    final id = r.param('channel_id');
    if (!world.seesChannel(r.person.id, id)) {
      throw const FakeProblem(404, 'not_found');
    }
    return id;
  }

  rest
    ..handle(
      'listSpaces',
      (r) => FakeResponse(200, {'items': world.spaceViews(r.person.id)}),
    )
    ..handle(
      'getSpace',
      (r) => FakeResponse(200, world.spaceView(r.person.id, visibleSpace(r))),
    )
    ..handle(
      'listSpaceChannels',
      (r) => FakeResponse(200, {
        'items': world.channelViews(r.person.id, spaceId: visibleSpace(r)),
      }),
    )
    ..handle(
      'getChannel',
      (r) =>
          FakeResponse(200, world.channelView(r.person.id, visibleChannel(r))),
    )
    ..handle('listChannelThreads', (r) {
      final channel = visibleChannel(r);
      final state = r.query('state');
      final tag = r.query('tag_id');
      final key = r.query('sort') == 'created'
          ? 'created_at'
          : 'last_activity_at';
      final items =
          [
            for (final t in world.threads.values)
              // A deleted thread leaves the list at once (D-DEL-3).
              if (t['channel_id'] == channel &&
                  t['deleted_at'] == null &&
                  (state == null || t['state'] == state) &&
                  (tag == null ||
                      (t['tag_ids']! as List<Object?>).contains(tag)))
                t,
          ]..sort((a, b) {
            // Newest first; equal times by ID descending, as the post list sorts (01 6.1).
            final byTime = (b[key]! as String).compareTo(a[key]! as String);
            return byTime != 0
                ? byTime
                : (b['id']! as String).compareTo(a['id']! as String);
          });
      // The cursor is the offset of the next page, opaque to the client.
      final start = int.tryParse(r.query('cursor') ?? '') ?? 0;
      final limit = int.tryParse(r.query('limit') ?? '') ?? 50;
      final end = start + limit < items.length ? start + limit : items.length;
      return FakeResponse(200, {
        'items': start < items.length ? items.sublist(start, end) : <Object?>[],
        if (end < items.length) 'next_cursor': '$end',
      });
    })
    ..handle(
      'listForumTags',
      (r) => FakeResponse(200, {'items': world.tagsOf(visibleChannel(r))}),
    )
    ..handle(
      'createThread',
      (r) => FakeResponse(
        201,
        world.createThread(r.person.id, r.param('channel_id'), r.json),
      ),
    )
    ..handle(
      'getThread',
      (r) => FakeResponse(
        200,
        world.visibleThread(r.person.id, r.param('thread_id')),
      ),
    );
}
