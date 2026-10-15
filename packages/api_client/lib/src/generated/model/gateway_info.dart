//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'gateway_info.g.dart';

/// GatewayInfo
///
/// Properties:
/// * [url]
/// * [protocolVersions]
@BuiltValue()
abstract class GatewayInfo implements Built<GatewayInfo, GatewayInfoBuilder> {
  @BuiltValueField(wireName: r'url')
  String get url;

  @BuiltValueField(wireName: r'protocol_versions')
  BuiltList<int> get protocolVersions;

  GatewayInfo._();

  factory GatewayInfo([void updates(GatewayInfoBuilder b)]) = _$GatewayInfo;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GatewayInfoBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GatewayInfo> get serializer => _$GatewayInfoSerializer();
}

class _$GatewayInfoSerializer implements PrimitiveSerializer<GatewayInfo> {
  @override
  final Iterable<Type> types = const [GatewayInfo, _$GatewayInfo];

  @override
  final String wireName = r'GatewayInfo';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GatewayInfo object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'url';
    yield serializers.serialize(
      object.url,
      specifiedType: const FullType(String),
    );
    yield r'protocol_versions';
    yield serializers.serialize(
      object.protocolVersions,
      specifiedType: const FullType(BuiltList, [FullType(int)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    GatewayInfo object, {
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
    required GatewayInfoBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'url':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.url = valueDes;
          break;
        case r'protocol_versions':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(int)]),
          ) as BuiltList<int>;
          result.protocolVersions.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GatewayInfo deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GatewayInfoBuilder();
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
