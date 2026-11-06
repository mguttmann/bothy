//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/agent_definition_mcp_server.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_interview_catalogue_entry.g.dart';

/// AgentInterviewCatalogueEntry
///
/// Properties:
/// * [slug]
/// * [name]
/// * [description]
/// * [toolClassWish]
/// * [mcpServers]
@BuiltValue()
abstract class AgentInterviewCatalogueEntry
    implements
        Built<
          AgentInterviewCatalogueEntry,
          AgentInterviewCatalogueEntryBuilder
        > {
  @BuiltValueField(wireName: r'slug')
  String get slug;

  @BuiltValueField(wireName: r'name')
  String get name;

  @BuiltValueField(wireName: r'description')
  String get description;

  @BuiltValueField(wireName: r'tool_class_wish')
  AgentInterviewCatalogueEntryToolClassWishEnum get toolClassWish;
  // enum toolClassWishEnum {  chat,  scoped,  host,  };

  @BuiltValueField(wireName: r'mcp_servers')
  BuiltList<AgentDefinitionMcpServer> get mcpServers;

  AgentInterviewCatalogueEntry._();

  factory AgentInterviewCatalogueEntry([
    void updates(AgentInterviewCatalogueEntryBuilder b),
  ]) = _$AgentInterviewCatalogueEntry;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentInterviewCatalogueEntryBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentInterviewCatalogueEntry> get serializer =>
      _$AgentInterviewCatalogueEntrySerializer();
}

class _$AgentInterviewCatalogueEntrySerializer
    implements PrimitiveSerializer<AgentInterviewCatalogueEntry> {
  @override
  final Iterable<Type> types = const [
    AgentInterviewCatalogueEntry,
    _$AgentInterviewCatalogueEntry,
  ];

  @override
  final String wireName = r'AgentInterviewCatalogueEntry';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentInterviewCatalogueEntry object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'slug';
    yield serializers.serialize(
      object.slug,
      specifiedType: const FullType(String),
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
    yield r'tool_class_wish';
    yield serializers.serialize(
      object.toolClassWish,
      specifiedType: const FullType(
        AgentInterviewCatalogueEntryToolClassWishEnum,
      ),
    );
    yield r'mcp_servers';
    yield serializers.serialize(
      object.mcpServers,
      specifiedType: const FullType(BuiltList, [
        FullType(AgentDefinitionMcpServer),
      ]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentInterviewCatalogueEntry object, {
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
    required AgentInterviewCatalogueEntryBuilder result,
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
        case r'tool_class_wish':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(
              AgentInterviewCatalogueEntryToolClassWishEnum,
            ),
          ) as AgentInterviewCatalogueEntryToolClassWishEnum;
          result.toolClassWish = valueDes;
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
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentInterviewCatalogueEntry deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentInterviewCatalogueEntryBuilder();
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

class AgentInterviewCatalogueEntryToolClassWishEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'chat')
  static const AgentInterviewCatalogueEntryToolClassWishEnum chat =
      _$agentInterviewCatalogueEntryToolClassWishEnum_chat;
  @BuiltValueEnumConst(wireName: r'scoped')
  static const AgentInterviewCatalogueEntryToolClassWishEnum scoped =
      _$agentInterviewCatalogueEntryToolClassWishEnum_scoped;
  @BuiltValueEnumConst(wireName: r'host')
  static const AgentInterviewCatalogueEntryToolClassWishEnum host =
      _$agentInterviewCatalogueEntryToolClassWishEnum_host;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AgentInterviewCatalogueEntryToolClassWishEnum
  unknownDefaultOpenApi =
      _$agentInterviewCatalogueEntryToolClassWishEnum_unknownDefaultOpenApi;

  static Serializer<AgentInterviewCatalogueEntryToolClassWishEnum>
  get serializer => _$agentInterviewCatalogueEntryToolClassWishEnumSerializer;

  const AgentInterviewCatalogueEntryToolClassWishEnum._(String name)
    : super(name);

  static BuiltSet<AgentInterviewCatalogueEntryToolClassWishEnum> get values =>
      _$agentInterviewCatalogueEntryToolClassWishEnumValues;
  static AgentInterviewCatalogueEntryToolClassWishEnum valueOf(String name) =>
      _$agentInterviewCatalogueEntryToolClassWishEnumValueOf(name);
}
