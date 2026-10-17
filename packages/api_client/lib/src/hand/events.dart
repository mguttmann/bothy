// SPDX-License-Identifier: Apache-2.0

import 'package:api_client/src/generated/api.dart';
import 'package:api_client/src/generated/model/event.dart';
import 'package:api_client/src/generated/serializers.dart';
import 'package:built_value/serializer.dart';
import 'package:dio/dio.dart';

/// One decoded element of an event page or one gateway `dispatch` envelope.
///
/// The sync engine (M4-14) applies only [KnownEvent]s and advances its cursor over
/// [UnknownEvent]s (docs/spec/04-sync-protocol 9 item 3, 06 13).
sealed class DecodedEvent {
  /// Wraps the [raw] JSON of the element; only the two subclasses below exist.
  const DecodedEvent({required this.raw});

  /// The element as received; for logs and for a later client that knows the type.
  final Map<String, Object?> raw;

  /// The member `type`.
  String get type => raw['type']! as String;

  /// The member `seq`; `null` for ephemeral events.
  int? get seq => raw['seq'] is int ? raw['seq']! as int : null;

  /// Whether the server cut the payload to `sync.event_payload_max_bytes`; the client then
  /// loads the resource over REST (04 2.2).
  bool get truncated => raw['truncated'] == true;
}

/// An event whose `type` the generated serializers know, decoded into the generated union.
///
/// Unknown members are ignored and unknown enum values decode to the generated unknown
/// default case (G5).
final class KnownEvent extends DecodedEvent {
  /// Wraps the decoded [event] and its [raw] JSON.
  const KnownEvent({required this.event, required super.raw});

  /// The generated union; `event.oneOf.value` is the typed event, for example
  /// `MessageCreatedEvent`.
  final Event event;
}

/// An event of a type this client does not know; kept, never thrown (06 13).
final class UnknownEvent extends DecodedEvent {
  /// Wraps the [raw] JSON of the element.
  const UnknownEvent({required super.raw});

  /// The member `id`, when it is a string.
  String? get id => raw['id'] is String ? raw['id']! as String : null;

  /// The member `schema_version`, when it is an integer.
  int? get schemaVersion =>
      raw['schema_version'] is int ? raw['schema_version']! as int : null;
}

/// An event page (`EventPage`, `GET /api/v1/events`) decoded element by element.
class DecodedEventsPage {
  /// Creates a page from its members.
  const DecodedEventsPage({
    required this.events,
    required this.nextCursor,
    required this.headSeq,
    required this.syncEpoch,
  });

  /// The elements in server order, known and unknown.
  final List<DecodedEvent> events;

  /// Highest checked `seq`; the `after` of the next call, even when [events] is empty.
  final int nextCursor;

  /// The newest `seq` of the log.
  final int headSeq;

  /// The time line of the log; stored with the cursor (04 7.5).
  final String syncEpoch;
}

/// Tolerant decoding of events (docs/DECISIONS.md D-M4-02-3).
///
/// The generated `Event` union throws on an unknown discriminator value, so one new event type
/// would make a whole page unreadable; this decoder takes each element on its own.
abstract final class EventsPageDecoder {
  /// Decodes the JSON of an `EventPage`.
  ///
  /// Throws a [FormatException] when a required member is missing or of the wrong type, or
  /// when an element has no string `type`.
  ///
  /// Example:
  /// ```dart
  /// final page = EventsPageDecoder.decode(json);
  /// for (final e in page.events.whereType<KnownEvent>()) apply(e.event);
  /// cursor = page.nextCursor;
  /// ```
  static DecodedEventsPage decode(Map<String, Object?> json) {
    final events = json['events'];
    final nextCursor = json['next_cursor'];
    final headSeq = json['head_seq'];
    final syncEpoch = json['sync_epoch'];
    if (events is! List ||
        nextCursor is! int ||
        headSeq is! int ||
        syncEpoch is! String) {
      throw const FormatException(
        'not an EventPage: events, next_cursor, head_seq or sync_epoch',
      );
    }
    return DecodedEventsPage(
      events: [
        for (final e in events)
          if (e is Map)
            decodeEnvelope(e.cast<String, Object?>())
          else
            throw const FormatException('an event is not a JSON object'),
      ],
      nextCursor: nextCursor,
      headSeq: headSeq,
      syncEpoch: syncEpoch,
    );
  }

  /// Decodes one event: an element of a page or the `d` member of a gateway `dispatch` frame.
  ///
  /// Throws a [FormatException] when the event has no string `type`; a known type that does
  /// not match its schema fails with the error of the generated serializer.
  static DecodedEvent decodeEnvelope(Map<String, Object?> json) {
    if (json['type'] is! String) {
      throw const FormatException('an event without a string member type');
    }
    if (!Event.discriminatorMapping.containsKey(json['type'])) {
      return UnknownEvent(raw: json);
    }
    final event =
        standardSerializers.deserialize(
              json,
              specifiedType: const FullType(Event),
            )!
            as Event;
    return KnownEvent(event: event, raw: json);
  }
}

/// Reads one page of `GET /api/v1/events` (04 7.4) through [client] and decodes it with
/// [EventsPageDecoder], so an unknown event type never fails the page.
///
/// [after] is the last processed `seq`, [epoch] the stored sync epoch, [limit] at most
/// `sync.catchup_page_size`. Errors are those of the client: a [DioException] carrying an
/// `ApiProblem`, or a [FormatException] for a body that is not an event page.
Future<DecodedEventsPage> fetchEvents(
  ApiClient client, {
  required int after,
  String? epoch,
  int? limit,
}) async {
  final response = await client.dio.get<Object?>(
    '/events',
    queryParameters: {'after': after, 'epoch': ?epoch, 'limit': ?limit},
    // The same security marker the generated operation sets, so the auth interceptors of
    // M4-09 sign this request as they sign `listEvents`.
    options: Options(
      extra: {
        'secure': [
          {'type': 'http', 'scheme': 'DPoP', 'name': 'dpopAuth'},
          {'type': 'http', 'scheme': 'bearer', 'name': 'bearerAuth'},
        ],
      },
    ),
  );
  final data = response.data;
  if (data is! Map) {
    throw const FormatException('the events response is not a JSON object');
  }
  return EventsPageDecoder.decode(data.cast<String, Object?>());
}
