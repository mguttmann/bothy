//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'gateway_reauth.g.dart';

/// GatewayReauth
///
/// Properties:
/// * [accessToken]
/// * [dpop]
@BuiltValue()
abstract class GatewayReauth
    implements Built<GatewayReauth, GatewayReauthBuilder> {
  @BuiltValueField(wireName: r'access_token')
  String get accessToken;

  @BuiltValueField(wireName: r'dpop')
  String get dpop;

  GatewayReauth._();

  factory GatewayReauth([void updates(GatewayReauthBuilder b)]) =
      _$GatewayReauth;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(GatewayReauthBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<GatewayReauth> get serializer =>
      _$GatewayReauthSerializer();
}

class _$GatewayReauthSerializer implements PrimitiveSerializer<GatewayReauth> {
  @override
  final Iterable<Type> types = const [GatewayReauth, _$GatewayReauth];

  @override
  final String wireName = r'GatewayReauth';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    GatewayReauth object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'access_token';
    yield serializers.serialize(
      object.accessToken,
      specifiedType: const FullType(String),
    );
    yield r'dpop';
    yield serializers.serialize(
      object.dpop,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    GatewayReauth object, {
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
    required GatewayReauthBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'access_token':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.accessToken = valueDes;
          break;
        case r'dpop':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.dpop = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  GatewayReauth deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = GatewayReauthBuilder();
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
