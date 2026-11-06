//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/agent_definition_origin.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/agent_definition_settings.dart';
import 'package:api_client/src/generated/model/agent_definition_skill.dart';
import 'package:api_client/src/generated/model/agent_definition_mcp_server.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_definition.g.dart';

/// Schema `agent_definition.v1` of a built-in agent (26 4.2), closed at every level. Every string is valid UTF-8 without control or bidi characters, except line feed and carriage return in `instructions`, a skill `body` and a file `content`. The limits are configuration values: `instructions` at most `agents.definition.instructions_max_bytes` bytes; at most `agents.definition.max_skills` skills and `agents.definition.max_mcp_servers` MCP servers. The text rules of 26 4.3 (frontmatter, inline shell, `@` imports outside the agent folder) and the rules of docs/decisions.d/A1-02.md apply; a finding is a field error of `400 validation_failed`.
///
/// Properties:
/// * [instructions] - Markdown that becomes `CLAUDE.md` in the agent folder; at most `agents.definition.instructions_max_bytes` bytes.
/// * [skills] - At most `agents.definition.max_skills` skills, names unique.
/// * [mcpServers] - At most `agents.definition.max_mcp_servers` servers, names unique. A `stdio` server needs the tool class `host` (`tool_class_too_low` below it, 26 4.4).
/// * [settings]
/// * [origin]
@BuiltValue()
abstract class AgentDefinition
    implements Built<AgentDefinition, AgentDefinitionBuilder> {
  /// Markdown that becomes `CLAUDE.md` in the agent folder; at most `agents.definition.instructions_max_bytes` bytes.
  @BuiltValueField(wireName: r'instructions')
  String get instructions;

  /// At most `agents.definition.max_skills` skills, names unique.
  @BuiltValueField(wireName: r'skills')
  BuiltList<AgentDefinitionSkill> get skills;

  /// At most `agents.definition.max_mcp_servers` servers, names unique. A `stdio` server needs the tool class `host` (`tool_class_too_low` below it, 26 4.4).
  @BuiltValueField(wireName: r'mcp_servers')
  BuiltList<AgentDefinitionMcpServer> get mcpServers;

  @BuiltValueField(wireName: r'settings')
  AgentDefinitionSettings get settings;

  @BuiltValueField(wireName: r'origin')
  AgentDefinitionOrigin get origin;

  AgentDefinition._();

  factory AgentDefinition([void updates(AgentDefinitionBuilder b)]) =
      _$AgentDefinition;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentDefinitionBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentDefinition> get serializer =>
      _$AgentDefinitionSerializer();
}

class _$AgentDefinitionSerializer
    implements PrimitiveSerializer<AgentDefinition> {
  @override
  final Iterable<Type> types = const [AgentDefinition, _$AgentDefinition];

  @override
  final String wireName = r'AgentDefinition';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentDefinition object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'instructions';
    yield serializers.serialize(
      object.instructions,
      specifiedType: const FullType(String),
    );
    yield r'skills';
    yield serializers.serialize(
      object.skills,
      specifiedType: const FullType(BuiltList, [
        FullType(AgentDefinitionSkill),
      ]),
    );
    yield r'mcp_servers';
    yield serializers.serialize(
      object.mcpServers,
      specifiedType: const FullType(BuiltList, [
        FullType(AgentDefinitionMcpServer),
      ]),
    );
    yield r'settings';
    yield serializers.serialize(
      object.settings,
      specifiedType: const FullType(AgentDefinitionSettings),
    );
    yield r'origin';
    yield serializers.serialize(
      object.origin,
      specifiedType: const FullType(AgentDefinitionOrigin),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentDefinition object, {
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
    required AgentDefinitionBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'instructions':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.instructions = valueDes;
          break;
        case r'skills':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(AgentDefinitionSkill),
            ]),
          ) as BuiltList<AgentDefinitionSkill>;
          result.skills.replace(valueDes);
          break;
        case r'mcp_servers':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(AgentDefinitionMcpServer),
            ]),
          ) as BuiltList<AgentDefinitionMcpServer>;
          result.mcpServers.replace(valueDes);
          break;
        case r'settings':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AgentDefinitionSettings),
          ) as AgentDefinitionSettings;
          result.settings.replace(valueDes);
          break;
        case r'origin':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AgentDefinitionOrigin),
          ) as AgentDefinitionOrigin;
          result.origin.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentDefinition deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentDefinitionBuilder();
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
