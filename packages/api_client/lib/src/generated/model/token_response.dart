//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/device.dart';
import 'package:api_client/src/generated/model/session.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'token_response.g.dart';

/// Token pair after login or refresh. `token_type` is always `DPoP` (RFC 9449).
///
/// Properties:
/// * [accessToken]
/// * [tokenType]
/// * [expiresIn] - Seconds until the access token expires.
/// * [refreshToken]
/// * [refreshExpiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [session]
/// * [device]
@BuiltValue()
abstract class TokenResponse
    implements Built<TokenResponse, TokenResponseBuilder> {
  @BuiltValueField(wireName: r'access_token')
  String get accessToken;

  @BuiltValueField(wireName: r'token_type')
  TokenResponseTokenTypeEnum get tokenType;
  // enum tokenTypeEnum {  DPoP,  };

  /// Seconds until the access token expires.
  @BuiltValueField(wireName: r'expires_in')
  int get expiresIn;

  @BuiltValueField(wireName: r'refresh_token')
  String get refreshToken;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'refresh_expires_at')
  DateTime get refreshExpiresAt;

  @BuiltValueField(wireName: r'session')
  Session get session;

  @BuiltValueField(wireName: r'device')
  Device get device;

  TokenResponse._();

  factory TokenResponse([void updates(TokenResponseBuilder b)]) =
      _$TokenResponse;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(TokenResponseBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<TokenResponse> get serializer =>
      _$TokenResponseSerializer();
}

class _$TokenResponseSerializer implements PrimitiveSerializer<TokenResponse> {
  @override
  final Iterable<Type> types = const [TokenResponse, _$TokenResponse];

  @override
  final String wireName = r'TokenResponse';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    TokenResponse object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'access_token';
    yield serializers.serialize(
      object.accessToken,
      specifiedType: const FullType(String),
    );
    yield r'token_type';
    yield serializers.serialize(
      object.tokenType,
      specifiedType: const FullType(TokenResponseTokenTypeEnum),
    );
    yield r'expires_in';
    yield serializers.serialize(
      object.expiresIn,
      specifiedType: const FullType(int),
    );
    yield r'refresh_token';
    yield serializers.serialize(
      object.refreshToken,
      specifiedType: const FullType(String),
    );
    yield r'refresh_expires_at';
    yield serializers.serialize(
      object.refreshExpiresAt,
      specifiedType: const FullType(DateTime),
    );
    yield r'session';
    yield serializers.serialize(
      object.session,
      specifiedType: const FullType(Session),
    );
    yield r'device';
    yield serializers.serialize(
      object.device,
      specifiedType: const FullType(Device),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    TokenResponse object, {
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
    required TokenResponseBuilder result,
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
        case r'token_type':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(TokenResponseTokenTypeEnum),
          ) as TokenResponseTokenTypeEnum;
          result.tokenType = valueDes;
          break;
        case r'expires_in':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.expiresIn = valueDes;
          break;
        case r'refresh_token':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.refreshToken = valueDes;
          break;
        case r'refresh_expires_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.refreshExpiresAt = valueDes;
          break;
        case r'session':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(Session),
          ) as Session;
          result.session.replace(valueDes);
          break;
        case r'device':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(Device),
          ) as Device;
          result.device.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  TokenResponse deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = TokenResponseBuilder();
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

class TokenResponseTokenTypeEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'DPoP')
  static const TokenResponseTokenTypeEnum dPoP =
      _$tokenResponseTokenTypeEnum_dPoP;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const TokenResponseTokenTypeEnum unknownDefaultOpenApi =
      _$tokenResponseTokenTypeEnum_unknownDefaultOpenApi;

  static Serializer<TokenResponseTokenTypeEnum> get serializer =>
      _$tokenResponseTokenTypeEnumSerializer;

  const TokenResponseTokenTypeEnum._(String name) : super(name);

  static BuiltSet<TokenResponseTokenTypeEnum> get values =>
      _$tokenResponseTokenTypeEnumValues;
  static TokenResponseTokenTypeEnum valueOf(String name) =>
      _$tokenResponseTokenTypeEnumValueOf(name);
}
