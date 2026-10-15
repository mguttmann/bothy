//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/device_registration.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'oidc_complete_request.g.dart';

/// OidcCompleteRequest
///
/// Properties:
/// * [loginCode] - One-time code the server appended to `app_redirect_uri` after the provider callback.
/// * [clientVerifier]
/// * [device]
@BuiltValue()
abstract class OidcCompleteRequest
    implements Built<OidcCompleteRequest, OidcCompleteRequestBuilder> {
  /// One-time code the server appended to `app_redirect_uri` after the provider callback.
  @BuiltValueField(wireName: r'login_code')
  String get loginCode;

  @BuiltValueField(wireName: r'client_verifier')
  String get clientVerifier;

  @BuiltValueField(wireName: r'device')
  DeviceRegistration get device;

  OidcCompleteRequest._();

  factory OidcCompleteRequest([void updates(OidcCompleteRequestBuilder b)]) =
      _$OidcCompleteRequest;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(OidcCompleteRequestBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<OidcCompleteRequest> get serializer =>
      _$OidcCompleteRequestSerializer();
}

class _$OidcCompleteRequestSerializer
    implements PrimitiveSerializer<OidcCompleteRequest> {
  @override
  final Iterable<Type> types = const [
    OidcCompleteRequest,
    _$OidcCompleteRequest,
  ];

  @override
  final String wireName = r'OidcCompleteRequest';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    OidcCompleteRequest object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'login_code';
    yield serializers.serialize(
      object.loginCode,
      specifiedType: const FullType(String),
    );
    yield r'client_verifier';
    yield serializers.serialize(
      object.clientVerifier,
      specifiedType: const FullType(String),
    );
    yield r'device';
    yield serializers.serialize(
      object.device,
      specifiedType: const FullType(DeviceRegistration),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    OidcCompleteRequest object, {
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
    required OidcCompleteRequestBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'login_code':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.loginCode = valueDes;
          break;
        case r'client_verifier':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.clientVerifier = valueDes;
          break;
        case r'device':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DeviceRegistration),
          ) as DeviceRegistration;
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
  OidcCompleteRequest deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = OidcCompleteRequestBuilder();
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
