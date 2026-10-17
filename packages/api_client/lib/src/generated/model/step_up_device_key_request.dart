//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'step_up_device_key_request.g.dart';

/// StepUpDeviceKeyRequest
///
/// Properties:
/// * [challengeId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [signature] - Compact JWS signed by the active `step_up` key of the session's device after user verification on the device. Header `alg` only (no `jwk`, `jku`, `x5u`, `x5c`, `crit`); payload `{\"challenge\": <challenge>, \"sid\": <session id>, \"iat\": <NumericDate>}` (08, 6.2).
@BuiltValue()
abstract class StepUpDeviceKeyRequest
    implements Built<StepUpDeviceKeyRequest, StepUpDeviceKeyRequestBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'challenge_id')
  String get challengeId;

  /// Compact JWS signed by the active `step_up` key of the session's device after user verification on the device. Header `alg` only (no `jwk`, `jku`, `x5u`, `x5c`, `crit`); payload `{\"challenge\": <challenge>, \"sid\": <session id>, \"iat\": <NumericDate>}` (08, 6.2).
  @BuiltValueField(wireName: r'signature')
  String get signature;

  StepUpDeviceKeyRequest._();

  factory StepUpDeviceKeyRequest([
    void updates(StepUpDeviceKeyRequestBuilder b),
  ]) = _$StepUpDeviceKeyRequest;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StepUpDeviceKeyRequestBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StepUpDeviceKeyRequest> get serializer =>
      _$StepUpDeviceKeyRequestSerializer();
}

class _$StepUpDeviceKeyRequestSerializer
    implements PrimitiveSerializer<StepUpDeviceKeyRequest> {
  @override
  final Iterable<Type> types = const [
    StepUpDeviceKeyRequest,
    _$StepUpDeviceKeyRequest,
  ];

  @override
  final String wireName = r'StepUpDeviceKeyRequest';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StepUpDeviceKeyRequest object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'challenge_id';
    yield serializers.serialize(
      object.challengeId,
      specifiedType: const FullType(String),
    );
    yield r'signature';
    yield serializers.serialize(
      object.signature,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    StepUpDeviceKeyRequest object, {
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
    required StepUpDeviceKeyRequestBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'challenge_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.challengeId = valueDes;
          break;
        case r'signature':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.signature = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StepUpDeviceKeyRequest deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StepUpDeviceKeyRequestBuilder();
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
