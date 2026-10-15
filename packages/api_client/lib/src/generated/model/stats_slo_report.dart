//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/stats_slo_entry.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'stats_slo_report.g.dart';

/// StatsSloReport
///
/// Properties:
/// * [at] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [measuredBy] - Always `own`; the values come from the own measurement, not from Prometheus.
/// * [items]
@BuiltValue()
abstract class StatsSloReport
    implements Built<StatsSloReport, StatsSloReportBuilder> {
  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'at')
  DateTime get at;

  /// Always `own`; the values come from the own measurement, not from Prometheus.
  @BuiltValueField(wireName: r'measured_by')
  StatsSloReportMeasuredByEnum get measuredBy;
  // enum measuredByEnum {  own,  };

  @BuiltValueField(wireName: r'items')
  BuiltList<StatsSloEntry> get items;

  StatsSloReport._();

  factory StatsSloReport([void updates(StatsSloReportBuilder b)]) =
      _$StatsSloReport;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatsSloReportBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatsSloReport> get serializer =>
      _$StatsSloReportSerializer();
}

class _$StatsSloReportSerializer
    implements PrimitiveSerializer<StatsSloReport> {
  @override
  final Iterable<Type> types = const [StatsSloReport, _$StatsSloReport];

  @override
  final String wireName = r'StatsSloReport';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatsSloReport object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'at';
    yield serializers.serialize(
      object.at,
      specifiedType: const FullType(DateTime),
    );
    yield r'measured_by';
    yield serializers.serialize(
      object.measuredBy,
      specifiedType: const FullType(StatsSloReportMeasuredByEnum),
    );
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(StatsSloEntry)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    StatsSloReport object, {
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
    required StatsSloReportBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.at = valueDes;
          break;
        case r'measured_by':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(StatsSloReportMeasuredByEnum),
          ) as StatsSloReportMeasuredByEnum;
          result.measuredBy = valueDes;
          break;
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(StatsSloEntry)]),
          ) as BuiltList<StatsSloEntry>;
          result.items.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StatsSloReport deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatsSloReportBuilder();
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

/// Always `own`; the values come from the own measurement, not from Prometheus.
class StatsSloReportMeasuredByEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'own')
  static const StatsSloReportMeasuredByEnum own =
      _$statsSloReportMeasuredByEnum_own;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const StatsSloReportMeasuredByEnum unknownDefaultOpenApi =
      _$statsSloReportMeasuredByEnum_unknownDefaultOpenApi;

  static Serializer<StatsSloReportMeasuredByEnum> get serializer =>
      _$statsSloReportMeasuredByEnumSerializer;

  const StatsSloReportMeasuredByEnum._(String name) : super(name);

  static BuiltSet<StatsSloReportMeasuredByEnum> get values =>
      _$statsSloReportMeasuredByEnumValues;
  static StatsSloReportMeasuredByEnum valueOf(String name) =>
      _$statsSloReportMeasuredByEnumValueOf(name);
}
