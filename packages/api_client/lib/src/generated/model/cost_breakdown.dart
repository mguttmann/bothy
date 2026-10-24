//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/cost_total.dart';
import 'package:api_client/src/generated/model/run_cost.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'cost_breakdown.g.dart';

/// CostBreakdown
///
/// Properties:
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [totals] - Valid sums per billing mode and currency, never converted; sums of mode `personal` only when `billing.show_personal_estimates` is on.
/// * [runs] - Runs of the thread with at least one valid cost entry, by `run_id` ascending.
@BuiltValue()
abstract class CostBreakdown
    implements Built<CostBreakdown, CostBreakdownBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'thread_id')
  String get threadId;

  /// Valid sums per billing mode and currency, never converted; sums of mode `personal` only when `billing.show_personal_estimates` is on.
  @BuiltValueField(wireName: r'totals')
  BuiltList<CostTotal> get totals;

  /// Runs of the thread with at least one valid cost entry, by `run_id` ascending.
  @BuiltValueField(wireName: r'runs')
  BuiltList<RunCost> get runs;

  CostBreakdown._();

  factory CostBreakdown([void updates(CostBreakdownBuilder b)]) =
      _$CostBreakdown;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(CostBreakdownBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<CostBreakdown> get serializer =>
      _$CostBreakdownSerializer();
}

class _$CostBreakdownSerializer implements PrimitiveSerializer<CostBreakdown> {
  @override
  final Iterable<Type> types = const [CostBreakdown, _$CostBreakdown];

  @override
  final String wireName = r'CostBreakdown';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    CostBreakdown object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'thread_id';
    yield serializers.serialize(
      object.threadId,
      specifiedType: const FullType(String),
    );
    yield r'totals';
    yield serializers.serialize(
      object.totals,
      specifiedType: const FullType(BuiltList, [FullType(CostTotal)]),
    );
    yield r'runs';
    yield serializers.serialize(
      object.runs,
      specifiedType: const FullType(BuiltList, [FullType(RunCost)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    CostBreakdown object, {
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
    required CostBreakdownBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'thread_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.threadId = valueDes;
          break;
        case r'totals':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(CostTotal)]),
          ) as BuiltList<CostTotal>;
          result.totals.replace(valueDes);
          break;
        case r'runs':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(RunCost)]),
          ) as BuiltList<RunCost>;
          result.runs.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  CostBreakdown deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = CostBreakdownBuilder();
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
