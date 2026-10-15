//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/field_error.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'manifest_validation.g.dart';

/// ManifestValidation
///
/// Properties:
/// * [valid]
/// * [errors]
@BuiltValue()
abstract class ManifestValidation
    implements Built<ManifestValidation, ManifestValidationBuilder> {
  @BuiltValueField(wireName: r'valid')
  bool get valid;

  @BuiltValueField(wireName: r'errors')
  BuiltList<FieldError> get errors;

  ManifestValidation._();

  factory ManifestValidation([void updates(ManifestValidationBuilder b)]) =
      _$ManifestValidation;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ManifestValidationBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ManifestValidation> get serializer =>
      _$ManifestValidationSerializer();
}

class _$ManifestValidationSerializer
    implements PrimitiveSerializer<ManifestValidation> {
  @override
  final Iterable<Type> types = const [ManifestValidation, _$ManifestValidation];

  @override
  final String wireName = r'ManifestValidation';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ManifestValidation object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'valid';
    yield serializers.serialize(
      object.valid,
      specifiedType: const FullType(bool),
    );
    yield r'errors';
    yield serializers.serialize(
      object.errors,
      specifiedType: const FullType(BuiltList, [FullType(FieldError)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ManifestValidation object, {
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
    required ManifestValidationBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'valid':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.valid = valueDes;
          break;
        case r'errors':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(FieldError)]),
          ) as BuiltList<FieldError>;
          result.errors.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ManifestValidation deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ManifestValidationBuilder();
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
