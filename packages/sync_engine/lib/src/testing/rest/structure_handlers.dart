// SPDX-License-Identifier: Apache-2.0

/// Handlers of the structure reads: spaces, channels and threads, filtered by visibility.
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
    if (!world.seesChannel(r.person.id, id))
      throw const FakeProblem(404, 'not_found');
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
      return FakeResponse(200, {
        'items': [
          for (final t in world.threads.values)
            if (t['channel_id'] == channel) t,
        ],
      });
    })
    ..handle(
      'getThread',
      (r) => FakeResponse(
        200,
        world.visibleThread(r.person.id, r.param('thread_id')),
      ),
    );
}
