//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/notification_preference.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'notification_pref_payload.g.dart';

/// NotificationPrefPayload
///
/// Properties:
/// * [preference]
/// * [deleted]
@BuiltValue()
abstract class NotificationPrefPayload
    implements Built<NotificationPrefPayload, NotificationPrefPayloadBuilder> {
  @BuiltValueField(wireName: r'preference')
  NotificationPreference get preference;

  @BuiltValueField(wireName: r'deleted')
  bool? get deleted;

  NotificationPrefPayload._();

  factory NotificationPrefPayload([
    void updates(NotificationPrefPayloadBuilder b),
  ]) = _$NotificationPrefPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(NotificationPrefPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<NotificationPrefPayload> get serializer =>
      _$NotificationPrefPayloadSerializer();
}

class _$NotificationPrefPayloadSerializer
    implements PrimitiveSerializer<NotificationPrefPayload> {
  @override
  final Iterable<Type> types = const [
    NotificationPrefPayload,
    _$NotificationPrefPayload,
  ];

  @override
  final String wireName = r'NotificationPrefPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    NotificationPrefPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'preference';
    yield serializers.serialize(
      object.preference,
      specifiedType: const FullType(NotificationPreference),
    );
    if (object.deleted != null) {
      yield r'deleted';
      yield serializers.serialize(
        object.deleted,
        specifiedType: const FullType(bool),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    NotificationPrefPayload object, {
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
    required NotificationPrefPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'preference':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(NotificationPreference),
          ) as NotificationPreference;
          result.preference.replace(valueDes);
          break;
        case r'deleted':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.deleted = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  NotificationPrefPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = NotificationPrefPayloadBuilder();
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
