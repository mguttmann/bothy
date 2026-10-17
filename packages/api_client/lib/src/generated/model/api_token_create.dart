//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/app_scope.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'api_token_create.g.dart';

/// ApiTokenCreate
///
/// Properties:
/// * [name]
/// * [scopes] - Only for app tokens, a subset of the granted scopes of the installation. Agent tokens (all agent scopes implicitly) and personal tokens (the grants of the person) carry none.
/// * [dpopJkt] - RFC 7638 thumbprint (43 base64url characters) of the key the token is bound to; requests must then use DPoP. Required for the kinds in `auth.dpop_required_for`.
/// * [expiresAt] - Required; at most `auth.api_token_max_ttl` (agents, apps) or `auth.personal_token_max_ttl` (personal) ahead.
@BuiltValue()
abstract class ApiTokenCreate
    implements Built<ApiTokenCreate, ApiTokenCreateBuilder> {
  @BuiltValueField(wireName: r'name')
  String get name;

  /// Only for app tokens, a subset of the granted scopes of the installation. Agent tokens (all agent scopes implicitly) and personal tokens (the grants of the person) carry none.
  @BuiltValueField(wireName: r'scopes')
  BuiltList<AppScope>? get scopes;

  /// RFC 7638 thumbprint (43 base64url characters) of the key the token is bound to; requests must then use DPoP. Required for the kinds in `auth.dpop_required_for`.
  @BuiltValueField(wireName: r'dpop_jkt')
  String? get dpopJkt;

  /// Required; at most `auth.api_token_max_ttl` (agents, apps) or `auth.personal_token_max_ttl` (personal) ahead.
  @BuiltValueField(wireName: r'expires_at')
  DateTime get expiresAt;

  ApiTokenCreate._();

  factory ApiTokenCreate([void updates(ApiTokenCreateBuilder b)]) =
      _$ApiTokenCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ApiTokenCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ApiTokenCreate> get serializer =>
      _$ApiTokenCreateSerializer();
}

class _$ApiTokenCreateSerializer
    implements PrimitiveSerializer<ApiTokenCreate> {
  @override
  final Iterable<Type> types = const [ApiTokenCreate, _$ApiTokenCreate];

  @override
  final String wireName = r'ApiTokenCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ApiTokenCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    if (object.scopes != null) {
      yield r'scopes';
      yield serializers.serialize(
        object.scopes,
        specifiedType: const FullType(BuiltList, [FullType(AppScope)]),
      );
    }
    if (object.dpopJkt != null) {
      yield r'dpop_jkt';
      yield serializers.serialize(
        object.dpopJkt,
        specifiedType: const FullType(String),
      );
    }
    yield r'expires_at';
    yield serializers.serialize(
      object.expiresAt,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ApiTokenCreate object, {
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
    required ApiTokenCreateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.name = valueDes;
          break;
        case r'scopes':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(AppScope),
            ]),
          ) as BuiltList<AppScope>?;
          if (valueDes == null) continue;
          result.scopes.replace(valueDes);
          break;
        case r'dpop_jkt':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.dpopJkt = valueDes;
          break;
        case r'expires_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.expiresAt = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ApiTokenCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ApiTokenCreateBuilder();
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
