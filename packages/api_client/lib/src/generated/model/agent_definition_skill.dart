//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/agent_definition_skill_file.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_definition_skill.g.dart';

/// One skill. The runner writes the frontmatter of `SKILL.md` from `name` and `description` only (26 5.2).
///
/// Properties:
/// * [name] - `^[a-z0-9][a-z0-9-]{0,63}$`, unique among the skills (`invalid_name`, `duplicate_name`).
/// * [description] - One line, not blank, at most `agents.definition.skill_description_max_chars` characters.
/// * [body] - Markdown, at most `agents.definition.skill_body_max_bytes` bytes; its first line is never `---` (`skill_frontmatter`).
/// * [files] - At most `agents.definition.max_skill_files` extra files, written flat next to `SKILL.md`.
@BuiltValue()
abstract class AgentDefinitionSkill
    implements Built<AgentDefinitionSkill, AgentDefinitionSkillBuilder> {
  /// `^[a-z0-9][a-z0-9-]{0,63}$`, unique among the skills (`invalid_name`, `duplicate_name`).
  @BuiltValueField(wireName: r'name')
  String get name;

  /// One line, not blank, at most `agents.definition.skill_description_max_chars` characters.
  @BuiltValueField(wireName: r'description')
  String get description;

  /// Markdown, at most `agents.definition.skill_body_max_bytes` bytes; its first line is never `---` (`skill_frontmatter`).
  @BuiltValueField(wireName: r'body')
  String get body;

  /// At most `agents.definition.max_skill_files` extra files, written flat next to `SKILL.md`.
  @BuiltValueField(wireName: r'files')
  BuiltList<AgentDefinitionSkillFile>? get files;

  AgentDefinitionSkill._();

  factory AgentDefinitionSkill([void updates(AgentDefinitionSkillBuilder b)]) =
      _$AgentDefinitionSkill;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentDefinitionSkillBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentDefinitionSkill> get serializer =>
      _$AgentDefinitionSkillSerializer();
}

class _$AgentDefinitionSkillSerializer
    implements PrimitiveSerializer<AgentDefinitionSkill> {
  @override
  final Iterable<Type> types = const [
    AgentDefinitionSkill,
    _$AgentDefinitionSkill,
  ];

  @override
  final String wireName = r'AgentDefinitionSkill';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentDefinitionSkill object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    yield r'description';
    yield serializers.serialize(
      object.description,
      specifiedType: const FullType(String),
    );
    yield r'body';
    yield serializers.serialize(
      object.body,
      specifiedType: const FullType(String),
    );
    if (object.files != null) {
      yield r'files';
      yield serializers.serialize(
        object.files,
        specifiedType: const FullType(BuiltList, [
          FullType(AgentDefinitionSkillFile),
        ]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentDefinitionSkill object, {
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
    required AgentDefinitionSkillBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.name = valueDes;
          break;
        case r'description':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.description = valueDes;
          break;
        case r'body':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.body = valueDes;
          break;
        case r'files':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(AgentDefinitionSkillFile),
            ]),
          ) as BuiltList<AgentDefinitionSkillFile>?;
          if (valueDes == null) continue;
          result.files.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentDefinitionSkill deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentDefinitionSkillBuilder();
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
