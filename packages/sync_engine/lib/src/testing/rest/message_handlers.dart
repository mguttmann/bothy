// SPDX-License-Identifier: Apache-2.0

/// Handlers of the message reads and writes (04 5.1, 04 13.3): `listThreadMessages`,
/// `getMessage`, `createMessage`, `updateMessage`, `deleteMessage`, `addReaction`,
/// `removeReaction`.
library;

import '../rest_fake.dart';
import '../world.dart';

/// The page size of `listThreadMessages` without `limit`.
const int defaultMessagePage = 50;

/// Registers the message handlers on [rest].
///
/// `createMessage` follows 04 5.1: a repeated `client_msg_id` with the same body answers the
/// original with `200` and `Idempotent-Replay: true`, a different body
/// `422 idempotency_key_reuse`. `updateMessage` checks `If-Match` (`412 version_conflict` with
/// the current revision, `410 message_deleted`); `deleteMessage` is idempotent (`204`).
void registerMessageHandlers(RestFake rest) {
  final world = rest.world;
  rest
    ..handle('listThreadMessages', (r) {
      final all = world.messagesOf(world.visibleThread(r.person.id, r.param('thread_id'))['id']! as String);
      final limit = int.tryParse(r.query('limit') ?? '') ?? defaultMessagePage;
      final before = int.tryParse(r.query('before_seq') ?? '');
      final after = int.tryParse(r.query('after_seq') ?? '');
      final around = int.tryParse(r.query('around_seq') ?? '');
      int seq(JsonMap m) => m['thread_seq']! as int;
      List<JsonMap> page;
      if (after != null) {
        page = all.where((m) => seq(m) > after).take(limit).toList();
      } else if (around != null) {
        final from = around - limit ~/ 2;
        page = all.where((m) => seq(m) >= from).take(limit).toList();
      } else {
        final upTo = all.where((m) => before == null || seq(m) < before).toList();
        page = upTo.sublist(upTo.length > limit ? upTo.length - limit : 0);
      }
      return FakeResponse(200, {
        'items': page,
        'has_more_before': page.isNotEmpty && all.isNotEmpty && seq(page.first) > seq(all.first),
        'has_more_after': page.isNotEmpty && all.isNotEmpty && seq(page.last) < seq(all.last),
      });
    })
    ..handle(
      'getMessage',
      (r) => FakeResponse(200, world.visibleMessage(r.person.id, r.param('message_id'))),
    )
    ..handle('createMessage', (r) {
      final result = world.createMessage(r.person.id, r.param('thread_id'), r.json);
      return result.replay
          ? FakeResponse(200, result.message, const {'Idempotent-Replay': 'true'})
          : FakeResponse(201, result.message);
    })
    ..handle('updateMessage', (r) {
      final match = RegExp(r'^"?(\d+)"?$').firstMatch(r.headers['if-match'] ?? '');
      if (match == null) throw const FakeProblem(428, 'precondition_required');
      final m = world.updateMessage(
        r.person.id,
        r.param('message_id'),
        int.parse(match[1]!),
        r.json,
      );
      return FakeResponse(200, m, {'ETag': '"${m['version']}"'});
    })
    ..handle('deleteMessage', (r) {
      world.deleteMessage(r.person.id, r.param('message_id'));
      return const FakeResponse.noContent();
    })
    ..handle('addReaction', (r) {
      world.react(r.person.id, r.param('message_id'), r.param('emoji'), add: true);
      return const FakeResponse.noContent();
    })
    ..handle('removeReaction', (r) {
      world.react(r.person.id, r.param('message_id'), r.param('emoji'), add: false);
      return const FakeResponse.noContent();
    });
}
