//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_api_retry.g.dart';

/// Last retry of a model API call, shown in the live box while set.
///
/// Properties:
/// * [attempt]
/// * [maxRetries]
/// * [retryDelayMs]
/// * [error]
@BuiltValue()
abstract class RunApiRetry implements Built<RunApiRetry, RunApiRetryBuilder> {
  @BuiltValueField(wireName: r'attempt')
  int get attempt;

  @BuiltValueField(wireName: r'max_retries')
  int? get maxRetries;

  @BuiltValueField(wireName: r'retry_delay_ms')
  int? get retryDelayMs;

  @BuiltValueField(wireName: r'error')
  String? get error;

  RunApiRetry._();

  factory RunApiRetry([void updates(RunApiRetryBuilder b)]) = _$RunApiRetry;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunApiRetryBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunApiRetry> get serializer => _$RunApiRetrySerializer();
}

class _$RunApiRetrySerializer implements PrimitiveSerializer<RunApiRetry> {
  @override
  final Iterable<Type> types = const [RunApiRetry, _$RunApiRetry];

  @override
  final String wireName = r'RunApiRetry';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunApiRetry object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'attempt';
    yield serializers.serialize(
      object.attempt,
      specifiedType: const FullType(int),
    );
    if (object.maxRetries != null) {
      yield r'max_retries';
      yield serializers.serialize(
        object.maxRetries,
        specifiedType: const FullType(int),
      );
    }
    if (object.retryDelayMs != null) {
      yield r'retry_delay_ms';
      yield serializers.serialize(
        object.retryDelayMs,
        specifiedType: const FullType(int),
      );
    }
    if (object.error != null) {
      yield r'error';
      yield serializers.serialize(
        object.error,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    RunApiRetry object, {
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
    required RunApiRetryBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'attempt':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.attempt = valueDes;
          break;
        case r'max_retries':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.maxRetries = valueDes;
          break;
        case r'retry_delay_ms':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.retryDelayMs = valueDes;
          break;
        case r'error':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.error = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunApiRetry deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunApiRetryBuilder();
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
