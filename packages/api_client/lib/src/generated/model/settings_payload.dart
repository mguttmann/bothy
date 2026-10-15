//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/user_setting.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'settings_payload.g.dart';

/// SettingsPayload
///
/// Properties:
/// * [setting]
/// * [deleted]
@BuiltValue()
abstract class SettingsPayload
    implements Built<SettingsPayload, SettingsPayloadBuilder> {
  @BuiltValueField(wireName: r'setting')
  UserSetting get setting;

  @BuiltValueField(wireName: r'deleted')
  bool? get deleted;

  SettingsPayload._();

  factory SettingsPayload([void updates(SettingsPayloadBuilder b)]) =
      _$SettingsPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SettingsPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SettingsPayload> get serializer =>
      _$SettingsPayloadSerializer();
}

class _$SettingsPayloadSerializer
    implements PrimitiveSerializer<SettingsPayload> {
  @override
  final Iterable<Type> types = const [SettingsPayload, _$SettingsPayload];

  @override
  final String wireName = r'SettingsPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SettingsPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'setting';
    yield serializers.serialize(
      object.setting,
      specifiedType: const FullType(UserSetting),
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
    SettingsPayload object, {
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
    required SettingsPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'setting':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(UserSetting),
          ) as UserSetting;
          result.setting.replace(valueDes);
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
  SettingsPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SettingsPayloadBuilder();
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
