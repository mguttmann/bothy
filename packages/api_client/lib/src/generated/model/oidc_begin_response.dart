//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'oidc_begin_response.g.dart';

/// OidcBeginResponse
///
/// Properties:
/// * [loginAttemptId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [authorizationUrl]
/// * [expiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class OidcBeginResponse
    implements Built<OidcBeginResponse, OidcBeginResponseBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'login_attempt_id')
  String get loginAttemptId;

  @BuiltValueField(wireName: r'authorization_url')
  String get authorizationUrl;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'expires_at')
  DateTime get expiresAt;

  OidcBeginResponse._();

  factory OidcBeginResponse([void updates(OidcBeginResponseBuilder b)]) =
      _$OidcBeginResponse;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(OidcBeginResponseBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<OidcBeginResponse> get serializer =>
      _$OidcBeginResponseSerializer();
}

class _$OidcBeginResponseSerializer
    implements PrimitiveSerializer<OidcBeginResponse> {
  @override
  final Iterable<Type> types = const [OidcBeginResponse, _$OidcBeginResponse];

  @override
  final String wireName = r'OidcBeginResponse';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    OidcBeginResponse object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'login_attempt_id';
    yield serializers.serialize(
      object.loginAttemptId,
      specifiedType: const FullType(String),
    );
    yield r'authorization_url';
    yield serializers.serialize(
      object.authorizationUrl,
      specifiedType: const FullType(String),
    );
    yield r'expires_at';
    yield serializers.serialize(
      object.expiresAt,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    OidcBeginResponse object, {
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
    required OidcBeginResponseBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'login_attempt_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.loginAttemptId = valueDes;
          break;
        case r'authorization_url':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.authorizationUrl = valueDes;
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
  OidcBeginResponse deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = OidcBeginResponseBuilder();
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
