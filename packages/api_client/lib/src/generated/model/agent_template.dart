//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/agent_definition.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_template.g.dart';

/// One agent template (26 7.1): its catalogue entry and the template as a definition draft. A shipped template comes from `templates/agents/<slug>/` of the release, a private one from the folder `agents.templates_dir` of the installation.
///
/// Properties:
/// * [slug] - The folder name of the template, `^[a-z0-9][a-z0-9-]{0,63}$`; unique across shipped and private templates.
/// * [source_] - `shipped` for a template of the release, `private` for one of `agents.templates_dir`.
/// * [name] - One line for the template picker.
/// * [description] - One line on what an agent from this template does.
/// * [version] - The version of the template, `^[0-9A-Za-z][0-9A-Za-z.+-]{0,63}$`; also `definition.origin.template_version`.
/// * [requiredSecrets] - The names of the secrets the template's MCP servers name as `env:<NAME>`; the host operator provides them to the hosting runner. Names only, never a value.
/// * [definition]
@BuiltValue()
abstract class AgentTemplate
    implements Built<AgentTemplate, AgentTemplateBuilder> {
  /// The folder name of the template, `^[a-z0-9][a-z0-9-]{0,63}$`; unique across shipped and private templates.
  @BuiltValueField(wireName: r'slug')
  String get slug;

  /// `shipped` for a template of the release, `private` for one of `agents.templates_dir`.
  @BuiltValueField(wireName: r'source')
  AgentTemplateSource_Enum get source_;
  // enum source_Enum {  shipped,  private,  };

  /// One line for the template picker.
  @BuiltValueField(wireName: r'name')
  String get name;

  /// One line on what an agent from this template does.
  @BuiltValueField(wireName: r'description')
  String get description;

  /// The version of the template, `^[0-9A-Za-z][0-9A-Za-z.+-]{0,63}$`; also `definition.origin.template_version`.
  @BuiltValueField(wireName: r'version')
  String get version;

  /// The names of the secrets the template's MCP servers name as `env:<NAME>`; the host operator provides them to the hosting runner. Names only, never a value.
  @BuiltValueField(wireName: r'required_secrets')
  BuiltList<String> get requiredSecrets;

  @BuiltValueField(wireName: r'definition')
  AgentDefinition get definition;

  AgentTemplate._();

  factory AgentTemplate([void updates(AgentTemplateBuilder b)]) =
      _$AgentTemplate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentTemplateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentTemplate> get serializer =>
      _$AgentTemplateSerializer();
}

class _$AgentTemplateSerializer implements PrimitiveSerializer<AgentTemplate> {
  @override
  final Iterable<Type> types = const [AgentTemplate, _$AgentTemplate];

  @override
  final String wireName = r'AgentTemplate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentTemplate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'slug';
    yield serializers.serialize(
      object.slug,
      specifiedType: const FullType(String),
    );
    yield r'source';
    yield serializers.serialize(
      object.source_,
      specifiedType: const FullType(AgentTemplateSource_Enum),
    );
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
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(String),
    );
    yield r'required_secrets';
    yield serializers.serialize(
      object.requiredSecrets,
      specifiedType: const FullType(BuiltList, [FullType(String)]),
    );
    yield r'definition';
    yield serializers.serialize(
      object.definition,
      specifiedType: const FullType(AgentDefinition),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentTemplate object, {
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
    required AgentTemplateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'slug':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.slug = valueDes;
          break;
        case r'source':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AgentTemplateSource_Enum),
          ) as AgentTemplateSource_Enum;
          result.source_ = valueDes;
          break;
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
        case r'version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.version = valueDes;
          break;
        case r'required_secrets':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(String)]),
          ) as BuiltList<String>;
          result.requiredSecrets.replace(valueDes);
          break;
        case r'definition':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AgentDefinition),
          ) as AgentDefinition;
          result.definition.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentTemplate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentTemplateBuilder();
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

/// `shipped` for a template of the release, `private` for one of `agents.templates_dir`.
class AgentTemplateSource_Enum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'shipped')
  static const AgentTemplateSource_Enum shipped =
      _$agentTemplateSourceEnum_shipped;
  @BuiltValueEnumConst(wireName: r'private')
  static const AgentTemplateSource_Enum private =
      _$agentTemplateSourceEnum_private;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AgentTemplateSource_Enum unknownDefaultOpenApi =
      _$agentTemplateSourceEnum_unknownDefaultOpenApi;

  static Serializer<AgentTemplateSource_Enum> get serializer =>
      _$agentTemplateSourceEnumSerializer;

  const AgentTemplateSource_Enum._(String name) : super(name);

  static BuiltSet<AgentTemplateSource_Enum> get values =>
      _$agentTemplateSourceEnumValues;
  static AgentTemplateSource_Enum valueOf(String name) =>
      _$agentTemplateSourceEnumValueOf(name);
}
