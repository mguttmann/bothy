//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/device_registration.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'passkey_login_complete_request.g.dart';

/// PasskeyLoginCompleteRequest
///
/// Properties:
/// * [challengeId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [credential] - WebAuthn `PublicKeyCredential` in JSON form.
/// * [device]
@BuiltValue()
abstract class PasskeyLoginCompleteRequest
    implements
        Built<PasskeyLoginCompleteRequest, PasskeyLoginCompleteRequestBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'challenge_id')
  String get challengeId;

  /// WebAuthn `PublicKeyCredential` in JSON form.
  @BuiltValueField(wireName: r'credential')
  BuiltMap<String, JsonObject?> get credential;

  @BuiltValueField(wireName: r'device')
  DeviceRegistration get device;

  PasskeyLoginCompleteRequest._();

  factory PasskeyLoginCompleteRequest([
    void updates(PasskeyLoginCompleteRequestBuilder b),
  ]) = _$PasskeyLoginCompleteRequest;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(PasskeyLoginCompleteRequestBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<PasskeyLoginCompleteRequest> get serializer =>
      _$PasskeyLoginCompleteRequestSerializer();
}

class _$PasskeyLoginCompleteRequestSerializer
    implements PrimitiveSerializer<PasskeyLoginCompleteRequest> {
  @override
  final Iterable<Type> types = const [
    PasskeyLoginCompleteRequest,
    _$PasskeyLoginCompleteRequest,
  ];

  @override
  final String wireName = r'PasskeyLoginCompleteRequest';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    PasskeyLoginCompleteRequest object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'challenge_id';
    yield serializers.serialize(
      object.challengeId,
      specifiedType: const FullType(String),
    );
    yield r'credential';
    yield serializers.serialize(
      object.credential,
      specifiedType: const FullType(BuiltMap, [
        FullType(String),
        FullType.nullable(JsonObject),
      ]),
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
    PasskeyLoginCompleteRequest object, {
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
    required PasskeyLoginCompleteRequestBuilder result,
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
        case r'credential':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>;
          result.credential.replace(valueDes);
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
  PasskeyLoginCompleteRequest deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = PasskeyLoginCompleteRequestBuilder();
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
