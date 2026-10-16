//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/login_client.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'oidc_begin_request.g.dart';

/// OidcBeginRequest
///
/// Properties:
/// * [client]
/// * [providerId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [appRedirectUri] - Redirect target of the app (loopback or claimed scheme, RFC 8252). Must match the allow-list of the provider.
/// * [clientChallenge] - Base64url SHA-256 of a client verifier, binds the later `login_code` to this app instance.
/// * [enrollmentCode] - Optional enrollment code (purpose `first_login`, `recovery` or `identity_link`). Checked at begin, consumed at complete; an unknown subject is then linked to the person of the code (08, 3.1).
@BuiltValue()
abstract class OidcBeginRequest
    implements Built<OidcBeginRequest, OidcBeginRequestBuilder> {
  @BuiltValueField(wireName: r'client')
  LoginClient? get client;
  // enum clientEnum {  app,  console,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'provider_id')
  String get providerId;

  /// Redirect target of the app (loopback or claimed scheme, RFC 8252). Must match the allow-list of the provider.
  @BuiltValueField(wireName: r'app_redirect_uri')
  String get appRedirectUri;

  /// Base64url SHA-256 of a client verifier, binds the later `login_code` to this app instance.
  @BuiltValueField(wireName: r'client_challenge')
  String get clientChallenge;

  /// Optional enrollment code (purpose `first_login`, `recovery` or `identity_link`). Checked at begin, consumed at complete; an unknown subject is then linked to the person of the code (08, 3.1).
  @BuiltValueField(wireName: r'enrollment_code')
  String? get enrollmentCode;

  OidcBeginRequest._();

  factory OidcBeginRequest([void updates(OidcBeginRequestBuilder b)]) =
      _$OidcBeginRequest;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(OidcBeginRequestBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<OidcBeginRequest> get serializer =>
      _$OidcBeginRequestSerializer();
}

class _$OidcBeginRequestSerializer
    implements PrimitiveSerializer<OidcBeginRequest> {
  @override
  final Iterable<Type> types = const [OidcBeginRequest, _$OidcBeginRequest];

  @override
  final String wireName = r'OidcBeginRequest';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    OidcBeginRequest object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.client != null) {
      yield r'client';
      yield serializers.serialize(
        object.client,
        specifiedType: const FullType(LoginClient),
      );
    }
    yield r'provider_id';
    yield serializers.serialize(
      object.providerId,
      specifiedType: const FullType(String),
    );
    yield r'app_redirect_uri';
    yield serializers.serialize(
      object.appRedirectUri,
      specifiedType: const FullType(String),
    );
    yield r'client_challenge';
    yield serializers.serialize(
      object.clientChallenge,
      specifiedType: const FullType(String),
    );
    if (object.enrollmentCode != null) {
      yield r'enrollment_code';
      yield serializers.serialize(
        object.enrollmentCode,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    OidcBeginRequest object, {
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
    required OidcBeginRequestBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'client':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(LoginClient),
          ) as LoginClient?;
          if (valueDes == null) continue;
          result.client = valueDes;
          break;
        case r'provider_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.providerId = valueDes;
          break;
        case r'app_redirect_uri':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.appRedirectUri = valueDes;
          break;
        case r'client_challenge':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.clientChallenge = valueDes;
          break;
        case r'enrollment_code':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.enrollmentCode = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  OidcBeginRequest deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = OidcBeginRequestBuilder();
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
