//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_definition_skill_file.g.dart';

/// One extra file of a skill.
///
/// Properties:
/// * [name] - A flat name matching `^[a-z0-9][a-z0-9._-]{0,63}\\.md$`, never `SKILL.md` in any case, unique within the skill.
/// * [content] - Markdown, at most `agents.definition.skill_body_max_bytes` bytes; its first line is never `---`.
@BuiltValue()
abstract class AgentDefinitionSkillFile
    implements
        Built<AgentDefinitionSkillFile, AgentDefinitionSkillFileBuilder> {
  /// A flat name matching `^[a-z0-9][a-z0-9._-]{0,63}\\.md$`, never `SKILL.md` in any case, unique within the skill.
  @BuiltValueField(wireName: r'name')
  String get name;

  /// Markdown, at most `agents.definition.skill_body_max_bytes` bytes; its first line is never `---`.
  @BuiltValueField(wireName: r'content')
  String get content;

  AgentDefinitionSkillFile._();

  factory AgentDefinitionSkillFile([
    void updates(AgentDefinitionSkillFileBuilder b),
  ]) = _$AgentDefinitionSkillFile;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentDefinitionSkillFileBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentDefinitionSkillFile> get serializer =>
      _$AgentDefinitionSkillFileSerializer();
}

class _$AgentDefinitionSkillFileSerializer
    implements PrimitiveSerializer<AgentDefinitionSkillFile> {
  @override
  final Iterable<Type> types = const [
    AgentDefinitionSkillFile,
    _$AgentDefinitionSkillFile,
  ];

  @override
  final String wireName = r'AgentDefinitionSkillFile';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentDefinitionSkillFile object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    yield r'content';
    yield serializers.serialize(
      object.content,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentDefinitionSkillFile object, {
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
    required AgentDefinitionSkillFileBuilder result,
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
        case r'content':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.content = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentDefinitionSkillFile deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentDefinitionSkillFileBuilder();
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
