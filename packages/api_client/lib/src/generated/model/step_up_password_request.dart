//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'step_up_password_request.g.dart';

/// StepUpPasswordRequest
///
/// Properties:
/// * [challengeId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [password]
/// * [totpCode]
@BuiltValue()
abstract class StepUpPasswordRequest
    implements Built<StepUpPasswordRequest, StepUpPasswordRequestBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'challenge_id')
  String get challengeId;

  @BuiltValueField(wireName: r'password')
  String get password;

  @BuiltValueField(wireName: r'totp_code')
  String get totpCode;

  StepUpPasswordRequest._();

  factory StepUpPasswordRequest([
    void updates(StepUpPasswordRequestBuilder b),
  ]) = _$StepUpPasswordRequest;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StepUpPasswordRequestBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StepUpPasswordRequest> get serializer =>
      _$StepUpPasswordRequestSerializer();
}

class _$StepUpPasswordRequestSerializer
    implements PrimitiveSerializer<StepUpPasswordRequest> {
  @override
  final Iterable<Type> types = const [
    StepUpPasswordRequest,
    _$StepUpPasswordRequest,
  ];

  @override
  final String wireName = r'StepUpPasswordRequest';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StepUpPasswordRequest object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'challenge_id';
    yield serializers.serialize(
      object.challengeId,
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
  }

  @override
  Object serialize(
    Serializers serializers,
    StepUpPasswordRequest object, {
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
    required StepUpPasswordRequestBuilder result,
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
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StepUpPasswordRequest deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StepUpPasswordRequestBuilder();
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
