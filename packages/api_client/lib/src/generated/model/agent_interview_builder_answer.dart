//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/cost_report.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/agent_draft.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_interview_builder_answer.g.dart';

/// Answer of the builder agent to `agent.interview_requested` (action `agent.interview.respond`, 26 6.1 item 2). It has no member for settings files, hooks, permission rules, commands, subagents or plugin parts (26 4.3 item 1).
///
/// Properties:
/// * [kind]
/// * [questions] - Only with `kind = questions`; each at most `agents.interview.answer_max_chars` characters.
/// * [draft]
/// * [costs] - Usage of the model call, as in `run.finish`.
@BuiltValue()
abstract class AgentInterviewBuilderAnswer
    implements
        Built<AgentInterviewBuilderAnswer, AgentInterviewBuilderAnswerBuilder> {
  @BuiltValueField(wireName: r'kind')
  AgentInterviewBuilderAnswerKindEnum get kind;
  // enum kindEnum {  questions,  draft,  };

  /// Only with `kind = questions`; each at most `agents.interview.answer_max_chars` characters.
  @BuiltValueField(wireName: r'questions')
  BuiltList<String>? get questions;

  @BuiltValueField(wireName: r'draft')
  AgentDraft? get draft;

  /// Usage of the model call, as in `run.finish`.
  @BuiltValueField(wireName: r'costs')
  BuiltList<CostReport>? get costs;

  AgentInterviewBuilderAnswer._();

  factory AgentInterviewBuilderAnswer([
    void updates(AgentInterviewBuilderAnswerBuilder b),
  ]) = _$AgentInterviewBuilderAnswer;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentInterviewBuilderAnswerBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentInterviewBuilderAnswer> get serializer =>
      _$AgentInterviewBuilderAnswerSerializer();
}

class _$AgentInterviewBuilderAnswerSerializer
    implements PrimitiveSerializer<AgentInterviewBuilderAnswer> {
  @override
  final Iterable<Type> types = const [
    AgentInterviewBuilderAnswer,
    _$AgentInterviewBuilderAnswer,
  ];

  @override
  final String wireName = r'AgentInterviewBuilderAnswer';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentInterviewBuilderAnswer object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(AgentInterviewBuilderAnswerKindEnum),
    );
    if (object.questions != null) {
      yield r'questions';
      yield serializers.serialize(
        object.questions,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
    if (object.draft != null) {
      yield r'draft';
      yield serializers.serialize(
        object.draft,
        specifiedType: const FullType(AgentDraft),
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
    AgentInterviewBuilderAnswer object, {
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
    required AgentInterviewBuilderAnswerBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AgentInterviewBuilderAnswerKindEnum),
          ) as AgentInterviewBuilderAnswerKindEnum;
          result.kind = valueDes;
          break;
        case r'questions':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.questions.replace(valueDes);
          break;
        case r'draft':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(AgentDraft),
          ) as AgentDraft?;
          if (valueDes == null) continue;
          result.draft.replace(valueDes);
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
  AgentInterviewBuilderAnswer deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentInterviewBuilderAnswerBuilder();
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

class AgentInterviewBuilderAnswerKindEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'questions')
  static const AgentInterviewBuilderAnswerKindEnum questions =
      _$agentInterviewBuilderAnswerKindEnum_questions;
  @BuiltValueEnumConst(wireName: r'draft')
  static const AgentInterviewBuilderAnswerKindEnum draft =
      _$agentInterviewBuilderAnswerKindEnum_draft;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AgentInterviewBuilderAnswerKindEnum unknownDefaultOpenApi =
      _$agentInterviewBuilderAnswerKindEnum_unknownDefaultOpenApi;

  static Serializer<AgentInterviewBuilderAnswerKindEnum> get serializer =>
      _$agentInterviewBuilderAnswerKindEnumSerializer;

  const AgentInterviewBuilderAnswerKindEnum._(String name) : super(name);

  static BuiltSet<AgentInterviewBuilderAnswerKindEnum> get values =>
      _$agentInterviewBuilderAnswerKindEnumValues;
  static AgentInterviewBuilderAnswerKindEnum valueOf(String name) =>
      _$agentInterviewBuilderAnswerKindEnumValueOf(name);
}
