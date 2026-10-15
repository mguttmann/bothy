//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'user_setting_put.g.dart';

/// UserSettingPut
///
/// Properties:
/// * [value] - Any JSON value except null. The schema of the value is defined by its key.
@BuiltValue()
abstract class UserSettingPut
    implements Built<UserSettingPut, UserSettingPutBuilder> {
  /// Any JSON value except null. The schema of the value is defined by its key.
  @BuiltValueField(wireName: r'value')
  JsonObject? get value;

  UserSettingPut._();

  factory UserSettingPut([void updates(UserSettingPutBuilder b)]) =
      _$UserSettingPut;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(UserSettingPutBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<UserSettingPut> get serializer =>
      _$UserSettingPutSerializer();
}

class _$UserSettingPutSerializer
    implements PrimitiveSerializer<UserSettingPut> {
  @override
  final Iterable<Type> types = const [UserSettingPut, _$UserSettingPut];

  @override
  final String wireName = r'UserSettingPut';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    UserSettingPut object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'value';
    yield object.value == null
        ? null
        : serializers.serialize(
            object.value,
            specifiedType: const FullType.nullable(JsonObject),
          );
  }

  @override
  Object serialize(
    Serializers serializers,
    UserSettingPut object, {
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
    required UserSettingPutBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'value':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(JsonObject),
          ) as JsonObject?;
          if (valueDes == null) continue;
          result.value = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  UserSettingPut deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = UserSettingPutBuilder();
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
