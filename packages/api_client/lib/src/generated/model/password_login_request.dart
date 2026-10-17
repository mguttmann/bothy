//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/device_registration.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'password_login_request.g.dart';

/// PasswordLoginRequest
///
/// Properties:
/// * [loginName]
/// * [password]
/// * [totpCode]
/// * [device]
@BuiltValue()
abstract class PasswordLoginRequest
    implements Built<PasswordLoginRequest, PasswordLoginRequestBuilder> {
  @BuiltValueField(wireName: r'login_name')
  String get loginName;

  @BuiltValueField(wireName: r'password')
  String get password;

  @BuiltValueField(wireName: r'totp_code')
  String get totpCode;

  @BuiltValueField(wireName: r'device')
  DeviceRegistration get device;

  PasswordLoginRequest._();

  factory PasswordLoginRequest([void updates(PasswordLoginRequestBuilder b)]) =
      _$PasswordLoginRequest;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(PasswordLoginRequestBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<PasswordLoginRequest> get serializer =>
      _$PasswordLoginRequestSerializer();
}

class _$PasswordLoginRequestSerializer
    implements PrimitiveSerializer<PasswordLoginRequest> {
  @override
  final Iterable<Type> types = const [
    PasswordLoginRequest,
    _$PasswordLoginRequest,
  ];

  @override
  final String wireName = r'PasswordLoginRequest';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    PasswordLoginRequest object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'login_name';
    yield serializers.serialize(
      object.loginName,
      specifiedType: const FullType(String),
    );
    yield r'password';
    yield serializers.serialize(
      object.password,
      specifiedType: const FullType(String),
    );
    yield r'totp_code';
    yield serializers.serialize(
      object.totpCode,
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
    PasswordLoginRequest object, {
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
    required PasswordLoginRequestBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'login_name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.loginName = valueDes;
          break;
        case r'password':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.password = valueDes;
          break;
        case r'totp_code':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.totpCode = valueDes;
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
  PasswordLoginRequest deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = PasswordLoginRequestBuilder();
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
