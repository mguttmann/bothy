//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'field_error.g.dart';

/// FieldError
///
/// Properties:
/// * [pointer] - Where the violation is: a JSON Pointer (RFC 6901) to the offending member of the request body (for an unknown or missing member the pointer names that member, e.g. `/name`), or `#path/<name>`, `#query/<name>`, `#header/<name>` for parameters; empty for a rule of the whole request or the body as a whole.
/// * [code] - Machine-readable rule the field violates. Schema validation uses `required`, `unknown_member` (W7: request schemas are closed), `invalid_type`, `invalid_format`, `invalid_enum`, `too_long`, `too_short`, `too_large`, `too_small`, `too_many_items`, `too_few_items`, `pattern_mismatch` and `invalid` for any other rule; use cases add `limit_reached` and `not_enabled` (06 section 5). Clients treat unknown codes like `invalid`.
/// * [detail] - Short English explanation; never repeats the submitted value.
@BuiltValue()
abstract class FieldError implements Built<FieldError, FieldErrorBuilder> {
  /// Where the violation is: a JSON Pointer (RFC 6901) to the offending member of the request body (for an unknown or missing member the pointer names that member, e.g. `/name`), or `#path/<name>`, `#query/<name>`, `#header/<name>` for parameters; empty for a rule of the whole request or the body as a whole.
  @BuiltValueField(wireName: r'pointer')
  String get pointer;

  /// Machine-readable rule the field violates. Schema validation uses `required`, `unknown_member` (W7: request schemas are closed), `invalid_type`, `invalid_format`, `invalid_enum`, `too_long`, `too_short`, `too_large`, `too_small`, `too_many_items`, `too_few_items`, `pattern_mismatch` and `invalid` for any other rule; use cases add `limit_reached` and `not_enabled` (06 section 5). Clients treat unknown codes like `invalid`.
  @BuiltValueField(wireName: r'code')
  String get code;

  /// Short English explanation; never repeats the submitted value.
  @BuiltValueField(wireName: r'detail')
  String? get detail;

  FieldError._();

  factory FieldError([void updates(FieldErrorBuilder b)]) = _$FieldError;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(FieldErrorBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<FieldError> get serializer => _$FieldErrorSerializer();
}

class _$FieldErrorSerializer implements PrimitiveSerializer<FieldError> {
  @override
  final Iterable<Type> types = const [FieldError, _$FieldError];

  @override
  final String wireName = r'FieldError';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    FieldError object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'pointer';
    yield serializers.serialize(
      object.pointer,
      specifiedType: const FullType(String),
    );
    yield r'code';
    yield serializers.serialize(
      object.code,
      specifiedType: const FullType(String),
    );
    if (object.detail != null) {
      yield r'detail';
      yield serializers.serialize(
        object.detail,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    FieldError object, {
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
    required FieldErrorBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'pointer':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.pointer = valueDes;
          break;
        case r'code':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.code = valueDes;
          break;
        case r'detail':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.detail = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  FieldError deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = FieldErrorBuilder();
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
