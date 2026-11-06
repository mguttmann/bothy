//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_definition_origin.g.dart';

/// Where the definition came from; display only.
///
/// Properties:
/// * [kind]
/// * [templateSlug] - `^[a-z0-9][a-z0-9-]{0,63}$`.
/// * [templateVersion] - `^[0-9A-Za-z][0-9A-Za-z.+-]{0,63}$`.
@BuiltValue()
abstract class AgentDefinitionOrigin
    implements Built<AgentDefinitionOrigin, AgentDefinitionOriginBuilder> {
  @BuiltValueField(wireName: r'kind')
  AgentDefinitionOriginKindEnum get kind;
  // enum kindEnum {  template,  interview,  manual,  };

  /// `^[a-z0-9][a-z0-9-]{0,63}$`.
  @BuiltValueField(wireName: r'template_slug')
  String? get templateSlug;

  /// `^[0-9A-Za-z][0-9A-Za-z.+-]{0,63}$`.
  @BuiltValueField(wireName: r'template_version')
  String? get templateVersion;

  AgentDefinitionOrigin._();

  factory AgentDefinitionOrigin([
    void updates(AgentDefinitionOriginBuilder b),
  ]) = _$AgentDefinitionOrigin;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentDefinitionOriginBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentDefinitionOrigin> get serializer =>
      _$AgentDefinitionOriginSerializer();
}

class _$AgentDefinitionOriginSerializer
    implements PrimitiveSerializer<AgentDefinitionOrigin> {
  @override
  final Iterable<Type> types = const [
    AgentDefinitionOrigin,
    _$AgentDefinitionOrigin,
  ];

  @override
  final String wireName = r'AgentDefinitionOrigin';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentDefinitionOrigin object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'kind';
    yield serializers.serialize(
      object.kind,
      specifiedType: const FullType(AgentDefinitionOriginKindEnum),
    );
    if (object.templateSlug != null) {
      yield r'template_slug';
      yield serializers.serialize(
        object.templateSlug,
        specifiedType: const FullType(String),
      );
    }
    if (object.templateVersion != null) {
      yield r'template_version';
      yield serializers.serialize(
        object.templateVersion,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentDefinitionOrigin object, {
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
    required AgentDefinitionOriginBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AgentDefinitionOriginKindEnum),
          ) as AgentDefinitionOriginKindEnum;
          result.kind = valueDes;
          break;
        case r'template_slug':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.templateSlug = valueDes;
          break;
        case r'template_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.templateVersion = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentDefinitionOrigin deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentDefinitionOriginBuilder();
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

class AgentDefinitionOriginKindEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'template')
  static const AgentDefinitionOriginKindEnum template =
      _$agentDefinitionOriginKindEnum_template;
  @BuiltValueEnumConst(wireName: r'interview')
  static const AgentDefinitionOriginKindEnum interview =
      _$agentDefinitionOriginKindEnum_interview;
  @BuiltValueEnumConst(wireName: r'manual')
  static const AgentDefinitionOriginKindEnum manual =
      _$agentDefinitionOriginKindEnum_manual;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const AgentDefinitionOriginKindEnum unknownDefaultOpenApi =
      _$agentDefinitionOriginKindEnum_unknownDefaultOpenApi;

  static Serializer<AgentDefinitionOriginKindEnum> get serializer =>
      _$agentDefinitionOriginKindEnumSerializer;

  const AgentDefinitionOriginKindEnum._(String name) : super(name);

  static BuiltSet<AgentDefinitionOriginKindEnum> get values =>
      _$agentDefinitionOriginKindEnumValues;
  static AgentDefinitionOriginKindEnum valueOf(String name) =>
      _$agentDefinitionOriginKindEnumValueOf(name);
}
