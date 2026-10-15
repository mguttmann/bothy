//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'run_plan_items_inner.g.dart';

/// RunPlanItemsInner
///
/// Properties:
/// * [id]
/// * [text]
/// * [status]
@BuiltValue()
abstract class RunPlanItemsInner
    implements Built<RunPlanItemsInner, RunPlanItemsInnerBuilder> {
  @BuiltValueField(wireName: r'id')
  String get id;

  @BuiltValueField(wireName: r'text')
  String get text;

  @BuiltValueField(wireName: r'status')
  RunPlanItemsInnerStatusEnum get status;
  // enum statusEnum {  pending,  in_progress,  completed,  };

  RunPlanItemsInner._();

  factory RunPlanItemsInner([void updates(RunPlanItemsInnerBuilder b)]) =
      _$RunPlanItemsInner;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(RunPlanItemsInnerBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<RunPlanItemsInner> get serializer =>
      _$RunPlanItemsInnerSerializer();
}

class _$RunPlanItemsInnerSerializer
    implements PrimitiveSerializer<RunPlanItemsInner> {
  @override
  final Iterable<Type> types = const [RunPlanItemsInner, _$RunPlanItemsInner];

  @override
  final String wireName = r'RunPlanItemsInner';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    RunPlanItemsInner object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'text';
    yield serializers.serialize(
      object.text,
      specifiedType: const FullType(String),
    );
    yield r'status';
    yield serializers.serialize(
      object.status,
      specifiedType: const FullType(RunPlanItemsInnerStatusEnum),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    RunPlanItemsInner object, {
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
    required RunPlanItemsInnerBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.id = valueDes;
          break;
        case r'text':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.text = valueDes;
          break;
        case r'status':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(RunPlanItemsInnerStatusEnum),
          ) as RunPlanItemsInnerStatusEnum;
          result.status = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  RunPlanItemsInner deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = RunPlanItemsInnerBuilder();
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

class RunPlanItemsInnerStatusEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'pending')
  static const RunPlanItemsInnerStatusEnum pending =
      _$runPlanItemsInnerStatusEnum_pending;
  @BuiltValueEnumConst(wireName: r'in_progress')
  static const RunPlanItemsInnerStatusEnum inProgress =
      _$runPlanItemsInnerStatusEnum_inProgress;
  @BuiltValueEnumConst(wireName: r'completed')
  static const RunPlanItemsInnerStatusEnum completed =
      _$runPlanItemsInnerStatusEnum_completed;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const RunPlanItemsInnerStatusEnum unknownDefaultOpenApi =
      _$runPlanItemsInnerStatusEnum_unknownDefaultOpenApi;

  static Serializer<RunPlanItemsInnerStatusEnum> get serializer =>
      _$runPlanItemsInnerStatusEnumSerializer;

  const RunPlanItemsInnerStatusEnum._(String name) : super(name);

  static BuiltSet<RunPlanItemsInnerStatusEnum> get values =>
      _$runPlanItemsInnerStatusEnumValues;
  static RunPlanItemsInnerStatusEnum valueOf(String name) =>
      _$runPlanItemsInnerStatusEnumValueOf(name);
}
