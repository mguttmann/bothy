//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'stats_bucket.g.dart';

/// One time bucket. `group_key` is set when the query groups (space, agent, tool).
///
/// Properties:
/// * [start] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [groupKey]
/// * [metrics] - Metric name to value. Names per endpoint are listed in its description.
@BuiltValue()
abstract class StatsBucket implements Built<StatsBucket, StatsBucketBuilder> {
  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'start')
  DateTime get start;

  @BuiltValueField(wireName: r'group_key')
  String? get groupKey;

  /// Metric name to value. Names per endpoint are listed in its description.
  @BuiltValueField(wireName: r'metrics')
  BuiltMap<String, double> get metrics;

  StatsBucket._();

  factory StatsBucket([void updates(StatsBucketBuilder b)]) = _$StatsBucket;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatsBucketBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatsBucket> get serializer => _$StatsBucketSerializer();
}

class _$StatsBucketSerializer implements PrimitiveSerializer<StatsBucket> {
  @override
  final Iterable<Type> types = const [StatsBucket, _$StatsBucket];

  @override
  final String wireName = r'StatsBucket';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatsBucket object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'start';
    yield serializers.serialize(
      object.start,
      specifiedType: const FullType(DateTime),
    );
    if (object.groupKey != null) {
      yield r'group_key';
      yield serializers.serialize(
        object.groupKey,
        specifiedType: const FullType(String),
      );
    }
    yield r'metrics';
    yield serializers.serialize(
      object.metrics,
      specifiedType: const FullType(BuiltMap, [
        FullType(String),
        FullType(double),
      ]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    StatsBucket object, {
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
    required StatsBucketBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'start':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.start = valueDes;
          break;
        case r'group_key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.groupKey = valueDes;
          break;
        case r'metrics':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltMap, [
              FullType(String),
              FullType(double),
            ]),
          ) as BuiltMap<String, double>;
          result.metrics.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StatsBucket deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatsBucketBuilder();
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
