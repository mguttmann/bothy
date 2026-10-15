//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'local_credential_set.g.dart';

/// Sets or replaces the local password, starts a TOTP enrollment (`totp_enroll`, the response carries the new secret once) or confirms it (`totp_code`). At least one member; the first call sets `password`. The password path counts as established once TOTP is confirmed (08, 2.5).
///
/// Properties:
/// * [password]
/// * [totpEnroll]
/// * [totpCode] - Current code of the enrolled TOTP secret; confirms the enrollment.
@BuiltValue()
abstract class LocalCredentialSet
    implements Built<LocalCredentialSet, LocalCredentialSetBuilder> {
  @BuiltValueField(wireName: r'password')
  String? get password;

  @BuiltValueField(wireName: r'totp_enroll')
  bool? get totpEnroll;

  /// Current code of the enrolled TOTP secret; confirms the enrollment.
  @BuiltValueField(wireName: r'totp_code')
  String? get totpCode;

  LocalCredentialSet._();

  factory LocalCredentialSet([void updates(LocalCredentialSetBuilder b)]) =
      _$LocalCredentialSet;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(LocalCredentialSetBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<LocalCredentialSet> get serializer =>
      _$LocalCredentialSetSerializer();
}

class _$LocalCredentialSetSerializer
    implements PrimitiveSerializer<LocalCredentialSet> {
  @override
  final Iterable<Type> types = const [LocalCredentialSet, _$LocalCredentialSet];

  @override
  final String wireName = r'LocalCredentialSet';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    LocalCredentialSet object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.password != null) {
      yield r'password';
      yield serializers.serialize(
        object.password,
        specifiedType: const FullType(String),
      );
    }
    if (object.totpEnroll != null) {
      yield r'totp_enroll';
      yield serializers.serialize(
        object.totpEnroll,
        specifiedType: const FullType(bool),
      );
    }
    if (object.totpCode != null) {
      yield r'totp_code';
      yield serializers.serialize(
        object.totpCode,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    LocalCredentialSet object, {
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
    required LocalCredentialSetBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'password':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.password = valueDes;
          break;
        case r'totp_enroll':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.totpEnroll = valueDes;
          break;
        case r'totp_code':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
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
  LocalCredentialSet deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = LocalCredentialSetBuilder();
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
