//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/device_platform.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'device_pending_payload.g.dart';

/// A new device of the human waits for approval (R-AZ-31). Carries only IDs and the platform.
///
/// Properties:
/// * [deviceId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [platform]
@BuiltValue()
abstract class DevicePendingPayload
    implements Built<DevicePendingPayload, DevicePendingPayloadBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'device_id')
  String get deviceId;

  @BuiltValueField(wireName: r'platform')
  DevicePlatform get platform;
  // enum platformEnum {  ios,  android,  macos,  windows,  web,  cli,  };

  DevicePendingPayload._();

  factory DevicePendingPayload([void updates(DevicePendingPayloadBuilder b)]) =
      _$DevicePendingPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(DevicePendingPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<DevicePendingPayload> get serializer =>
      _$DevicePendingPayloadSerializer();
}

class _$DevicePendingPayloadSerializer
    implements PrimitiveSerializer<DevicePendingPayload> {
  @override
  final Iterable<Type> types = const [
    DevicePendingPayload,
    _$DevicePendingPayload,
  ];

  @override
  final String wireName = r'DevicePendingPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    DevicePendingPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'device_id';
    yield serializers.serialize(
      object.deviceId,
      specifiedType: const FullType(String),
    );
    yield r'platform';
    yield serializers.serialize(
      object.platform,
      specifiedType: const FullType(DevicePlatform),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    DevicePendingPayload object, {
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
    required DevicePendingPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'device_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.deviceId = valueDes;
          break;
        case r'platform':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DevicePlatform),
          ) as DevicePlatform;
          result.platform = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  DevicePendingPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = DevicePendingPayloadBuilder();
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
