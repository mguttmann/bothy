//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/agent_definition.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_definition_version.g.dart';

/// One stored version of the definition of a built-in agent; `definition` only in `getAgentDefinition` and `putAgentDefinition`.
///
/// Properties:
/// * [agentId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [version]
/// * [originKind]
/// * [createdAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [createdBy] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [definition]
@BuiltValue()
abstract class AgentDefinitionVersion
    implements Built<AgentDefinitionVersion, AgentDefinitionVersionBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'agent_id')
  String get agentId;

  @BuiltValueField(wireName: r'version')
  int get version;

  @BuiltValueField(wireName: r'origin_kind')
  AgentDefinitionVersionOriginKindEnum get originKind;
  // enum originKindEnum {  template,  interview,  manual,  };

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'created_at')
  DateTime get createdAt;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'created_by')
  String get createdBy;

  @BuiltValueField(wireName: r'definition')
  AgentDefinition? get definition;

  AgentDefinitionVersion._();

  factory AgentDefinitionVersion([
    void updates(AgentDefinitionVersionBuilder b),
  ]) = _$AgentDefinitionVersion;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentDefinitionVersionBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentDefinitionVersion> get serializer =>
      _$AgentDefinitionVersionSerializer();
}

class _$AgentDefinitionVersionSerializer
    implements PrimitiveSerializer<AgentDefinitionVersion> {
  @override
  final Iterable<Type> types = const [
    AgentDefinitionVersion,
    _$AgentDefinitionVersion,
  ];

  @override
  final String wireName = r'AgentDefinitionVersion';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentDefinitionVersion object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'agent_id';
    yield serializers.serialize(
      object.agentId,
      specifiedType: const FullType(String),
    );
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(int),
    );
    yield r'origin_kind';
    yield serializers.serialize(
      object.originKind,
      specifiedType: const FullType(AgentDefinitionVersionOriginKindEnum),
    );
    yield r'created_at';
    yield serializers.serialize(
      object.createdAt,
      specifiedType: const FullType(DateTime),
    );
    yield r'created_by';
    yield serializers.serialize(
      object.createdBy,
      specifiedType: const FullType(String),
    );
    if (object.definition != null) {
      yield r'definition';
      yield serializers.serialize(
        object.definition,
        specifiedType: const FullType(AgentDefinition),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentDefinitionVersion object, {
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
    required AgentDefinitionVersionBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'agent_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.agentId = valueDes;
          break;
        case r'version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.version = valueDes;
          break;
        case r'origin_kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AgentDefinitionVersionOriginKindEnum),
          ) as AgentDefinitionVersionOriginKindEnum;
          result.originKind = valueDes;
          break;
        case r'created_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.createdAt = valueDes;
          break;
        case r'created_by':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.createdBy = valueDes;
          break;
        case r'definition':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(AgentDefinition),
          ) as AgentDefinition?;
          if (valueDes == null) continue;
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
  AgentDefinitionVersion deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentDefinitionVersionBuilder();
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

class AgentDefinitionVersionOriginKindEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'template')
  static const AgentDefinitionVersionOriginKindEnum template =
      _$agentDefinitionVersionOriginKindEnum_template;
  @BuiltValueEnumConst(wireName: r'interview')
  static const AgentDefinitionVersionOriginKindEnum interview =
      _$agentDefinitionVersionOriginKindEnum_interview;
  @BuiltValueEnumConst(wireName: r'manual')
  static const AgentDefinitionVersionOriginKindEnum manual =
      _$agentDefinitionVersionOriginKindEnum_manual;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AgentDefinitionVersionOriginKindEnum unknownDefaultOpenApi =
      _$agentDefinitionVersionOriginKindEnum_unknownDefaultOpenApi;

  static Serializer<AgentDefinitionVersionOriginKindEnum> get serializer =>
      _$agentDefinitionVersionOriginKindEnumSerializer;

  const AgentDefinitionVersionOriginKindEnum._(String name) : super(name);

  static BuiltSet<AgentDefinitionVersionOriginKindEnum> get values =>
      _$agentDefinitionVersionOriginKindEnumValues;
  static AgentDefinitionVersionOriginKindEnum valueOf(String name) =>
      _$agentDefinitionVersionOriginKindEnumValueOf(name);
}
