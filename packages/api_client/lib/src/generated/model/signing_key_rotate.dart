//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/signing_key_purpose.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'signing_key_rotate.g.dart';

/// SigningKeyRotate
///
/// Properties:
/// * [purpose]
/// * [mode] - `scheduled` announces the next key (state `next`); the rotation job makes it `active` after `agent.jwks_max_age`. `emergency` retires every key of the purpose at once and creates a new `active` key. Required, because the two modes have opposite effects.
/// * [reason]
@BuiltValue()
abstract class SigningKeyRotate
    implements Built<SigningKeyRotate, SigningKeyRotateBuilder> {
  @BuiltValueField(wireName: r'purpose')
  SigningKeyPurpose get purpose;
  // enum purposeEnum {  access_token,  invocation,  terminal,  webhook_out,  audit_checkpoint,  };

  /// `scheduled` announces the next key (state `next`); the rotation job makes it `active` after `agent.jwks_max_age`. `emergency` retires every key of the purpose at once and creates a new `active` key. Required, because the two modes have opposite effects.
  @BuiltValueField(wireName: r'mode')
  SigningKeyRotateModeEnum get mode;
  // enum modeEnum {  scheduled,  emergency,  };

  @BuiltValueField(wireName: r'reason')
  String get reason;

  SigningKeyRotate._();

  factory SigningKeyRotate([void updates(SigningKeyRotateBuilder b)]) =
      _$SigningKeyRotate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SigningKeyRotateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SigningKeyRotate> get serializer =>
      _$SigningKeyRotateSerializer();
}

class _$SigningKeyRotateSerializer
    implements PrimitiveSerializer<SigningKeyRotate> {
  @override
  final Iterable<Type> types = const [SigningKeyRotate, _$SigningKeyRotate];

  @override
  final String wireName = r'SigningKeyRotate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SigningKeyRotate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'purpose';
    yield serializers.serialize(
      object.purpose,
      specifiedType: const FullType(SigningKeyPurpose),
    );
    yield r'mode';
    yield serializers.serialize(
      object.mode,
      specifiedType: const FullType(SigningKeyRotateModeEnum),
    );
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    SigningKeyRotate object, {
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
    required SigningKeyRotateBuilder result,
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
        case r'mode':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(SigningKeyRotateModeEnum),
          ) as SigningKeyRotateModeEnum;
          result.mode = valueDes;
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.reason = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  SigningKeyRotate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SigningKeyRotateBuilder();
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

/// `scheduled` announces the next key (state `next`); the rotation job makes it `active` after `agent.jwks_max_age`. `emergency` retires every key of the purpose at once and creates a new `active` key. Required, because the two modes have opposite effects.
class SigningKeyRotateModeEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'scheduled')
  static const SigningKeyRotateModeEnum scheduled =
      _$signingKeyRotateModeEnum_scheduled;
  @BuiltValueEnumConst(wireName: r'emergency')
  static const SigningKeyRotateModeEnum emergency =
      _$signingKeyRotateModeEnum_emergency;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const SigningKeyRotateModeEnum unknownDefaultOpenApi =
      _$signingKeyRotateModeEnum_unknownDefaultOpenApi;

  static Serializer<SigningKeyRotateModeEnum> get serializer =>
      _$signingKeyRotateModeEnumSerializer;

  const SigningKeyRotateModeEnum._(String name) : super(name);

  static BuiltSet<SigningKeyRotateModeEnum> get values =>
      _$signingKeyRotateModeEnumValues;
  static SigningKeyRotateModeEnum valueOf(String name) =>
      _$signingKeyRotateModeEnumValueOf(name);
}
