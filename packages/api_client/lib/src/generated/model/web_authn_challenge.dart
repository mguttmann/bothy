//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/json_object.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'web_authn_challenge.g.dart';

/// WebAuthnChallenge
///
/// Properties:
/// * [challengeId] - ID of the stored ceremony challenge; valid once until `expires_at`.
/// * [publicKey] - WebAuthn `PublicKeyCredentialRequestOptions` or `PublicKeyCredentialCreationOptions` in JSON form.
/// * [expiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class WebAuthnChallenge
    implements Built<WebAuthnChallenge, WebAuthnChallengeBuilder> {
  /// ID of the stored ceremony challenge; valid once until `expires_at`.
  @BuiltValueField(wireName: r'challenge_id')
  String get challengeId;

  /// WebAuthn `PublicKeyCredentialRequestOptions` or `PublicKeyCredentialCreationOptions` in JSON form.
  @BuiltValueField(wireName: r'public_key')
  BuiltMap<String, JsonObject?> get publicKey;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'expires_at')
  DateTime get expiresAt;

  WebAuthnChallenge._();

  factory WebAuthnChallenge([void updates(WebAuthnChallengeBuilder b)]) =
      _$WebAuthnChallenge;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(WebAuthnChallengeBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<WebAuthnChallenge> get serializer =>
      _$WebAuthnChallengeSerializer();
}

class _$WebAuthnChallengeSerializer
    implements PrimitiveSerializer<WebAuthnChallenge> {
  @override
  final Iterable<Type> types = const [WebAuthnChallenge, _$WebAuthnChallenge];

  @override
  final String wireName = r'WebAuthnChallenge';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    WebAuthnChallenge object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'challenge_id';
    yield serializers.serialize(
      object.challengeId,
      specifiedType: const FullType(String),
    );
    yield r'public_key';
    yield serializers.serialize(
      object.publicKey,
      specifiedType: const FullType(BuiltMap, [
        FullType(String),
        FullType.nullable(JsonObject),
      ]),
    );
    yield r'expires_at';
    yield serializers.serialize(
      object.expiresAt,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    WebAuthnChallenge object, {
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
    required WebAuthnChallengeBuilder result,
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
        case r'public_key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltMap, [
              FullType(String),
              FullType.nullable(JsonObject),
            ]),
          ) as BuiltMap<String, JsonObject?>;
          result.publicKey.replace(valueDes);
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
  WebAuthnChallenge deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = WebAuthnChallengeBuilder();
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
