//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/stats_granularity.dart';
import 'package:api_client/src/generated/model/stats_bucket.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'stats_series.g.dart';

/// StatsSeries
///
/// Properties:
/// * [granularity]
/// * [buckets]
@BuiltValue()
abstract class StatsSeries implements Built<StatsSeries, StatsSeriesBuilder> {
  @BuiltValueField(wireName: r'granularity')
  StatsGranularity get granularity;
  // enum granularityEnum {  hour,  day,  week,  month,  };

  @BuiltValueField(wireName: r'buckets')
  BuiltList<StatsBucket> get buckets;

  StatsSeries._();

  factory StatsSeries([void updates(StatsSeriesBuilder b)]) = _$StatsSeries;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatsSeriesBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatsSeries> get serializer => _$StatsSeriesSerializer();
}

class _$StatsSeriesSerializer implements PrimitiveSerializer<StatsSeries> {
  @override
  final Iterable<Type> types = const [StatsSeries, _$StatsSeries];

  @override
  final String wireName = r'StatsSeries';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatsSeries object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'granularity';
    yield serializers.serialize(
      object.granularity,
      specifiedType: const FullType(StatsGranularity),
    );
    yield r'buckets';
    yield serializers.serialize(
      object.buckets,
      specifiedType: const FullType(BuiltList, [FullType(StatsBucket)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    StatsSeries object, {
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
    required StatsSeriesBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'granularity':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(StatsGranularity),
          ) as StatsGranularity;
          result.granularity = valueDes;
          break;
        case r'buckets':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(StatsBucket)]),
          ) as BuiltList<StatsBucket>;
          result.buckets.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StatsSeries deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatsSeriesBuilder();
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
