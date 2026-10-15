//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/notification_level.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'notification_preference_put.g.dart';

/// NotificationPreferencePut
///
/// Properties:
/// * [level]
/// * [mutedUntil] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class NotificationPreferencePut
    implements
        Built<NotificationPreferencePut, NotificationPreferencePutBuilder> {
  @BuiltValueField(wireName: r'level')
  NotificationLevel get level;
  // enum levelEnum {  all,  mentions,  mute,  };

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'muted_until')
  DateTime? get mutedUntil;

  NotificationPreferencePut._();

  factory NotificationPreferencePut([
    void updates(NotificationPreferencePutBuilder b),
  ]) = _$NotificationPreferencePut;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(NotificationPreferencePutBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<NotificationPreferencePut> get serializer =>
      _$NotificationPreferencePutSerializer();
}

class _$NotificationPreferencePutSerializer
    implements PrimitiveSerializer<NotificationPreferencePut> {
  @override
  final Iterable<Type> types = const [
    NotificationPreferencePut,
    _$NotificationPreferencePut,
  ];

  @override
  final String wireName = r'NotificationPreferencePut';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    NotificationPreferencePut object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'level';
    yield serializers.serialize(
      object.level,
      specifiedType: const FullType(NotificationLevel),
    );
    if (object.mutedUntil != null) {
      yield r'muted_until';
      yield serializers.serialize(
        object.mutedUntil,
        specifiedType: const FullType(DateTime),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    NotificationPreferencePut object, {
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
    required NotificationPreferencePutBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'level':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(NotificationLevel),
          ) as NotificationLevel;
          result.level = valueDes;
          break;
        case r'muted_until':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.mutedUntil = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  NotificationPreferencePut deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = NotificationPreferencePutBuilder();
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
