//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'instance_setting_put.g.dart';

/// InstanceSettingPut
///
/// Properties:
/// * [value] - Any JSON value except null. The schema of the value is defined by its key.
@BuiltValue()
abstract class InstanceSettingPut
    implements Built<InstanceSettingPut, InstanceSettingPutBuilder> {
  /// Any JSON value except null. The schema of the value is defined by its key.
  @BuiltValueField(wireName: r'value')
  JsonObject? get value;

  InstanceSettingPut._();

  factory InstanceSettingPut([void updates(InstanceSettingPutBuilder b)]) =
      _$InstanceSettingPut;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(InstanceSettingPutBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<InstanceSettingPut> get serializer =>
      _$InstanceSettingPutSerializer();
}

class _$InstanceSettingPutSerializer
    implements PrimitiveSerializer<InstanceSettingPut> {
  @override
  final Iterable<Type> types = const [InstanceSettingPut, _$InstanceSettingPut];

  @override
  final String wireName = r'InstanceSettingPut';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    InstanceSettingPut object, {
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
    InstanceSettingPut object, {
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
    required InstanceSettingPutBuilder result,
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
  InstanceSettingPut deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = InstanceSettingPutBuilder();
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
