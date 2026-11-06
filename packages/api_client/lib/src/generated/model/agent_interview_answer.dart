//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/field_error.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/agent_interview_cost.dart';
import 'package:api_client/src/generated/model/agent_draft.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_interview_answer.g.dart';

/// The answer of `interviewAgent` (26 6.1). Nothing of it is stored.
///
/// Properties:
/// * [outcome]
/// * [questions] - One or two questions of the builder; only with `outcome = questions`.
/// * [draft]
/// * [errors] - Only with `outcome = draft`; the findings of 26 4.3 and of the catalogue check (`mcp_not_in_catalogue`, 26 7.4) as JSON Pointers into the draft, empty when the draft passes for the class of its `settings.tool_class_wish`.
/// * [fallbackReason] - Only with `outcome = fallback`; the apps then offer the templates and the manual form.
/// * [costEstimate]
@BuiltValue()
abstract class AgentInterviewAnswer
    implements Built<AgentInterviewAnswer, AgentInterviewAnswerBuilder> {
  @BuiltValueField(wireName: r'outcome')
  AgentInterviewAnswerOutcomeEnum get outcome;
  // enum outcomeEnum {  questions,  draft,  fallback,  };

  /// One or two questions of the builder; only with `outcome = questions`.
  @BuiltValueField(wireName: r'questions')
  BuiltList<String>? get questions;

  @BuiltValueField(wireName: r'draft')
  AgentDraft? get draft;

  /// Only with `outcome = draft`; the findings of 26 4.3 and of the catalogue check (`mcp_not_in_catalogue`, 26 7.4) as JSON Pointers into the draft, empty when the draft passes for the class of its `settings.tool_class_wish`.
  @BuiltValueField(wireName: r'errors')
  BuiltList<FieldError>? get errors;

  /// Only with `outcome = fallback`; the apps then offer the templates and the manual form.
  @BuiltValueField(wireName: r'fallback_reason')
  AgentInterviewAnswerFallbackReasonEnum? get fallbackReason;
  // enum fallbackReasonEnum {  builder_not_configured,  builder_offline,  builder_timeout,  builder_invalid,  };

  @BuiltValueField(wireName: r'cost_estimate')
  AgentInterviewCost? get costEstimate;

  AgentInterviewAnswer._();

  factory AgentInterviewAnswer([void updates(AgentInterviewAnswerBuilder b)]) =
      _$AgentInterviewAnswer;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentInterviewAnswerBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentInterviewAnswer> get serializer =>
      _$AgentInterviewAnswerSerializer();
}

class _$AgentInterviewAnswerSerializer
    implements PrimitiveSerializer<AgentInterviewAnswer> {
  @override
  final Iterable<Type> types = const [
    AgentInterviewAnswer,
    _$AgentInterviewAnswer,
  ];

  @override
  final String wireName = r'AgentInterviewAnswer';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentInterviewAnswer object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'outcome';
    yield serializers.serialize(
      object.outcome,
      specifiedType: const FullType(AgentInterviewAnswerOutcomeEnum),
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
    if (object.errors != null) {
      yield r'errors';
      yield serializers.serialize(
        object.errors,
        specifiedType: const FullType(BuiltList, [FullType(FieldError)]),
      );
    }
    if (object.fallbackReason != null) {
      yield r'fallback_reason';
      yield serializers.serialize(
        object.fallbackReason,
        specifiedType: const FullType(AgentInterviewAnswerFallbackReasonEnum),
      );
    }
    if (object.costEstimate != null) {
      yield r'cost_estimate';
      yield serializers.serialize(
        object.costEstimate,
        specifiedType: const FullType(AgentInterviewCost),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentInterviewAnswer object, {
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
    required AgentInterviewAnswerBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'outcome':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AgentInterviewAnswerOutcomeEnum),
          ) as AgentInterviewAnswerOutcomeEnum;
          result.outcome = valueDes;
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
        case r'errors':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(FieldError),
            ]),
          ) as BuiltList<FieldError>?;
          if (valueDes == null) continue;
          result.errors.replace(valueDes);
          break;
        case r'fallback_reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(
              AgentInterviewAnswerFallbackReasonEnum,
            ),
          ) as AgentInterviewAnswerFallbackReasonEnum?;
          if (valueDes == null) continue;
          result.fallbackReason = valueDes;
          break;
        case r'cost_estimate':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(AgentInterviewCost),
          ) as AgentInterviewCost?;
          if (valueDes == null) continue;
          result.costEstimate.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentInterviewAnswer deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentInterviewAnswerBuilder();
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

class AgentInterviewAnswerOutcomeEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'questions')
  static const AgentInterviewAnswerOutcomeEnum questions =
      _$agentInterviewAnswerOutcomeEnum_questions;
  @BuiltValueEnumConst(wireName: r'draft')
  static const AgentInterviewAnswerOutcomeEnum draft =
      _$agentInterviewAnswerOutcomeEnum_draft;
  @BuiltValueEnumConst(wireName: r'fallback')
  static const AgentInterviewAnswerOutcomeEnum fallback =
      _$agentInterviewAnswerOutcomeEnum_fallback;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AgentInterviewAnswerOutcomeEnum unknownDefaultOpenApi =
      _$agentInterviewAnswerOutcomeEnum_unknownDefaultOpenApi;

  static Serializer<AgentInterviewAnswerOutcomeEnum> get serializer =>
      _$agentInterviewAnswerOutcomeEnumSerializer;

  const AgentInterviewAnswerOutcomeEnum._(String name) : super(name);

  static BuiltSet<AgentInterviewAnswerOutcomeEnum> get values =>
      _$agentInterviewAnswerOutcomeEnumValues;
  static AgentInterviewAnswerOutcomeEnum valueOf(String name) =>
      _$agentInterviewAnswerOutcomeEnumValueOf(name);
}

/// Only with `outcome = fallback`; the apps then offer the templates and the manual form.
class AgentInterviewAnswerFallbackReasonEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'builder_not_configured')
  static const AgentInterviewAnswerFallbackReasonEnum builderNotConfigured =
      _$agentInterviewAnswerFallbackReasonEnum_builderNotConfigured;
  @BuiltValueEnumConst(wireName: r'builder_offline')
  static const AgentInterviewAnswerFallbackReasonEnum builderOffline =
      _$agentInterviewAnswerFallbackReasonEnum_builderOffline;
  @BuiltValueEnumConst(wireName: r'builder_timeout')
  static const AgentInterviewAnswerFallbackReasonEnum builderTimeout =
      _$agentInterviewAnswerFallbackReasonEnum_builderTimeout;
  @BuiltValueEnumConst(wireName: r'builder_invalid')
  static const AgentInterviewAnswerFallbackReasonEnum builderInvalid =
      _$agentInterviewAnswerFallbackReasonEnum_builderInvalid;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AgentInterviewAnswerFallbackReasonEnum unknownDefaultOpenApi =
      _$agentInterviewAnswerFallbackReasonEnum_unknownDefaultOpenApi;

  static Serializer<AgentInterviewAnswerFallbackReasonEnum> get serializer =>
      _$agentInterviewAnswerFallbackReasonEnumSerializer;

  const AgentInterviewAnswerFallbackReasonEnum._(String name) : super(name);

  static BuiltSet<AgentInterviewAnswerFallbackReasonEnum> get values =>
      _$agentInterviewAnswerFallbackReasonEnumValues;
  static AgentInterviewAnswerFallbackReasonEnum valueOf(String name) =>
      _$agentInterviewAnswerFallbackReasonEnumValueOf(name);
}
