// SPDX-License-Identifier: Apache-2.0

// G5 for Dart (docs/spec/06-api.md 15): an unknown field, an unknown enum value and an unknown
// event type are skipped without an error, so an additive change of the spec never breaks an
// installed app (06 13).

import 'dart:convert';
import 'dart:io' as io;

import 'package:api_client/api_client.dart';
import 'package:test/test.dart';

import 'support/fake_adapter.dart';

Map<String, Object?> fixture(String name) =>
    jsonDecode(io.File('test/fixtures/g5/$name').readAsStringSync())
        as Map<String, Object?>;

void main() {
  test('unknown field is ignored', () async {
    final adapter = FakeAdapter()
      ..json(200, fixture('message_future_field.json'));
    final client = fakeClient(adapter);

    final message = (await client.getMessagesApi().getMessage(
      messageId: '0192f4a0-7b1c-7353-8d8c-dea659467151',
    )).data!;

    expect(message.id, '0192f4a0-7b1c-7353-8d8c-dea659467151');
    expect(message.threadSeq, 42);
    expect(message.author.handle, 'alex');
    expect(message.contentMd, 'Sam, the export runs again.');
    expect(message.kind, MessageKind.text);
  });

  test('unknown enum value decodes to the unknown default case', () async {
    final adapter = FakeAdapter()
      ..json(200, fixture('message_unknown_trigger_state.json'));
    final client = fakeClient(adapter);

    final message = (await client.getMessagesApi().getMessage(
      messageId: '0192f4a0-7b1c-7353-8d8c-dea659467151',
    )).data!;

    expect(message.triggerState, TriggerState.unknownDefaultOpenApi);
    expect(message.version, 1);
  });

  test('a received null counts as an absent field', () async {
    final json = fixture('message_future_field.json')
      ..['trigger_state'] = null
      ..['reply_to_message_id'] = null;
    final adapter = FakeAdapter()..json(200, json);

    final message = (await fakeClient(adapter).getMessagesApi().getMessage(
      messageId: '0192f4a0-7b1c-7353-8d8c-dea659467151',
    )).data!;

    expect(message.triggerState, isNull);
    expect(message.replyToMessageId, isNull);
  });

  test('unknown event type is skipped', () {
    final page = EventsPageDecoder.decode(fixture('events_unknown_type.json'));

    expect(page.events, hasLength(3));
    expect(page.events.whereType<KnownEvent>().map((e) => e.seq), [101, 103]);
    final unknown = page.events[1] as UnknownEvent;
    expect(unknown.type, 'future.happened');
    expect(unknown.seq, 102);
    expect(unknown.schemaVersion, 1);
    expect(unknown.raw['payload'], {
      'rocket': {'stage': 2},
    });
    expect(page.nextCursor, 103);
    expect(page.headSeq, 140);
    expect(page.syncEpoch, '0192f4a0-7b1c-7e00-8e00-0000000000e1');
  });

  test('unknown event schema version is read', () {
    final decoded = EventsPageDecoder.decodeEnvelope(
      fixture('event_schema_version_99.json'),
    );

    final known = decoded as KnownEvent;
    final event = known.event.oneOf.value as MessageCreatedEvent;
    expect(event.schemaVersion, 99);
    expect(event.payload.message.id, '0192f4a0-7b1c-7353-8d8c-000000000104');
    expect(known.seq, 104);
    expect(known.type, 'message.created');
  });
}
