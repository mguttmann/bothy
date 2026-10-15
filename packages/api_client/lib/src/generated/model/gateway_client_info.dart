//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/device_platform.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'gateway_client_info.g.dart';

/// GatewayClientInfo
///
/// Properties:
/// * [platform]
/// * [version]
@BuiltValue()
abstract class GatewayClientInfo
    implements Built<GatewayClientInfo, GatewayClientInfoBuilder> {
  @BuiltValueField(wireName: r'platform')
  DevicePlatform get platform;
  // enum platformEnum {  ios,  android,  macos,  windows,  web,  cli,  };

  @BuiltValueField(wireName: r'version')
  String get version;

  GatewayClientInfo._();

  factory GatewayClientInfo([void updates(GatewayClientInfoBuilder b)]) =
      _$GatewayClientInfo;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GatewayClientInfoBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GatewayClientInfo> get serializer =>
      _$GatewayClientInfoSerializer();
}

class _$GatewayClientInfoSerializer
    implements PrimitiveSerializer<GatewayClientInfo> {
  @override
  final Iterable<Type> types = const [GatewayClientInfo, _$GatewayClientInfo];

  @override
  final String wireName = r'GatewayClientInfo';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GatewayClientInfo object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'platform';
    yield serializers.serialize(
      object.platform,
      specifiedType: const FullType(DevicePlatform),
    );
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    GatewayClientInfo object, {
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
    required GatewayClientInfoBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'platform':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DevicePlatform),
          ) as DevicePlatform;
          result.platform = valueDes;
          break;
        case r'version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.version = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GatewayClientInfo deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GatewayClientInfoBuilder();
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
