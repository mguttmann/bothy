//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/cost_bucket.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/stats_granularity.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'cost_series.g.dart';

/// CostSeries
///
/// Properties:
/// * [granularity]
/// * [currency]
/// * [buckets]
@BuiltValue()
abstract class CostSeries implements Built<CostSeries, CostSeriesBuilder> {
  @BuiltValueField(wireName: r'granularity')
  StatsGranularity get granularity;
  // enum granularityEnum {  hour,  day,  week,  month,  };

  @BuiltValueField(wireName: r'currency')
  String get currency;

  @BuiltValueField(wireName: r'buckets')
  BuiltList<CostBucket> get buckets;

  CostSeries._();

  factory CostSeries([void updates(CostSeriesBuilder b)]) = _$CostSeries;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(CostSeriesBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<CostSeries> get serializer => _$CostSeriesSerializer();
}

class _$CostSeriesSerializer implements PrimitiveSerializer<CostSeries> {
  @override
  final Iterable<Type> types = const [CostSeries, _$CostSeries];

  @override
  final String wireName = r'CostSeries';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    CostSeries object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'granularity';
    yield serializers.serialize(
      object.granularity,
      specifiedType: const FullType(StatsGranularity),
    );
    yield r'currency';
    yield serializers.serialize(
      object.currency,
      specifiedType: const FullType(String),
    );
    yield r'buckets';
    yield serializers.serialize(
      object.buckets,
      specifiedType: const FullType(BuiltList, [FullType(CostBucket)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    CostSeries object, {
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
    required CostSeriesBuilder result,
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
        case r'currency':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.currency = valueDes;
          break;
        case r'buckets':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(CostBucket)]),
          ) as BuiltList<CostBucket>;
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
  CostSeries deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = CostSeriesBuilder();
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
