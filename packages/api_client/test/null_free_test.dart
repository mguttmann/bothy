// SPDX-License-Identifier: Apache-2.0

// No JSON null on the wire (ADR 0053, docs/spec/06-api.md 2): the request models the apps use
// in M4, with only their required fields set, are sent without a null member.

import 'package:api_client/api_client.dart';
import 'package:built_value/json_object.dart';
import 'package:test/test.dart';

import 'support/fake_adapter.dart';

const _id = '0192f4a0-7b1c-7353-8d8c-dea659467151';

bool hasNull(Object? json) => switch (json) {
  null => true,
  final Map<Object?, Object?> m => m.values.any(hasNull),
  final List<Object?> l => l.any(hasNull),
  _ => false,
};

void main() {
  test('hasNull finds a nested null', () {
    expect(
      hasNull({
        'a': [
          1,
          {'b': null},
        ],
      }),
      isTrue,
    );
    expect(
      hasNull({
        'a': [
          1,
          {'b': 2},
        ],
      }),
      isFalse,
    );
  });

  test('the request models of M4 send no null', () async {
    final adapter = FakeAdapter();
    for (var i = 0; i < 4; i++) {
      adapter.empty(204);
    }
    final client = fakeClient(adapter);

    Future<void> quietly(Future<Object?> call) async {
      try {
        await call;
      } on Object {
        // The fake answers 204 without a body; only the request matters here.
      }
    }

    await quietly(
      client.getMessagesApi().createMessage(
        threadId: _id,
        messageCreate: MessageCreate((b) => b..clientMsgId = _id),
      ),
    );
    await quietly(
      client.getMessagesApi().updateMessage(
        messageId: _id,
        ifMatch: '"1"',
        messageUpdate: MessageUpdate(),
      ),
    );
    await quietly(
      client.getInteractionsApi().createInteraction(
        componentId: _id,
        interactionCreate: InteractionCreate(
          (b) => b..clientInteractionId = _id,
        ),
      ),
    );
    await quietly(
      client.getPreferencesApi().putMySetting(
        key: 'theme',
        userSettingPut: UserSettingPut((b) => b..value = JsonObject('dark')),
      ),
    );

    expect(adapter.requests, hasLength(4));
    final bodies = [for (final r in adapter.requests) r.json];
    expect(bodies, [
      {'client_msg_id': _id},
      <String, Object?>{},
      {'client_interaction_id': _id},
      {'value': 'dark'},
    ]);
    for (final body in bodies) {
      expect(hasNull(body), isFalse, reason: '$body');
    }
  });
}
