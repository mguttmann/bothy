//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/cost_report.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/run_metrics.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_finish.g.dart';

/// RunFinish
///
/// Properties:
/// * [state]
/// * [reason] - Required for `failed` and `cancelled`, e.g. `billing_mode_mismatch`, `adapter_error`, `user_cancelled`.
/// * [summary]
/// * [metrics]
/// * [costs]
@BuiltValue()
abstract class RunFinish implements Built<RunFinish, RunFinishBuilder> {
  @BuiltValueField(wireName: r'state')
  RunFinishStateEnum get state;
  // enum stateEnum {  completed,  failed,  cancelled,  };

  /// Required for `failed` and `cancelled`, e.g. `billing_mode_mismatch`, `adapter_error`, `user_cancelled`.
  @BuiltValueField(wireName: r'reason')
  String? get reason;

  @BuiltValueField(wireName: r'summary')
  String? get summary;

  @BuiltValueField(wireName: r'metrics')
  RunMetrics? get metrics;

  @BuiltValueField(wireName: r'costs')
  BuiltList<CostReport>? get costs;

  RunFinish._();

  factory RunFinish([void updates(RunFinishBuilder b)]) = _$RunFinish;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunFinishBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunFinish> get serializer => _$RunFinishSerializer();
}

class _$RunFinishSerializer implements PrimitiveSerializer<RunFinish> {
  @override
  final Iterable<Type> types = const [RunFinish, _$RunFinish];

  @override
  final String wireName = r'RunFinish';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunFinish object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(RunFinishStateEnum),
    );
    if (object.reason != null) {
      yield r'reason';
      yield serializers.serialize(
        object.reason,
        specifiedType: const FullType(String),
      );
    }
    if (object.summary != null) {
      yield r'summary';
      yield serializers.serialize(
        object.summary,
        specifiedType: const FullType(String),
      );
    }
    if (object.metrics != null) {
      yield r'metrics';
      yield serializers.serialize(
        object.metrics,
        specifiedType: const FullType(RunMetrics),
      );
    }
    if (object.costs != null) {
      yield r'costs';
      yield serializers.serialize(
        object.costs,
        specifiedType: const FullType(BuiltList, [FullType(CostReport)]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    RunFinish object, {
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
    required RunFinishBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(RunFinishStateEnum),
          ) as RunFinishStateEnum;
          result.state = valueDes;
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.reason = valueDes;
          break;
        case r'summary':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.summary = valueDes;
          break;
        case r'metrics':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(RunMetrics),
          ) as RunMetrics?;
          if (valueDes == null) continue;
          result.metrics.replace(valueDes);
          break;
        case r'costs':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(CostReport),
            ]),
          ) as BuiltList<CostReport>?;
          if (valueDes == null) continue;
          result.costs.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunFinish deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunFinishBuilder();
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

class RunFinishStateEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'completed')
  static const RunFinishStateEnum completed = _$runFinishStateEnum_completed;
  @BuiltValueEnumConst(wireName: r'failed')
  static const RunFinishStateEnum failed = _$runFinishStateEnum_failed;
  @BuiltValueEnumConst(wireName: r'cancelled')
  static const RunFinishStateEnum cancelled = _$runFinishStateEnum_cancelled;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const RunFinishStateEnum unknownDefaultOpenApi =
      _$runFinishStateEnum_unknownDefaultOpenApi;

  static Serializer<RunFinishStateEnum> get serializer =>
      _$runFinishStateEnumSerializer;

  const RunFinishStateEnum._(String name) : super(name);

  static BuiltSet<RunFinishStateEnum> get values => _$runFinishStateEnumValues;
  static RunFinishStateEnum valueOf(String name) =>
      _$runFinishStateEnumValueOf(name);
}
