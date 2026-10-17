//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/stats_backend_agent.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'stats_backend_report.g.dart';

/// StatsBackendReport
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class StatsBackendReport
    implements Built<StatsBackendReport, StatsBackendReportBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<StatsBackendAgent> get items;

  StatsBackendReport._();

  factory StatsBackendReport([void updates(StatsBackendReportBuilder b)]) =
      _$StatsBackendReport;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatsBackendReportBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatsBackendReport> get serializer =>
      _$StatsBackendReportSerializer();
}

class _$StatsBackendReportSerializer
    implements PrimitiveSerializer<StatsBackendReport> {
  @override
  final Iterable<Type> types = const [StatsBackendReport, _$StatsBackendReport];

  @override
  final String wireName = r'StatsBackendReport';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatsBackendReport object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(StatsBackendAgent)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    StatsBackendReport object, {
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
    required StatsBackendReportBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(StatsBackendAgent),
            ]),
          ) as BuiltList<StatsBackendAgent>;
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
  StatsBackendReport deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatsBackendReportBuilder();
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
