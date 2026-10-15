//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/device_registration.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'trusted_header_login_request.g.dart';

/// TrustedHeaderLoginRequest
///
/// Properties:
/// * [device]
/// * [enrollmentCode] - Optional enrollment code; links an unknown header login to the person of the code (08, 3.1).
@BuiltValue()
abstract class TrustedHeaderLoginRequest
    implements
        Built<TrustedHeaderLoginRequest, TrustedHeaderLoginRequestBuilder> {
  @BuiltValueField(wireName: r'device')
  DeviceRegistration get device;

  /// Optional enrollment code; links an unknown header login to the person of the code (08, 3.1).
  @BuiltValueField(wireName: r'enrollment_code')
  String? get enrollmentCode;

  TrustedHeaderLoginRequest._();

  factory TrustedHeaderLoginRequest([
    void updates(TrustedHeaderLoginRequestBuilder b),
  ]) = _$TrustedHeaderLoginRequest;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(TrustedHeaderLoginRequestBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<TrustedHeaderLoginRequest> get serializer =>
      _$TrustedHeaderLoginRequestSerializer();
}

class _$TrustedHeaderLoginRequestSerializer
    implements PrimitiveSerializer<TrustedHeaderLoginRequest> {
  @override
  final Iterable<Type> types = const [
    TrustedHeaderLoginRequest,
    _$TrustedHeaderLoginRequest,
  ];

  @override
  final String wireName = r'TrustedHeaderLoginRequest';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    TrustedHeaderLoginRequest object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'device';
    yield serializers.serialize(
      object.device,
      specifiedType: const FullType(DeviceRegistration),
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
    TrustedHeaderLoginRequest object, {
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
    required TrustedHeaderLoginRequestBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'device':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DeviceRegistration),
          ) as DeviceRegistration;
          result.device.replace(valueDes);
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
  TrustedHeaderLoginRequest deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = TrustedHeaderLoginRequestBuilder();
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
