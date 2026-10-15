//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/signing_key_purpose.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'signing_key_rotation.g.dart';

/// SigningKeyRotation
///
/// Properties:
/// * [purpose]
/// * [newKid]
/// * [previousKid]
/// * [previousNotAfter] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class SigningKeyRotation
    implements Built<SigningKeyRotation, SigningKeyRotationBuilder> {
  @BuiltValueField(wireName: r'purpose')
  SigningKeyPurpose get purpose;
  // enum purposeEnum {  access_token,  invocation,  terminal,  webhook_out,  audit_checkpoint,  };

  @BuiltValueField(wireName: r'new_kid')
  String get newKid;

  @BuiltValueField(wireName: r'previous_kid')
  String get previousKid;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'previous_not_after')
  DateTime get previousNotAfter;

  SigningKeyRotation._();

  factory SigningKeyRotation([void updates(SigningKeyRotationBuilder b)]) =
      _$SigningKeyRotation;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SigningKeyRotationBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SigningKeyRotation> get serializer =>
      _$SigningKeyRotationSerializer();
}

class _$SigningKeyRotationSerializer
    implements PrimitiveSerializer<SigningKeyRotation> {
  @override
  final Iterable<Type> types = const [SigningKeyRotation, _$SigningKeyRotation];

  @override
  final String wireName = r'SigningKeyRotation';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SigningKeyRotation object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'purpose';
    yield serializers.serialize(
      object.purpose,
      specifiedType: const FullType(SigningKeyPurpose),
    );
    yield r'new_kid';
    yield serializers.serialize(
      object.newKid,
      specifiedType: const FullType(String),
    );
    yield r'previous_kid';
    yield serializers.serialize(
      object.previousKid,
      specifiedType: const FullType(String),
    );
    yield r'previous_not_after';
    yield serializers.serialize(
      object.previousNotAfter,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    SigningKeyRotation object, {
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
    required SigningKeyRotationBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'purpose':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(SigningKeyPurpose),
          ) as SigningKeyPurpose;
          result.purpose = valueDes;
          break;
        case r'new_kid':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.newKid = valueDes;
          break;
        case r'previous_kid':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.previousKid = valueDes;
          break;
        case r'previous_not_after':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.previousNotAfter = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  SigningKeyRotation deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SigningKeyRotationBuilder();
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
