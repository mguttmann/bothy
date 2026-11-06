//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_interview_limits.g.dart';

/// The limits a draft must keep (the values of `agents.definition.*`, `agents.mcp_stdio_runners` and `agents.interview.answer_max_chars`).
///
/// Properties:
/// * [instructionsMaxBytes]
/// * [maxSkills]
/// * [skillDescriptionMaxChars]
/// * [skillBodyMaxBytes]
/// * [maxSkillFiles]
/// * [maxMcpServers]
/// * [maxAllowedTools]
/// * [maxCostUsdPerRun]
/// * [mcpStdioRunners]
/// * [questionMaxChars] - The longest question of an answer.
@BuiltValue()
abstract class AgentInterviewLimits
    implements Built<AgentInterviewLimits, AgentInterviewLimitsBuilder> {
  @BuiltValueField(wireName: r'instructions_max_bytes')
  int get instructionsMaxBytes;

  @BuiltValueField(wireName: r'max_skills')
  int get maxSkills;

  @BuiltValueField(wireName: r'skill_description_max_chars')
  int get skillDescriptionMaxChars;

  @BuiltValueField(wireName: r'skill_body_max_bytes')
  int get skillBodyMaxBytes;

  @BuiltValueField(wireName: r'max_skill_files')
  int get maxSkillFiles;

  @BuiltValueField(wireName: r'max_mcp_servers')
  int get maxMcpServers;

  @BuiltValueField(wireName: r'max_allowed_tools')
  int get maxAllowedTools;

  @BuiltValueField(wireName: r'max_cost_usd_per_run')
  num get maxCostUsdPerRun;

  @BuiltValueField(wireName: r'mcp_stdio_runners')
  BuiltList<String> get mcpStdioRunners;

  /// The longest question of an answer.
  @BuiltValueField(wireName: r'question_max_chars')
  int get questionMaxChars;

  AgentInterviewLimits._();

  factory AgentInterviewLimits([void updates(AgentInterviewLimitsBuilder b)]) =
      _$AgentInterviewLimits;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentInterviewLimitsBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentInterviewLimits> get serializer =>
      _$AgentInterviewLimitsSerializer();
}

class _$AgentInterviewLimitsSerializer
    implements PrimitiveSerializer<AgentInterviewLimits> {
  @override
  final Iterable<Type> types = const [
    AgentInterviewLimits,
    _$AgentInterviewLimits,
  ];

  @override
  final String wireName = r'AgentInterviewLimits';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentInterviewLimits object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'instructions_max_bytes';
    yield serializers.serialize(
      object.instructionsMaxBytes,
      specifiedType: const FullType(int),
    );
    yield r'max_skills';
    yield serializers.serialize(
      object.maxSkills,
      specifiedType: const FullType(int),
    );
    yield r'skill_description_max_chars';
    yield serializers.serialize(
      object.skillDescriptionMaxChars,
      specifiedType: const FullType(int),
    );
    yield r'skill_body_max_bytes';
    yield serializers.serialize(
      object.skillBodyMaxBytes,
      specifiedType: const FullType(int),
    );
    yield r'max_skill_files';
    yield serializers.serialize(
      object.maxSkillFiles,
      specifiedType: const FullType(int),
    );
    yield r'max_mcp_servers';
    yield serializers.serialize(
      object.maxMcpServers,
      specifiedType: const FullType(int),
    );
    yield r'max_allowed_tools';
    yield serializers.serialize(
      object.maxAllowedTools,
      specifiedType: const FullType(int),
    );
    yield r'max_cost_usd_per_run';
    yield serializers.serialize(
      object.maxCostUsdPerRun,
      specifiedType: const FullType(num),
    );
    yield r'mcp_stdio_runners';
    yield serializers.serialize(
      object.mcpStdioRunners,
      specifiedType: const FullType(BuiltList, [FullType(String)]),
    );
    yield r'question_max_chars';
    yield serializers.serialize(
      object.questionMaxChars,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentInterviewLimits object, {
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
    required AgentInterviewLimitsBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'instructions_max_bytes':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.instructionsMaxBytes = valueDes;
          break;
        case r'max_skills':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.maxSkills = valueDes;
          break;
        case r'skill_description_max_chars':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.skillDescriptionMaxChars = valueDes;
          break;
        case r'skill_body_max_bytes':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.skillBodyMaxBytes = valueDes;
          break;
        case r'max_skill_files':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.maxSkillFiles = valueDes;
          break;
        case r'max_mcp_servers':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.maxMcpServers = valueDes;
          break;
        case r'max_allowed_tools':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.maxAllowedTools = valueDes;
          break;
        case r'max_cost_usd_per_run':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(num),
          ) as num;
          result.maxCostUsdPerRun = valueDes;
          break;
        case r'mcp_stdio_runners':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(String)]),
          ) as BuiltList<String>;
          result.mcpStdioRunners.replace(valueDes);
          break;
        case r'question_max_chars':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.questionMaxChars = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentInterviewLimits deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentInterviewLimitsBuilder();
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
