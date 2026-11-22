// SPDX-License-Identifier: Apache-2.0

// The team handle of a team copy (09 3.9.2, FIX-TEAM-MEMBER-ATTRIBUTION): `message.created`
// carries `team_handle` for a member's mirrored reply, the engine stores it with the message and
// reads it back; a message without it stores none.

import 'package:domain/domain.dart';
import 'package:test/test.dart';

import 'support.dart';

void main() {
  late World w;
  setUp(() => w = World());
  tearDown(() => w.dispose());

  test('message.created stores the team handle of a mirror', () async {
    await w.connect(cursor: 30);
    final mirror = messageCreated(31, 1, content: '@helper please check');
    ((mirror['payload']! as Map)['message']! as Map)['team_handle'] = 'coding';
    await w.dispatch([mirror, messageCreated(32, 2, content: 'status?')]);
    await w.push('cursor', {'seq': 32});
    final m = await w.db.messagesDao.byId(MessageId.parse(messageId(1, 1)));
    expect(m!.teamHandle, 'coding');
    final plain = await w.db.messagesDao.byId(MessageId.parse(messageId(1, 2)));
    expect(plain!.teamHandle, isNull);
  });
}
