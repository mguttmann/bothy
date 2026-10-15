//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'enrollment_code.g.dart';

/// One-time code shown once to the owner, handed to the person out of band. A replay of the creating request with the same `Idempotency-Key` does not repeat it (`409 conflict`, `reason: one_time_secret`).
///
/// Properties:
/// * [code]
/// * [purpose]
/// * [expiresAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class EnrollmentCode
    implements Built<EnrollmentCode, EnrollmentCodeBuilder> {
  @BuiltValueField(wireName: r'code')
  String get code;

  @BuiltValueField(wireName: r'purpose')
  EnrollmentCodePurposeEnum get purpose;
  // enum purposeEnum {  first_login,  recovery,  identity_link,  };

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'expires_at')
  DateTime get expiresAt;

  EnrollmentCode._();

  factory EnrollmentCode([void updates(EnrollmentCodeBuilder b)]) =
      _$EnrollmentCode;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(EnrollmentCodeBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<EnrollmentCode> get serializer =>
      _$EnrollmentCodeSerializer();
}

class _$EnrollmentCodeSerializer
    implements PrimitiveSerializer<EnrollmentCode> {
  @override
  final Iterable<Type> types = const [EnrollmentCode, _$EnrollmentCode];

  @override
  final String wireName = r'EnrollmentCode';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    EnrollmentCode object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'code';
    yield serializers.serialize(
      object.code,
      specifiedType: const FullType(String),
    );
    yield r'purpose';
    yield serializers.serialize(
      object.purpose,
      specifiedType: const FullType(EnrollmentCodePurposeEnum),
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
    EnrollmentCode object, {
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
    required EnrollmentCodeBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'code':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.code = valueDes;
          break;
        case r'purpose':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(EnrollmentCodePurposeEnum),
          ) as EnrollmentCodePurposeEnum;
          result.purpose = valueDes;
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
  EnrollmentCode deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = EnrollmentCodeBuilder();
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

class EnrollmentCodePurposeEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'first_login')
  static const EnrollmentCodePurposeEnum firstLogin =
      _$enrollmentCodePurposeEnum_firstLogin;
  @BuiltValueEnumConst(wireName: r'recovery')
  static const EnrollmentCodePurposeEnum recovery =
      _$enrollmentCodePurposeEnum_recovery;
  @BuiltValueEnumConst(wireName: r'identity_link')
  static const EnrollmentCodePurposeEnum identityLink =
      _$enrollmentCodePurposeEnum_identityLink;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const EnrollmentCodePurposeEnum unknownDefaultOpenApi =
      _$enrollmentCodePurposeEnum_unknownDefaultOpenApi;

  static Serializer<EnrollmentCodePurposeEnum> get serializer =>
      _$enrollmentCodePurposeEnumSerializer;

  const EnrollmentCodePurposeEnum._(String name) : super(name);

  static BuiltSet<EnrollmentCodePurposeEnum> get values =>
      _$enrollmentCodePurposeEnumValues;
  static EnrollmentCodePurposeEnum valueOf(String name) =>
      _$enrollmentCodePurposeEnumValueOf(name);
}
