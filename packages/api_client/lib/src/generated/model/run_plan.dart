//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/run_plan_items_inner.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_plan.g.dart';

/// Schema `run_plan.v1`, shown in the live box.
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class RunPlan implements Built<RunPlan, RunPlanBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<RunPlanItemsInner> get items;

  RunPlan._();

  factory RunPlan([void updates(RunPlanBuilder b)]) = _$RunPlan;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunPlanBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunPlan> get serializer => _$RunPlanSerializer();
}

class _$RunPlanSerializer implements PrimitiveSerializer<RunPlan> {
  @override
  final Iterable<Type> types = const [RunPlan, _$RunPlan];

  @override
  final String wireName = r'RunPlan';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunPlan object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(RunPlanItemsInner)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    RunPlan object, {
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
    required RunPlanBuilder result,
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
              FullType(RunPlanItemsInner),
            ]),
          ) as BuiltList<RunPlanItemsInner>;
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
  RunPlan deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunPlanBuilder();
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
