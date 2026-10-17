//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/device_platform.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'client_version_requirement.g.dart';

/// ClientVersionRequirement
///
/// Properties:
/// * [platform]
/// * [minVersion] - Below this version the app does not connect (`client.min_version.<platform>`, 18 section 9).
/// * [recommendedVersion] - Below this version the app shows an update note (`client.recommended_version.<platform>`).
@BuiltValue()
abstract class ClientVersionRequirement
    implements
        Built<ClientVersionRequirement, ClientVersionRequirementBuilder> {
  @BuiltValueField(wireName: r'platform')
  DevicePlatform get platform;
  // enum platformEnum {  ios,  android,  macos,  windows,  web,  cli,  };

  /// Below this version the app does not connect (`client.min_version.<platform>`, 18 section 9).
  @BuiltValueField(wireName: r'min_version')
  String get minVersion;

  /// Below this version the app shows an update note (`client.recommended_version.<platform>`).
  @BuiltValueField(wireName: r'recommended_version')
  String? get recommendedVersion;

  ClientVersionRequirement._();

  factory ClientVersionRequirement([
    void updates(ClientVersionRequirementBuilder b),
  ]) = _$ClientVersionRequirement;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ClientVersionRequirementBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ClientVersionRequirement> get serializer =>
      _$ClientVersionRequirementSerializer();
}

class _$ClientVersionRequirementSerializer
    implements PrimitiveSerializer<ClientVersionRequirement> {
  @override
  final Iterable<Type> types = const [
    ClientVersionRequirement,
    _$ClientVersionRequirement,
  ];

  @override
  final String wireName = r'ClientVersionRequirement';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ClientVersionRequirement object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'platform';
    yield serializers.serialize(
      object.platform,
      specifiedType: const FullType(DevicePlatform),
    );
    yield r'min_version';
    yield serializers.serialize(
      object.minVersion,
      specifiedType: const FullType(String),
    );
    if (object.recommendedVersion != null) {
      yield r'recommended_version';
      yield serializers.serialize(
        object.recommendedVersion,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    ClientVersionRequirement object, {
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
    required ClientVersionRequirementBuilder result,
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
        case r'min_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.minVersion = valueDes;
          break;
        case r'recommended_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.recommendedVersion = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ClientVersionRequirement deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ClientVersionRequirementBuilder();
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
