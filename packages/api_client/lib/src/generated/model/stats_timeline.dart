//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/stats_timeline_run.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/stats_timeline_bucket.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'stats_timeline.g.dart';

/// StatsTimeline
///
/// Properties:
/// * [from] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [to] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [buckets]
/// * [runs]
/// * [runsTruncated]
@BuiltValue()
abstract class StatsTimeline
    implements Built<StatsTimeline, StatsTimelineBuilder> {
  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'from')
  DateTime get from;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'to')
  DateTime get to;

  @BuiltValueField(wireName: r'buckets')
  BuiltList<StatsTimelineBucket> get buckets;

  @BuiltValueField(wireName: r'runs')
  BuiltList<StatsTimelineRun> get runs;

  @BuiltValueField(wireName: r'runs_truncated')
  bool get runsTruncated;

  StatsTimeline._();

  factory StatsTimeline([void updates(StatsTimelineBuilder b)]) =
      _$StatsTimeline;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatsTimelineBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatsTimeline> get serializer =>
      _$StatsTimelineSerializer();
}

class _$StatsTimelineSerializer implements PrimitiveSerializer<StatsTimeline> {
  @override
  final Iterable<Type> types = const [StatsTimeline, _$StatsTimeline];

  @override
  final String wireName = r'StatsTimeline';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatsTimeline object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'from';
    yield serializers.serialize(
      object.from,
      specifiedType: const FullType(DateTime),
    );
    yield r'to';
    yield serializers.serialize(
      object.to,
      specifiedType: const FullType(DateTime),
    );
    yield r'buckets';
    yield serializers.serialize(
      object.buckets,
      specifiedType: const FullType(BuiltList, [FullType(StatsTimelineBucket)]),
    );
    yield r'runs';
    yield serializers.serialize(
      object.runs,
      specifiedType: const FullType(BuiltList, [FullType(StatsTimelineRun)]),
    );
    yield r'runs_truncated';
    yield serializers.serialize(
      object.runsTruncated,
      specifiedType: const FullType(bool),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    StatsTimeline object, {
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
    required StatsTimelineBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'from':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.from = valueDes;
          break;
        case r'to':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.to = valueDes;
          break;
        case r'buckets':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(StatsTimelineBucket),
            ]),
          ) as BuiltList<StatsTimelineBucket>;
          result.buckets.replace(valueDes);
          break;
        case r'runs':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(StatsTimelineRun),
            ]),
          ) as BuiltList<StatsTimelineRun>;
          result.runs.replace(valueDes);
          break;
        case r'runs_truncated':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.runsTruncated = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StatsTimeline deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatsTimelineBuilder();
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
