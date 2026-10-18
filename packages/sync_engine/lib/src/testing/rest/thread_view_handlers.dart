// SPDX-License-Identifier: Apache-2.0

/// Handlers of the thread view (brief M4-20 Scope): `listMessageRevisions`,
/// `markThreadUnread`, `pinMessage`, `unpinMessage`, `listPins`, `closeThread`,
/// `reopenThread`, `setThreadTitle`, `moveThread`, `listMyTemplates`,
/// `createPersonalTemplate`, `listTemplates`, and `createThread` with `parent_message_id`.
library;

import '../rest_fake.dart';
import '../world.dart';

/// The version of an `If-Match` header (`"3"` or `3`); `428 precondition_required` without one.
int _ifMatch(FakeRequest r) {
  final match = RegExp(r'^"?(\d+)"?$').firstMatch(r.headers['if-match'] ?? '');
  if (match == null) throw const FakeProblem(428, 'precondition_required');
  return int.parse(match[1]!);
}

/// Registers the thread view handlers on [rest]; `createThread` replaces the forum handler of
/// M4-19 and hands a body without `parent_message_id` to it unchanged.
void registerThreadViewHandlers(RestFake rest) {
  final world = rest.world;
  rest
    ..handle(
      'listMessageRevisions',
      (r) => FakeResponse(200, {
        'items': world.revisionsOf(r.person.id, r.param('message_id')),
      }),
    )
    ..handle('markThreadUnread', (r) {
      final state = world.markUnread(
        r.person.id,
        r.param('thread_id'),
        r.json['last_read_seq']! as int,
        _ifMatch(r),
        deviceId: r.session!.deviceId,
      );
      return FakeResponse(200, state, {'ETag': '"${state['version']}"'});
    })
    ..handle('pinMessage', (r) {
      world.pin(r.person.id, r.param('message_id'), pinned: true);
      return const FakeResponse.noContent();
    })
    ..handle('unpinMessage', (r) {
      world.pin(r.person.id, r.param('message_id'), pinned: false);
      return const FakeResponse.noContent();
    })
    ..handle(
      'listPins',
      (r) => FakeResponse(200, {
        'items': world.pinsOf(r.person.id, r.param('thread_id')),
      }),
    )
    ..handle(
      'closeThread',
      (r) => FakeResponse(
        200,
        world.setThreadState(r.person.id, r.param('thread_id'), closed: true),
      ),
    )
    ..handle(
      'reopenThread',
      (r) => FakeResponse(
        200,
        world.setThreadState(r.person.id, r.param('thread_id'), closed: false),
      ),
    )
    ..handle('setThreadTitle', (r) {
      final thread = world.setTitle(
        r.person.id,
        r.param('thread_id'),
        _ifMatch(r),
        r.json['title']! as String,
      );
      return FakeResponse(200, thread, {'ETag': '"${thread['version']}"'});
    })
    ..handle(
      'moveThread',
      (r) => FakeResponse(
        200,
        world.moveThread(
          r.person.id,
          r.param('thread_id'),
          r.json['target_channel_id']! as String,
        ),
      ),
    )
    ..handle(
      'listMyTemplates',
      (r) => FakeResponse(200, {'items': world.personalTemplates(r.person.id)}),
    )
    ..handle(
      'createPersonalTemplate',
      (r) => FakeResponse(
        201,
        world.addTemplate(
          scope: 'personal',
          ownerId: r.person.id,
          name: r.json['name']! as String,
          contentMd: r.json['content_md']! as String,
        ),
      ),
    )
    ..handle(
      'listTemplates',
      (r) => FakeResponse(200, {
        'items': world.channelTemplates(r.person.id, r.param('channel_id')),
      }),
    )
    ..handle(
      'createThread',
      (r) => FakeResponse(
        201,
        r.json['parent_message_id'] == null
            ? world.createThread(r.person.id, r.param('channel_id'), r.json)
            : world.createThreadFromMessage(
                r.person.id,
                r.param('channel_id'),
                r.json,
              ),
      ),
    );
}
