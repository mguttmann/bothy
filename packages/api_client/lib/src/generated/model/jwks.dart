//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'jwks.g.dart';

/// JSON Web Key Set (RFC 7517) with the public keys for access tokens, invocation tokens and terminal session tokens.
///
/// Properties:
/// * [keys]
@BuiltValue()
abstract class Jwks implements Built<Jwks, JwksBuilder> {
  @BuiltValueField(wireName: r'keys')
  BuiltList<BuiltMap<String, JsonObject?>> get keys;

  Jwks._();

  factory Jwks([void updates(JwksBuilder b)]) = _$Jwks;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(JwksBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<Jwks> get serializer => _$JwksSerializer();
}

class _$JwksSerializer implements PrimitiveSerializer<Jwks> {
  @override
  final Iterable<Type> types = const [Jwks, _$Jwks];

  @override
  final String wireName = r'Jwks';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    Jwks object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'keys';
    yield serializers.serialize(
      object.keys,
      specifiedType: const FullType(BuiltList, [
        FullType(BuiltMap, [FullType(String), FullType.nullable(JsonObject)]),
      ]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    Jwks object, {
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
    required JwksBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'keys':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(BuiltMap, [
                FullType(String),
                FullType.nullable(JsonObject),
              ]),
            ]),
          ) as BuiltList<BuiltMap<String, JsonObject?>>;
          result.keys.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  Jwks deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = JwksBuilder();
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
