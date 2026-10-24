//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'stats_bucket.g.dart';

/// One time bucket of one group. `group_key` is set when the query groups (space, agent, tool).
///
/// Properties:
/// * [start] - Start of the bucket as a UTC instant (local midnight of `stats.time_zone` for `day`, `week` and `month`).
/// * [groupKey] - Space ID, agent ID or tool name, as the operation names it; never a person.
/// * [metrics] - Metric name to value. Names per endpoint are listed in its description; a metric without a value is absent.
/// * [belowMinGroup] - Metrics that count people and were withheld because their value is below `privacy.stats_min_group_size`; the client shows \"fewer than\" the minimum group.
@BuiltValue()
abstract class StatsBucket implements Built<StatsBucket, StatsBucketBuilder> {
  /// Start of the bucket as a UTC instant (local midnight of `stats.time_zone` for `day`, `week` and `month`).
  @BuiltValueField(wireName: r'start')
  DateTime get start;

  /// Space ID, agent ID or tool name, as the operation names it; never a person.
  @BuiltValueField(wireName: r'group_key')
  String? get groupKey;

  /// Metric name to value. Names per endpoint are listed in its description; a metric without a value is absent.
  @BuiltValueField(wireName: r'metrics')
  BuiltMap<String, double> get metrics;

  /// Metrics that count people and were withheld because their value is below `privacy.stats_min_group_size`; the client shows \"fewer than\" the minimum group.
  @BuiltValueField(wireName: r'below_min_group')
  BuiltList<String>? get belowMinGroup;

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
    if (object.belowMinGroup != null) {
      yield r'below_min_group';
      yield serializers.serialize(
        object.belowMinGroup,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
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
        case r'below_min_group':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.belowMinGroup.replace(valueDes);
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
