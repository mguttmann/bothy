// SPDX-License-Identifier: Apache-2.0

// Tolerant decoding of single gateway dispatch envelopes and of event pages fetched through the
// client (brief M4-02 Design note 6; 04 2.2 and 7.4).

import 'dart:convert';
import 'dart:io' as io;

import 'package:api_client/api_client.dart';
import 'package:test/test.dart';

import 'support/fake_adapter.dart';

Map<String, Object?> fixture(String name) =>
    jsonDecode(io.File('test/fixtures/g5/$name').readAsStringSync())
        as Map<String, Object?>;

/// A gateway frame `dispatch` around the first event of the G5 page.
Map<String, Object?> dispatch(Map<String, Object?> event) => {
  'op': 'dispatch',
  'd': event,
};

Map<String, Object?> firstEvent() =>
    (fixture('events_unknown_type.json')['events']! as List<Object?>).first!
        as Map<String, Object?>;

void main() {
  test('decodeEnvelope of a message.created dispatch', () {
    final frame = dispatch(firstEvent());

    final decoded = EventsPageDecoder.decodeEnvelope(
      frame['d']! as Map<String, Object?>,
    );

    final known = decoded as KnownEvent;
    expect(known.type, 'message.created');
    expect(known.seq, 101);
    expect(known.truncated, isFalse);
    final event = known.event.oneOf.value as MessageCreatedEvent;
    expect(event.payload.message.contentMd, 'Sam, the export runs again.');
  });

  test('decodeEnvelope of an unknown type', () {
    final raw = {...firstEvent(), 'type': 'future.happened', 'truncated': true};

    final decoded = EventsPageDecoder.decodeEnvelope(raw);

    final unknown = decoded as UnknownEvent;
    expect(unknown.type, 'future.happened');
    expect(unknown.id, raw['id']);
    expect(unknown.seq, 101);
    expect(unknown.schemaVersion, 1);
    expect(unknown.truncated, isTrue);
    expect(unknown.raw, raw);
  });

  test('decodeEnvelope keeps truncated: true', () {
    final event = firstEvent();
    final payload = event['payload']! as Map<String, Object?>;
    final message = {
      ...payload['message']! as Map<String, Object?>,
      'content_truncated': true,
    };
    final raw = {
      ...event,
      'truncated': true,
      'payload': {'message': message},
    };

    final known = EventsPageDecoder.decodeEnvelope(raw) as KnownEvent;

    expect(known.truncated, isTrue);
    expect((known.event.oneOf.value as MessageCreatedEvent).truncated, isTrue);
  });

  test('an ephemeral event without seq decodes', () {
    final raw = {...firstEvent()}..remove('seq');

    expect(EventsPageDecoder.decodeEnvelope(raw).seq, isNull);
    expect(
      EventsPageDecoder.decodeEnvelope({...raw, 'type': 'future.happened'}).seq,
      isNull,
    );
  });

  test('an element without a string type is a format error', () {
    expect(
      () => EventsPageDecoder.decodeEnvelope({...firstEvent()}..remove('type')),
      throwsFormatException,
    );
    expect(
      () => EventsPageDecoder.decode({
        'events': 'x',
        'next_cursor': 1,
        'head_seq': 1,
        'sync_epoch': 'e',
      }),
      throwsFormatException,
    );
  });

  test(
    'fetchEvents reads a page with an unknown type through the client',
    () async {
      final adapter = FakeAdapter()
        ..json(200, fixture('events_unknown_type.json'));
      final client = fakeClient(adapter);

      final page = await fetchEvents(
        client,
        after: 100,
        epoch: 'e1',
        limit: 50,
      );

      expect(page.events.map((e) => e.type), [
        'message.created',
        'future.happened',
        'message.created',
      ]);
      expect(page.nextCursor, 103);
      final request = adapter.requests.single.options;
      expect(request.method, 'GET');
      expect(request.uri.path, '/api/v1/events');
      expect(request.uri.queryParameters, {
        'after': '100',
        'epoch': 'e1',
        'limit': '50',
      });
      expect(request.headers['traceparent'], isNotNull);
    },
  );
}
