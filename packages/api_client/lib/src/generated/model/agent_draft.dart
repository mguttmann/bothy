//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/agent_definition_origin.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/agent_definition_settings.dart';
import 'package:api_client/src/generated/model/agent_definition_skill.dart';
import 'package:api_client/src/generated/model/agent_definition_mcp_server.dart';
import 'package:api_client/src/generated/model/agent_draft_channel.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_draft.g.dart';

/// A definition draft of the builder (26 6.1 item 2): the members of `AgentDefinition` plus the agent's name, handle, description and an optional channel suggestion. The server checks it like a definition for the class of `settings.tool_class_wish` and against the template catalogue; it never stores it.
///
/// Properties:
/// * [instructions] - Markdown that becomes `CLAUDE.md`; at most `agents.definition.instructions_max_bytes` bytes.
/// * [skills] - At most `agents.definition.max_skills` skills.
/// * [mcpServers] - At most `agents.definition.max_mcp_servers` servers, each equal to a server of the catalogue except `name`.
/// * [settings]
/// * [origin]
/// * [name] - The display name the builder suggests.
/// * [handle] - The handle the builder suggests; `createAgent` checks it.
/// * [description] - One line about what the agent does.
/// * [channelSuggestion]
@BuiltValue()
abstract class AgentDraft implements Built<AgentDraft, AgentDraftBuilder> {
  /// Markdown that becomes `CLAUDE.md`; at most `agents.definition.instructions_max_bytes` bytes.
  @BuiltValueField(wireName: r'instructions')
  String get instructions;

  /// At most `agents.definition.max_skills` skills.
  @BuiltValueField(wireName: r'skills')
  BuiltList<AgentDefinitionSkill> get skills;

  /// At most `agents.definition.max_mcp_servers` servers, each equal to a server of the catalogue except `name`.
  @BuiltValueField(wireName: r'mcp_servers')
  BuiltList<AgentDefinitionMcpServer> get mcpServers;

  @BuiltValueField(wireName: r'settings')
  AgentDefinitionSettings get settings;

  @BuiltValueField(wireName: r'origin')
  AgentDefinitionOrigin get origin;

  /// The display name the builder suggests.
  @BuiltValueField(wireName: r'name')
  String get name;

  /// The handle the builder suggests; `createAgent` checks it.
  @BuiltValueField(wireName: r'handle')
  String get handle;

  /// One line about what the agent does.
  @BuiltValueField(wireName: r'description')
  String get description;

  @BuiltValueField(wireName: r'channel_suggestion')
  AgentDraftChannel? get channelSuggestion;

  AgentDraft._();

  factory AgentDraft([void updates(AgentDraftBuilder b)]) = _$AgentDraft;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentDraftBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentDraft> get serializer => _$AgentDraftSerializer();
}

class _$AgentDraftSerializer implements PrimitiveSerializer<AgentDraft> {
  @override
  final Iterable<Type> types = const [AgentDraft, _$AgentDraft];

  @override
  final String wireName = r'AgentDraft';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentDraft object, {
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
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    yield r'handle';
    yield serializers.serialize(
      object.handle,
      specifiedType: const FullType(String),
    );
    yield r'description';
    yield serializers.serialize(
      object.description,
      specifiedType: const FullType(String),
    );
    if (object.channelSuggestion != null) {
      yield r'channel_suggestion';
      yield serializers.serialize(
        object.channelSuggestion,
        specifiedType: const FullType(AgentDraftChannel),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentDraft object, {
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
    required AgentDraftBuilder result,
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
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.name = valueDes;
          break;
        case r'handle':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.handle = valueDes;
          break;
        case r'description':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.description = valueDes;
          break;
        case r'channel_suggestion':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(AgentDraftChannel),
          ) as AgentDraftChannel?;
          if (valueDes == null) continue;
          result.channelSuggestion.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentDraft deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentDraftBuilder();
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
