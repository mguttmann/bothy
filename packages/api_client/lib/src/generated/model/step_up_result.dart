//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'step_up_result.g.dart';

/// StepUpResult
///
/// Properties:
/// * [stepUpAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [validUntil] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
@BuiltValue()
abstract class StepUpResult
    implements Built<StepUpResult, StepUpResultBuilder> {
  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'step_up_at')
  DateTime get stepUpAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'valid_until')
  DateTime get validUntil;

  StepUpResult._();

  factory StepUpResult([void updates(StepUpResultBuilder b)]) = _$StepUpResult;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StepUpResultBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StepUpResult> get serializer => _$StepUpResultSerializer();
}

class _$StepUpResultSerializer implements PrimitiveSerializer<StepUpResult> {
  @override
  final Iterable<Type> types = const [StepUpResult, _$StepUpResult];

  @override
  final String wireName = r'StepUpResult';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StepUpResult object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'step_up_at';
    yield serializers.serialize(
      object.stepUpAt,
      specifiedType: const FullType(DateTime),
    );
    yield r'valid_until';
    yield serializers.serialize(
      object.validUntil,
      specifiedType: const FullType(DateTime),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    StepUpResult object, {
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
    required StepUpResultBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'step_up_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.stepUpAt = valueDes;
          break;
        case r'valid_until':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.validUntil = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StepUpResult deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StepUpResultBuilder();
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
