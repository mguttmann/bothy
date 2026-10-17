//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/event.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'event_page.g.dart';

/// EventPage
///
/// Properties:
/// * [events]
/// * [nextCursor] - Highest checked `seq`; use as `after` for the next call even if `events` is empty.
/// * [headSeq]
/// * [syncEpoch] - Time line of the event log; a client stores it with its cursor (04, 7.5).
@BuiltValue()
abstract class EventPage implements Built<EventPage, EventPageBuilder> {
  @BuiltValueField(wireName: r'events')
  BuiltList<Event> get events;

  /// Highest checked `seq`; use as `after` for the next call even if `events` is empty.
  @BuiltValueField(wireName: r'next_cursor')
  int get nextCursor;

  @BuiltValueField(wireName: r'head_seq')
  int get headSeq;

  /// Time line of the event log; a client stores it with its cursor (04, 7.5).
  @BuiltValueField(wireName: r'sync_epoch')
  String get syncEpoch;

  EventPage._();

  factory EventPage([void updates(EventPageBuilder b)]) = _$EventPage;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(EventPageBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<EventPage> get serializer => _$EventPageSerializer();
}

class _$EventPageSerializer implements PrimitiveSerializer<EventPage> {
  @override
  final Iterable<Type> types = const [EventPage, _$EventPage];

  @override
  final String wireName = r'EventPage';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    EventPage object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'events';
    yield serializers.serialize(
      object.events,
      specifiedType: const FullType(BuiltList, [FullType(Event)]),
    );
    yield r'next_cursor';
    yield serializers.serialize(
      object.nextCursor,
      specifiedType: const FullType(int),
    );
    yield r'head_seq';
    yield serializers.serialize(
      object.headSeq,
      specifiedType: const FullType(int),
    );
    yield r'sync_epoch';
    yield serializers.serialize(
      object.syncEpoch,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    EventPage object, {
    FullType specifiedType = FullType.unspecified,
  }) {
    return _serializeProperties(
      serializers,
      object,
      specifiedType: specifiedType,
    ).toList();
  }

  void _deserializeProperties(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
    required List<Object?> serializedList,
    required EventPageBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'events':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(Event)]),
          ) as BuiltList<Event>;
          result.events.replace(valueDes);
          break;
        case r'next_cursor':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.nextCursor = valueDes;
          break;
        case r'head_seq':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.headSeq = valueDes;
          break;
        case r'sync_epoch':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.syncEpoch = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  EventPage deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = EventPageBuilder();
    final serializedList = (serialized as Iterable<Object?>).toList();
    final unhandled = <Object?>[];
    _deserializeProperties(
      serializers,
      serialized,
      specifiedType: specifiedType,
      serializedList: serializedList,
      unhandled: unhandled,
      result: result,
    );
    return result.build();
  }
}
