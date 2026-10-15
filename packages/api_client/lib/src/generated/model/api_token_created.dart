//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/api_token.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'api_token_created.g.dart';

/// ApiTokenCreated
///
/// Properties:
/// * [token]
/// * [secret] - Full token value `bothy_<kind>_<random>_<checksum>` (prefix for secret scanners). Shown exactly once.
@BuiltValue()
abstract class ApiTokenCreated
    implements Built<ApiTokenCreated, ApiTokenCreatedBuilder> {
  @BuiltValueField(wireName: r'token')
  ApiToken get token;

  /// Full token value `bothy_<kind>_<random>_<checksum>` (prefix for secret scanners). Shown exactly once.
  @BuiltValueField(wireName: r'secret')
  String get secret;

  ApiTokenCreated._();

  factory ApiTokenCreated([void updates(ApiTokenCreatedBuilder b)]) =
      _$ApiTokenCreated;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ApiTokenCreatedBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ApiTokenCreated> get serializer =>
      _$ApiTokenCreatedSerializer();
}

class _$ApiTokenCreatedSerializer
    implements PrimitiveSerializer<ApiTokenCreated> {
  @override
  final Iterable<Type> types = const [ApiTokenCreated, _$ApiTokenCreated];

  @override
  final String wireName = r'ApiTokenCreated';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ApiTokenCreated object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'token';
    yield serializers.serialize(
      object.token,
      specifiedType: const FullType(ApiToken),
    );
    yield r'secret';
    yield serializers.serialize(
      object.secret,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ApiTokenCreated object, {
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
    required ApiTokenCreatedBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'token':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ApiToken),
          ) as ApiToken;
          result.token.replace(valueDes);
          break;
        case r'secret':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.secret = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ApiTokenCreated deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ApiTokenCreatedBuilder();
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
