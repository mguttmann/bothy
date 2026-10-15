//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'stats_live_step.g.dart';

/// Current step of a run; only kind and name (for tools the tool name), never title or detail.
///
/// Properties:
/// * [kind]
/// * [name]
@BuiltValue()
abstract class StatsLiveStep
    implements Built<StatsLiveStep, StatsLiveStepBuilder> {
  @BuiltValueField(wireName: r'kind')
  StatsLiveStepKindEnum get kind;
  // enum kindEnum {  tool_call,  status,  subagent,  workflow,  background_job,  };

  @BuiltValueField(wireName: r'name')
  String get name;

  StatsLiveStep._();

  factory StatsLiveStep([void updates(StatsLiveStepBuilder b)]) =
      _$StatsLiveStep;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(StatsLiveStepBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<StatsLiveStep> get serializer =>
      _$StatsLiveStepSerializer();
}

class _$StatsLiveStepSerializer implements PrimitiveSerializer<StatsLiveStep> {
  @override
  final Iterable<Type> types = const [StatsLiveStep, _$StatsLiveStep];

  @override
  final String wireName = r'StatsLiveStep';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    StatsLiveStep object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(StatsLiveStepKindEnum),
    );
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    StatsLiveStep object, {
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
    required StatsLiveStepBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(StatsLiveStepKindEnum),
          ) as StatsLiveStepKindEnum;
          result.kind = valueDes;
          break;
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.name = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  StatsLiveStep deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = StatsLiveStepBuilder();
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

class StatsLiveStepKindEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'tool_call')
  static const StatsLiveStepKindEnum toolCall =
      _$statsLiveStepKindEnum_toolCall;
  @BuiltValueEnumConst(wireName: r'status')
  static const StatsLiveStepKindEnum status = _$statsLiveStepKindEnum_status;
  @BuiltValueEnumConst(wireName: r'subagent')
  static const StatsLiveStepKindEnum subagent =
      _$statsLiveStepKindEnum_subagent;
  @BuiltValueEnumConst(wireName: r'workflow')
  static const StatsLiveStepKindEnum workflow =
      _$statsLiveStepKindEnum_workflow;
  @BuiltValueEnumConst(wireName: r'background_job')
  static const StatsLiveStepKindEnum backgroundJob =
      _$statsLiveStepKindEnum_backgroundJob;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const StatsLiveStepKindEnum unknownDefaultOpenApi =
      _$statsLiveStepKindEnum_unknownDefaultOpenApi;

  static Serializer<StatsLiveStepKindEnum> get serializer =>
      _$statsLiveStepKindEnumSerializer;

  const StatsLiveStepKindEnum._(String name) : super(name);

  static BuiltSet<StatsLiveStepKindEnum> get values =>
      _$statsLiveStepKindEnumValues;
  static StatsLiveStepKindEnum valueOf(String name) =>
      _$statsLiveStepKindEnumValueOf(name);
}
