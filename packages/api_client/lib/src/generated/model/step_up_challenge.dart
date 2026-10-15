//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'step_up_challenge.g.dart';

/// StepUpChallenge
///
/// Properties:
/// * [challengeId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [challenge] - Base64url random value to sign. Single use, valid `auth.step_up_challenge_ttl`. Clients show their own localized prompt text.
/// * [passkeyOptions] - WebAuthn request options when the user has passkeys; their challenge is this step-up challenge.
/// * [expiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class StepUpChallenge
    implements Built<StepUpChallenge, StepUpChallengeBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'challenge_id')
  String get challengeId;

  /// Base64url random value to sign. Single use, valid `auth.step_up_challenge_ttl`. Clients show their own localized prompt text.
  @BuiltValueField(wireName: r'challenge')
  String get challenge;

  /// WebAuthn request options when the user has passkeys; their challenge is this step-up challenge.
  @BuiltValueField(wireName: r'passkey_options')
  BuiltMap<String, JsonObject?>? get passkeyOptions;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'expires_at')
  DateTime get expiresAt;

  StepUpChallenge._();

  factory StepUpChallenge([void updates(StepUpChallengeBuilder b)]) =
      _$StepUpChallenge;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StepUpChallengeBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StepUpChallenge> get serializer =>
      _$StepUpChallengeSerializer();
}

class _$StepUpChallengeSerializer
    implements PrimitiveSerializer<StepUpChallenge> {
  @override
  final Iterable<Type> types = const [StepUpChallenge, _$StepUpChallenge];

  @override
  final String wireName = r'StepUpChallenge';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StepUpChallenge object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'challenge_id';
    yield serializers.serialize(
      object.challengeId,
      specifiedType: const FullType(String),
    );
    yield r'challenge';
    yield serializers.serialize(
      object.challenge,
      specifiedType: const FullType(String),
    );
    if (object.passkeyOptions != null) {
      yield r'passkey_options';
      yield serializers.serialize(
        object.passkeyOptions,
        specifiedType: const FullType(BuiltMap, [
          FullType(String),
          FullType.nullable(JsonObject),
        ]),
      );
    }
    yield r'expires_at';
    yield serializers.serialize(
      object.expiresAt,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    StepUpChallenge object, {
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
    required StepUpChallengeBuilder result,
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
        case r'challenge':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.challenge = valueDes;
          break;
        case r'passkey_options':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>?;
          if (valueDes == null) continue;
          result.passkeyOptions.replace(valueDes);
          break;
        case r'expires_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.expiresAt = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StepUpChallenge deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StepUpChallengeBuilder();
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
