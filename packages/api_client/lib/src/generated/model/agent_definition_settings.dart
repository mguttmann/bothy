//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/tool_class.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'agent_definition_settings.g.dart';

/// Settings of the built-in agent; the tool class itself is the agent's `tool_class` (07 2.11), never set here.
///
/// Properties:
/// * [model] - The pattern of `default_model` (09 2.2), not starting with `-`; absent means the runner's default model.
/// * [toolClassWish]
/// * [allowedTools] - At most `agents.definition.max_allowed_tools` distinct tool names (a CLI tool name, optionally with a rule in parentheses, or `mcp__<server>[__<tool>]` of a listed server). Under `scoped` it narrows `runner.scoped_allowed_tools` of the hosting runner; absent means no narrowing.
/// * [maxCostUsdPerRun] - Cost cap of one run in USD, above zero and at most `agents.definition.max_cost_usd_per_run`; absent means no cap.
@BuiltValue()
abstract class AgentDefinitionSettings
    implements Built<AgentDefinitionSettings, AgentDefinitionSettingsBuilder> {
  /// The pattern of `default_model` (09 2.2), not starting with `-`; absent means the runner's default model.
  @BuiltValueField(wireName: r'model')
  String? get model;

  @BuiltValueField(wireName: r'tool_class_wish')
  ToolClass get toolClassWish;
  // enum toolClassWishEnum {  chat,  scoped,  host,  };

  /// At most `agents.definition.max_allowed_tools` distinct tool names (a CLI tool name, optionally with a rule in parentheses, or `mcp__<server>[__<tool>]` of a listed server). Under `scoped` it narrows `runner.scoped_allowed_tools` of the hosting runner; absent means no narrowing.
  @BuiltValueField(wireName: r'allowed_tools')
  BuiltList<String>? get allowedTools;

  /// Cost cap of one run in USD, above zero and at most `agents.definition.max_cost_usd_per_run`; absent means no cap.
  @BuiltValueField(wireName: r'max_cost_usd_per_run')
  double? get maxCostUsdPerRun;

  AgentDefinitionSettings._();

  factory AgentDefinitionSettings([
    void updates(AgentDefinitionSettingsBuilder b),
  ]) = _$AgentDefinitionSettings;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AgentDefinitionSettingsBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AgentDefinitionSettings> get serializer =>
      _$AgentDefinitionSettingsSerializer();
}

class _$AgentDefinitionSettingsSerializer
    implements PrimitiveSerializer<AgentDefinitionSettings> {
  @override
  final Iterable<Type> types = const [
    AgentDefinitionSettings,
    _$AgentDefinitionSettings,
  ];

  @override
  final String wireName = r'AgentDefinitionSettings';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AgentDefinitionSettings object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.model != null) {
      yield r'model';
      yield serializers.serialize(
        object.model,
        specifiedType: const FullType(String),
      );
    }
    yield r'tool_class_wish';
    yield serializers.serialize(
      object.toolClassWish,
      specifiedType: const FullType(ToolClass),
    );
    if (object.allowedTools != null) {
      yield r'allowed_tools';
      yield serializers.serialize(
        object.allowedTools,
        specifiedType: const FullType(BuiltList, [FullType(String)]),
      );
    }
    if (object.maxCostUsdPerRun != null) {
      yield r'max_cost_usd_per_run';
      yield serializers.serialize(
        object.maxCostUsdPerRun,
        specifiedType: const FullType(double),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    AgentDefinitionSettings object, {
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
    required AgentDefinitionSettingsBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'model':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.model = valueDes;
          break;
        case r'tool_class_wish':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ToolClass),
          ) as ToolClass;
          result.toolClassWish = valueDes;
          break;
        case r'allowed_tools':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(String),
            ]),
          ) as BuiltList<String>?;
          if (valueDes == null) continue;
          result.allowedTools.replace(valueDes);
          break;
        case r'max_cost_usd_per_run':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(double),
          ) as double?;
          if (valueDes == null) continue;
          result.maxCostUsdPerRun = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AgentDefinitionSettings deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AgentDefinitionSettingsBuilder();
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
