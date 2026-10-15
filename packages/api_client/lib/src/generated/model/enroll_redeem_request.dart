//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/device_registration.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'enroll_redeem_request.g.dart';

/// Redeems an enrollment code with device keys only; the redeeming device becomes `active` (08, 3.2).
///
/// Properties:
/// * [enrollmentCode]
/// * [device]
@BuiltValue()
abstract class EnrollRedeemRequest
    implements Built<EnrollRedeemRequest, EnrollRedeemRequestBuilder> {
  @BuiltValueField(wireName: r'enrollment_code')
  String get enrollmentCode;

  @BuiltValueField(wireName: r'device')
  DeviceRegistration get device;

  EnrollRedeemRequest._();

  factory EnrollRedeemRequest([void updates(EnrollRedeemRequestBuilder b)]) =
      _$EnrollRedeemRequest;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(EnrollRedeemRequestBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<EnrollRedeemRequest> get serializer =>
      _$EnrollRedeemRequestSerializer();
}

class _$EnrollRedeemRequestSerializer
    implements PrimitiveSerializer<EnrollRedeemRequest> {
  @override
  final Iterable<Type> types = const [
    EnrollRedeemRequest,
    _$EnrollRedeemRequest,
  ];

  @override
  final String wireName = r'EnrollRedeemRequest';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    EnrollRedeemRequest object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'enrollment_code';
    yield serializers.serialize(
      object.enrollmentCode,
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
    EnrollRedeemRequest object, {
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
    required EnrollRedeemRequestBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'enrollment_code':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.enrollmentCode = valueDes;
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
  EnrollRedeemRequest deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = EnrollRedeemRequestBuilder();
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
