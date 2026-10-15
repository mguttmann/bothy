//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'local_credential_result.g.dart';

/// LocalCredentialResult
///
/// Properties:
/// * [passwordSet]
/// * [totpEnabled] - True once a TOTP enrollment is confirmed.
/// * [totpSecretUri] - otpauth URI, returned once when `totp_enroll` was set.
@BuiltValue()
abstract class LocalCredentialResult
    implements Built<LocalCredentialResult, LocalCredentialResultBuilder> {
  @BuiltValueField(wireName: r'password_set')
  bool get passwordSet;

  /// True once a TOTP enrollment is confirmed.
  @BuiltValueField(wireName: r'totp_enabled')
  bool get totpEnabled;

  /// otpauth URI, returned once when `totp_enroll` was set.
  @BuiltValueField(wireName: r'totp_secret_uri')
  String? get totpSecretUri;

  LocalCredentialResult._();

  factory LocalCredentialResult([
    void updates(LocalCredentialResultBuilder b),
  ]) = _$LocalCredentialResult;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(LocalCredentialResultBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<LocalCredentialResult> get serializer =>
      _$LocalCredentialResultSerializer();
}

class _$LocalCredentialResultSerializer
    implements PrimitiveSerializer<LocalCredentialResult> {
  @override
  final Iterable<Type> types = const [
    LocalCredentialResult,
    _$LocalCredentialResult,
  ];

  @override
  final String wireName = r'LocalCredentialResult';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    LocalCredentialResult object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'password_set';
    yield serializers.serialize(
      object.passwordSet,
      specifiedType: const FullType(bool),
    );
    yield r'totp_enabled';
    yield serializers.serialize(
      object.totpEnabled,
      specifiedType: const FullType(bool),
    );
    if (object.totpSecretUri != null) {
      yield r'totp_secret_uri';
      yield serializers.serialize(
        object.totpSecretUri,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    LocalCredentialResult object, {
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
    required LocalCredentialResultBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'password_set':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.passwordSet = valueDes;
          break;
        case r'totp_enabled':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.totpEnabled = valueDes;
          break;
        case r'totp_secret_uri':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.totpSecretUri = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  LocalCredentialResult deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = LocalCredentialResultBuilder();
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
