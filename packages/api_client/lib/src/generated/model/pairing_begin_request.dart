//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/device_registration.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'pairing_begin_request.g.dart';

/// PairingBeginRequest
///
/// Properties:
/// * [device]
@BuiltValue()
abstract class PairingBeginRequest
    implements Built<PairingBeginRequest, PairingBeginRequestBuilder> {
  @BuiltValueField(wireName: r'device')
  DeviceRegistration get device;

  PairingBeginRequest._();

  factory PairingBeginRequest([void updates(PairingBeginRequestBuilder b)]) =
      _$PairingBeginRequest;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(PairingBeginRequestBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<PairingBeginRequest> get serializer =>
      _$PairingBeginRequestSerializer();
}

class _$PairingBeginRequestSerializer
    implements PrimitiveSerializer<PairingBeginRequest> {
  @override
  final Iterable<Type> types = const [
    PairingBeginRequest,
    _$PairingBeginRequest,
  ];

  @override
  final String wireName = r'PairingBeginRequest';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    PairingBeginRequest object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'device';
    yield serializers.serialize(
      object.device,
      specifiedType: const FullType(DeviceRegistration),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    PairingBeginRequest object, {
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
    required PairingBeginRequestBuilder result,
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
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  PairingBeginRequest deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = PairingBeginRequestBuilder();
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
